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

	// 如果邮件缺少邮件头...
	...
	return nil
}

// Reset 重置会话
func (s *Session) Reset() {
	s.from = ""
	s.recipients = nil
}