package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gomailzero/gmz/internal/antispam"
	"github.com/gomailzero/gmz/internal/api"
	"github.com/gomailzero/gmz/internal/auth"
	"github.com/gomailzero/gmz/internal/config"
	"github.com/gomailzero/gmz/internal/imapd"
	"github.com/gomailzero/gmz/internal/logger"
	"github.com/gomailzero/gmz/internal/metrics"
	"github.com/gomailzero/gmz/internal/migrate"
	"github.com/gomailzero/gmz/internal/smtpclient"
	"github.com/gomailzero/gmz/internal/smtpd"
	"github.com/gomailzero/gmz/internal/storage"
	tlsconfig "github.com/gomailzero/gmz/internal/tls"
	"github.com/gomailzero/gmz/internal/web"
	"github.com/rs/zerolog/log"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	var (
		configPath = flag.String("c", "gmz.yml", "配置文件路径")
		version    = flag.Bool("version", false, "显示版本信息")
		migrateCmd = flag.String("migrate", "", "数据库迁移命令 (up|down|status|up-to|down-to)")
		migrateVer = flag.String("migrate-version", "", "迁移版本号（用于 up-to/down-to）")
	)
	flag.Parse()

	if *version {
		fmt.Printf("gmz version %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// 处理迁移命令
	if *migrateCmd != "" {
		if err := handleMigrateCommand(*migrateCmd, *migrateVer, *configPath); err != nil {
			fmt.Fprintf(os.Stderr, "迁移失败: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logger.Init(logger.LogConfig{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		Output: cfg.Log.Output,
	})
	log.Info().
		Str("version", Version).
		Str("build_time", BuildTime).
		Msg("GoMailZero 启动")

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化存储
	var storageDriver storage.Driver
	if cfg.Storage.Driver == "sqlite" {
		storageDriver, err = storage.NewSQLiteDriver(cfg.Storage.DSN)
		if err != nil {
			log.Fatal().Err(err).Msg("初始化存储失败")
		}
		defer storageDriver.Close()

		// 执行数据库迁移或初始化
		sqliteDriver := storageDriver.(*storage.SQLiteDriver)
		if cfg.Storage.AutoMigrate {
			// 使用 goose 迁移
			migrationsDir, err := migrate.GetMigrationsDir()
			if err != nil {
				log.Warn().Err(err).Msg("找不到迁移目录，使用 initSchema 初始化")
				if err := sqliteDriver.RunMigrations(ctx, "", false); err != nil {
					log.Fatal().Err(err).Msg("数据库初始化失败")
				}
			} else {
				if err := sqliteDriver.RunMigrations(ctx, migrationsDir, true); err != nil {
					log.Fatal().Err(err).Msg("数据库迁移失败")
				}
				log.Info().Msg("数据库迁移完成")
			}
		} else {
			// 使用 initSchema 初始化（向后兼容）
			if err := sqliteDriver.RunMigrations(ctx, "", false); err != nil {
				log.Fatal().Err(err).Msg("数据库初始化失败")
			}
			log.Info().Msg("数据库初始化完成")
		}
	} else {
		log.Fatal().Str("driver", cfg.Storage.Driver).Msg("不支持的存储驱动")
	}

	// 初始化 Maildir
	maildir, err := storage.NewMaildir(cfg.Storage.MaildirRoot)
	if err != nil {
		log.Fatal().Err(err).Msg("初始化 Maildir 失败")
	}

	// 加载 TLS 配置
	var tlsConfig *tls.Config
	if cfg.TLS.Enabled {
		tlsConfig, err = tlsconfig.LoadTLSConfig(&cfg.TLS)
		if err != nil {
			log.Warn().Err(err).Msg("加载 TLS 配置失败，继续运行")
		}
	}

	// 创建认证器
	smtpAuth := smtpd.NewDefaultAuthenticator(storageDriver)

	// 启动 SMTP 服务器
	if cfg.SMTP.Enabled {
		smtpServer := smtpd.NewServer(&smtpd.Config{
			Enabled:  cfg.SMTP.Enabled,
			Ports:    cfg.SMTP.Ports,
			Hostname: cfg.SMTP.Hostname,
			MaxSize:  parseSize(cfg.SMTP.MaxSize),
			TLS:      tlsConfig,
			Storage:  storageDriver,
			Maildir:  maildir,
			Auth:     smtpAuth,
		})

		go func() {
			if err := smtpServer.Start(ctx); err != nil {
				log.Error().Err(err).Msg("SMTP 服务器启动失败")
			}
		}()
	}

	// 启动 IMAP 服务器
	if cfg.IMAP.Enabled {
		// IMAP 服务器需要 TLS 配置（如果 TLS 已启用但加载失败，记录警告）
		if cfg.TLS.Enabled && tlsConfig == nil {
			log.Warn().Msg("TLS 已启用但配置加载失败，IMAP 服务器将允许非安全连接（仅用于开发环境）")
		}
		
		imapServer := imapd.NewServer(&imapd.Config{
			Enabled: cfg.IMAP.Enabled,
			Port:    cfg.IMAP.Port,
			TLS:     tlsConfig,
			Storage: storageDriver,
			Maildir: maildir, // 传递 Maildir 实例以支持读取邮件体
			Auth:    imapd.NewDefaultAuthenticator(storageDriver),
		})

		go func() {
			if err := imapServer.Start(ctx); err != nil {
				log.Error().Err(err).Msg("IMAP 服务器启动失败")
			}
		}()
	}

	// 启动管理 API
	if cfg.Admin.APIKey != "" {
		// 创建 JWT 管理器
		jwtSecret := cfg.Admin.JWTSecret
		if jwtSecret == "" {
			jwtSecret = "change-me-in-production" // 默认密钥（生产环境必须更改）
		}
		jwtManager := auth.NewJWTManager(jwtSecret, "gomailzero")

		// 创建 TOTP 管理器
		totpManager := auth.NewTOTPManager(storageDriver)

		apiServer := api.NewServer(&api.Config{
			Port:        cfg.Admin.Port,
			APIKey:      cfg.Admin.APIKey,
			Domain:      cfg.Domain,
			Storage:     storageDriver,
			JWTManager:  jwtManager,
			TOTPManager: totpManager,
		})

		go func() {
			if err := apiServer.Start(ctx); err != nil {
				log.Error().Err(err).Msg("管理 API 启动失败")
			}
		}()
	}

	// 启动指标服务器
	if cfg.Metrics.Enabled {
		exporter := metrics.NewExporter()
		mux := http.NewServeMux()
		mux.Handle(cfg.Metrics.Path, exporter.Handler())

		metricsServer := &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.Metrics.Port),
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second, // 防止 Slowloris 攻击
		}

		go func() {
			log.Info().Int("port", cfg.Metrics.Port).Str("path", cfg.Metrics.Path).Msg("指标服务器启动")
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("指标服务器错误")
			}
		}()
	}

	// 启动 WebMail 服务器
	if cfg.WebMail.Enabled {
		// 生成 JWT 密钥（如果未配置，使用默认值）
		jwtSecret := cfg.Admin.JWTSecret
		if jwtSecret == "" {
			jwtSecret = "change-me-in-production"
		}

		// 创建 TOTP 管理器
		totpManager := auth.NewTOTPManager(storageDriver)

		// 加载 DKIM（如果配置了）
		var dkim *antispam.DKIM
		if cfg.SMTP.DKIM.Enabled {
			dkimInstance, err := smtpclient.LoadDKIM(&cfg.SMTP.DKIM, cfg.Domain, cfg.WorkDir)
			if err != nil {
				log.Warn().Err(err).Msg("加载 DKIM 失败，将发送未签名的邮件")
			} else {
				dkim = dkimInstance
			}
		}

		webServer := web.NewServer(&web.Config{
			Path:        cfg.WebMail.Path,
			Port:        cfg.WebMail.Port,
			Domain:      cfg.Domain,
			Storage:     storageDriver,
			Maildir:     maildir,
			JWTSecret:   jwtSecret,
			JWTIssuer:   cfg.Domain,
			TOTPManager: totpManager,
			AdminPort:   cfg.Admin.Port, // 管理 API 端口，用于代理管理界面
			SMTPConfig:  &cfg.SMTP,      // SMTP 配置，用于外发邮件
			DKIM:        dkim,           // DKIM 签名器
		})

		go func() {
			if err := webServer.Start(ctx); err != nil {
				log.Error().Err(err).Msg("WebMail 服务器启动失败")
			}
		}()
	}

	log.Info().Msg("所有服务已启动")

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("收到退出信号")
	case <-ctx.Done():
		log.Info().Msg("上下文取消")
	}

	log.Info().Msg("GoMailZero 关闭")
}

// parseSize 解析大小字符串（如 "50MB"）为字节数
func parseSize(sizeStr string) int64 {
	// 简化实现，仅支持 MB
	if len(sizeStr) < 2 {
		return 50 * 1024 * 1024 // 默认 50MB
	}

	unit := sizeStr[len(sizeStr)-2:]
	value := sizeStr[:len(sizeStr)-2]

	var multiplier int64 = 1
	switch unit {
	case "MB":
		multiplier = 1024 * 1024
	case "KB":
		multiplier = 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	}

	var size int64
	if _, err := fmt.Sscanf(value, "%d", &size); err != nil {
		// 如果解析失败，返回 0
		return 0
	}
	return size * multiplier
}
