package antispam

import (
	"net"
	"testing"
)

func TestScorer_Score(t *testing.T) {
	scorer := NewScorer()

	tests := []struct {
		name         string
		req          *CheckRequest
		spfResult    Result
		dkimValid    bool
		dmarcPolicy  Policy
		wantMinScore int
		wantMaxScore int
	}{
		{
			name: "SPF 通过",
			req: &CheckRequest{
				IP:     net.ParseIP("192.168.1.1"),
				Domain: "example.com",
				HELO:   "mail.example.com",
			},
			spfResult:    ResultPass,
			dkimValid:    false,
			dmarcPolicy:  PolicyNone,
			wantMinScore: 0,
			wantMaxScore: 20,
		},
		{
			name: "SPF 失败",
			req: &CheckRequest{
				IP:     net.ParseIP("192.168.1.1"),
				Domain: "example.com",
				HELO:   "mail.example.com", // 避免 HELO 规则加分
			},
			spfResult:    ResultFail,
			dkimValid:    false,
			dmarcPolicy:  PolicyNone,
			wantMinScore: 30,
			wantMaxScore: 70, // 可能包含其他规则加分
		},
		{
			name: "DKIM 通过",
			req: &CheckRequest{
				IP:            net.ParseIP("192.168.1.1"),
				DKIMSignature: "test",
			},
			spfResult:    ResultNone,
			dkimValid:    true,
			dmarcPolicy:  PolicyNone,
			wantMinScore: 0,
			wantMaxScore: 20,
		},
		{
			name: "DMARC 拒绝",
			req: &CheckRequest{
				IP:     net.ParseIP("192.168.1.1"),
				Domain: "example.com",
			},
			spfResult:    ResultFail,
			dkimValid:    false,
			dmarcPolicy:  PolicyReject,
			wantMinScore: 80,
			wantMaxScore: 100,
		},
		{
			name: "可疑主题",
			req: &CheckRequest{
				IP:   net.ParseIP("192.168.1.1"),
				HELO: "mail.example.com",
				Headers: map[string]string{
					"Subject": "URGENT: Click here now!",
				},
			},
			spfResult:    ResultNone,
			dkimValid:    false,
			dmarcPolicy:  PolicyNone,
			wantMinScore: 15,
			wantMaxScore: 35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, reasons := scorer.Score(tt.req, tt.spfResult, tt.dkimValid, tt.dmarcPolicy)

			if score < tt.wantMinScore || score > tt.wantMaxScore {
				t.Errorf("Scorer.Score() Score = %d, want %d-%d", score, tt.wantMinScore, tt.wantMaxScore)
			}

			if score < 0 || score > 100 {
				t.Errorf("Scorer.Score() Score = %d, want 0-100", score)
			}

			if len(reasons) == 0 && score > 0 {
				t.Errorf("Scorer.Score() should have reasons when score > 0")
			}
		})
	}
}

func TestScorer_AddRule(t *testing.T) {
	scorer := NewScorer()

	initialCount := len(scorer.rules)

	customRule := ScoringRule{
		Name:   "custom",
		Weight: 25,
		Check: func(req *CheckRequest) (bool, string) {
			return true, "custom rule matched"
		},
	}

	scorer.AddRule(customRule)

	if len(scorer.rules) != initialCount+1 {
		t.Errorf("Scorer.AddRule() rules count = %d, want %d", len(scorer.rules), initialCount+1)
	}
}

