package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gomailzero/gmz/internal/logger"
	"github.com/gomailzero/gmz/internal/storage"
)

// WebAuthnManager WebAuthn 管理器
type WebAuthnManager struct {
	webauthn *webauthn.WebAuthn
	storage  storage.Driver
}

// WebAuthnConfig WebAuthn 配置
type WebAuthnConfig struct {
	RPID          string // Relying Party ID（通常是域名）
	RPOrigin      string // Relying Party Origin（例如：https://mail.example.com）
	RPDisplayName string // Relying Party Display Name
}

// NewWebAuthnManager 创建 WebAuthn 管理器
func NewWebAuthnManager(cfg WebAuthnConfig, storage storage.Driver) (*WebAuthnManager, error) {
	// 解析 Origin URL
	originURL, err := url.Parse(cfg.RPOrigin)
	if err != nil {
		return nil, fmt.Errorf("解析 RPOrigin 失败: %w", err)
	}

	// 创建 WebAuthn 实例
	w, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.RPDisplayName,
		RPID:          cfg.RPID,
		RPOrigins:    []string{originURL.String()},
		// 使用默认的挑战超时时间（60秒）
		// 使用默认的认证器选择器
	})
	if err != nil {
		return nil, fmt.Errorf("创建 WebAuthn 实例失败: %w", err)
	}

	return &WebAuthnManager{
		webauthn: w,
		storage:  storage,
	}, nil
}

// WebAuthnUser WebAuthn 用户接口实现
type WebAuthnUser struct {
	ID          []byte
	Email       string
	Credentials []webauthn.Credential
}

// WebAuthnID 返回用户的 WebAuthn ID（通常是用户 ID 的字节表示）
func (u *WebAuthnUser) WebAuthnID() []byte {
	return u.ID
}

// WebAuthnName 返回用户的显示名称
func (u *WebAuthnUser) WebAuthnName() string {
	return u.Email
}

// WebAuthnDisplayName 返回用户的显示名称
func (u *WebAuthnUser) WebAuthnDisplayName() string {
	return u.Email
}

// WebAuthnIcon 返回用户的图标 URL（可选）
func (u *WebAuthnUser) WebAuthnIcon() string {
	return ""
}

// WebAuthnCredentials 返回用户的 WebAuthn 凭证列表
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

// BeginRegistration 开始注册流程
func (m *WebAuthnManager) BeginRegistration(ctx context.Context, userEmail string) (*webauthn.SessionData, *protocol.CredentialCreation, error) {
	// 获取用户
	user, err := m.storage.GetUser(ctx, userEmail)
	if err != nil {
		return nil, nil, fmt.Errorf("获取用户失败: %w", err)
	}

	// 加载用户的现有凭证
	credentials, err := m.loadCredentials(ctx, userEmail)
	if err != nil {
		logger.Warn().Err(err).Str("user", userEmail).Msg("加载 WebAuthn 凭证失败，继续注册")
		credentials = []webauthn.Credential{}
	}

	// 创建 WebAuthn 用户
	webauthnUser := &WebAuthnUser{
		ID:          []byte(fmt.Sprintf("%d", user.ID)),
		Email:       user.Email,
		Credentials: credentials,
	}

	// 开始注册
	options, session, err := m.webauthn.BeginRegistration(webauthnUser)
	if err != nil {
		return nil, nil, fmt.Errorf("开始注册失败: %w", err)
	}

	// TODO: 保存 session 到临时存储（Redis 或内存缓存）
	// 这里简化实现，返回 session 给调用者保存

	return session, options, nil
}

// FinishRegistration 完成注册流程
func (m *WebAuthnManager) FinishRegistration(ctx context.Context, userEmail string, session *webauthn.SessionData, response *protocol.ParsedCredentialCreationData) (*webauthn.Credential, error) {
	// 获取用户
	user, err := m.storage.GetUser(ctx, userEmail)
	if err != nil {
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}

	// 创建 WebAuthn 用户
	webauthnUser := &WebAuthnUser{
		ID:          []byte(fmt.Sprintf("%d", user.ID)),
		Email:       user.Email,
		Credentials: []webauthn.Credential{}, // 注册时还没有凭证
	}

	// 完成注册
	// 注意：FinishRegistration 需要 *http.Request，这里需要调用 CreateCredential
	credential, err := m.webauthn.CreateCredential(webauthnUser, *session, response)
	if err != nil {
		return nil, fmt.Errorf("完成注册失败: %w", err)
	}

	// 保存凭证到数据库
	if err := m.saveCredential(ctx, userEmail, credential); err != nil {
		return nil, fmt.Errorf("保存凭证失败: %w", err)
	}

	logger.Info().Str("user", userEmail).Msg("WebAuthn 凭证注册成功")
	return credential, nil
}

