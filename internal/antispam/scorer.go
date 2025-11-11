package antispam

import (
	"strings"
)

// Scorer 评分器
type Scorer struct {
	rules []ScoringRule
}

// ScoringRule 评分规则
type ScoringRule struct {
	Name   string
	Weight int                                    // 权重（-100 到 100）
	Check  func(req *CheckRequest) (bool, string) // 返回 (是否匹配, 原因)
}

// NewScorer 创建评分器
func NewScorer() *Scorer {
	return &Scorer{
		rules: []ScoringRule{
			// SPF 规则
			{
				Name:   "spf_pass",
				Weight: -10,
				Check: func(req *CheckRequest) (bool, string) {
					// 由引擎调用，这里仅占位
					return false, ""
				},
			},
			{
				Name:   "spf_fail",
				Weight: 40,
				Check: func(req *CheckRequest) (bool, string) {
					return false, ""
				},
			},
			{
				Name:   "spf_softfail",
				Weight: 20,
				Check: func(req *CheckRequest) (bool, string) {
					return false, ""
				},
			},
			// DKIM 规则
			{
				Name:   "dkim_pass",
				Weight: -15,
				Check: func(req *CheckRequest) (bool, string) {
					return false, ""
				},
			},
			{
				Name:   "dkim_fail",
				Weight: 30,
				Check: func(req *CheckRequest) (bool, string) {
					return false, ""
				},
			},
			// DMARC 规则
			{
				Name:   "dmarc_reject",
				Weight: 50,
				Check: func(req *CheckRequest) (bool, string) {
					return false, ""
				},
			},
			{
				Name:   "dmarc_quarantine",
				Weight: 30,
				Check: func(req *CheckRequest) (bool, string) {
					return false, ""
				},
			},
			// HELO 规则
			{
				Name:   "helo_invalid",
				Weight: 10,
				Check: func(req *CheckRequest) (bool, string) {
					if req.HELO == "" || req.HELO == "localhost" {
						return true, "HELO 主机名无效"
					}
					return false, ""
				},
			},
			{
				Name:   "helo_mismatch",
				Weight: 15,
				Check: func(req *CheckRequest) (bool, string) {
					// 检查 HELO 是否与发件人域名匹配
					if req.HELO != "" && req.Domain != "" {
						if !strings.HasSuffix(req.HELO, req.Domain) && req.HELO != req.Domain {
							return true, "HELO 与发件人域名不匹配"
						}
					}
					return false, ""
				},
			},
			// 内容规则
			{
				Name:   "suspicious_subject",
				Weight: 20,
				Check: func(req *CheckRequest) (bool, string) {
					subject := strings.ToLower(req.Headers["Subject"])
					suspicious := []string{
						"urgent", "act now", "limited time", "click here",
						"winner", "prize", "free money", "guaranteed",
					}
					for _, word := range suspicious {
						if strings.Contains(subject, word) {
							return true, "可疑的主题关键词"
						}
					}
					return false, ""
				},
			},
			{
				Name:   "suspicious_from",
				Weight: 15,
				Check: func(req *CheckRequest) (bool, string) {
					from := strings.ToLower(req.From)
					// 检查发件人地址格式
					if !strings.Contains(from, "@") {
						return true, "发件人地址格式无效"
					}
					// 检查可疑域名
					suspiciousDomains := []string{
						"noreply", "no-reply", "donotreply",
					}
					for _, domain := range suspiciousDomains {
						if strings.Contains(from, domain) {
							return true, "可疑的发件人域名"
						}
					}
					return false, ""
				},
			},
		},
	}
}

// Score 计算分数
func (s *Scorer) Score(req *CheckRequest, spfResult Result, dkimValid bool, dmarcPolicy Policy) (int, []string) {
	score := 0
	reasons := []string{}

	// 应用规则
	for _, rule := range s.rules {
		matched, reason := rule.Check(req)
		if matched {
			score += rule.Weight
			if reason != "" {
				reasons = append(reasons, reason)
			}
		}
	}

	// 应用 SPF 结果
	switch spfResult {
	case ResultPass:
		score -= 10
		reasons = append(reasons, "SPF 验证通过")
	case ResultFail:
		score += 40
		reasons = append(reasons, "SPF 验证失败")
	case ResultSoftFail:
		score += 20
		reasons = append(reasons, "SPF 软失败")
	}

	// 应用 DKIM 结果
	if dkimValid {
		score -= 15
		reasons = append(reasons, "DKIM 验证通过")
	} else if req.DKIMSignature != "" {
		score += 30
		reasons = append(reasons, "DKIM 验证失败")
	}

	// 应用 DMARC 结果
	switch dmarcPolicy {
	case PolicyReject:
		score += 50
		reasons = append(reasons, "DMARC 策略：拒绝")
	case PolicyQuarantine:
		score += 30
		reasons = append(reasons, "DMARC 策略：隔离")
	}

	// 确保分数在合理范围内
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score, reasons
}

// AddRule 添加自定义规则
func (s *Scorer) AddRule(rule ScoringRule) {
	s.rules = append(s.rules, rule)
}
