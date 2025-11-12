package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gomailzero/gmz/internal/auth"
	"github.com/gomailzero/gmz/internal/storage"
)

// jwtMiddleware JWT 认证中间件
func jwtMiddleware(jwtManager *auth.JWTManager, driver storage.Driver) gin.HandlerFunc {
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

		// 验证用户是否仍然存在于数据库中
		ctx := c.Request.Context()
		user, err := driver.GetUser(ctx, claims.Email)
		if err != nil {
			// 检查是否是用户不存在的错误
			if errors.Is(err, storage.ErrNotFound) || strings.Contains(err.Error(), "用户不存在") {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "用户不存在或已被删除",
				})
			} else {
				// 其他错误（如数据库连接错误）返回 500
				c.Error(err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "验证用户失败",
				})
			}
			c.Abort()
			return
		}

		// 检查用户是否被禁用
		if !user.Active {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "用户已被禁用",
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

