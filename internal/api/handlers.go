package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomailzero/gmz/internal/auth"
	"github.com/gomailzero/gmz/internal/crypto"
	"github.com/gomailzero/gmz/internal/storage"
)

// listDomainsHandler 列出域名
func listDomainsHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		domains, err := driver.ListDomains(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"domains": domains,
		})
	}
}

// createDomainHandler 创建域名
func createDomainHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name   string `json:"name" binding:"required"`
			Active bool   `json:"active"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		domain := &storage.Domain{
			Name:   req.Name,
			Active: req.Active,
		}
		// 设置默认值
		if !req.Active {
			domain.Active = true // 默认激活
		}

		ctx := c.Request.Context()
		if err := driver.CreateDomain(ctx, domain); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, domain)
	}
}

// getDomainHandler 获取域名
func getDomainHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		ctx := c.Request.Context()

		domain, err := driver.GetDomain(ctx, name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "域名不存在",
			})
			return
		}

		c.JSON(http.StatusOK, domain)
	}
}

// updateDomainHandler 更新域名
func updateDomainHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var req struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		ctx := c.Request.Context()
		existing, err := driver.GetDomain(ctx, name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "域名不存在",
			})
			return
		}

		domain := existing
		if req.Name != "" {
			domain.Name = req.Name
		}
		domain.Active = req.Active

		if err := driver.UpdateDomain(ctx, domain); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, domain)
	}
}

// deleteDomainHandler 删除域名
func deleteDomainHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		ctx := c.Request.Context()

		if err := driver.DeleteDomain(ctx, name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "域名已删除",
		})
	}
}

// listUsersHandler 列出用户
func listUsersHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		ctx := c.Request.Context()
		users, err := driver.ListUsers(ctx, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"users": users,
		})
	}
}

// createUserHandler 创建用户
func createUserHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required"`
			Password string `json:"password" binding:"required"`
			Quota    int64  `json:"quota"`
			Active   bool   `json:"active"`
			IsAdmin  bool   `json:"is_admin"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 哈希密码
		passwordHash, err := crypto.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "密码哈希失败",
			})
			return
		}

		user := &storage.User{
			Email:        req.Email,
			PasswordHash: passwordHash,
			Quota:        req.Quota,
			Active:       req.Active,
			IsAdmin:      req.IsAdmin,
		}
		// 设置默认值
		if !req.Active {
			user.Active = true // 默认激活
		}

		ctx := c.Request.Context()
		if err := driver.CreateUser(ctx, user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 不返回密码哈希
		user.PasswordHash = ""
		c.JSON(http.StatusCreated, user)
	}
}

// getUserHandler 获取用户
func getUserHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.Param("email")
		ctx := c.Request.Context()

		user, err := driver.GetUser(ctx, email)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "用户不存在",
			})
			return
		}

		// 不返回密码哈希
		user.PasswordHash = ""
		c.JSON(http.StatusOK, user)
	}
}

// updateUserHandler 更新用户
func updateUserHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.Param("email")
		var req struct {
			Password string `json:"password"`
			Quota    int64  `json:"quota"`
			Active   bool   `json:"active"`
			IsAdmin  *bool  `json:"is_admin"` // 使用指针以区分未设置和 false
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		ctx := c.Request.Context()
		user, err := driver.GetUser(ctx, email)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "用户不存在",
			})
			return
		}

		// 更新字段
		if req.Password != "" {
			passwordHash, err := crypto.HashPassword(req.Password)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "密码哈希失败",
				})
				return
			}
			user.PasswordHash = passwordHash
		}
		if req.Quota > 0 {
			user.Quota = req.Quota
		}
		user.Active = req.Active
		if req.IsAdmin != nil {
			user.IsAdmin = *req.IsAdmin
		}

		if err := driver.UpdateUser(ctx, user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		user.PasswordHash = ""
		c.JSON(http.StatusOK, user)
	}
}

// deleteUserHandler 删除用户
func deleteUserHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.Param("email")
		ctx := c.Request.Context()

		if err := driver.DeleteUser(ctx, email); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "用户已删除",
		})
	}
}

// listAliasesHandler 列出别名
func listAliasesHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Query("domain")
		ctx := c.Request.Context()

		aliases, err := driver.ListAliases(ctx, domain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"aliases": aliases,
		})
	}
}

// createAliasHandler 创建别名
func createAliasHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			From   string `json:"from" binding:"required"`
			To     string `json:"to" binding:"required"`
			Domain string `json:"domain" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		alias := &storage.Alias{
			From:   req.From,
			To:     req.To,
			Domain: req.Domain,
		}

		ctx := c.Request.Context()
		if err := driver.CreateAlias(ctx, alias); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, alias)
	}
}

