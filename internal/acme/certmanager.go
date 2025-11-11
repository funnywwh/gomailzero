package acme

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/gomailzero/gmz/internal/config"
	"github.com/gomailzero/gmz/internal/logger"
)

// Manager 证书管理器
type Manager struct {
	client   *Client
	config   *config.ACMEConfig
	certificates map[string]*tls.Certificate
	mu       sync.RWMutex
	stopCh   chan struct{}
}

// NewManager 创建证书管理器
func NewManager(cfg *config.ACMEConfig) (*Manager, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 ACME 客户端失败: %w", err)
	}

	// 注册账户
	ctx := context.Background()
	if err := client.Register(ctx); err != nil {
		return nil, fmt.Errorf("注册 ACME 账户失败: %w", err)
	}

	return &Manager{
		client:      client,
		config:      cfg,
		certificates: make(map[string]*tls.Certificate),
		stopCh:      make(chan struct{}),
	}, nil
}

// Start 启动证书管理器（自动续期）
func (m *Manager) Start(ctx context.Context, domains []string) error {
	// 初始获取证书
	for _, domain := range domains {
		cert, err := m.client.RenewCertificate(ctx, domain)
		if err != nil {
			logger.Warn().Err(err).Str("domain", domain).Msg("获取证书失败")
			continue
		}
		m.mu.Lock()
		m.certificates[domain] = cert
		m.mu.Unlock()
	}

	// 启动自动续期协程
	go m.autoRenew(ctx, domains)

	return nil
}

// Stop 停止证书管理器
func (m *Manager) Stop() {
	close(m.stopCh)
}

// GetCertificate 获取证书（实现 tls.Config.GetCertificate）
func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	domain := hello.ServerName
	if domain == "" {
		return nil, fmt.Errorf("未指定服务器名称")
	}

	m.mu.RLock()
	cert, ok := m.certificates[domain]
	m.mu.RUnlock()

	if ok && cert != nil {
		return cert, nil
	}

	// 证书不存在，尝试获取
	ctx := context.Background()
	newCert, err := m.client.RenewCertificate(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("获取证书失败: %w", err)
	}

	m.mu.Lock()
	m.certificates[domain] = newCert
	m.mu.Unlock()

	return newCert, nil
}

// autoRenew 自动续期证书
func (m *Manager) autoRenew(ctx context.Context, domains []string) {
	ticker := time.NewTicker(24 * time.Hour) // 每天检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, domain := range domains {
				cert, err := m.client.RenewCertificate(ctx, domain)
				if err != nil {
					logger.Error().Err(err).Str("domain", domain).Msg("续期证书失败")
					continue
				}

				m.mu.Lock()
				m.certificates[domain] = cert
				m.mu.Unlock()

				logger.Info().Str("domain", domain).Msg("证书续期成功")
			}
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// ReloadCertificate 重新加载证书（热更新）
func (m *Manager) ReloadCertificate(domain string) error {
	ctx := context.Background()
	cert, err := m.client.RenewCertificate(ctx, domain)
	if err != nil {
		return fmt.Errorf("重新加载证书失败: %w", err)
	}

	m.mu.Lock()
	m.certificates[domain] = cert
	m.mu.Unlock()

	logger.Info().Str("domain", domain).Msg("证书已重新加载")
	return nil
}

