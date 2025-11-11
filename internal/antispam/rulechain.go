package antispam

import (
	"context"
	"fmt"
	"time"
)

// RuleChain 规则链
type RuleChain struct {
	rules []Rule
}

// Rule 规则接口
type Rule interface {
	Name() string
	Priority() int // 优先级（数字越小优先级越高）
	Check(ctx context.Context, req *CheckRequest) (*RuleResult, error)
}

// RuleResult 规则结果
type RuleResult struct {
	Action   Action // 动作
	Score    int    // 分数调整
	Reason   string // 原因
	Continue bool   // 是否继续执行下一个规则
}

// Action 动作
type Action int

const (
	ActionContinue Action = iota // 继续
	ActionAccept                 // 接受
	ActionQuarantine             // 隔离
	ActionTempReject             // 临时拒绝
	ActionReject                 // 拒绝
)

// String 返回动作的字符串表示
func (a Action) String() string {
	switch a {
	case ActionAccept:
		return "accept"
	case ActionQuarantine:
		return "quarantine"
	case ActionTempReject:
		return "temp_reject"
	case ActionReject:
		return "reject"
	default:
		return "continue"
	}
}

// NewRuleChain 创建规则链
func NewRuleChain() *RuleChain {
	return &RuleChain{
		rules: []Rule{},
	}
}

// AddRule 添加规则
func (r *RuleChain) AddRule(rule Rule) {
	r.rules = append(r.rules, rule)
	// 按优先级排序
	for i := len(r.rules) - 1; i > 0; i-- {
		if r.rules[i].Priority() < r.rules[i-1].Priority() {
			r.rules[i], r.rules[i-1] = r.rules[i-1], r.rules[i]
		} else {
			break
		}
	}
}

// Execute 执行规则链
func (r *RuleChain) Execute(ctx context.Context, req *CheckRequest) (*CheckResult, error) {
	result := &CheckResult{
		Score:    0,
		Reasons:  []string{},
		Decision: DecisionAccept,
	}

	// 按优先级执行规则
	for _, rule := range r.rules {
		ruleResult, err := rule.Check(ctx, req)
		if err != nil {
			// 规则执行错误，记录但继续
			result.Reasons = append(result.Reasons, fmt.Sprintf("规则 %s 执行失败: %v", rule.Name(), err))
			continue
		}

		// 应用分数
		result.Score += ruleResult.Score
		if ruleResult.Reason != "" {
			result.Reasons = append(result.Reasons, ruleResult.Reason)
		}

		// 根据动作决定是否继续
		switch ruleResult.Action {
		case ActionAccept:
			result.Decision = DecisionAccept
			return result, nil
		case ActionReject:
			result.Decision = DecisionReject
			return result, nil
		case ActionQuarantine:
			result.Decision = DecisionQuarantine
			if !ruleResult.Continue {
				return result, nil
			}
		case ActionTempReject:
			result.Decision = DecisionTempReject
			if !ruleResult.Continue {
				return result, nil
			}
		}
	}

	// 根据最终分数决定
	if result.Score >= 100 {
		result.Decision = DecisionReject
	} else if result.Score >= 50 {
		result.Decision = DecisionQuarantine
	} else if result.Score >= 30 {
		result.Decision = DecisionTempReject
	}

	return result, nil
}

// RateLimitRule 速率限制规则
type RateLimitRule struct {
	limiter *RateLimiter
	limit   int
	window  time.Duration
}

// NewRateLimitRule 创建速率限制规则
func NewRateLimitRule(limiter *RateLimiter, limit int, window time.Duration) *RateLimitRule {
	return &RateLimitRule{
		limiter: limiter,
		limit:   limit,
		window:  window,
	}
}

// Name 返回规则名称
func (r *RateLimitRule) Name() string {
	return "rate_limit"
}

// Priority 返回优先级
func (r *RateLimitRule) Priority() int {
	return 1 // 最高优先级
}

// Check 检查速率限制
func (r *RateLimitRule) Check(ctx context.Context, req *CheckRequest) (*RuleResult, error) {
	if r.limiter == nil {
		return &RuleResult{Action: ActionContinue, Continue: true}, nil
	}

	allowed := r.limiter.CheckIP(req.IP.String(), r.limit, r.window)
	if !allowed {
		return &RuleResult{
			Action:   ActionReject,
			Score:    50,
			Reason:   "速率限制：IP 发送频率过高",
			Continue: false,
		}, nil
	}

	return &RuleResult{Action: ActionContinue, Continue: true}, nil
}

// GreylistRule 灰名单规则
type GreylistRule struct {
	greylist *Greylist
}

// NewGreylistRule 创建灰名单规则
func NewGreylistRule(greylist *Greylist) *GreylistRule {
	return &GreylistRule{
		greylist: greylist,
	}
}

// Name 返回规则名称
func (r *GreylistRule) Name() string {
	return "greylist"
}

// Priority 返回优先级
func (r *GreylistRule) Priority() int {
	return 2
}

// Check 检查灰名单
func (r *GreylistRule) Check(ctx context.Context, req *CheckRequest) (*RuleResult, error) {
	if r.greylist == nil {
		return &RuleResult{Action: ActionContinue, Continue: true}, nil
	}

	allowed, err := r.greylist.Check(ctx, req.IP.String(), req.From, req.To)
	if err != nil {
		return &RuleResult{Action: ActionContinue, Continue: true}, err
	}

	if !allowed {
		return &RuleResult{
			Action:   ActionTempReject,
			Score:    30,
			Reason:   "灰名单：首次发送，需要延迟",
			Continue: false,
		}, nil
	}

	return &RuleResult{Action: ActionContinue, Continue: true}, nil
}

// SPFRule SPF 规则
type SPFRule struct {
	spf *SPF
}