// deleteAliasHandler 删除别名
func deleteAliasHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		from := c.Param("from")
		ctx := c.Request.Context()

		if err := driver.DeleteAlias(ctx, from); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "别名已删除",
		})
	}
}

// getQuotaHandler 获取配额
func getQuotaHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.Param("email")
		ctx := c.Request.Context()

		quota, err := driver.GetQuota(ctx, email)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "用户不存在",
			})
			return
		}

		c.JSON(http.StatusOK, quota)
	}
}

// updateQuotaHandler 更新配额
func updateQuotaHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.Param("email")
		var req struct {
			Limit int64 `json:"limit" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		quota := &storage.Quota{
			UserEmail: email,
			Limit:     req.Limit,
		}

		ctx := c.Request.Context()
		if err := driver.UpdateQuota(ctx, email, quota); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, quota)
	}
}

// checkInitHandler 检查系统是否需要初始化
func checkInitHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// 检查是否有用户
		users, err := driver.ListUsers(ctx, 1, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "检查初始化状态失败",
			})
			return
		}

		needsInit := len(users) == 0
		c.JSON(http.StatusOK, gin.H{
			"needs_init": needsInit,
		})
	}
}

// initSystemHandler 初始化系统（创建 admin 账户和域名）
func initSystemHandler(driver storage.Driver, jwtManager *auth.JWTManager, domain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required"`
			Password string `json:"password" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		ctx := c.Request.Context()

		// 检查是否已有用户
		users, err := driver.ListUsers(ctx, 1, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "检查用户列表失败",
			})
			return
		}

		if len(users) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "系统已初始化，无法重复初始化",
			})
			return
		}

		// 验证邮箱格式
		if !strings.Contains(req.Email, "@") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "邮箱格式无效",
			})
			return
		}

		// 验证密码长度
		if len(req.Password) < 8 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "密码长度至少为 8 位",
			})
			return
		}

		// 哈希密码
		passwordHash, err := crypto.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "密码哈希失败",
			})
			return
		}

		// 创建 admin 用户
		adminUser := &storage.User{
			Email:        req.Email,
			PasswordHash: passwordHash,
			Quota:        0, // 无限制
			Active:       true,
			IsAdmin:      true, // 初始化时创建的用户是管理员
		}

		if err := driver.CreateUser(ctx, adminUser); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("创建用户失败: %v", err),
			})
			return
		}

		// 确定域名（从邮箱或配置中获取）
		userDomain := domain
		if userDomain == "" {
			parts := strings.Split(req.Email, "@")
			if len(parts) == 2 {
				userDomain = parts[1]
			} else {
				userDomain = "example.com"
			}
		}

		// 创建域名（如果不存在）
		_, err = driver.GetDomain(ctx, userDomain)
		if err != nil {
			domainObj := &storage.Domain{
				Name:   userDomain,
				Active: true,
			}
			if err := driver.CreateDomain(ctx, domainObj); err != nil {
				// 域名创建失败不影响初始化，只记录警告
				// 可以继续
			}
		}

		// 生成 JWT token（自动登录）
		token, err := jwtManager.GenerateToken(adminUser.Email, adminUser.ID, false, 24*time.Hour)
		if err != nil {
			// Token 生成失败不影响初始化，但需要用户手动登录
			token = ""
		}

		// 返回初始化结果和密码（仅此一次显示）
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "系统初始化成功",
			"user": gin.H{
				"email": adminUser.Email,
			},
			"password": req.Password, // 返回明文密码（仅此一次）
			"token":    token,        // 如果生成成功，自动登录
		})
	}
}
