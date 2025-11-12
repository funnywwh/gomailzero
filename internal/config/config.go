package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	NodeID   string         `yaml:"node_id" mapstructure:"node_id"`
	Domain   string         `yaml:"domain" mapstructure:"domain"`
	WorkDir  string         `yaml:"workdir" mapstructure:"workdir"` // 工作目录，所有相对路径基于此目录
	TLS      TLSConfig      `yaml:"tls" mapstructure:"tls"`
	Storage  StorageConfig  `yaml:"storage" mapstructure:"storage"`
	SMTP     SMTPConfig     `yaml:"smtp" mapstructure:"smtp"`
	IMAP     IMAPConfig     `yaml:"imap" mapstructure:"imap"`
	AntiSpam AntiSpamConfig `yaml:"antispam" mapstructure:"antispam"`
	WebMail  WebMailConfig  `yaml:"webmail" mapstructure:"webmail"`
	Admin    AdminConfig    `yaml:"admin" mapstructure:"admin"`
	Log      LogConfig      `yaml:"log" mapstructure:"log"`
	Metrics  MetricsConfig  `yaml:"metrics" mapstructure:"metrics"`
}

// TLSConfig TLS 配置
type TLSConfig struct {
	Enabled    bool       `yaml:"enabled" mapstructure:"enabled"`
	CertFile   string     `yaml:"cert_file" mapstructure:"cert_file"`
	KeyFile    string     `yaml:"key_file" mapstructure:"key_file"`
	ACME       ACMEConfig `yaml:"acme" mapstructure:"acme"`
	MinVersion string     `yaml:"min_version" mapstructure:"min_version"`
}

// ACMEConfig ACME 配置
type ACMEConfig struct {
	Enabled  bool   `yaml:"enabled" mapstructure:"enabled"`
	Email    string `yaml:"email" mapstructure:"email"`
	Dir      string `yaml:"dir" mapstructure:"dir"`
	Provider string `yaml:"provider" mapstructure:"provider"` // letsencrypt, zerossl
}

// StorageConfig 存储配置
type StorageConfig struct {
	Driver      string `yaml:"driver" mapstructure:"driver"` // sqlite, postgres
	DSN         string `yaml:"dsn" mapstructure:"dsn"`
	MaildirRoot string `yaml:"maildir_root" mapstructure:"maildir_root"`
	AutoMigrate bool   `yaml:"auto_migrate" mapstructure:"auto_migrate"`
}

// SMTPConfig SMTP 配置
type SMTPConfig struct {
	Enabled  bool   `yaml:"enabled" mapstructure:"enabled"`
	Ports    []int  `yaml:"ports" mapstructure:"ports"`
	MaxSize  string `yaml:"max_size" mapstructure:"max_size"`
	Hostname string `yaml:"hostname" mapstructure:"hostname"`
	// 外发邮件中继配置（可选）
	Relay RelayConfig `yaml:"relay" mapstructure:"relay"`
	// DKIM 配置（用于直接投递时提高发送成功率）
	DKIM DKIMConfig `yaml:"dkim" mapstructure:"dkim"`
}

