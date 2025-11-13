package smtpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/gomailzero/gmz/internal/logger"
)

// Client SMTP 客户端
type Client struct {
	timeout  time.Duration
	hostname string // EHLO 主机名
}

// NewClient 创建 SMTP 客户端
// hostname 是 EHLO 命令使用的主机名，如果为空则从系统获取或使用邮箱域名
func NewClient(hostname string) *Client {
	// 如果没有提供 hostname，尝试从系统获取
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	// 如果系统主机名也不可用，使用默认值
	if hostname == "" {
		hostname = "localhost"
	}
	return &Client{
		timeout:  30 * time.Second,
		hostname: hostname,
	}
}

// getEHLOHostname 获取 EHLO 主机名
// 如果配置了 hostname 就使用，否则从邮箱地址提取域名
func (c *Client) getEHLOHostname(fromEmail string) string {
	// 如果配置了 hostname 且不是 localhost，使用配置的
	if c.hostname != "" && c.hostname != "localhost" {
		return c.hostname
	}
	// 否则从邮箱地址提取域名
	if parts := strings.Split(fromEmail, "@"); len(parts) == 2 {
		return parts[1]
	}
	// 最后的后备方案
	return c.hostname
}

// SendMail 发送邮件到外部服务器
func (c *Client) SendMail(ctx context.Context, from string, to []string, data []byte) error {
	if len(to) == 0 {
		return fmt.Errorf("没有收件人")
	}

	// 按域名分组收件人
	domainRecipients := make(map[string][]string)
	for _, recipient := range to {
		parts := strings.Split(recipient, "@")
		if len(parts) != 2 {
			logger.WarnCtx(ctx).Str("recipient", recipient).Msg("无效的邮箱地址")
			continue
		}
		domain := parts[1]
		domainRecipients[domain] = append(domainRecipients[domain], recipient)
	}

	// 为每个域名发送邮件
	var lastErr error
	for domain, recipients := range domainRecipients {
		if err := c.sendToDomain(ctx, from, domain, recipients, data); err != nil {
			logger.ErrorCtx(ctx).
				Err(err).
				Str("domain", domain).
				Strs("recipients", recipients).
				Msg("发送邮件到域名失败")
			lastErr = err
			// 继续尝试其他域名
		} else {
			logger.InfoCtx(ctx).
				Str("domain", domain).
				Strs("recipients", recipients).
				Msg("成功发送邮件到域名")
		}
	}

	return lastErr
}

// sendToDomain 发送邮件到指定域名的 MX 服务器
func (c *Client) sendToDomain(ctx context.Context, from, domain string, recipients []string, data []byte) error {
	// 查找 MX 记录
	mxRecords, err := net.LookupMX(domain)
	if err != nil {
		return fmt.Errorf("查找 MX 记录失败: %w", err)
	}

	if len(mxRecords) == 0 {
		return fmt.Errorf("域名 %s 没有 MX 记录", domain)
	}

	// 使用优先级最高的 MX 记录
	mxHost := strings.TrimSuffix(mxRecords[0].Host, ".")

	// 尝试连接到 MX 服务器（端口 25）
	addr := net.JoinHostPort(mxHost, "25")

	logger.DebugCtx(ctx).
		Str("domain", domain).
		Str("mx_host", mxHost).
		Str("addr", addr).
		Msg("连接到 MX 服务器")

	// 创建带超时的连接
	dialer := &net.Dialer{
		Timeout: c.timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("连接 MX 服务器失败: %w", err)
	}
	defer conn.Close()

	// 创建 SMTP 客户端
	client, err := smtp.NewClient(conn, mxHost)
	if err != nil {
		return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
	}
	defer client.Close()

	// EHLO（使用配置的主机名或从邮箱地址提取的域名）
	ehloHostname := c.getEHLOHostname(from)
	if err := client.Hello(ehloHostname); err != nil {
		return fmt.Errorf("EHLO 失败: %w", err)
	}

	// 检查是否支持 STARTTLS
	if ok, _ := client.Extension("STARTTLS"); ok {
		config := &tls.Config{
			ServerName: mxHost,
		}
		if err := client.StartTLS(config); err != nil {
			logger.WarnCtx(ctx).Err(err).Str("mx_host", mxHost).Msg("STARTTLS 失败，继续发送")
			// STARTTLS 失败不影响发送，继续
		}
	}

	// MAIL FROM
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM 失败: %w", err)
	}

	// RCPT TO
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			logger.WarnCtx(ctx).Err(err).Str("recipient", recipient).Msg("RCPT TO 失败")
			// 继续尝试其他收件人
			continue
		}
	}

	// DATA
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA 失败: %w", err)
	}

	// 写入邮件数据
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return fmt.Errorf("写入邮件数据失败: %w", err)
	}

	// 关闭 writer 完成发送
	if err := writer.Close(); err != nil {
		return fmt.Errorf("完成发送失败: %w", err)
	}

	// QUIT
	if err := client.Quit(); err != nil {
		logger.WarnCtx(ctx).Err(err).Msg("QUIT 失败")
		// QUIT 失败不影响邮件发送
	}

	return nil
}

