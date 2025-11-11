//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/gomailzero/gmz/internal/crypto"
	"github.com/gomailzero/gmz/internal/storage"
)

// TestIMAPLogin 测试 IMAP 登录
func TestIMAPLogin(t *testing.T) {
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

	// TODO: 启动 IMAP 服务器并测试登录
	t.Skip("需要完整的 IMAP 服务器启动流程")
}

// TestIMAPListMailboxes 测试列出邮箱
func TestIMAPListMailboxes(t *testing.T) {
	t.Skip("需要完整的 IMAP 服务器启动流程")
}

// TestIMAPFetchMail 测试获取邮件
func TestIMAPFetchMail(t *testing.T) {
	t.Skip("需要完整的 IMAP 服务器启动流程")
}

// TestIMAPSearch 测试搜索邮件
func TestIMAPSearch(t *testing.T) {
	t.Skip("需要完整的 IMAP 服务器启动流程")
}

