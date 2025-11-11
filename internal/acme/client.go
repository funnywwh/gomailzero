package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gomailzero/gmz/internal/config"
	"github.com/gomailzero/gmz/internal/logger"
	"golang.org/x/crypto/acme"
)

// Client ACME 客户端
type Client struct {
	config     *config.ACMEConfig
	acmeClient *acme.Client
	key        *ecdsa.PrivateKey
	account    *acme.Account
}

// NewClient 创建 ACME 客户端
func NewClient(cfg *config.ACMEConfig) (*Client, error) {
	// 生成账户密钥
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成账户密钥失败: %w", err)
	}

	// 选择 ACME 目录 URL
	var directoryURL string
	switch cfg.Provider {
	case "letsencrypt":
		directoryURL = acme.LetsEncryptURL
	case "zerossl":
		directoryURL = "https://acme.zerossl.com/v2/DV90"
	default:
		directoryURL = acme.LetsEncryptURL
	}

	// 创建 ACME 客户端
	client := &acme.Client{
		Key:          key,
		DirectoryURL: directoryURL,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	}

	// 确保证书目录存在
	// #nosec G301 -- 0755 权限允许组和其他用户读取证书，这是 ACME 证书的标准权限
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, fmt.Errorf("创建证书目录失败: %w", err)
	}

	return &Client{
		config:     cfg,
		acmeClient: client,
		key:        key,
	}, nil
}

// Register 注册 ACME 账户
func (c *Client) Register(ctx context.Context) error {
	account := &acme.Account{
		Contact: []string{"mailto:" + c.config.Email},
	}

	var err error
	c.account, err = c.acmeClient.Register(ctx, account, acme.AcceptTOS)
	if err != nil {
		// 如果账户已存在，尝试获取
		if acmeErr, ok := err.(*acme.Error); ok && acmeErr.StatusCode == http.StatusConflict {
			c.account, err = c.acmeClient.GetReg(ctx, "")
			if err != nil {
				return fmt.Errorf("获取现有账户失败: %w", err)
			}
			logger.Info().Msg("使用现有 ACME 账户")
			return nil
		}
		return fmt.Errorf("注册 ACME 账户失败: %w", err)
	}

	logger.Info().Msg("ACME 账户注册成功")
	return nil
}

// ObtainCertificate 获取证书
func (c *Client) ObtainCertificate(ctx context.Context, domain string) (*tls.Certificate, error) {
	// 创建证书请求
	csr, key, err := c.createCSR(domain)
	if err != nil {
		return nil, fmt.Errorf("创建证书请求失败: %w", err)
	}

	// 授权域名
	authz, err := c.acmeClient.Authorize(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("授权域名失败: %w", err)
	}

	// 完成 HTTP-01 挑战
	if err := c.completeHTTP01Challenge(ctx, authz); err != nil {
		return nil, fmt.Errorf("完成挑战失败: %w", err)
	}

	// 等待授权生效
	if _, err := c.acmeClient.WaitAuthorization(ctx, authz.URI); err != nil {
		return nil, fmt.Errorf("等待授权失败: %w", err)
	}

	// 申请证书
	// TODO: 迁移到 RFC 8555 的 CreateOrderCert API
	// nolint:staticcheck // CreateCert 已弃用，但暂时保留以保持兼容性
	certChain, _, err := c.acmeClient.CreateCert(ctx, csr, time.Hour*24*90, true)
	if err != nil {
		return nil, fmt.Errorf("申请证书失败: %w", err)
	}

	// 合并证书链
	var certPEM []byte
	for _, cert := range certChain {
		block := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert,
		}
		certPEM = append(certPEM, pem.EncodeToMemory(block)...)
	}

	// 保存证书和密钥
	if err := c.saveCertificate(domain, certPEM, key); err != nil {
		return nil, fmt.Errorf("保存证书失败: %w", err)
	}

	// 加载证书
	tlsCert, err := tls.X509KeyPair(certPEM, key)
	if err != nil {
		return nil, fmt.Errorf("加载证书失败: %w", err)
	}

	logger.Info().Str("domain", domain).Msg("证书获取成功")
	return &tlsCert, nil
}

