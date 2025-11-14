package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomailzero/gmz/internal/storage"
)

func TestListDomainsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	driver := &MockStorageDriver{}
	handler := listDomainsHandler(driver)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/domains", nil)
	c.Request.Header.Set("X-API-Key", "test-key")

	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("listDomainsHandler() status = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if _, ok := response["domains"]; !ok {
		t.Errorf("listDomainsHandler() response missing 'domains' key")
	}
}

func TestCreateDomainHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	driver := &MockStorageDriver{}
	handler := createDomainHandler(driver)

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "正常创建",
			body: map[string]interface{}{
				"name":   "example.com",
				"active": true,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "缺少名称",
			body: map[string]interface{}{
				"active": true,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/domains", bytes.NewReader(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("X-API-Key", "test-key")

			handler(c)

			if w.Code != tt.wantStatus {
				t.Errorf("createDomainHandler() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestCreateUserHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	driver := &MockStorageDriver{}
	handler := createUserHandler(driver)

	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "正常创建",
			body: map[string]interface{}{
				"email":    "test@example.com",
				"password": "password123",
				"quota":    1073741824, // 1GB
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "缺少邮箱",
			body: map[string]interface{}{
				"password": "password123",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "缺少密码",
			body: map[string]interface{}{
				"email": "test@example.com",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tt.body)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("X-API-Key", "test-key")

			handler(c)

			if w.Code != tt.wantStatus {
				t.Errorf("createUserHandler() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

// MockStorageDriver 模拟存储驱动
type MockStorageDriver struct{}

func (m *MockStorageDriver) CreateUser(ctx context.Context, user *storage.User) error {
	return nil
}

func (m *MockStorageDriver) GetUser(ctx context.Context, email string) (*storage.User, error) {
	return &storage.User{
		Email:  email,
		Active: true,
	}, nil
}

func (m *MockStorageDriver) UpdateUser(ctx context.Context, user *storage.User) error {
	return nil
}

func (m *MockStorageDriver) DeleteUser(ctx context.Context, email string) error {
	return nil
}

func (m *MockStorageDriver) ListUsers(ctx context.Context, limit, offset int) ([]*storage.User, error) {
	return []*storage.User{}, nil
}

func (m *MockStorageDriver) CreateDomain(ctx context.Context, domain *storage.Domain) error {
	return nil
}

func (m *MockStorageDriver) GetDomain(ctx context.Context, name string) (*storage.Domain, error) {
	return &storage.Domain{
		Name:   name,
		Active: true,
	}, nil
}

func (m *MockStorageDriver) UpdateDomain(ctx context.Context, domain *storage.Domain) error {
	return nil
}

func (m *MockStorageDriver) DeleteDomain(ctx context.Context, name string) error {
	return nil
}

func (m *MockStorageDriver) ListDomains(ctx context.Context) ([]*storage.Domain, error) {
	return []*storage.Domain{}, nil
}

func (m *MockStorageDriver) CreateAlias(ctx context.Context, alias *storage.Alias) error {
	return nil
}

func (m *MockStorageDriver) GetAlias(ctx context.Context, from string) (*storage.Alias, error) {
	return nil, nil
}

func (m *MockStorageDriver) DeleteAlias(ctx context.Context, from string) error {
	return nil
}

func (m *MockStorageDriver) ListAliases(ctx context.Context, domain string) ([]*storage.Alias, error) {
	return []*storage.Alias{}, nil
}

func (m *MockStorageDriver) StoreMail(ctx context.Context, mail *storage.Mail) error {
	return nil
}

func (m *MockStorageDriver) GetMail(ctx context.Context, id string) (*storage.Mail, error) {
	return nil, nil
}

func (m *MockStorageDriver) GetMailBody(ctx context.Context, userEmail string, folder string, mailID string) ([]byte, error) {
	return nil, nil
}

func (m *MockStorageDriver) ListMails(ctx context.Context, userEmail string, folder string, limit, offset int) ([]*storage.Mail, error) {
	return []*storage.Mail{}, nil
}

func (m *MockStorageDriver) DeleteMail(ctx context.Context, id string) error {
	return nil
}

func (m *MockStorageDriver) UpdateMailFlags(ctx context.Context, id string, flags []string) error {
	return nil
}

func (m *MockStorageDriver) SearchMails(ctx context.Context, userEmail string, query string, folder string, limit, offset int) ([]*storage.Mail, error) {
	return []*storage.Mail{}, nil
}

func (m *MockStorageDriver) ListFolders(ctx context.Context, userEmail string) ([]string, error) {
	return []string{"INBOX"}, nil
}

func (m *MockStorageDriver) GetQuota(ctx context.Context, userEmail string) (*storage.Quota, error) {
	return &storage.Quota{
		UserEmail: userEmail,
		Used:      0,
		Limit:     1073741824,
	}, nil
}

func (m *MockStorageDriver) UpdateQuota(ctx context.Context, userEmail string, quota *storage.Quota) error {
	return nil
}

func (m *MockStorageDriver) SaveTOTPSecret(ctx context.Context, userEmail string, secret string) error {
	return nil
}

func (m *MockStorageDriver) GetTOTPSecret(ctx context.Context, userEmail string) (string, error) {
	return "", nil
}

func (m *MockStorageDriver) DeleteTOTPSecret(ctx context.Context, userEmail string) error {
	return nil
}

func (m *MockStorageDriver) IsTOTPEnabled(ctx context.Context, userEmail string) (bool, error) {
	return false, nil
}

func (m *MockStorageDriver) Close() error {
	return nil
}
