package smtpd

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/gomailzero/gmz/internal/auth"
	"github.com/gomailzero/gmz/internal/crypto"
	"github.com/gomailzero/gmz/internal/logger"
	"github.com/gomailzero/gmz/internal/storage"
)

// Authenticator 认证接口
type Authenticator interface {
	Authenticate(ctx context.Context, username, password string) (*storage.User, error)
}

// PlainAuth PLAIN 认证机制
type PlainAuth struct {
	backend *Backend
}

// Start 开始认证
func (a *PlainAuth) Start() (mech string, ir []byte, err error) {
	return "PLAIN", nil, nil
}

// Next 继续认证
func (a *PlainAuth) Next(fromServer []byte) (toServer []byte, more bool, err error) {
	// 解码客户端响应
	decoded, err := base64.StdEncoding.DecodeString(string(fromServer))
	if err != nil {
		return nil, false, fmt.Errorf("解码认证信息失败: %w", err)
	}

	parts := strings.Split(string(decoded), "\x00")
	if len(parts) != 3 {
		return nil, false, fmt.Errorf("无效的认证信息格式")
	}

	username := parts[1]
	password := parts[2]

	// 执行认证
	ctx := context.Background()
	_, err = a.backend.auth.Authenticate(ctx, username, password)
	if err != nil {
		return nil, false, fmt.Errorf("认证失败: %w", err)
	}

	return nil, false, nil
}

// Authenticate 执行认证（用于直接调用）
func (a *PlainAuth) Authenticate(username, password string) (*storage.User, error) {
	ctx := context.Background()
	return a.backend.auth.Authenticate(ctx, username, password)
}

// LoginAuth LOGIN 认证机制
type LoginAuth struct {
	backend *Backend
}

// Start 开始认证
func (l *LoginAuth) Start() (string, []byte, error) {
	return "LOGIN", []byte("Username:"), nil
}

// Next 继续认证
func (l *LoginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	// 解码服务器挑战
	challenge, err := base64.StdEncoding.DecodeString(string(fromServer))
	if err != nil {
		return nil, fmt.Errorf("解码挑战失败: %w", err)
	}

	challengeStr := string(challenge)
	if strings.HasPrefix(challengeStr, "Username:") {
		// 请求用户名
		return []byte("Username:"), nil
	} else if strings.HasPrefix(challengeStr, "Password:") {
		// 请求密码
		return []byte("Password:"), nil
	}

	return nil, fmt.Errorf("未知的挑战: %s", challengeStr)
}

// Authenticate 执行认证
func (l *LoginAuth) Authenticate(username, password string) (*storage.User, error) {
	ctx := context.Background()
	return l.backend.auth.Authenticate(ctx, username, password)
}

// DefaultAuthenticator 默认认证器
type DefaultAuthenticator struct {
	storage     storage.Driver
	totpManager *auth.TOTPManager
}

// NewDefaultAuthenticator 创建默认认证器
func NewDefaultAuthenticator(storage storage.Driver) *DefaultAuthenticator {
	return &DefaultAuthenticator{
		storage:     storage,
		totpManager: auth.NewTOTPManager(storage),
	}
}

// Authenticate 认证用户
func (a *DefaultAuthenticator) Authenticate(ctx context.Context, username, password string) (*storage.User, error) {
	user, err := a.storage.GetUser(ctx, username)
	if err != nil {
		logger.Warn().Str("username", username).Msg("用户不存在")
		return nil, fmt.Errorf("认证失败")
	}

	if !user.Active {
		logger.Warn().Str("username", username).Msg("用户未激活")
		return nil, fmt.Errorf("认证失败")
	}

	// 解析密码和 TOTP 代码（格式：password 或 password:TOTP_CODE）
	actualPassword := password
	totpCode := ""
	if strings.Contains(password, ":") {
		parts := strings.SplitN(password, ":", 2)
		if len(parts) == 2 {
			actualPassword = parts[0]
			totpCode = parts[1]
		}
	}

	// 验证密码（使用 Argon2id）
	valid, err := crypto.VerifyPassword(actualPassword, user.PasswordHash)
	if err != nil {
		logger.Warn().Err(err).Str("username", username).Msg("密码验证失败")
		return nil, fmt.Errorf("认证失败")
	}
	if !valid {
		logger.Warn().Str("username", username).Msg("密码错误")
		return nil, fmt.Errorf("认证失败")
	}

	// 检查是否启用了 TOTP
	totpEnabled, err := a.totpManager.IsEnabled(ctx, username)
	if err != nil {
		logger.Warn().Err(err).Str("username", username).Msg("检查 TOTP 状态失败")
		// 如果检查失败，继续认证（不强制 TOTP）
	} else if totpEnabled {
		// 如果启用了 TOTP，必须提供 TOTP 代码
		if totpCode == "" {
			logger.Warn().Str("username", username).Msg("用户启用了 TOTP，但未提供 TOTP 代码")
			return nil, fmt.Errorf("需要 TOTP 代码")
		}

		// 验证 TOTP 代码
		valid, err := a.totpManager.Verify(ctx, username, totpCode)
		if err != nil {
			logger.Warn().Err(err).Str("username", username).Msg("TOTP 验证失败")
			return nil, fmt.Errorf("TOTP 验证失败")
		}
		if !valid {
			logger.Warn().Str("username", username).Msg("TOTP 代码错误")
			return nil, fmt.Errorf("TOTP 代码错误")
		}
	}

	logger.Info().Str("username", username).Bool("totp_used", totpEnabled && totpCode != "").Msg("用户认证成功")
	return user, nil
}
