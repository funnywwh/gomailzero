package web

import (
	"bytes"
	"fmt"
	"mime"
	"strings"
	"time"

	"github.com/gomailzero/gmz/internal/antispam"
	"github.com/gomailzero/gmz/internal/logger"
)

// formatEmailAddress 格式化邮件地址（支持显示名称）
// 如果 displayName 为空，只返回邮箱地址
// 如果 displayName 不为空，返回 "Display Name <email@domain.com>" 格式
func formatEmailAddress(email, displayName string) string {
	if displayName == "" {
		return email
	}
	// 对显示名称进行编码（处理特殊字符和非 ASCII 字符）
	encodedName := mime.QEncoding.Encode("UTF-8", displayName)
	return fmt.Sprintf("%s <%s>", encodedName, email)
}

// buildMailMessage 构建邮件消息（包含 DKIM 签名）
// fromDisplayName 是可选的显示名称，如果为空则只使用邮箱地址
func buildMailMessage(from, fromDisplayName string, to, cc, bcc []string, subject, body string, dkim *antispam.DKIM) ([]byte, error) {
	var buf bytes.Buffer

	// 生成 Message-ID
	messageID := generateMessageID(from)

	// 构建邮件头
	headers := make(map[string]string)
	headers["From"] = formatEmailAddress(from, fromDisplayName)
	headers["To"] = strings.Join(to, ", ")
	if len(cc) > 0 {
		headers["Cc"] = strings.Join(cc, ", ")
	}
	if len(bcc) > 0 {
		headers["Bcc"] = strings.Join(bcc, ", ")
	}
	headers["Subject"] = subject
	headers["Date"] = time.Now().Format(time.RFC1123Z)
	headers["Message-ID"] = messageID
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=UTF-8"

	// 如果启用了 DKIM，添加签名
	if dkim != nil {
		dkimHeader, err := dkim.Sign(headers, []byte(body))
		if err != nil {
			// 注意：这里没有 context，使用普通 logger
			logger.Warn().Err(err).Msg("DKIM 签名失败，继续发送未签名的邮件")
		} else {
			headers["DKIM-Signature"] = dkimHeader
		}
	}

	// 写入邮件头
	for key, value := range headers {
		buf.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}

	// 空行分隔头和正文
	buf.WriteString("\r\n")

	// 写入邮件正文
	buf.WriteString(body)

	return buf.Bytes(), nil
}

// generateMessageID 生成 Message-ID
func generateMessageID(from string) string {
	// 格式: <timestamp.random@domain>
	domain := "localhost"
	if parts := strings.Split(from, "@"); len(parts) == 2 {
		domain = parts[1]
	}
	timestamp := time.Now().UnixNano()
	random := fmt.Sprintf("%x", timestamp%1000000)
	return fmt.Sprintf("<%d.%s@%s>", timestamp, random, domain)
}
