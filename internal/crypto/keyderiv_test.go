package crypto

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "test-password-123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("哈希密码失败: %v", err)
	}

	if hash == "" {
		t.Error("哈希不应该为空")
	}

	// 相同密码应该生成不同的哈希（因为 salt 不同）
	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("哈希密码失败: %v", err)
	}

	if hash == hash2 {
		t.Error("相同密码应该生成不同的哈希（由于随机 salt）")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "test-password-123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("哈希密码失败: %v", err)
	}

	// 验证正确密码
	valid, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("验证密码失败: %v", err)
	}
	if !valid {
		t.Error("正确密码应该验证通过")
	}

	// 验证错误密码
	valid, err = VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("验证密码失败: %v", err)
	}
	if valid {
		t.Error("错误密码应该验证失败")
	}
}

func TestDeriveKey(t *testing.T) {
	password := "test-password-123"
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("生成 salt 失败: %v", err)
	}

	key1, err := DeriveKey(password, salt)
	if err != nil {
		t.Fatalf("派生密钥失败: %v", err)
	}

	if len(key1) != argon2KeyLen {
		t.Errorf("密钥长度不匹配: got %d, want %d", len(key1), argon2KeyLen)
	}

	// 相同密码和 salt 应该生成相同密钥
	key2, err := DeriveKey(password, salt)
	if err != nil {
		t.Fatalf("派生密钥失败: %v", err)
	}

	if string(key1) != string(key2) {
		t.Error("相同密码和 salt 应该生成相同密钥")
	}

	// 不同 salt 应该生成不同密钥
	salt2, err := GenerateSalt()
	if err != nil {
		t.Fatalf("生成 salt 失败: %v", err)
	}
	key3, err := DeriveKey(password, salt2)
	if err != nil {
		t.Fatalf("派生密钥失败: %v", err)
	}

	if string(key1) == string(key3) {
		t.Error("不同 salt 应该生成不同密钥")
	}
}

func TestGenerateSalt(t *testing.T) {
	salt1, err := GenerateSalt()
	if err != nil {
		t.Fatalf("生成 salt 失败: %v", err)
	}

	if len(salt1) != saltSize {
		t.Errorf("salt 长度不匹配: got %d, want %d", len(salt1), saltSize)
	}

	// 每次生成的 salt 应该不同
	salt2, err := GenerateSalt()
	if err != nil {
		t.Fatalf("生成 salt 失败: %v", err)
	}

	if string(salt1) == string(salt2) {
		t.Error("每次生成的 salt 应该不同")
	}
}

