package web

import (
	"bytes"
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

		// 验证 TOTP（如果启用）
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

		// 生成 JWT token
		token, err := jwtManager.GenerateToken(user.Email, user.ID, false, 24*time.Hour)
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

// listMailsHandler 列出邮件
func listMailsHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 JWT 获取用户邮箱
		userEmail, exists := c.Get("user_email")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未授权",
			})
			c.Abort()
			return
		}

		email := userEmail.(string)
		folder := c.DefaultQuery("folder", "INBOX")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		ctx := c.Request.Context()
		mails, err := driver.ListMails(ctx, email, folder, limit, offset)
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
func getMailHandler(driver storage.Driver, maildir *storage.Maildir) gin.HandlerFunc {
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

		// 检查权限（只能访问自己的邮件）
		userEmail, _ := c.Get("user_email")
		if mail.UserEmail != userEmail {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "无权访问此邮件",
			})
			return
		}

		// 读取邮件体（从 Maildir）
		bodyText := ""
		bodyHTML := ""
		if maildir != nil {
			// 邮件 ID 就是 Maildir 中的文件名
			body, err := maildir.ReadMail(mail.UserEmail, mail.Folder, id)
			if err == nil {
				// 解析邮件体（简单实现：查找 text/plain 和 text/html 部分）
				bodyStr := string(body)
				
				// 检查是否是 MIME 格式
				if strings.Contains(bodyStr, "Content-Type:") {
					// 简单的 MIME 解析
					// 查找 text/plain 部分
					if idx := strings.Index(bodyStr, "Content-Type: text/plain"); idx >= 0 {
						// 找到正文开始位置
						bodyStart := strings.Index(bodyStr[idx:], "\r\n\r\n")
						if bodyStart >= 0 {
							plainText := bodyStr[idx+bodyStart+4:]
							// 移除后续的 MIME 部分
							if nextBoundary := strings.Index(plainText, "\r\n--"); nextBoundary >= 0 {
								plainText = plainText[:nextBoundary]
							}
							bodyText = strings.TrimSpace(plainText)
						}
					}
					
					// 查找 text/html 部分
					if idx := strings.Index(bodyStr, "Content-Type: text/html"); idx >= 0 {
						bodyStart := strings.Index(bodyStr[idx:], "\r\n\r\n")
						if bodyStart >= 0 {
							htmlText := bodyStr[idx+bodyStart+4:]
							if nextBoundary := strings.Index(htmlText, "\r\n--"); nextBoundary >= 0 {
								htmlText = htmlText[:nextBoundary]
							}
							bodyHTML = strings.TrimSpace(htmlText)
						}
					}
				} else {
					// 纯文本邮件
					bodyText = bodyStr
				}
			}
			// 如果读取失败，忽略错误（可能邮件体不存在）
		}

		// 构建响应
		response := gin.H{
			"id":          mail.ID,
			"user_email":  mail.UserEmail,
			"folder":      mail.Folder,
			"from":        mail.From,
			"to":          mail.To,
			"cc":          mail.Cc,
			"bcc":         mail.Bcc,
			"subject":     mail.Subject,
			"body":        bodyText,  // 纯文本正文
			"body_html":   bodyHTML, // HTML 正文
			"size":        mail.Size,
			"flags":       mail.Flags,
			"received_at": mail.ReceivedAt,
			"created_at":  mail.CreatedAt,
		}

		c.JSON(http.StatusOK, response)
	}
}

