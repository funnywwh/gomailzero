//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/gomailzero/gmz/internal/crypto"
	"github.com/gomailzero/gmz/internal/storage"
)

// TestSMTPBasicFlow 测试基本的 SMTP 流程
func TestSMTPBasicFlow(t *testing.T) {
	// 创建测试存储
	driver, err := storage.NewSQLiteDriver(":memory:")
	if err != nil {
		t.Fatalf("创建存储驱动失败: %v", err)
	}
	defer driver.Close()

	ctx := context.Background()

	// 创建测试用户
	passwordHash, err := crypto.HashPassword("testpass123")
	if err != nil {
		t.Fatalf("哈希密码失败: %v", err)
	}

	user := &storage.User{
		Email:        "test@example.com",
		PasswordHash: passwordHash,
		Active:       true,
	}
	if err := driver.CreateUser(ctx, user); err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	// 创建测试域名
	domain := &storage.Domain{
		Name:   "example.com",
		Active: true,
	}
	if err := driver.CreateDomain(ctx, domain); err != nil {
		t.Fatalf("创建域名失败: %v", err)
	}

	// TODO: 启动 SMTP 服务器（需要 mock 或实际启动）
	// 这里暂时跳过，因为需要完整的服务器启动流程
	t.Skip("需要完整的 SMTP 服务器启动流程")
}

// TestSMTPAuth 测试 SMTP 认证
func TestSMTPAuth(t *testing.T) {
	t.Skip("需要完整的 SMTP 服务器启动流程")
}

// TestSMTPSTARTTLS 测试 SMTP STARTTLS
func TestSMTPSTARTTLS(t *testing.T) {
	t.Skip("需要完整的 SMTP 服务器启动流程")
}

// TestSMTPDelivery 测试邮件投递
func TestSMTPDelivery(t *testing.T) {
	t.Skip("需要完整的 SMTP 服务器启动流程")
}

// TestSMTPRejection 测试邮件拒绝（垃圾邮件）
func TestSMTPRejection(t *testing.T) {
	t.Skip("需要完整的 SMTP 服务器启动流程")
}
