package web

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomailzero/gmz/internal/auth"
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
	Path      string
	Port      int
	Storage   storage.Driver
	JWTSecret string
	JWTIssuer string
}

// NewServer 创建 WebMail 服务器
func NewServer(cfg *Config) *Server {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware())

	// 静态文件服务
	router.StaticFS("/static", http.FS(staticFiles))

	// 创建 JWT 管理器
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer)

	// API 路由
	api := router.Group("/api")
	{
		// 公开端点（不需要认证）
		api.POST("/login", loginHandler(cfg.Storage, jwtManager))
		
		// 需要认证的端点
		api.Use(jwtMiddleware(jwtManager))
		{
			api.GET("/mails", listMailsHandler(cfg.Storage))
			api.GET("/mails/:id", getMailHandler(cfg.Storage))
			api.POST("/mails", sendMailHandler(cfg.Storage))
			api.DELETE("/mails/:id", deleteMailHandler(cfg.Storage))
			api.PUT("/mails/:id/flags", updateMailFlagsHandler(cfg.Storage))
		}
	}

	return &Server{
		config:     cfg,
		storage:    cfg.Storage,
		jwtManager: jwtManager,
		router:     router,
	}
	}

	// SPA 路由（所有其他路由返回 index.html）
	router.NoRoute(func(c *gin.Context) {
		// 返回 index.html
		c.FileFromFS("static/index.html", http.FS(staticFiles))
	})

	return &Server{
		config:     cfg,
		storage:    cfg.Storage,
		jwtManager: auth.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer),
		router:     router,
	}
}

// Start 启动服务器
func (s *Server) Start(ctx context.Context) error {
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
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

		logger.Info().
			Int("status", status).
			Str("method", c.Request.Method).
			Str("path", path).
			Dur("latency", latency).
			Str("ip", c.ClientIP()).
			Msg("WebMail 请求")
	}
}
