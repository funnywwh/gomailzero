package smtpd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-smtp"
	"github.com/gomailzero/gmz/internal/logger"
	"github.com/gomailzero/gmz/internal/storage"
)

// Backend SMTP 后端
type Backend struct {
	storage storage.Driver
	maildir *storage.Maildir
	auth    Authenticator
}

// NewBackend 创建后端
func NewBackend(storage storage.Driver, maildir *storage.Maildir, auth Authenticator) *Backend {
	return &Backend{
		storage: storage,
		maildir: maildir,
		auth:    auth,
	}
}

// NewSession 创建新会话
func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{
		backend: b,
		conn:    c,
	}, nil
}

// Session SMTP 会话
type Session struct {
	backend    *Backend
	conn       *smtp.Conn
	from       string
	recipients []string
}

// Auth 认证（在 Session 中不需要实现，由 Server 处理）

// Mail 设置发件人
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	logger.Debug().Str("from", from).Msg("MAIL FROM")
	return nil
}

// Rcpt 设置收件人（检查中继）
func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	// 提取域名
	parts := strings.Split(to, "@")[1]

	// 检查域名是否存在
	ctx := context.Background()
	_, err := s.backend.storage.GetDomain(ctx, parts)
	if err != nil {
		return fmt.Errorf("无效的邮箱地址: %s", to)
	}

	s.recipients = append(s.recipients, to)
	logger.Debug().Str("to", to).Msg("RCPT TO")
	return nil
}

// Data 接收邮件数据
func (s *Session) Data(r io.Reader) error {
	// 限制读取大小以防 OOM
	const MaxMailSize = 50 * 1024 * 1024 // 50 MiB
	limited := io.LimitReader(r, MaxMailSize+1)
	rawData, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("读取邮件数据失败: %w", err)
	}
	if int64(len(rawData)) > MaxMailSize {
		logger.Warn().Int("size", len(rawData)).Msg("邮件超过允许大小，拒绝接收")
		return fmt.Errorf("552 Message size exceeds fixed maximum message size")
	}

	// 尝试解析邮件
	msg, err := message.Read(bytes.NewReader(rawData))
	if err != nil {
		previewLen := 1024
		logger.Warn().Err(err).Hex("preview", rawData[:previewLen]).Msg("邮件解析失败，尝试重新构建邮件头")
	}

	// 解析邮件头
	var fromHeader, to, subject string
	var hasHeaders bool
	if msg != nil {
		header := msg.Header
		fromHeader = header.Get("From")
		to = header.Get("To")
		subject = header.Get("Subject")
		hasHeaders = fromHeader != "" || to != "" || subject != "" || header.Get("Date") != "" || header.Get("Message-ID") != ""
	}

	// 如果邮件缺少邮件头，重新构建完整的邮件
	if !hasHeaders {
		// 使用 buildCompleteEmail 重新构建邮件
		completeEmail := s.buildCompleteEmail(fromHeader, to, subject, rawData)
		rawData = completeEmail
		logger.Debug().Msg("邮件缺少邮件头，已重新构建完整邮件")
	}

	// 存储邮件到 Maildir
	ctx := context.Background()
	for _, recipient := range s.recipients {
		// 提取用户邮箱（去除显示名称）
		userEmail := recipient
		if idx := strings.Index(recipient, "<"); idx >= 0 {
			if idx2 := strings.Index(recipient, ">"); idx2 > idx {
				userEmail = recipient[idx+1 : idx2]
			}
		}
		userEmail = strings.TrimSpace(userEmail)

		// 存储到 Maildir
		if s.backend.maildir != nil {
			if err := s.backend.maildir.EnsureUserMaildir(userEmail); err != nil {
				logger.Warn().Err(err).Str("user", userEmail).Msg("创建用户 Maildir 失败")
				continue
			}
			filename, err := s.backend.maildir.StoreMail(userEmail, "INBOX", rawData)
			if err != nil {
				logger.Warn().Err(err).Str("user", userEmail).Msg("存储邮件到 Maildir 失败")
				continue
			}

			// 解析邮件头以获取元数据
			msg, err := message.Read(bytes.NewReader(rawData))
			if err != nil {
				logger.Warn().Err(err).Str("user", userEmail).Msg("解析邮件失败")
				continue
			}

			header := msg.Header
			from := header.Get("From")
			toStr := header.Get("To")
			subject := header.Get("Subject")

			// 解析收件人列表
			var toList []string
			if toStr != "" {
				toList = []string{toStr}
			} else {
				toList = []string{userEmail}
			}

			// 存储邮件元数据到数据库
			mail := &storage.Mail{
				ID:         filename,
				UserEmail:  userEmail,
				Folder:     "INBOX",
				From:       from,
				To:         toList,
				Subject:    subject,
				Size:       int64(len(rawData)),
				Flags:      []string{"\\Recent"},
				ReceivedAt: time.Now(),
				CreatedAt:  time.Now(),
			}

			if err := s.backend.storage.StoreMail(ctx, mail); err != nil {
				logger.Warn().Err(err).Str("user", userEmail).Msg("存储邮件元数据失败")
			} else {
				logger.Info().
					Str("user", userEmail).
					Str("from", from).
					Str("subject", subject).
					Msg("邮件已存储")
			}
		}
	}

	return nil
}

