package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		wantError bool
	}{
		{
			name: "valid config",
			config: `
node_id: mx1
domain: example.com
storage:
  driver: sqlite
  dsn: /tmp/test.db
tls:
  enabled: true
  acme:
    enabled: true
    email: admin@example.com
`,
			wantError: false,
		},
		{
			name: "missing domain",
			config: `
node_id: mx1
domain: ""
storage:
  driver: sqlite
`,
			wantError: true,
		},
		{
			name: "invalid storage driver",
			config: `
domain: example.com
storage:
  driver: mysql
`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建临时配置文件
			tmpfile, err := os.CreateTemp("", "gmz-test-*.yml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.config)); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			cfg, err := Load(tmpfile.Name())
			if (err != nil) != tt.wantError {
				t.Errorf("Load() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && cfg == nil {
				t.Error("Load() 应该返回配置对象")
			}

			if !tt.wantError && cfg != nil {
				if cfg.Domain == "" {
					t.Error("配置应该包含 domain")
				}
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "gmz-test-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// 最小配置
	config := `domain: example.com`
	if _, err := tmpfile.Write([]byte(config)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load() 失败: %v", err)
	}

	// 检查默认值
	if cfg.NodeID != "mx1" {
		t.Errorf("NodeID = %v, want mx1", cfg.NodeID)
	}
	if cfg.Storage.Driver != "sqlite" {
		t.Errorf("Storage.Driver = %v, want sqlite", cfg.Storage.Driver)
	}
	if !cfg.TLS.Enabled {
		t.Error("TLS.Enabled 应该默认为 true")
	}
	if !cfg.SMTP.Enabled {
		t.Error("SMTP.Enabled 应该默认为 true")
	}
	if !cfg.IMAP.Enabled {
		t.Error("IMAP.Enabled 应该默认为 true")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		wantError bool
	}{
		{
			name: "valid config",
			config: `
domain: example.com
storage:
  driver: sqlite
  dsn: /tmp/test.db
`,
			wantError: false,
		},
		{
			name: "empty domain",
			config: `
domain: ""
storage:
  driver: sqlite
`,
			wantError: true,
		},
		{
			name: "invalid driver",
			config: `
domain: example.com
storage:
  driver: invalid
`,
			wantError: true,
		},
		{
			name: "TLS enabled without cert files",
			config: `
domain: example.com
storage:
  driver: sqlite
tls:
  enabled: true
  acme:
    enabled: false
`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "gmz-test-*.yml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.config)); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			_, err = Load(tmpfile.Name())
			if (err != nil) != tt.wantError {
				t.Errorf("Load() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