// DKIMConfig DKIM 配置
type DKIMConfig struct {
	Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`         // 是否启用 DKIM 签名
	Selector   string `yaml:"selector" mapstructure:"selector"`       // DKIM 选择器（如 default）
	PrivateKey string `yaml:"private_key" mapstructure:"private_key"` // DKIM 私钥文件路径（相对于 workdir）
	Domain     string `yaml:"domain" mapstructure:"domain"`           // 签名域名（留空使用主域名）
}

// RelayConfig SMTP 中继配置
type RelayConfig struct {
	Enabled  bool   `yaml:"enabled" mapstructure:"enabled"`
	Host     string `yaml:"host" mapstructure:"host"`         // 中继服务器地址（如 smtp.qq.com）
	Port     int    `yaml:"port" mapstructure:"port"`         // 中继服务器端口（如 587）
	Username string `yaml:"username" mapstructure:"username"` // 邮箱账号
	Password string `yaml:"password" mapstructure:"password"` // 邮箱密码或授权码
	UseTLS   bool   `yaml:"use_tls" mapstructure:"use_tls"`   // 是否使用 TLS（端口 587 通常需要）
}

// IMAPConfig IMAP 配置
type IMAPConfig struct {
	Enabled       bool `yaml:"enabled" mapstructure:"enabled"`
	Port          int  `yaml:"port" mapstructure:"port"`
	MaxAuthErrors int  `yaml:"max_auth_errors" mapstructure:"max_auth_errors"`
}

// AntiSpamConfig 反垃圾配置
type AntiSpamConfig struct {
	Enabled   bool   `yaml:"enabled" mapstructure:"enabled"`
	RspamdURL string `yaml:"rspamd_url" mapstructure:"rspamd_url"`
	ClamAVURL string `yaml:"clamav_url" mapstructure:"clamav_url"`
	Greylist  bool   `yaml:"greylist" mapstructure:"greylist"`
	RateLimit bool   `yaml:"rate_limit" mapstructure:"rate_limit"`
}

// WebMailConfig WebMail 配置
type WebMailConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Path    string `yaml:"path" mapstructure:"path"`
	Port    int    `yaml:"port" mapstructure:"port"`
}

// AdminConfig 管理配置
type AdminConfig struct {
	APIKey    string `yaml:"api_key" mapstructure:"api_key"`
	JWTSecret string `yaml:"jwt_secret" mapstructure:"jwt_secret"`
	Port      int    `yaml:"port" mapstructure:"port"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `yaml:"level" mapstructure:"level"`   // trace, debug, info, warn, error, fatal
	Format string `yaml:"format" mapstructure:"format"` // json, text
	Output string `yaml:"output" mapstructure:"output"` // stdout, file path
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Path    string `yaml:"path" mapstructure:"path"`
	Port    int    `yaml:"port" mapstructure:"port"`
}

// Load 加载配置
func Load(path string) (*Config, error) {
	v := viper.New()

	// 设置配置文件路径
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// 设置环境变量前缀
	v.SetEnvPrefix("GMZ")
	v.AutomaticEnv()

	// 设置默认值
	setDefaults(v)

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
		// 配置文件不存在时使用默认值
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 解析工作目录和相对路径
	if err := resolvePaths(&cfg); err != nil {
		return nil, fmt.Errorf("解析路径失败: %w", err)
	}

	// 验证配置
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &cfg, nil
}

// resolvePaths 解析工作目录和相对路径
func resolvePaths(cfg *Config) error {
	// 如果没有指定工作目录，使用当前工作目录
	if cfg.WorkDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("获取当前工作目录失败: %w", err)
		}
		cfg.WorkDir = wd
	}

	// 将工作目录转换为绝对路径
	workDir, err := filepath.Abs(cfg.WorkDir)
	if err != nil {
		return fmt.Errorf("解析工作目录失败: %w", err)
	}
	cfg.WorkDir = workDir

	// 解析相对路径为绝对路径（基于工作目录）
	resolvePath := func(path string) string {
		if path == "" {
			return path
		}
		// 如果已经是绝对路径，直接返回
		if filepath.IsAbs(path) {
			return path
		}
		// 相对路径基于工作目录解析
		return filepath.Join(workDir, path)
	}

	// 解析存储相关路径
	if cfg.Storage.DSN != "" && !filepath.IsAbs(cfg.Storage.DSN) && cfg.Storage.Driver == "sqlite" {
		// SQLite DSN 如果是相对路径，基于工作目录解析
		cfg.Storage.DSN = resolvePath(cfg.Storage.DSN)
	}
	cfg.Storage.MaildirRoot = resolvePath(cfg.Storage.MaildirRoot)

	// 解析 TLS 相关路径
	cfg.TLS.CertFile = resolvePath(cfg.TLS.CertFile)
	cfg.TLS.KeyFile = resolvePath(cfg.TLS.KeyFile)
	cfg.TLS.ACME.Dir = resolvePath(cfg.TLS.ACME.Dir)

	// 解析日志输出路径（如果不是 stdout）
	if cfg.Log.Output != "" && cfg.Log.Output != "stdout" && cfg.Log.Output != "stderr" {
		cfg.Log.Output = resolvePath(cfg.Log.Output)
	}

	return nil
}