// RenewCertificate 续期证书
func (c *Client) RenewCertificate(ctx context.Context, domain string) (*tls.Certificate, error) {
	certFile := filepath.Join(c.config.Dir, domain+".crt")
	keyFile := filepath.Join(c.config.Dir, domain+".key")

	// 检查证书是否存在
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		// 证书不存在，申请新证书
		return c.ObtainCertificate(ctx, domain)
	}

	// 检查证书是否即将过期（30 天内）
	cert, err := c.loadCertificate(certFile)
	if err != nil {
		return nil, fmt.Errorf("加载证书失败: %w", err)
	}

	if time.Until(cert.NotAfter) < 30*24*time.Hour {
		logger.Info().Str("domain", domain).Msg("证书即将过期，开始续期")
		return c.ObtainCertificate(ctx, domain)
	}

	// 证书仍然有效，加载并返回
	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("加载证书失败: %w", err)
	}

	return &tlsCert, nil
}

// createCSR 创建证书签名请求
func (c *Client) createCSR(domain string) ([]byte, []byte, error) {
	// 生成私钥
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("生成私钥失败: %w", err)
	}

	// 创建 CSR
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: domain,
		},
		DNSNames: []string{domain},
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		return nil, nil, fmt.Errorf("创建 CSR 失败: %w", err)
	}

	// 编码私钥
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("编码私钥失败: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	return csr, keyPEM, nil
}

// completeHTTP01Challenge 完成 HTTP-01 挑战
func (c *Client) completeHTTP01Challenge(ctx context.Context, authz *acme.Authorization) error {
	// 找到 HTTP-01 挑战
	var chal *acme.Challenge
	for _, c := range authz.Challenges {
		if c.Type == "http-01" {
			chal = c
			break
		}
	}

	if chal == nil {
		return fmt.Errorf("未找到 HTTP-01 挑战")
	}

	// 接受挑战
	_, err := c.acmeClient.Accept(ctx, chal)
	if err != nil {
		return fmt.Errorf("接受挑战失败: %w", err)
	}

	return nil
}

// saveCertificate 保存证书
func (c *Client) saveCertificate(domain string, cert []byte, key []byte) error {
	certFile := filepath.Join(c.config.Dir, domain+".crt")
	keyFile := filepath.Join(c.config.Dir, domain+".key")

	// #nosec G306 -- 证书文件需要可读权限，0644 是标准权限
	if err := os.WriteFile(certFile, cert, 0644); err != nil {
		return fmt.Errorf("保存证书文件失败: %w", err)
	}

	if err := os.WriteFile(keyFile, key, 0600); err != nil {
		return fmt.Errorf("保存密钥文件失败: %w", err)
	}

	return nil
}

// loadCertificate 加载证书
func (c *Client) loadCertificate(certFile string) (*x509.Certificate, error) {
	// 验证文件路径在证书目录下（防止路径遍历攻击）
	// #nosec G304 -- certFile 已经通过 filepath.Join 和已验证的 c.config.Dir 构建，是安全的
	if !strings.HasPrefix(certFile, c.config.Dir) {
		return nil, fmt.Errorf("无效的证书文件路径")
	}

	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("读取证书文件失败: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("解析证书 PEM 失败")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析证书失败: %w", err)
	}

	return cert, nil
}

// GetCertificate 获取证书（用于 TLS 配置）
func (c *Client) GetCertificate(domain string) (*tls.Certificate, error) {
	certFile := filepath.Join(c.config.Dir, domain+".crt")
	keyFile := filepath.Join(c.config.Dir, domain+".key")

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("加载证书失败: %w", err)
	}

	return &cert, nil
}
