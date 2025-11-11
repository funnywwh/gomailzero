package web

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gomailzero/gmz/internal/auth"
)

// jwtMiddleware JWT 认证中间件
func jwtMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未授权",
			})
			c.Abort()
			return
		}

		// 提取 Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "无效的认证格式",
			})
			c.Abort()
			return
		}

		token := parts[1]

		// 验证 token
		claims, err := jwtManager.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "无效的令牌",
			})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文
		c.Set("user_email", claims.Email)
		c.Set("user_id", claims.UserID)
		c.Set("is_admin", claims.IsAdmin)

		c.Next()
	}
}

// optionalJWTMiddleware 可选的 JWT 认证中间件（用于某些公开端点）
func optionalJWTMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				claims, err := jwtManager.ValidateToken(parts[1])
				if err == nil {
					c.Set("user_email", claims.Email)
					c.Set("user_id", claims.UserID)
					c.Set("is_admin", claims.IsAdmin)
				}
			}
		}
		c.Next()
	}
}

