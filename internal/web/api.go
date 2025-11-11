package web

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gomailzero/gmz/internal/crypto"
	"github.com/gomailzero/gmz/internal/storage"
)

// loginHandler 登录处理器
func loginHandler(driver storage.Driver) gin.HandlerFunc {
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

		// TODO: 验证 TOTP（如果启用）

		// 生成 JWT token（简化实现）
		token := "dummy-token" // TODO: 实现 JWT

		c.JSON(http.StatusOK, gin.H{
			"token": token,
			"user": gin.H{
				"email": user.Email,
				"quota": user.Quota,
			},
		})
	}
}

// listMailsHandler 列出邮件
func listMailsHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: 从 JWT 获取用户邮箱
		userEmail := c.GetHeader("X-User-Email")
		if userEmail == "" {
			userEmail = c.Query("user")
		}

		folder := c.DefaultQuery("folder", "INBOX")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		ctx := c.Request.Context()
		mails, err := driver.ListMails(ctx, userEmail, folder, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"mails": mails,
		})
	}
}

// getMailHandler 获取邮件
func getMailHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ctx := c.Request.Context()

		mail, err := driver.GetMail(ctx, id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "邮件不存在",
			})
			return
		}

		c.JSON(http.StatusOK, mail)
	}
}

// sendMailHandler 发送邮件
func sendMailHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: 实现邮件发送
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "未实现",
		})
	}
}

// deleteMailHandler 删除邮件
func deleteMailHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ctx := c.Request.Context()

		if err := driver.DeleteMail(ctx, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "邮件已删除",
		})
	}
}

