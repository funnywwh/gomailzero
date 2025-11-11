package antispam

import (
	"context"
	"net"
	"testing"
)

func TestEngine_Check(t *testing.T) {
	// 创建测试用的组件（不使用灰名单，避免首次发送被拒绝）
	dnsResolver := &MockDNSResolver{}
	spf := NewSPF(dnsResolver)
	dkim := &DKIM{} // 简化测试，不实际验证
	dmarc := NewDMARC(dnsResolver)
	// greylist 设为 nil，避免首次发送被拒绝
	ratelimit := NewRateLimiter()

	engine := NewEngine(spf, dkim, dmarc, nil, ratelimit)

	tests := []struct {
		name    string
		req     *CheckRequest
		want    Decision
		wantErr bool
	}{
		{
			name: "正常邮件通过",
			req: &CheckRequest{
				IP:     net.ParseIP("192.168.1.1"),
				From:   "sender@example.com",
				To:     "recipient@example.com",
				Domain: "example.com",
				HELO:   "mail.example.com",
				Headers: map[string]string{
					"From":    "sender@example.com",
					"To":      "recipient@example.com",
					"Subject": "Test",
				},
				Body: []byte("Test body"),
			},
			want: DecisionAccept,
		},
		{
			name: "无效 HELO 主机名",
			req: &CheckRequest{
				IP:     net.ParseIP("192.168.1.1"),
				From:   "sender@example.com",
				To:     "recipient@example.com",
				Domain: "example.com",
				HELO:   "localhost",
				Headers: map[string]string{
					"From": "sender@example.com",
				},
				Body: []byte("Test body"),
			},
			want: DecisionAccept, // HELO 检查只加分，不直接拒绝
		},
		{
			name: "空 HELO",
			req: &CheckRequest{
				IP:     net.ParseIP("192.168.1.1"),
				From:   "sender@example.com",
				To:     "recipient@example.com",
				Domain: "example.com",
				HELO:   "",
				Headers: map[string]string{
					"From": "sender@example.com",
				},
				Body: []byte("Test body"),
			},
			want: DecisionAccept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := engine.Check(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Engine.Check() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result.Decision != tt.want {
				t.Errorf("Engine.Check() Decision = %v, want %v", result.Decision, tt.want)
			}
			if result.Score < 0 || result.Score > 100 {
				t.Errorf("Engine.Check() Score = %d, want 0-100", result.Score)
			}
		})
	}
}

func TestEngine_CheckLegacy(t *testing.T) {
	dnsResolver := &MockDNSResolver{}
	spf := NewSPF(dnsResolver)
	dkim := &DKIM{}
	dmarc := NewDMARC(dnsResolver)
	// greylist 设为 nil，避免首次发送被拒绝
	ratelimit := NewRateLimiter()

	engine := NewEngine(spf, dkim, dmarc, nil, ratelimit)

	tests := []struct {
		name    string
		req     *CheckRequest
		want    Decision
		wantErr bool
	}{
		{
			name: "正常邮件通过（旧版）",
			req: &CheckRequest{
				IP:     net.ParseIP("192.168.1.1"),
				From:   "sender@example.com",
				To:     "recipient@example.com",
				Domain: "example.com",
				HELO:   "mail.example.com",
				Headers: map[string]string{
					"From": "sender@example.com",
				},
				Body: []byte("Test body"),
			},
			want: DecisionAccept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := engine.CheckLegacy(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Engine.CheckLegacy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result.Decision != tt.want {
				t.Errorf("Engine.CheckLegacy() Decision = %v, want %v", result.Decision, tt.want)
			}
		})
	}
}

// MockDNSResolver 模拟 DNS 解析器
type MockDNSResolver struct{}

func (m *MockDNSResolver) LookupTXT(domain string) ([]string, error) {
	// 返回空的 TXT 记录
	return []string{}, nil
}