// setDefaults 设置默认值
func setDefaults(v *viper.Viper) {
	// 基础配置
	v.SetDefault("node_id", "mx1")
	v.SetDefault("domain", "example.com")
	v.SetDefault("workdir", "") // 默认使用当前工作目录

	// TLS 配置
	v.SetDefault("tls.enabled", true)
	v.SetDefault("tls.min_version", "1.3")
	v.SetDefault("tls.acme.enabled", true)
	v.SetDefault("tls.acme.provider", "letsencrypt")
	v.SetDefault("tls.acme.dir", "/var/lib/gmz/certs")

	// 存储配置
	v.SetDefault("storage.driver", "sqlite")
	v.SetDefault("storage.dsn", "/var/lib/gmz/data.db")
	v.SetDefault("storage.maildir_root", "/var/lib/gmz/mail")
	v.SetDefault("storage.auto_migrate", true)

	// SMTP 配置
	v.SetDefault("smtp.enabled", true)
	v.SetDefault("smtp.ports", []int{25, 465, 587})
	v.SetDefault("smtp.max_size", "50MB")
	v.SetDefault("smtp.hostname", "")

	// IMAP 配置
	v.SetDefault("imap.enabled", true)
	v.SetDefault("imap.port", 993)
	v.SetDefault("imap.max_auth_errors", 5)

	// 反垃圾配置
	v.SetDefault("antispam.enabled", true)
	v.SetDefault("antispam.greylist", true)
	v.SetDefault("antispam.rate_limit", true)

	// WebMail 配置
	v.SetDefault("webmail.enabled", true)
	v.SetDefault("webmail.path", "/webmail")
	v.SetDefault("webmail.port", 8080)

	// 管理配置
	v.SetDefault("admin.port", 8081)

	// 日志配置
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("log.output", "stdout")

	// 指标配置
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.path", "/metrics")
	v.SetDefault("metrics.port", 9090)
}

// validate 验证配置
func validate(cfg *Config) error {
	if cfg.Domain == "" {
		return fmt.Errorf("domain 不能为空")
	}

	if cfg.Storage.Driver != "sqlite" && cfg.Storage.Driver != "postgres" {
		return fmt.Errorf("不支持的存储驱动: %s", cfg.Storage.Driver)
	}

	if cfg.TLS.Enabled && !cfg.TLS.ACME.Enabled {
		if cfg.TLS.CertFile == "" || cfg.TLS.KeyFile == "" {
			return fmt.Errorf("TLS 已启用但未配置证书文件")
		}
		// 检查证书文件是否存在
		if _, err := os.Stat(cfg.TLS.CertFile); err != nil {
			return fmt.Errorf("证书文件不存在: %w", err)
		}
		if _, err := os.Stat(cfg.TLS.KeyFile); err != nil {
			return fmt.Errorf("密钥文件不存在: %w", err)
		}
	}

	return nil
}

// Watch 监听配置文件变化
func Watch(path string, callback func(*Config) error) error {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("GMZ")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		var cfg Config
		if err := v.Unmarshal(&cfg); err != nil {
			// 使用标准输出记录错误（避免循环依赖）
			fmt.Fprintf(os.Stderr, "配置热更新失败: 解析错误: %v\n", err)
			return
		}

		if err := validate(&cfg); err != nil {
			fmt.Fprintf(os.Stderr, "配置热更新失败: 验证错误: %v\n", err)
			return
		}

		if err := callback(&cfg); err != nil {
			fmt.Fprintf(os.Stderr, "配置热更新失败: 回调错误: %v\n", err)
			return
		}

		fmt.Fprintf(os.Stdout, "配置热更新成功\n")
	})

	return nil
}
