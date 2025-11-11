package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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
			Active:       true,
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

