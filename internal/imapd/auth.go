package imapd

import (
	"context"
	"fmt"

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
	storage storage.Driver
}

// NewDefaultAuthenticator 创建默认认证器
func NewDefaultAuthenticator(storage storage.Driver) *DefaultAuthenticator {
	return &DefaultAuthenticator{storage: storage}
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

	// 验证密码（使用 Argon2id）
	valid, err := crypto.VerifyPassword(password, user.PasswordHash)
	if err != nil {
		logger.Warn().Err(err).Str("username", username).Msg("密码验证失败")
		return nil, fmt.Errorf("认证失败")
	}
	if !valid {
		logger.Warn().Str("username", username).Msg("密码错误")
		return nil, fmt.Errorf("认证失败")
	}

	logger.Info().Str("username", username).Msg("IMAP 用户认证成功")
	return user, nil
}
