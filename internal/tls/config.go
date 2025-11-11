package tls

import (
	"crypto/tls"
	"fmt"

	"github.com/gomailzero/gmz/internal/config"
	"github.com/gomailzero/gmz/internal/logger"
)

// LoadTLSConfig 加载 TLS 配置
func LoadTLSConfig(cfg *config.TLSConfig) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	// 设置最低 TLS 版本
	switch cfg.MinVersion {
	case "1.3":
		tlsConfig.MinVersion = tls.VersionTLS13
	case "1.2":
		tlsConfig.MinVersion = tls.VersionTLS12
	default:
		tlsConfig.MinVersion = tls.VersionTLS13
	}

	// 如果启用了 ACME，证书将由 ACME 客户端管理
	if cfg.ACME.Enabled {
		// TODO: 从 ACME 客户端获取证书
		logger.Info().Msg("使用 ACME 证书")
		return tlsConfig, nil
	}

	// 加载手动配置的证书
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("加载证书失败: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
		logger.Info().
			Str("cert_file", cfg.CertFile).
			Str("key_file", cfg.KeyFile).
			Msg("加载 TLS 证书")
		return tlsConfig, nil
	}

	return nil, fmt.Errorf("TLS 已启用但未配置证书")
}

// ReloadCertificate 重新加载证书（用于热更新）
func ReloadCertificate(tlsConfig *tls.Config, certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("重新加载证书失败: %w", err)
	}

	tlsConfig.Certificates = []tls.Certificate{cert}
	logger.Info().Msg("TLS 证书已重新加载")
	return nil
}

// GetCertificate 获取证书（用于 ACME）
func GetCertificate(domain string) (*tls.Certificate, error) {
	// TODO: 从 ACME 客户端获取证书
	return nil, fmt.Errorf("未实现")
}

// CheckCertificateExpiry 检查证书过期时间
func CheckCertificateExpiry(certFile string) error {
	// TODO: 解析证书并检查过期时间
	return nil
}

