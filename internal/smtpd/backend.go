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
	parts := strings.Split(to, "@")
	if len(parts) != 2 {
		return fmt.Errorf("无效的邮箱地址: %s", to)
	}
	domain := parts[1]

	// 检查域名是否存在
	ctx := context.Background()
	_, err := s.backend.storage.GetDomain(ctx, domain)
	if err != nil {
		// 域名不存在，拒绝中继
		return fmt.Errorf("550 Relay denied: domain not found")
	}

	// 检查用户或别名是否存在
	_, err = s.backend.storage.GetUser(ctx, to)
	if err != nil {
		// 检查别名
		_, err = s.backend.storage.GetAlias(ctx, to)
		if err != nil {
			return fmt.Errorf("550 Relay denied: recipient not found")
		}
	}

	s.recipients = append(s.recipients, to)
	logger.Debug().Str("to", to).Msg("RCPT TO")
	return nil
}

// Data 接收邮件数据
func (s *Session) Data(r io.Reader) error {
	// 先读取完整的原始邮件数据
	rawData, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("读取邮件数据失败: %w", err)
	}

	// 尝试解析邮件
	msg, err := message.Read(bytes.NewReader(rawData))
	if err != nil {
		// 如果解析失败，可能是缺少邮件头，需要重新构建
		logger.Warn().Err(err).Msg("邮件解析失败，尝试重新构建邮件头")
	}

	// 解析邮件头
	var fromHeader, to, subject string
	var hasHeaders bool
	if msg != nil {
		header := msg.Header
		fromHeader = header.Get("From")
		to = header.Get("To")
		subject = header.Get("Subject")
		// 检查是否有基本的邮件头
		hasHeaders = fromHeader != "" || to != "" || subject != "" || header.Get("Date") != "" || header.Get("Message-ID") != ""
	}

	// 如果邮件缺少邮件头，需要重新构建
	if !hasHeaders || strings.HasPrefix(strings.TrimSpace(string(rawData)), "This is a multi-part message in MIME format.") {
		logger.Info().Msg("检测到缺少邮件头的邮件，重新构建完整邮件")
		
		// 读取邮件体（如果解析成功）
		var bodyData []byte
		if msg != nil {
			bodyData, _ = io.ReadAll(msg.Body)
		} else {
			// 如果解析失败，使用原始数据作为邮件体
			bodyData = rawData
		}
		
		// 重新构建完整的邮件（包含邮件头）
		rawData = s.buildCompleteEmail(fromHeader, to, subject, bodyData)
		
		// 重新解析邮件以获取正确的邮件头
		msg, err = message.Read(bytes.NewReader(rawData))
		if err != nil {
			logger.Warn().Err(err).Msg("重新构建后的邮件解析失败")
		} else {
			header := msg.Header
			fromHeader = header.Get("From")
			to = header.Get("To")
			subject = header.Get("Subject")
		}
	}

	// 优先使用邮件头中的 From 字段，如果为空或无效才使用 MAIL FROM 命令的值
	fromAddr := fromHeader
	if fromAddr == "" || fromAddr == "<>" {
		fromAddr = s.from
	}
	// 如果仍然为空，使用默认值
	if fromAddr == "" || fromAddr == "<>" {
		fromAddr = "unknown@unknown"
	}
	
	// 清理 From 地址：去除可能的引号和尖括号
	fromAddr = strings.TrimSpace(fromAddr)
	// 如果包含 < >，提取邮箱地址
	if idx := strings.Index(fromAddr, "<"); idx >= 0 {
		if idx2 := strings.Index(fromAddr, ">"); idx2 > idx {
			fromAddr = fromAddr[idx+1 : idx2]
		}
	}
	// 去除引号
	fromAddr = strings.Trim(fromAddr, "\"")
	fromAddr = strings.TrimSpace(fromAddr)
	
	// 如果清理后仍然为空，使用默认值
	if fromAddr == "" || fromAddr == "<>" {
		fromAddr = "unknown@unknown"
	}

	// 如果 subject 为空，设置默认值
	if subject == "" {
		subject = "(无主题)"
	}

	logger.Info().
		Str("from_header", fromHeader).
		Str("from_mail", s.from).
		Str("from_final", fromAddr).
		Str("to", to).
		Str("subject", subject).
		Int("raw_size", len(rawData)).
		Msg("接收邮件")

	// 存储邮件
	ctx := context.Background()
	for _, recipient := range s.recipients {
		// 获取用户
		user, err := s.backend.storage.GetUser(ctx, recipient)
		if err != nil {
			// 检查别名
			alias, err := s.backend.storage.GetAlias(ctx, recipient)
			if err != nil {
				logger.Warn().Str("recipient", recipient).Msg("收件人不存在")
				continue
			}
			user, err = s.backend.storage.GetUser(ctx, alias.To)
			if err != nil {
				logger.Warn().Str("recipient", recipient).Msg("别名目标用户不存在")
				continue
			}
		}

		// 确保用户 Maildir 目录存在
		if err := s.backend.maildir.EnsureUserMaildir(user.Email); err != nil {
			logger.Error().Err(err).Str("user", user.Email).Msg("创建用户 Maildir 失败")
			continue
		}

		// 存储完整的邮件到 Maildir（包含邮件头）
		filename, err := s.backend.maildir.StoreMail(user.Email, "INBOX", rawData)
		if err != nil {
			logger.Error().Err(err).Str("user", user.Email).Msg("存储邮件到 Maildir 失败")
			continue
		}

		// 存储邮件元数据到数据库
		mail := &storage.Mail{
			ID:         filename,
			UserEmail:  user.Email,
			Folder:     "INBOX",
			From:       fromAddr, // 使用解析后的发件人地址
			To:         []string{recipient},
			Subject:    subject,
			Size:       int64(len(rawData)),
			Flags:      []string{"\\Recent"}, // 新邮件设置 \Recent 标志
			ReceivedAt: time.Now(),
		}

		if err := s.backend.storage.StoreMail(ctx, mail); err != nil {
			logger.Error().Err(err).Str("user", user.Email).Msg("存储邮件元数据失败")
			// 继续处理其他收件人
			continue
		}

		logger.Info().
			Str("recipient", recipient).
			Str("filename", filename).
			Int("size", len(rawData)).
			Msg("邮件存储成功")
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
			rand.Read(randomBytes)
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
	rand.Read(randomBytes)
	random := hex.EncodeToString(randomBytes)
	
	// 获取主机名
	hostname := "localhost"
	if s.backend.maildir != nil {
		// 尝试从配置中获取域名
		// 这里简化处理，使用 localhost
	}
	
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("<%d.%s@%s>", timestamp, random, hostname)
}

// Logout 登出
func (s *Session) Logout() error {
	return nil
}
