package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestSQLiteDriver(t *testing.T) {
	// 创建临时数据库
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	// 创建驱动
	driver, err := NewSQLiteDriver(tmpfile.Name())
	if err != nil {
		t.Fatalf("创建驱动失败: %v", err)
	}
	defer driver.Close()

	ctx := context.Background()

	t.Run("CreateUser", func(t *testing.T) {
		user := &User{
			Email:        "test@example.com",
			PasswordHash: "hash123",
			Quota:        1024 * 1024 * 100, // 100MB
			Active:       true,
		}

		err := driver.CreateUser(ctx, user)
		if err != nil {
			t.Fatalf("创建用户失败: %v", err)
		}
	})

	t.Run("GetUser", func(t *testing.T) {
		user, err := driver.GetUser(ctx, "test@example.com")
		if err != nil {
			t.Fatalf("获取用户失败: %v", err)
		}

		if user.Email != "test@example.com" {
			t.Errorf("用户邮箱不匹配: got %s, want test@example.com", user.Email)
		}

		if user.PasswordHash != "hash123" {
			t.Errorf("密码哈希不匹配")
		}

		if !user.Active {
			t.Error("用户应该处于激活状态")
		}
	})

	t.Run("GetUser_NotFound", func(t *testing.T) {
		_, err := driver.GetUser(ctx, "notfound@example.com")
		if err == nil {
			t.Error("应该返回错误")
		}
		if err != ErrNotFound && err.Error() != "用户不存在: not found" {
			t.Errorf("错误类型不匹配: %v", err)
		}
	})

	t.Run("CreateDomain", func(t *testing.T) {
		domain := &Domain{
			Name:   "example.com",
			Active: true,
		}

		err := driver.CreateDomain(ctx, domain)
		if err != nil {
			t.Fatalf("创建域名失败: %v", err)
		}
	})

	t.Run("GetDomain", func(t *testing.T) {
		domain, err := driver.GetDomain(ctx, "example.com")
		if err != nil {
			t.Fatalf("获取域名失败: %v", err)
		}

		if domain.Name != "example.com" {
			t.Errorf("域名不匹配: got %s, want example.com", domain.Name)
		}
	})

	t.Run("CreateAlias", func(t *testing.T) {
		alias := &Alias{
			From:   "alias@example.com",
			To:     "test@example.com",
			Domain: "example.com",
		}

		err := driver.CreateAlias(ctx, alias)
		if err != nil {
			t.Fatalf("创建别名失败: %v", err)
		}
	})

	t.Run("GetAlias", func(t *testing.T) {
		alias, err := driver.GetAlias(ctx, "alias@example.com")
		if err != nil {
			t.Fatalf("获取别名失败: %v", err)
		}

		if alias.From != "alias@example.com" {
			t.Errorf("别名源地址不匹配: got %s, want alias@example.com", alias.From)
		}

		if alias.To != "test@example.com" {
			t.Errorf("别名目标地址不匹配: got %s, want test@example.com", alias.To)
		}
	})

	t.Run("GetQuota", func(t *testing.T) {
		quota, err := driver.GetQuota(ctx, "test@example.com")
		if err != nil {
			t.Fatalf("获取配额失败: %v", err)
		}

		if quota.UserEmail != "test@example.com" {
			t.Errorf("用户邮箱不匹配: got %s, want test@example.com", quota.UserEmail)
		}

		if quota.Limit != 1024*1024*100 {
			t.Errorf("配额限制不匹配: got %d, want %d", quota.Limit, 1024*1024*100)
		}
	})
}

func TestSQLiteDriver_Concurrent(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	driver, err := NewSQLiteDriver(tmpfile.Name())
	if err != nil {
		t.Fatalf("创建驱动失败: %v", err)
	}
	defer driver.Close()

	ctx := context.Background()

	// 并发创建用户
	const numUsers = 10
	done := make(chan error, numUsers)

	for i := 0; i < numUsers; i++ {
		go func(i int) {
			user := &User{
				Email:        fmt.Sprintf("user%d@example.com", i),
				PasswordHash: "hash",
				Active:       true,
			}
			done <- driver.CreateUser(ctx, user)
		}(i)
	}

	for i := 0; i < numUsers; i++ {
		if err := <-done; err != nil {
			t.Errorf("并发创建用户失败: %v", err)
		}
	}

	// 验证所有用户都已创建
	users, err := driver.ListUsers(ctx, 100, 0)
	if err != nil {
		t.Fatalf("列出用户失败: %v", err)
	}

	if len(users) != numUsers {
		t.Errorf("用户数量不匹配: got %d, want %d", len(users), numUsers)
	}
}

