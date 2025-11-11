package smtpd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-message"
	"github.com/emersion/go-smtp"
	"github.com/gomailzero/gmz/internal/logger"
	"github.com/gomailzero/gmz/internal/storage"
)

// Backend SMTP 后端
type Backend struct {
	storage storage.Driver
	auth    Authenticator
}

// NewBackend 创建后端
func NewBackend(storage storage.Driver, auth Authenticator) *Backend {
	return &Backend{
		storage: storage,
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
	backend   *Backend
	conn      *smtp.Conn
	user      *storage.User
	from      string
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
	// 读取邮件
	msg, err := message.Read(r)
	if err != nil {
		return fmt.Errorf("读取邮件失败: %w", err)
	}

	// 解析邮件头
	header := msg.Header
	from := header.Get("From")
	to := header.Get("To")
	subject := header.Get("Subject")

	logger.Info().
		Str("from", from).
		Str("to", to).
		Str("subject", subject).
		Msg("接收邮件")

	// 读取邮件体
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return fmt.Errorf("读取邮件体失败: %w", err)
	}

	// 存储邮件
	ctx := context.Background()
	for _, recipient := range s.recipients {
		// 获取用户
		_, err := s.backend.storage.GetUser(ctx, recipient)
		if err != nil {
			// 检查别名
			alias, err := s.backend.storage.GetAlias(ctx, recipient)
			if err != nil {
				logger.Warn().Str("recipient", recipient).Msg("收件人不存在")
				continue
			}
			_, err = s.backend.storage.GetUser(ctx, alias.To)
			if err != nil {
				logger.Warn().Str("recipient", recipient).Msg("别名目标用户不存在")
				continue
			}
		}

		// TODO: 存储邮件到 Maildir
		// TODO: 更新数据库元数据
		logger.Info().
			Str("recipient", recipient).
			Int("size", len(body)).
			Msg("存储邮件")
	}

	return nil
}

// Reset 重置会话
func (s *Session) Reset() {
	s.from = ""
	s.recipients = nil
}

// Logout 登出
func (s *Session) Logout() error {
	return nil
}