// NewSPFRule 创建 SPF 规则
func NewSPFRule(spf *SPF) *SPFRule {
	return &SPFRule{
		spf: spf,
	}
}

// Name 返回规则名称
func (r *SPFRule) Name() string {
	return "spf"
}

// Priority 返回优先级
func (r *SPFRule) Priority() int {
	return 3
}

// Check 检查 SPF
func (r *SPFRule) Check(ctx context.Context, req *CheckRequest) (*RuleResult, error) {
	if r.spf == nil || req.Domain == "" {
		return &RuleResult{Action: ActionContinue, Continue: true}, nil
	}

	spfResult, err := r.spf.Check(req.IP, req.Domain, req.HELO)
	if err != nil {
		return &RuleResult{Action: ActionContinue, Continue: true}, err
	}

	switch spfResult {
	case ResultFail:
		return &RuleResult{
			Action:   ActionContinue,
			Score:    40,
			Reason:   "SPF 验证失败",
			Continue: true,
		}, nil
	case ResultSoftFail:
		return &RuleResult{
			Action:   ActionContinue,
			Score:    20,
			Reason:   "SPF 软失败",
			Continue: true,
		}, nil
	case ResultPass:
		return &RuleResult{
			Action:   ActionContinue,
			Score:    -10,
			Reason:   "SPF 验证通过",
			Continue: true,
		}, nil
	default:
		return &RuleResult{Action: ActionContinue, Continue: true}, nil
	}
}

// DKIMRule DKIM 规则
type DKIMRule struct {
	dkim *DKIM
}

// NewDKIMRule 创建 DKIM 规则
func NewDKIMRule(dkim *DKIM) *DKIMRule {
	return &DKIMRule{
		dkim: dkim,
	}
}

// Name 返回规则名称
func (r *DKIMRule) Name() string {
	return "dkim"
}

// Priority 返回优先级
func (r *DKIMRule) Priority() int {
	return 4
}

// Check 检查 DKIM
func (r *DKIMRule) Check(ctx context.Context, req *CheckRequest) (*RuleResult, error) {
	if r.dkim == nil || req.DKIMSignature == "" {
		return &RuleResult{Action: ActionContinue, Continue: true}, nil
	}

	valid, err := r.dkim.Verify(req.Headers, req.Body, req.DKIMSignature)
	if err != nil {
		return &RuleResult{Action: ActionContinue, Continue: true}, err
	}

	if !valid {
		return &RuleResult{
			Action:   ActionContinue,
			Score:    30,
			Reason:   "DKIM 验证失败",
			Continue: true,
		}, nil
	}

	return &RuleResult{
		Action:   ActionContinue,
		Score:    -15,
		Reason:   "DKIM 验证通过",
		Continue: true,
	}, nil
}

// DMARCRule DMARC 规则
type DMARCRule struct {
	dmarc *DMARC
	spf   *SPF
	dkim  *DKIM
}

// NewDMARCRule 创建 DMARC 规则
func NewDMARCRule(dmarc *DMARC, spf *SPF, dkim *DKIM) *DMARCRule {
	return &DMARCRule{
		dmarc: dmarc,
		spf:   spf,
		dkim:  dkim,
	}
}

// Name 返回规则名称
func (r *DMARCRule) Name() string {
	return "dmarc"
}

// Priority 返回优先级
func (r *DMARCRule) Priority() int {
	return 5
}

// Check 检查 DMARC
func (r *DMARCRule) Check(ctx context.Context, req *CheckRequest) (*RuleResult, error) {
	if r.dmarc == nil || req.Domain == "" {
		return &RuleResult{Action: ActionContinue, Continue: true}, nil
	}

	// 获取 SPF 结果
	spfResult := ResultNone
	if r.spf != nil {
		spfResult, _ = r.spf.Check(req.IP, req.Domain, req.HELO)
	}

	// 获取 DKIM 结果
	dkimValid := false
	if r.dkim != nil && req.DKIMSignature != "" {
		dkimValid, _ = r.dkim.Verify(req.Headers, req.Body, req.DKIMSignature)
	}

	policy, err := r.dmarc.Check(req.Domain, spfResult, dkimValid)
	if err != nil {
		return &RuleResult{Action: ActionContinue, Continue: true}, err
	}

	switch policy {
	case PolicyReject:
		return &RuleResult{
			Action:   ActionReject,
			Score:    50,
			Reason:   "DMARC 策略：拒绝",
			Continue: false,
		}, nil
	case PolicyQuarantine:
		return &RuleResult{
			Action:   ActionQuarantine,
			Score:    30,
			Reason:   "DMARC 策略：隔离",
			Continue: true,
		}, nil
	default:
		return &RuleResult{Action: ActionContinue, Continue: true}, nil
	}
}

// HELORule HELO 规则
type HELORule struct{}

// NewHELORule 创建 HELO 规则
func NewHELORule() *HELORule {
	return &HELORule{}
}

// Name 返回规则名称
func (r *HELORule) Name() string {
	return "helo"
}

// Priority 返回优先级
func (r *HELORule) Priority() int {
	return 6
}

// Check 检查 HELO
func (r *HELORule) Check(ctx context.Context, req *CheckRequest) (*RuleResult, error) {
	if req.HELO == "" {
		return &RuleResult{
			Action:   ActionContinue,
			Score:    10,
			Reason:   "HELO 主机名为空",
			Continue: true,
		}, nil
	}

	if req.HELO == "localhost" {
		return &RuleResult{
			Action:   ActionContinue,
			Score:    10,
			Reason:   "HELO 主机名无效",
			Continue: true,
		}, nil
	}

	return &RuleResult{Action: ActionContinue, Continue: true}, nil
}

