package imapd

import (
	"context"
	"fmt"

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

	// TODO: 验证密码（使用 Argon2id）
	// 这里暂时跳过密码验证，后续实现加密模块时补充

	return user, nil
}

