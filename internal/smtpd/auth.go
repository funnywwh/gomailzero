package smtpd

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

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
		logger.Warn().Str("username", username).Msg("用户不存在")
		return nil, fmt.Errorf("认证失败")
	}

	if !user.Active {
		logger.Warn().Str("username", username).Msg("用户未激活")
		return nil, fmt.Errorf("认证失败")
	}

	// TODO: 验证密码（使用 Argon2id）
	// 这里暂时跳过密码验证，后续实现加密模块时补充

	logger.Info().Str("username", username).Msg("用户认证成功")
	return user, nil
}

