package web

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomailzero/gmz/internal/antispam"
	"github.com/gomailzero/gmz/internal/auth"
	"github.com/gomailzero/gmz/internal/config"
	"github.com/gomailzero/gmz/internal/logger"
	"github.com/gomailzero/gmz/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

// Server WebMail 服务器
type Server struct {
	config     *Config
	storage    storage.Driver
	jwtManager *auth.JWTManager
	router     *gin.Engine
	server     *http.Server
}

// Config WebMail 配置
type Config struct {
	Path        string
	Port        int
	Domain      string // 主域名，用于初始化
	Storage     storage.Driver
	Maildir     *storage.Maildir // Maildir 实例，用于读取邮件体
	JWTSecret   string
	JWTIssuer   string
	TOTPManager *auth.TOTPManager
	AdminPort   int                // 管理 API 端口，用于代理管理界面
	SMTPConfig  *config.SMTPConfig // SMTP 配置，用于外发邮件
	DKIM        *antispam.DKIM     // DKIM 签名器（可选）
}

// NewServer 创建 WebMail 服务器
func NewServer(cfg *Config) *Server {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(traceIDMiddleware()) // trace_id 中间件必须在最前面
	router.Use(loggerMiddleware())

	// 静态文件服务
	router.StaticFS("/static", http.FS(staticFiles))
	// 支持 /assets 路径（前端资源），映射到 static/assets
	assetsFS, err := fs.Sub(staticFiles, "static/assets")
	if err == nil {
		router.StaticFS("/assets", http.FS(assetsFS))
	}

	// 创建 JWT 管理器
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer)

	// 管理界面代理（代理到管理 API 服务器）
	// 注意：必须在 WebMail API 路由之前注册，确保 /api/v1 优先匹配
	if cfg.AdminPort > 0 {
		adminURL, err := url.Parse(fmt.Sprintf("http://localhost:%d", cfg.AdminPort))
		if err == nil {
			proxy := httputil.NewSingleHostReverseProxy(adminURL)

			// 修改请求路径，确保代理到正确的后端路径
			originalDirector := proxy.Director
			proxy.Director = func(req *http.Request) {
				originalDirector(req)
				// 保持原始路径不变（后端已经配置了 /admin 路由）
			}

			// 代理管理 API 请求（/api/v1/*）- 必须在 WebMail API 之前
			apiV1Group := router.Group("/api/v1")
			apiV1Group.Any("/*path", func(c *gin.Context) {
				proxy.ServeHTTP(c.Writer, c.Request)
			})

			// 代理管理界面静态资源
			router.Any("/admin", func(c *gin.Context) {
				proxy.ServeHTTP(c.Writer, c.Request)
			})
			router.Any("/admin/*path", func(c *gin.Context) {
				proxy.ServeHTTP(c.Writer, c.Request)
			})
		}
	}

	// API 路由（WebMail API，注意 /api/v1 已经在上面被代理了）
	api := router.Group("/api")
	{
		// 公开端点（不需要认证）
		api.GET("/init/check", checkInitHandler(cfg.Storage))
		api.POST("/init", initSystemHandler(cfg.Storage, jwtManager, cfg.Domain))
		api.POST("/login", loginHandler(cfg.Storage, jwtManager, cfg.TOTPManager))

		// 需要认证的端点
		api.Use(jwtMiddleware(jwtManager, cfg.Storage))
		{
			api.GET("/me", getCurrentUserHandler(cfg.Storage)) // 获取当前用户信息
			api.GET("/mails", listMailsHandler(cfg.Storage))
			api.GET("/mails/search", searchMailsHandler(cfg.Storage))
			api.GET("/mails/:id", getMailHandler(cfg.Storage, cfg.Maildir))
			api.POST("/mails", sendMailHandler(cfg.Storage, cfg.Maildir, cfg.SMTPConfig, cfg.DKIM))
			api.POST("/mails/drafts", saveDraftHandler(cfg.Storage))
			api.DELETE("/mails/:id", deleteMailHandler(cfg.Storage))
			api.PUT("/mails/:id/flags", updateMailFlagsHandler(cfg.Storage))
			api.GET("/folders", listFoldersHandler(cfg.Storage))
		}
	}

	// 根路径返回 index.html
	router.GET("/", func(c *gin.Context) {
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "无法加载页面")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// SPA 路由（所有其他路由返回 index.html）
	router.NoRoute(func(c *gin.Context) {
		// 排除 API、静态资源和管理界面路径
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/static") || strings.HasPrefix(path, "/assets") || strings.HasPrefix(path, "/admin") {
			c.Status(http.StatusNotFound)
			return
		}
		// 返回 index.html
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "无法加载页面")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	return &Server{
		config:     cfg,
		storage:    cfg.Storage,
		jwtManager: jwtManager,
		router:     router,
	}
}

// Start 启动服务器
func (s *Server) Start(ctx context.Context) error {
	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.config.Port),
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second, // 防止 Slowloris 攻击
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info().Int("port", s.config.Port).Str("path", s.config.Path).Msg("WebMail 服务器启动")

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("WebMail 服务器错误: %w", err)
	}

	return nil
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("关闭 WebMail 服务器失败: %w", err)
	}

	logger.Info().Msg("WebMail 服务器已停止")
	return nil
}

// loggerMiddleware 日志中间件
func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		// 获取错误信息（如果有）
		errs := c.Errors
		var errMsg string
		if len(errs) > 0 {
			errMsg = errs.String()
		}

		// 获取 trace_id
		traceID, _ := c.Get("trace_id")
		traceIDStr := ""
		if traceID != nil {
			traceIDStr = traceID.(string)
		}

		logEntry := logger.Info().
			Str("trace_id", traceIDStr).
			Int("status", status).
			Str("method", c.Request.Method).
			Str("path", path).
			Dur("latency", latency).
			Str("ip", c.ClientIP())

		// 如果是错误状态，记录错误信息
		if status >= 400 {
			if errMsg != "" {
				logEntry = logEntry.Str("error", errMsg)
			}
			// 记录请求参数（仅用于调试）
			if c.Request.URL.RawQuery != "" {
				logEntry = logEntry.Str("query", c.Request.URL.RawQuery)
			}
		}

		logEntry.Msg("WebMail 请求")
	}
}