// sendMailHandler 发送邮件
func sendMailHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 JWT 获取用户邮箱
		userEmail, exists := c.Get("user_email")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未授权",
			})
			c.Abort()
			return
		}

		var req struct {
			To      []string `json:"to" binding:"required"`
			Cc      []string `json:"cc"`
			Bcc     []string `json:"bcc"`
			Subject string   `json:"subject" binding:"required"`
			Body    string   `json:"body" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 构建邮件
		from := userEmail.(string)
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
		buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(req.To, ", ")))
		if len(req.Cc) > 0 {
			buf.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(req.Cc, ", ")))
		}
		buf.WriteString(fmt.Sprintf("Subject: %s\r\n", req.Subject))
		buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		buf.WriteString("\r\n")
		buf.WriteString(req.Body)

		// 存储到 Sent 文件夹
		ctx := c.Request.Context()
		mail := &storage.Mail{
			ID:         fmt.Sprintf("sent-%d", time.Now().UnixNano()),
			UserEmail:  from,
			Folder:     "Sent",
			From:       from,
			To:         req.To,
			Cc:         req.Cc,
			Bcc:        req.Bcc,
			Subject:    req.Subject,
			Body:       []byte(req.Body),
			Size:       int64(buf.Len()),
			Flags:      []string{},
			ReceivedAt: time.Now(),
			CreatedAt:  time.Now(),
		}

		if err := driver.StoreMail(ctx, mail); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "保存邮件失败",
			})
			return
		}

		// 发送邮件到外部服务器（通过本地 SMTP 服务器）
		// 注意：这里简化实现，实际应该通过 SMTP 客户端发送到外部服务器
		// 当前实现将邮件存储到 Sent 文件夹，实际发送需要配置 SMTP 中继服务器
		// TODO: 实现 SMTP 中继客户端，将邮件发送到外部服务器

		c.JSON(http.StatusOK, gin.H{
			"message": "邮件已保存到已发送文件夹",
			"id":      mail.ID,
			"note":    "实际发送功能需要配置 SMTP 中继服务器",
		})
	}
}

// updateMailFlagsHandler 更新邮件标志
func updateMailFlagsHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Flags []string `json:"flags" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		ctx := c.Request.Context()
		if err := driver.UpdateMailFlags(ctx, id, req.Flags); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "标志已更新",
		})
	}
}

// deleteMailHandler 删除邮件
func deleteMailHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ctx := c.Request.Context()

		// 检查权限
		mail, err := driver.GetMail(ctx, id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "邮件不存在",
			})
			return
		}

		userEmail, _ := c.Get("user_email")
		if mail.UserEmail != userEmail {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "无权删除此邮件",
			})
			return
		}

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

// searchMailsHandler 搜索邮件
func searchMailsHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		userEmail, exists := c.Get("user_email")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未授权",
			})
			c.Abort()
			return
		}

		email := userEmail.(string)
		query := c.Query("q")
		folder := c.DefaultQuery("folder", "")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "搜索查询不能为空",
			})
			return
		}

		ctx := c.Request.Context()
		mails, err := driver.SearchMails(ctx, email, query, folder, limit, offset)
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

// listFoldersHandler 列出文件夹
func listFoldersHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		userEmail, exists := c.Get("user_email")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未授权",
			})
			c.Abort()
			return
		}

		email := userEmail.(string)
		ctx := c.Request.Context()
		folders, err := driver.ListFolders(ctx, email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"folders": folders,
		})
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

// saveDraftHandler 保存草稿
func saveDraftHandler(driver storage.Driver) gin.HandlerFunc {
	return func(c *gin.Context) {
		userEmail, exists := c.Get("user_email")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未授权",
			})
			c.Abort()
			return
		}

		var req struct {
			ID      string   `json:"id"` // 如果提供，更新现有草稿
			To      []string `json:"to"`
			Cc      []string `json:"cc"`
			Bcc     []string `json:"bcc"`
			Subject string   `json:"subject"`
			Body    string   `json:"body"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		from := userEmail.(string)
		ctx := c.Request.Context()

		mailID := req.ID
		if mailID == "" {
			mailID = fmt.Sprintf("draft-%d", time.Now().UnixNano())
		}

		mail := &storage.Mail{
			ID:         mailID,
			UserEmail:  from,
			Folder:     "Drafts",
			From:       from,
			To:         req.To,
			Cc:         req.Cc,
			Bcc:        req.Bcc,
			Subject:    req.Subject,
			Body:       []byte(req.Body),
			Size:       int64(len(req.Body)),
			Flags:      []string{},
			ReceivedAt: time.Now(),
			CreatedAt:  time.Now(),
		}

		if err := driver.StoreMail(ctx, mail); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "保存草稿失败",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "草稿已保存",
			"id":      mailID,
		})
	}
}