// BeginLogin 开始登录流程
func (m *WebAuthnManager) BeginLogin(ctx context.Context, userEmail string) (*webauthn.SessionData, *protocol.CredentialAssertion, error) {
	// 获取用户
	user, err := m.storage.GetUser(ctx, userEmail)
	if err != nil {
		return nil, nil, fmt.Errorf("获取用户失败: %w", err)
	}

	// 加载用户的凭证
	credentials, err := m.loadCredentials(ctx, userEmail)
	if err != nil {
		return nil, nil, fmt.Errorf("加载凭证失败: %w", err)
	}

	if len(credentials) == 0 {
		return nil, nil, fmt.Errorf("用户未注册 WebAuthn 凭证")
	}

	// 创建 WebAuthn 用户
	webauthnUser := &WebAuthnUser{
		ID:          []byte(fmt.Sprintf("%d", user.ID)),
		Email:       user.Email,
		Credentials: credentials,
	}

	// 开始登录
	options, session, err := m.webauthn.BeginLogin(webauthnUser)
	if err != nil {
		return nil, nil, fmt.Errorf("开始登录失败: %w", err)
	}

	// TODO: 保存 session 到临时存储

	return session, options, nil
}

// FinishLogin 完成登录流程
func (m *WebAuthnManager) FinishLogin(ctx context.Context, userEmail string, session *webauthn.SessionData, response *protocol.ParsedCredentialAssertionData) (*webauthn.Credential, error) {
	// 获取用户
	user, err := m.storage.GetUser(ctx, userEmail)
	if err != nil {
		return nil, fmt.Errorf("获取用户失败: %w", err)
	}

	// 加载用户的凭证
	credentials, err := m.loadCredentials(ctx, userEmail)
	if err != nil {
		return nil, fmt.Errorf("加载凭证失败: %w", err)
	}

	// 创建 WebAuthn 用户
	webauthnUser := &WebAuthnUser{
		ID:          []byte(fmt.Sprintf("%d", user.ID)),
		Email:       user.Email,
		Credentials: credentials,
	}

	// 完成登录
	// 注意：FinishLogin 需要 *http.Request，这里需要调用 ValidateLogin
	credential, err := m.webauthn.ValidateLogin(webauthnUser, *session, response)
	if err != nil {
		return nil, fmt.Errorf("完成登录失败: %w", err)
	}

	// 更新凭证的签名计数
	if err := m.updateCredential(ctx, userEmail, credential); err != nil {
		logger.Warn().Err(err).Str("user", userEmail).Msg("更新凭证签名计数失败")
	}

	logger.Info().Str("user", userEmail).Msg("WebAuthn 登录成功")
	return credential, nil
}

// loadCredentials 从数据库加载用户的 WebAuthn 凭证
func (m *WebAuthnManager) loadCredentials(ctx context.Context, userEmail string) ([]webauthn.Credential, error) {
	// TODO: 从数据库加载凭证
	// 当前简化实现，返回空列表
	// 需要实现：
	// 1. 在 storage 接口中添加 GetWebAuthnCredentials 方法
	// 2. 在 SQLite 驱动中实现该方法
	// 3. 从数据库读取并反序列化凭证

	return []webauthn.Credential{}, nil
}

// saveCredential 保存凭证到数据库
func (m *WebAuthnManager) saveCredential(ctx context.Context, userEmail string, credential *webauthn.Credential) error {
	// TODO: 保存凭证到数据库
	// 需要实现：
	// 1. 在 storage 接口中添加 SaveWebAuthnCredential 方法
	// 2. 在 SQLite 驱动中实现该方法
	// 3. 序列化凭证并保存到数据库

	// 临时实现：记录日志
	credJSON, _ := json.Marshal(credential)
	logger.Info().
		Str("user", userEmail).
		RawJSON("credential", credJSON).
		Msg("WebAuthn 凭证（待保存到数据库）")

	return nil
}

// updateCredential 更新凭证（主要是签名计数）
func (m *WebAuthnManager) updateCredential(ctx context.Context, userEmail string, credential *webauthn.Credential) error {
	// TODO: 更新凭证到数据库
	// 需要实现：
	// 1. 在 storage 接口中添加 UpdateWebAuthnCredential 方法
	// 2. 在 SQLite 驱动中实现该方法
	// 3. 更新凭证的签名计数

	return nil
}

