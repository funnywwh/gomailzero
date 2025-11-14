package api

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomailzero/gmz/internal/auth"
	"github.com/gomailzero/gmz/internal/crypto"
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

// GetRouter 获取路由（用于测试）
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

// Config API 配置
type Config struct {
	Port        int
	APIKey      string
	Domain      string // 主域名，用于初始化
	Storage     storage.Driver
	JWTManager  *auth.JWTManager
	TOTPManager *auth.TOTPManager
}

// NewServer 创建 API 服务器
func NewServer(cfg *Config) *Server {
	// 设置 Gin 模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware())

	// 静态文件服务（管理界面）
	// go:embed static/* 会包含 static/ 前缀，所以直接使用
	router.StaticFS("/admin/static", http.FS(staticFiles))
	// 支持 /admin/assets 路径（前端资源），映射到 static/assets
	assetsFS, err := fs.Sub(staticFiles, "static/assets")
	if err == nil {
		router.StaticFS("/admin/assets", http.FS(assetsFS))
	}

	// 健康检查
	router.GET("/health", healthHandler)

	// 公开端点：初始化和登录
	router.GET("/api/v1/init/check", checkInitHandler(cfg.Storage))
	router.POST("/api/v1/init", initSystemHandler(cfg.Storage, cfg.JWTManager, cfg.Domain))
	router.POST("/api/v1/auth/login", loginHandler(cfg.Storage, cfg.JWTManager, cfg.TOTPManager))

	// API 路由组
	api := router.Group("/api/v1")
	// 支持 API Key 和 JWT 两种认证方式
	api.Use(authMiddleware(cfg.APIKey, cfg.JWTManager))

	// 域名管理（敏感操作需要 TOTP）
	api.GET("/domains", listDomainsHandler(cfg.Storage))
	api.POST("/domains", totpRequiredMiddleware(cfg.TOTPManager, cfg.Storage), createDomainHandler(cfg.Storage))
	api.GET("/domains/:name", getDomainHandler(cfg.Storage))
	api.PUT("/domains/:name", totpRequiredMiddleware(cfg.TOTPManager, cfg.Storage), updateDomainHandler(cfg.Storage))
	api.DELETE("/domains/:name", totpRequiredMiddleware(cfg.TOTPManager, cfg.Storage), deleteDomainHandler(cfg.Storage))

	// 用户管理
	api.GET("/users", listUsersHandler(cfg.Storage))
	// 创建用户需要 TOTP（如果启用）
	api.POST("/users", totpRequiredMiddleware(cfg.TOTPManager, cfg.Storage), createUserHandler(cfg.Storage))
	api.GET("/users/:email", getUserHandler(cfg.Storage))
	// 更新和删除用户需要 TOTP（如果启用）
	api.PUT("/users/:email", totpRequiredMiddleware(cfg.TOTPManager, cfg.Storage), updateUserHandler(cfg.Storage))
	api.DELETE("/users/:email", totpRequiredMiddleware(cfg.TOTPManager, cfg.Storage), deleteUserHandler(cfg.Storage))

	// 别名管理
	api.GET("/aliases", listAliasesHandler(cfg.Storage))
	api.POST("/aliases", createAliasHandler(cfg.Storage))
	api.DELETE("/aliases/:from", deleteAliasHandler(cfg.Storage))

	// 配额管理
	api.GET("/users/:email/quota", getQuotaHandler(cfg.Storage))
	api.PUT("/users/:email/quota", updateQuotaHandler(cfg.Storage))

	// 管理界面路由（SPA）
	router.GET("/admin", func(c *gin.Context) {
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "无法加载管理界面")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// SPA 路由（所有 /admin/* 路由返回 index.html，除了 API 和静态资源）
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// 只处理 /admin 路径下的路由
		if strings.HasPrefix(path, "/admin") {
			// 排除 API 和静态资源路径
			if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/admin/static") || strings.HasPrefix(path, "/admin/assets") {
				c.Status(http.StatusNotFound)
				return
			}
			// 返回 index.html
			data, err := staticFiles.ReadFile("static/index.html")
			if err != nil {
				c.String(http.StatusInternalServerError, "无法加载管理界面")
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
			return
		}
		c.Status(http.StatusNotFound)
	})

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

