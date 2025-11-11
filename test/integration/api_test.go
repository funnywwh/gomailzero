//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomailzero/gmz/internal/api"
	"github.com/gomailzero/gmz/internal/auth"
	"github.com/gomailzero/gmz/internal/crypto"
	"github.com/gomailzero/gmz/internal/storage"
)

// TestAPILogin 测试 API 登录
func TestAPILogin(t *testing.T) {
	gin.SetMode(gin.TestMode)

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

	// 创建 API 服务器
	jwtManager := auth.NewJWTManager("test-secret", "test")
	totpManager := auth.NewTOTPManager(driver)

	apiServer := api.NewServer(&api.Config{
		Port:       8080,
		APIKey:     "test-key",
		Storage:    driver,
		JWTManager: jwtManager,
		TOTPManager: totpManager,
	})

	// 测试登录
	loginReq := map[string]string{
		"email":    "test@example.com",
		"password": "testpass123",
	}
	body, _ := json.Marshal(loginReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	apiServer.GetRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("登录失败: status = %d, body = %s", w.Code, w.Body.String())
		return
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if _, ok := response["token"]; !ok {
		t.Error("响应中缺少 token")
	}
}

// TestAPICreateUser 测试创建用户 API
func TestAPICreateUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试存储
	driver, err := storage.NewSQLiteDriver(":memory:")
	if err != nil {
		t.Fatalf("创建存储驱动失败: %v", err)
	}
	defer driver.Close()

	// 创建 API 服务器
	jwtManager := auth.NewJWTManager("test-secret", "test")
	totpManager := auth.NewTOTPManager(driver)

	apiServer := api.NewServer(&api.Config{
		Port:       8080,
		APIKey:     "test-key",
		Storage:    driver,
		JWTManager: jwtManager,
		TOTPManager: totpManager,
	})

	// 测试创建用户（使用 API Key）
	createReq := map[string]interface{}{
		"email":    "newuser@example.com",
		"password": "newpass123",
		"quota":    1024 * 1024 * 100, // 100MB
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()

	apiServer.GetRouter().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("创建用户失败: status = %d, body = %s", w.Code, w.Body.String())
		return
	}

	var user storage.User
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if user.Email != "newuser@example.com" {
		t.Errorf("用户邮箱不匹配: got %s, want newuser@example.com", user.Email)
	}
}

// TestAPIGetUser 测试获取用户 API
func TestAPIGetUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

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

	// 创建 API 服务器
	jwtManager := auth.NewJWTManager("test-secret", "test")
	totpManager := auth.NewTOTPManager(driver)

	apiServer := api.NewServer(&api.Config{
		Port:       8080,
		APIKey:     "test-key",
		Storage:    driver,
		JWTManager: jwtManager,
		TOTPManager: totpManager,
	})

	// 测试获取用户
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/test@example.com", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()

	apiServer.GetRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("获取用户失败: status = %d, body = %s", w.Code, w.Body.String())
		return
	}

	var retrievedUser storage.User
	if err := json.Unmarshal(w.Body.Bytes(), &retrievedUser); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if retrievedUser.Email != "test@example.com" {
		t.Errorf("用户邮箱不匹配: got %s, want test@example.com", retrievedUser.Email)
	}
}

