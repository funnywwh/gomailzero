package crypto

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32) // XChaCha20-Poly1305 需要 32 字节密钥
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("Hello, World! This is a test message.")

	// 加密
	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	if len(ciphertext) <= len(plaintext) {
		t.Error("密文应该比明文长（包含 nonce）")
	}

	// 解密
	decrypted, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("解密结果不匹配: got %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestEncryptDecrypt_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1 // 不同的密钥

	plaintext := []byte("Hello, World!")

	ciphertext, err := Encrypt(key1, plaintext)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	// 使用错误的密钥解密应该失败
	_, err = Decrypt(key2, ciphertext)
	if err == nil {
		t.Error("使用错误密钥解密应该失败")
	}
}

func TestEncryptDecrypt_InvalidKeySize(t *testing.T) {
	key := make([]byte, 16) // 错误的密钥长度
	plaintext := []byte("Hello, World!")

	_, err := Encrypt(key, plaintext)
	if err == nil {
		t.Error("使用错误长度的密钥应该失败")
	}
}

func TestEncryptDecrypt_InvalidCiphertext(t *testing.T) {
	key := make([]byte, 32)
	invalidCiphertext := []byte("too short")

	_, err := Decrypt(key, invalidCiphertext)
	if err == nil {
		t.Error("解密无效密文应该失败")
	}
}

