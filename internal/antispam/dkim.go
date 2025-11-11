package antispam

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// DKIM DKIM 签名和验证
type DKIM struct {
	privateKey crypto.PrivateKey
	publicKey  crypto.PublicKey
	selector   string
	domain     string
}

// NewDKIM 创建 DKIM 实例
func NewDKIM(domain, selector string, privateKey crypto.PrivateKey) (*DKIM, error) {
	var publicKey crypto.PublicKey

	switch key := privateKey.(type) {
	case *rsa.PrivateKey:
		publicKey = &key.PublicKey
	case ed25519.PrivateKey:
		publicKey = key.Public()
	default:
		return nil, fmt.Errorf("不支持的密钥类型")
	}

	return &DKIM{
		privateKey: privateKey,
		publicKey:  publicKey,
		selector:   selector,
		domain:     domain,
	}, nil
}

// Sign 对邮件进行 DKIM 签名
func (d *DKIM) Sign(headers map[string]string, body []byte) (string, error) {
	// 构建签名头
	signature := d.buildSignature(headers, body)

	// 计算签名
	hash := sha256.New()
	hash.Write([]byte(signature))
	hashed := hash.Sum(nil)

	var signatureBytes []byte
	var err error

	switch key := d.privateKey.(type) {
	case *rsa.PrivateKey:
		signatureBytes, err = rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hashed)
	case ed25519.PrivateKey:
		signatureBytes, err = key.Sign(rand.Reader, hashed, crypto.Hash(0))
	default:
		return "", fmt.Errorf("不支持的密钥类型")
	}

	if err != nil {
		return "", fmt.Errorf("签名失败: %w", err)
	}

	// 编码签名
	signatureB64 := base64.StdEncoding.EncodeToString(signatureBytes)

	// 构建完整的 DKIM-Signature 头
	dkimHeader := fmt.Sprintf("v=1; a=rsa-sha256; c=relaxed/relaxed; d=%s; s=%s; t=%d; h=%s; bh=%s; b=%s",
		d.domain,
		d.selector,
		time.Now().Unix(),
		strings.Join(d.getSignedHeaders(headers), ":"),
		base64.StdEncoding.EncodeToString(hashed),
		signatureB64,
	)

	return dkimHeader, nil
}

// Verify 验证 DKIM 签名
func (d *DKIM) Verify(headers map[string]string, body []byte, dkimSignature string) (bool, error) {
	// 解析 DKIM 签名
	params := d.parseDKIMSignature(dkimSignature)

	// 提取签名值
	signatureB64, ok := params["b"]
	if !ok {
		return false, fmt.Errorf("未找到签名值")
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false, fmt.Errorf("解码签名失败: %w", err)
	}

	// 重建签名字符串
	signature := d.buildSignature(headers, body)

	// 计算哈希
	hash := sha256.New()
	hash.Write([]byte(signature))
	hashed := hash.Sum(nil)

	// 验证签名
	switch key := d.publicKey.(type) {
	case *rsa.PublicKey:
		err = rsa.VerifyPKCS1v15(key, crypto.SHA256, hashed, signatureBytes)
	case ed25519.PublicKey:
		if !ed25519.Verify(key, hashed, signatureBytes) {
			err = fmt.Errorf("签名验证失败")
		}
	default:
		return false, fmt.Errorf("不支持的密钥类型")
	}

	if err != nil {
		return false, nil
	}

	return true, nil
}

// buildSignature 构建签名字符串
func (d *DKIM) buildSignature(headers map[string]string, body []byte) string {
	// 简化实现：仅包含关键头
	signedHeaders := []string{"From", "To", "Subject", "Date"}
	var headerLines []string

	for _, h := range signedHeaders {
		if val, ok := headers[h]; ok {
			headerLines = append(headerLines, fmt.Sprintf("%s: %s", strings.ToLower(h), d.canonicalizeHeader(val)))
		}
	}

	// 规范化邮件体
	canonicalBody := d.canonicalizeBody(string(body))

	return strings.Join(headerLines, "\r\n") + "\r\n" + canonicalBody
}

// canonicalizeHeader 规范化邮件头
func (d *DKIM) canonicalizeHeader(header string) string {
	// relaxed 规范化：去除多余空格，转换为小写
	header = strings.TrimSpace(header)
	header = strings.ToLower(header)
	return header
}

// canonicalizeBody 规范化邮件体
func (d *DKIM) canonicalizeBody(body string) string {
	// relaxed 规范化：去除行尾空格，空行压缩
	lines := strings.Split(body, "\n")
	var canonicalLines []string

	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			canonicalLines = append(canonicalLines, "")
		} else {
			canonicalLines = append(canonicalLines, line)
		}
	}

	return strings.Join(canonicalLines, "\r\n")
}

// getSignedHeaders 获取已签名的头列表
func (d *DKIM) getSignedHeaders(headers map[string]string) []string {
	var signed []string
	for h := range headers {
		signed = append(signed, strings.ToLower(h))
	}
	return signed
}

// parseDKIMSignature 解析 DKIM 签名
func (d *DKIM) parseDKIMSignature(signature string) map[string]string {
	params := make(map[string]string)
	parts := strings.Split(signature, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "="); idx > 0 {
			key := strings.TrimSpace(part[:idx])
			val := strings.TrimSpace(part[idx+1:])
			params[key] = val
		}
	}

	return params
}

// GenerateKeyPair 生成 DKIM 密钥对
func GenerateKeyPair(algorithm string) (crypto.PrivateKey, crypto.PublicKey, error) {
	switch algorithm {
	case "rsa":
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, nil, fmt.Errorf("生成 RSA 密钥失败: %w", err)
		}
		return key, &key.PublicKey, nil
	case "ed25519":
		publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, nil, fmt.Errorf("生成 Ed25519 密钥失败: %w", err)
		}
		return privateKey, publicKey, nil
	default:
		return nil, nil, fmt.Errorf("不支持的算法: %s", algorithm)
	}
}

// GetPublicKeyDNS 获取公钥的 DNS TXT 记录格式
func GetPublicKeyDNS(publicKey crypto.PublicKey) (string, error) {
	// TODO: 实现公钥到 DNS TXT 记录的转换
	return "", fmt.Errorf("未实现")
}