// SendMailToRelay 通过中继服务器发送邮件（如果配置了中继服务器）
func (c *Client) SendMailToRelay(ctx context.Context, relayHost string, relayPort int, username, password string, useTLS bool, from string, to []string, data []byte) error {
	addr := fmt.Sprintf("%s:%d", relayHost, relayPort)

	logger.DebugCtx(ctx).
		Str("relay", addr).
		Str("from", from).
		Strs("to", to).
		Msg("通过中继服务器发送邮件")

	// 创建带超时的连接
	dialer := &net.Dialer{
		Timeout: c.timeout,
	}

	var conn net.Conn
	var err error

	// 如果使用 TLS，直接建立 TLS 连接
	if useTLS && relayPort == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			ServerName: relayHost,
		})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}

	if err != nil {
		return fmt.Errorf("连接中继服务器失败: %w", err)
	}
	defer conn.Close()

	// 创建 SMTP 客户端
	client, err := smtp.NewClient(conn, relayHost)
	if err != nil {
		return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
	}
	defer client.Close()

	// EHLO（使用配置的主机名或从邮箱地址提取的域名）
	ehloHostname := c.getEHLOHostname(from)
	if err := client.Hello(ehloHostname); err != nil {
		return fmt.Errorf("EHLO 失败: %w", err)
	}

	// 如果使用 TLS（端口 587），启动 STARTTLS
	if useTLS && relayPort != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			config := &tls.Config{
				ServerName: relayHost,
			}
			if err := client.StartTLS(config); err != nil {
				return fmt.Errorf("STARTTLS 失败: %w", err)
			}
			// STARTTLS 后需要重新发送 EHLO 以获取新的扩展列表
			if err := client.Hello(ehloHostname); err != nil {
				return fmt.Errorf("STARTTLS 后 EHLO 失败: %w", err)
			}
		}
	}

	// 认证（如果提供了用户名和密码）
	if username != "" && password != "" {
		// 检查服务器是否支持 AUTH 扩展（STARTTLS 后需要重新检查）
		supportsAuth, authMethods := client.Extension("AUTH")
		if !supportsAuth {
			logger.WarnCtx(ctx).Msg("中继服务器不支持 AUTH 扩展，跳过认证")
		} else {
			logger.DebugCtx(ctx).Str("auth_methods", authMethods).Msg("中继服务器支持的认证方式")

			// 优先使用 PLAIN 认证（最常见）
			// 如果服务器不支持 PLAIN，尝试其他方式
			var auth smtp.Auth
			if strings.Contains(authMethods, "PLAIN") {
				auth = smtp.PlainAuth("", username, password, relayHost)
			} else if strings.Contains(authMethods, "LOGIN") {
				// 如果服务器只支持 LOGIN，使用 PLAIN 作为后备（LOGIN 是 PLAIN 的变体）
				auth = smtp.PlainAuth("", username, password, relayHost)
			} else {
				// 如果都不支持，尝试使用 PLAIN（某些服务器可能支持但不声明）
				logger.WarnCtx(ctx).Str("supported_auths", authMethods).Msg("服务器不支持 PLAIN 或 LOGIN，尝试使用 PLAIN")
				auth = smtp.PlainAuth("", username, password, relayHost)
			}

			if err := client.Auth(auth); err != nil {
				// 提供更详细的错误信息，帮助排查认证问题
				return fmt.Errorf("SMTP 认证失败 (服务器支持的认证方式: %s): %w", authMethods, err)
			}
			logger.DebugCtx(ctx).Str("username", username).Msg("SMTP 认证成功")
		}
	}

	// MAIL FROM
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM 失败: %w", err)
	}

	// RCPT TO
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("RCPT TO 失败 (%s): %w", recipient, err)
		}
	}

	// DATA
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA 失败: %w", err)
	}

	// 写入邮件数据
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return fmt.Errorf("写入邮件数据失败: %w", err)
	}

	// 关闭 writer 完成发送
	if err := writer.Close(); err != nil {
		return fmt.Errorf("完成发送失败: %w", err)
	}

	// QUIT
	if err := client.Quit(); err != nil {
		logger.WarnCtx(ctx).Err(err).Msg("QUIT 失败")
	}

	return nil
}
