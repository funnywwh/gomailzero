package storage

import (
	"context"
	"testing"
)

func TestSQLiteDriver_TOTPOperations(t *testing.T) {
	// 创建临时数据库
	driver, err := NewSQLiteDriver(":memory:")
	if err != nil {
		t.Fatalf("创建 SQLite 驱动失败: %v", err)
	}
	defer driver.Close()

	// 初始化数据库表结构（测试环境）
	if err := driver.initSchema(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	ctx := context.Background()

	// 创建测试用户
	user := &User{
		Email:        "test@example.com",
		PasswordHash: "test_hash",
		Active:       true,
	}
	if err := driver.CreateUser(ctx, user); err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	// 测试保存 TOTP 密钥
	secret := "JBSWY3DPEHPK3PXP"
	if err := driver.SaveTOTPSecret(ctx, "test@example.com", secret); err != nil {
		t.Fatalf("保存 TOTP 密钥失败: %v", err)
	}

	// 测试获取 TOTP 密钥
	retrievedSecret, err := driver.GetTOTPSecret(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("获取 TOTP 密钥失败: %v", err)
	}
	if retrievedSecret != secret {
		t.Errorf("TOTP 密钥不匹配: 期望 %s, 得到 %s", secret, retrievedSecret)
	}

	// 测试删除 TOTP 密钥
	if err := driver.DeleteTOTPSecret(ctx, "test@example.com"); err != nil {
		t.Fatalf("删除 TOTP 密钥失败: %v", err)
	}

	// 验证已删除
	_, err = driver.GetTOTPSecret(ctx, "test@example.com")
	if err == nil {
		t.Error("期望获取已删除的 TOTP 密钥时返回错误")
	}
}

