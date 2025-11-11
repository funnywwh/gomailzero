package antispam

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestRuleChain_Execute(t *testing.T) {
	chain := NewRuleChain()

	// 添加测试规则
	chain.AddRule(&TestRule{
		name:     "test1",
		priority: 1,
		action:   ActionContinue,
		score:    10,
	})

	chain.AddRule(&TestRule{
		name:     "test2",
		priority: 2,
		action:   ActionAccept,
		score:    0,
	})

	req := &CheckRequest{
		IP:   net.ParseIP("192.168.1.1"),
		From: "sender@example.com",
		To:   "recipient@example.com",
	}

	ctx := context.Background()
	result, err := chain.Execute(ctx, req)
	if err != nil {
		t.Fatalf("RuleChain.Execute() error = %v", err)
	}

	if result.Decision != DecisionAccept {
		t.Errorf("RuleChain.Execute() Decision = %v, want %v", result.Decision, DecisionAccept)
	}

	if result.Score != 10 {
		t.Errorf("RuleChain.Execute() Score = %d, want 10", result.Score)
	}
}

func TestRuleChain_Priority(t *testing.T) {
	chain := NewRuleChain()

	// 添加不同优先级的规则
	chain.AddRule(&TestRule{
		name:     "low",
		priority: 10,
		action:   ActionContinue,
		score:    1,
	})

	chain.AddRule(&TestRule{
		name:     "high",
		priority: 1,
		action:   ActionContinue,
		score:    2,
	})

	chain.AddRule(&TestRule{
		name:     "medium",
		priority: 5,
		action:   ActionContinue,
		score:    3,
	})

	req := &CheckRequest{
		IP: net.ParseIP("192.168.1.1"),
	}

	ctx := context.Background()
	result, err := chain.Execute(ctx, req)
	if err != nil {
		t.Fatalf("RuleChain.Execute() error = %v", err)
	}

	// 应该按优先级执行：high(2) -> medium(3) -> low(1) = 6
	if result.Score != 6 {
		t.Errorf("RuleChain.Execute() Score = %d, want 6", result.Score)
	}
}

func TestRateLimitRule(t *testing.T) {
	limiter := NewRateLimiter()
	rule := NewRateLimitRule(limiter, 2, 1*time.Second)

	req := &CheckRequest{
		IP: net.ParseIP("192.168.1.1"),
	}

	ctx := context.Background()

	// 第一次应该通过
	result, err := rule.Check(ctx, req)
	if err != nil {
		t.Fatalf("RateLimitRule.Check() error = %v", err)
	}
	if result.Action != ActionContinue {
		t.Errorf("RateLimitRule.Check() Action = %v, want %v", result.Action, ActionContinue)
	}

	// 第二次应该通过
	result, err = rule.Check(ctx, req)
	if err != nil {
		t.Fatalf("RateLimitRule.Check() error = %v", err)
	}
	if result.Action != ActionContinue {
		t.Errorf("RateLimitRule.Check() Action = %v, want %v", result.Action, ActionContinue)
	}

	// 第三次应该被拒绝（超过限制）
	result, err = rule.Check(ctx, req)
	if err != nil {
		t.Fatalf("RateLimitRule.Check() error = %v", err)
	}
	if result.Action != ActionReject {
		t.Errorf("RateLimitRule.Check() Action = %v, want %v", result.Action, ActionReject)
	}
}

func TestSPFRule(t *testing.T) {
	dnsResolver := &MockDNSResolver{}
	spf := NewSPF(dnsResolver)
	rule := NewSPFRule(spf)

	req := &CheckRequest{
		IP:     net.ParseIP("192.168.1.1"),
		Domain: "example.com",
		HELO:   "mail.example.com",
	}

	ctx := context.Background()
	result, err := rule.Check(ctx, req)
	if err != nil {
		t.Fatalf("SPFRule.Check() error = %v", err)
	}

	if result.Action != ActionContinue {
		t.Errorf("SPFRule.Check() Action = %v, want %v", result.Action, ActionContinue)
	}
}

func TestHELORule(t *testing.T) {
	rule := NewHELORule()

	tests := []struct {
		name    string
		helo    string
		want    Action
		wantScore int
	}{
		{
			name:     "有效 HELO",
			helo:     "mail.example.com",
			want:     ActionContinue,
			wantScore: 0,
		},
		{
			name:     "localhost HELO",
			helo:     "localhost",
			want:     ActionContinue,
			wantScore: 10,
		},
		{
			name:     "空 HELO",
			helo:     "",
			want:     ActionContinue,
			wantScore: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &CheckRequest{
				IP:   net.ParseIP("192.168.1.1"),
				HELO: tt.helo,
			}

			ctx := context.Background()
			result, err := rule.Check(ctx, req)
			if err != nil {
				t.Fatalf("HELORule.Check() error = %v", err)
			}

			if result.Action != tt.want {
				t.Errorf("HELORule.Check() Action = %v, want %v", result.Action, tt.want)
			}

			if result.Score != tt.wantScore {
				t.Errorf("HELORule.Check() Score = %d, want %d", result.Score, tt.wantScore)
			}
		})
	}
}

// TestRule 测试规则
type TestRule struct {
	name     string
	priority int
	action   Action
	score    int
	reason   string
}

func (r *TestRule) Name() string {
	return r.name
}

func (r *TestRule) Priority() int {
	return r.priority
}

func (r *TestRule) Check(ctx context.Context, req *CheckRequest) (*RuleResult, error) {
	return &RuleResult{
		Action:   r.action,
		Score:    r.score,
		Reason:   r.reason,
		Continue: r.action == ActionContinue,
	}, nil
}

