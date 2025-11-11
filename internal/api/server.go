package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomailzero/gmz/internal/logger"
	"github.com/gomailzero/gmz/internal/storage"
)

// Server API 服务器
type Server struct {
	config  *Config
	storage storage.Driver
	router  *gin.Engine
	server  *http.Server
}

// Config API 配置
type Config struct {
	Port    int
	APIKey  string
	Storage storage.Driver
}

// NewServer 创建 API 服务器
func NewServer(cfg *Config) *Server {
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware())

	// 健康检查
	router.GET("/health", healthHandler)

	// API 路由组
	api := router.Group("/api/v1")
	// 支持 API Key 和 JWT 两种认证方式
	api.Use(authMiddleware(cfg.APIKey))

	// 域名管理
	api.GET("/domains", listDomainsHandler(cfg.Storage))
	api.POST("/domains", createDomainHandler(cfg.Storage))
	api.GET("/domains/:name", getDomainHandler(cfg.Storage))
	api.PUT("/domains/:name", updateDomainHandler(cfg.Storage))
	api.DELETE("/domains/:name", deleteDomainHandler(cfg.Storage))

	// 用户管理
	api.GET("/users", listUsersHandler(cfg.Storage))
	api.POST("/users", createUserHandler(cfg.Storage))
	api.GET("/users/:email", getUserHandler(cfg.Storage))
	api.PUT("/users/:email", updateUserHandler(cfg.Storage))
	api.DELETE("/users/:email", deleteUserHandler(cfg.Storage))

	// 别名管理
	api.GET("/aliases", listAliasesHandler(cfg.Storage))
	api.POST("/aliases", createAliasHandler(cfg.Storage))
	api.DELETE("/aliases/:from", deleteAliasHandler(cfg.Storage))

	// 配额管理
	api.GET("/users/:email/quota", getQuotaHandler(cfg.Storage))
	api.PUT("/users/:email/quota", updateQuotaHandler(cfg.Storage))

	return &Server{
		config:  cfg,
		storage: cfg.Storage,
		router:  router,
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

	logger.Info().Int("port", s.config.Port).Msg("管理 API 服务器启动")

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("API 服务器错误: %w", err)
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
		return fmt.Errorf("关闭 API 服务器失败: %w", err)
	}

	logger.Info().Msg("管理 API 服务器已停止")
	return nil
}

// loggerMiddleware 日志中间件
func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		logger.Info().
			Int("status", status).
			Str("method", c.Request.Method).
			Str("path", path).
			Str("query", query).
			Dur("latency", latency).
			Str("ip", c.ClientIP()).
			Msg("API 请求")
	}
}

// authMiddleware 认证中间件
func authMiddleware(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 API Key
		key := c.GetHeader("X-API-Key")
		if key == "" {
			// 尝试从 Query 参数获取
			key = c.Query("api_key")
		}

		if key != apiKey {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未授权",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// healthHandler 健康检查处理器
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}
