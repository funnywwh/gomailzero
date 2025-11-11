package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2id 参数（根据 OWASP 推荐）
	argon2Time    = 3
	argon2Memory  = 32 * 1024 // 32 MB
	argon2Threads = 4
	argon2KeyLen  = 32 // 32 字节用于 XChaCha20-Poly1305
	saltSize      = 16
)

// HashPassword 使用 Argon2id 哈希密码
func HashPassword(password string) (string, error) {
	// 生成随机 salt
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("生成 salt 失败: %w", err)
	}

	// 使用 Argon2id 派生密钥
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// 编码为 base64: salt:hash
	encoded := base64.StdEncoding.EncodeToString(append(salt, hash...))
	return encoded, nil
}

// VerifyPassword 验证密码
func VerifyPassword(password, encodedHash string) (bool, error) {
	// 解码
	decoded, err := base64.StdEncoding.DecodeString(encodedHash)
	if err != nil {
		return false, fmt.Errorf("解码哈希失败: %w", err)
	}

	if len(decoded) < saltSize {
		return false, fmt.Errorf("哈希格式无效")
	}

	// 提取 salt 和 hash
	salt := decoded[:saltSize]
	expectedHash := decoded[saltSize:]

	// 计算密码的哈希
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// 使用 constant-time 比较
	if subtle.ConstantTimeCompare(hash, expectedHash) == 1 {
		return true, nil
	}

	return false, nil
}

// DeriveKey 从密码派生加密密钥（用于邮件加密）
func DeriveKey(password string, salt []byte) ([]byte, error) {
	if len(salt) != saltSize {
		return nil, fmt.Errorf("salt 长度必须为 %d 字节", saltSize)
	}

	// 使用 Argon2id 派生密钥
	key := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return key, nil
}

// GenerateSalt 生成随机 salt
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("生成 salt 失败: %w", err)
	}
	return salt, nil
}