// authMiddleware 认证中间件（支持 API Key 和 JWT）
func authMiddleware(apiKey string, jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 优先检查 API Key
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.Query("api_key")
		}

		if key == apiKey {
			// API Key 认证成功
			c.Next()
			return
		}

		// 尝试 JWT 认证
		if jwtManager != nil {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					claims, err := jwtManager.ValidateToken(parts[1])
					if err == nil {
						// JWT 认证成功，将用户信息存储到上下文
						c.Set("user_email", claims.Email)
						c.Set("user_id", claims.UserID)
						c.Set("is_admin", claims.IsAdmin)
						c.Next()
						return
					}
				}
			}
		}

		// 认证失败
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未授权",
		})
		c.Abort()
	}
}

// totpRequiredMiddleware TOTP 验证中间件（用于敏感操作）
func totpRequiredMiddleware(totpManager *auth.TOTPManager, storage storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取用户邮箱（JWT 认证会设置，API Key 不会）
		userEmail, exists := c.Get("user_email")
		if !exists {
			// API Key 认证不需要 TOTP（管理员操作）
			c.Next()
			return
		}

		email := userEmail.(string)
		ctx := c.Request.Context()

		// 检查用户是否启用了 TOTP
		totpEnabled, err := totpManager.IsEnabled(ctx, email)
		if err != nil {
			logger.Warn().Err(err).Str("user", email).Msg("检查 TOTP 状态失败")
			c.Next() // 如果检查失败，继续（不强制 TOTP）
			return
		}

		if !totpEnabled {
			// 未启用 TOTP，直接通过
			c.Next()
			return
		}

		// 已启用 TOTP，需要验证 TOTP 代码
		totpCode := c.GetHeader("X-TOTP-Code")
		if totpCode == "" {
			totpCode = c.Query("totp_code")
		}

		if totpCode == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":        "需要 TOTP 代码",
				"requires_2fa": true,
			})
			c.Abort()
			return
		}

		// 验证 TOTP 代码
		valid, err := totpManager.Verify(ctx, email, totpCode)
		if err != nil {
			logger.Warn().Err(err).Str("user", email).Msg("TOTP 验证失败")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "TOTP 验证失败",
			})
			c.Abort()
			return
		}

		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "TOTP 代码错误",
			})
			c.Abort()
			return
		}

		// TOTP 验证通过
		c.Next()
	}
}

// loginHandler 登录处理器
func loginHandler(driver storage.Driver, jwtManager *auth.JWTManager, totpManager *auth.TOTPManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required"`
			Password string `json:"password" binding:"required"`
			TOTPCode string `json:"totp_code"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		ctx := c.Request.Context()
		user, err := driver.GetUser(ctx, req.Email)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "认证失败",
			})
			return
		}

		// 验证密码
		valid, err := crypto.VerifyPassword(req.Password, user.PasswordHash)
		if err != nil || !valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "认证失败",
			})
			return
		}

		// 检查是否启用了 TOTP
		if totpManager != nil {
			totpEnabled, err := totpManager.IsEnabled(ctx, req.Email)
			if err == nil && totpEnabled {
				// 如果启用了 TOTP，必须提供 TOTP 代码
				if req.TOTPCode == "" {
					c.JSON(http.StatusUnauthorized, gin.H{
						"error":        "需要 TOTP 代码",
						"requires_2fa": true,
					})
					return
				}

				// 验证 TOTP 代码
				valid, err := totpManager.Verify(ctx, req.Email, req.TOTPCode)
				if err != nil || !valid {
					c.JSON(http.StatusUnauthorized, gin.H{
						"error": "TOTP 代码错误",
					})
					return
				}
			}
		}

		// 检查用户是否是管理员（只有管理员才能登录管理后台）
		if !user.IsAdmin {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "只有管理员才能登录管理后台",
			})
			return
		}

		// 生成 JWT token
		if jwtManager == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "JWT 管理器未配置",
			})
			return
		}

		token, err := jwtManager.GenerateToken(user.Email, user.ID, user.IsAdmin, 24*time.Hour)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "生成令牌失败",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token": token,
			"user": gin.H{
				"email": user.Email,
				"quota": user.Quota,
			},
		})
	}
}

// healthHandler 健康检查处理器
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}