// Reset 重置会话
func (s *Session) Reset() {
	s.from = ""
	s.recipients = nil
}

// buildCompleteEmail 构建完整的邮件（包含邮件头）
func (s *Session) buildCompleteEmail(fromHeader, to, subject string, body []byte) []byte {
	var buf bytes.Buffer
	
	// 生成 Message-ID
	messageID := s.generateMessageID()
	
	// 获取当前时间（RFC 822 格式）
	now := time.Now()
	dateStr := now.Format(time.RFC1123Z)
	
	// 构建邮件头
	// From
	if fromHeader == "" || fromHeader == "<>" {
		if s.from != "" && s.from != "<>" {
			fromHeader = s.from
		} else {
			fromHeader = "unknown@unknown"
		}
	}
	// 清理 From 地址
	fromAddr := strings.TrimSpace(fromHeader)
	if idx := strings.Index(fromAddr, "<"); idx >= 0 {
		if idx2 := strings.Index(fromAddr, ">"); idx2 > idx {
			fromAddr = fromAddr[idx+1 : idx2]
		}
	}
	fromAddr = strings.Trim(fromAddr, "\"")
	fromAddr = strings.TrimSpace(fromAddr)
	if fromAddr == "" || fromAddr == "<>" {
		fromAddr = "unknown@unknown"
	}
	
	// To（使用第一个收件人）
	toAddr := to
	if toAddr == "" && len(s.recipients) > 0 {
		toAddr = s.recipients[0]
	}
	if toAddr == "" {
		toAddr = "unknown@unknown"
	}
	
	// Subject
	if subject == "" {
		subject = "(无主题)"
	}
	
	// 写入邮件头
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", dateStr))
	buf.WriteString(fmt.Sprintf("Message-ID: %s\r\n", messageID))
	buf.WriteString(fmt.Sprintf("From: %s\r\n", fromAddr))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", toAddr))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	
	// 检查邮件体是否已经是 MIME 格式
	bodyStr := string(body)
	if strings.HasPrefix(strings.TrimSpace(bodyStr), "This is a multi-part message in MIME format.") {
		// 已经是 MIME 格式，直接添加 Content-Type
		buf.WriteString("Content-Type: multipart/alternative; boundary=\"")
		// 尝试从邮件体中提取 boundary
		// 格式通常是: ------=_001_NextPart111350263035_=----
		// boundary 值是: _001_NextPart111350263035_=
		lines := strings.Split(bodyStr, "\n")
		boundaryFound := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// 匹配格式: ------=<boundary>=----
			if strings.HasPrefix(line, "------=") && strings.Contains(line, "=") && strings.HasSuffix(line, "----") {
				// 提取 boundary 值
				// 去掉开头的 "------=" 和结尾的 "----"
				boundary := strings.TrimPrefix(line, "------=")
				boundary = strings.TrimSuffix(boundary, "----")
				// 确保 boundary 不为空
				if boundary != "" {
					buf.WriteString(boundary)
					boundaryFound = true
					break
				}
			}
		}
		// 如果没找到 boundary，生成一个
		if !boundaryFound {
			randomBytes := make([]byte, 8)
			if _, err := rand.Read(randomBytes); err != nil { // #nosec G104 -- 随机数生成失败不影响功能
				// 如果随机数生成失败，使用时间戳作为后备
				randomBytes = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
			}
			random := hex.EncodeToString(randomBytes)
			buf.WriteString(fmt.Sprintf("_%s_", random))
		}
		buf.WriteString("\"\r\n")
	} else {
		// 普通文本，添加 Content-Type
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	}
	
	// 空行分隔邮件头和邮件体
	buf.WriteString("\r\n")
	
	// 写入邮件体
	buf.Write(body)
	
	return buf.Bytes()
}

// generateMessageID 生成 Message-ID
func (s *Session) generateMessageID() string {
	// 生成随机数
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil { // #nosec G104 -- 随机数生成失败不影响功能
		// 如果随机数生成失败，使用时间戳作为后备
		randomBytes = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
	}
	random := hex.EncodeToString(randomBytes)
	
	// 获取主机名
	hostname := "localhost"
	// 如果将来需要从 maildir 配置中获取域名，可以在这里添加逻辑
	_ = s.backend.maildir // 避免未使用变量警告
	
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("<%d.%s@%s>", timestamp, random, hostname)
}

// Logout 登出
func (s *Session) Logout() error {
	return nil
}
