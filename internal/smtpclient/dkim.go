package smtpclient

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gomailzero/gmz/internal/antispam"
	"github.com/gomailzero/gmz/internal/config"
	"github.com/gomailzero/gmz/internal/logger"
)

// LoadDKIM 加载 DKIM 配置
func LoadDKIM(cfg *config.DKIMConfig, domain, workDir string) (*antispam.DKIM, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if cfg.PrivateKey == "" {
		return nil, fmt.Errorf("DKIM 已启用但未配置私钥文件")
	}

	// 读取私钥文件
	keyPath := cfg.PrivateKey
	if !filepath.IsAbs(keyPath) {
		// 相对路径，基于工作目录解析
		keyPath = filepath.Join(workDir, keyPath)
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("读取 DKIM 私钥文件失败: %w", err)
	}

	// 解析 PEM 格式的私钥
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("无效的 PEM 格式")
	}

	var privateKey interface{}
	switch block.Type {
	case "RSA PRIVATE KEY":
		privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		privateKey, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("不支持的私钥类型: %s", block.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	// 转换为 RSA 私钥
	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("私钥不是 RSA 格式")
	}

	// 确定域名
	dkimDomain := cfg.Domain
	if dkimDomain == "" {
		dkimDomain = domain
	}
	if dkimDomain == "" {
		return nil, fmt.Errorf("DKIM 域名未配置")
	}

	// 确定选择器
	selector := cfg.Selector
	if selector == "" {
		selector = "default"
	}

	// 创建 DKIM 实例
	dkim, err := antispam.NewDKIM(dkimDomain, selector, rsaKey)
	if err != nil {
		return nil, fmt.Errorf("创建 DKIM 实例失败: %w", err)
	}

	// 注意：这里没有 context，使用普通 logger（初始化时）
	logger.Info().
		Str("domain", dkimDomain).
		Str("selector", selector).
		Msg("DKIM 签名已启用")

	return dkim, nil
}
