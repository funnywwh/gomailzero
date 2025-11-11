package imapd

import (
	"context"
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
		return nil, err
	}

	if !user.Active {
		return nil, fmt.Errorf("用户未激活")
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

	logger.Info().Str("username", username).Bool("totp_used", totpEnabled && totpCode != "").Msg("IMAP 用户认证成功")
	return user, nil
}
