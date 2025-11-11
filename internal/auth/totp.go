package auth

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"

	"github.com/gomailzero/gmz/internal/crypto"
	"github.com/gomailzero/gmz/internal/storage"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTPManager TOTP 管理器
type TOTPManager struct {
	storage storage.Driver
}

// NewTOTPManager 创建 TOTP 管理器
func NewTOTPManager(storage storage.Driver) *TOTPManager {
	return &TOTPManager{
		storage: storage,
	}
}

// GenerateSecret 为用户生成 TOTP 密钥
func (m *TOTPManager) GenerateSecret(ctx context.Context, userEmail string, issuer string) (string, string, error) {
	// 生成密钥
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: userEmail,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", fmt.Errorf("生成 TOTP 密钥失败: %w", err)
	}

	// 保存到数据库
	// TODO: 实现 TOTP 密钥存储（加密存储）

	return key.Secret(), key.URL(), nil
}

// Verify 验证 TOTP 代码
func (m *TOTPManager) Verify(ctx context.Context, userEmail string, code string) (bool, error) {
	// 从数据库获取加密的密钥
	// TODO: 从存储获取加密的密钥
	secret := "" // 临时，需要从存储获取

	if secret == "" {
		return false, fmt.Errorf("用户未启用 TOTP")
	}

	// 解密密钥
	decryptedSecret, err := m.decryptSecret(secret)
	if err != nil {
		return false, fmt.Errorf("解密密钥失败: %w", err)
	}

	// 验证代码
	valid := totp.Validate(code, decryptedSecret)
	return valid, nil
}

// encryptSecret 加密密钥
func (m *TOTPManager) encryptSecret(secret string) (string, error) {
	// 生成随机 salt
	_, err := crypto.GenerateSalt()
	if err != nil {
		return "", err
	}

	// TODO: 使用用户密码派生密钥
	// 这里简化实现，使用固定密钥（实际应该从用户密码派生）
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}

	// 加密
	encrypted, err := crypto.Encrypt(key, []byte(secret))
	if err != nil {
		return "", err
	}

	// 编码：salt:encrypted
	encoded := base64.StdEncoding.EncodeToString(encrypted)
	return encoded, nil
}

// decryptSecret 解密密钥
func (m *TOTPManager) decryptSecret(encrypted string) (string, error) {
	// 解码
	decoded, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	// TODO: 使用用户密码派生密钥
	key := make([]byte, 32) // 临时，应该从用户密码派生

	// 解密
	decrypted, err := crypto.Decrypt(key, decoded)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

// GenerateRecoveryCodes 生成恢复码
func (m *TOTPManager) GenerateRecoveryCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		// 生成 8 位随机码
		bytes := make([]byte, 4)
		if _, err := rand.Read(bytes); err != nil {
			return nil, fmt.Errorf("生成恢复码失败: %w", err)
		}
		code := base32.StdEncoding.EncodeToString(bytes)[:8]
		codes[i] = code
	}
	return codes, nil
}

// ValidateRecoveryCode 验证恢复码
func (m *TOTPManager) ValidateRecoveryCode(ctx context.Context, userEmail string, code string) (bool, error) {
	// TODO: 从存储获取恢复码列表并验证
	return false, fmt.Errorf("未实现")
}

// QRCodeURL 生成二维码 URL
func (m *TOTPManager) QRCodeURL(secret string, issuer string, accountName string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", issuer, accountName, secret, issuer)
}
