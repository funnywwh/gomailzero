package antispam

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gomailzero/gmz/internal/logger"
)

// Engine 反垃圾邮件引擎
type Engine struct {
	spf      *SPF
	dkim     *DKIM
	dmarc    *DMARC
	greylist *Greylist
	ratelimit *RateLimiter
	scorer   *Scorer
	chain    *RuleChain
	mu       sync.RWMutex
}

// NewEngine 创建反垃圾邮件引擎
func NewEngine(spf *SPF, dkim *DKIM, dmarc *DMARC, greylist *Greylist, ratelimit *RateLimiter) *Engine {
	engine := &Engine{
		spf:      spf,
		dkim:     dkim,
		dmarc:    dmarc,
		greylist: greylist,
		ratelimit: ratelimit,
		scorer:   NewScorer(),
		chain:    NewRuleChain(),
	}

	// 构建规则链
	if ratelimit != nil {
		engine.chain.AddRule(NewRateLimitRule(ratelimit, 100, 1*time.Minute))
	}
	if greylist != nil {
		engine.chain.AddRule(NewGreylistRule(greylist))
	}
	if spf != nil {
		engine.chain.AddRule(NewSPFRule(spf))
	}
	if dkim != nil {
		engine.chain.AddRule(NewDKIMRule(dkim))
	}
	if dmarc != nil {
		engine.chain.AddRule(NewDMARCRule(dmarc, spf, dkim))
	}
	engine.chain.AddRule(NewHELORule())

	return engine
}

// Check 检查邮件（使用规则链）
func (e *Engine) Check(ctx context.Context, req *CheckRequest) (*CheckResult, error) {
	// 使用规则链执行检查
	return e.chain.Execute(ctx, req)
}

// CheckLegacy 检查邮件（旧版实现，保留用于兼容）
func (e *Engine) CheckLegacy(ctx context.Context, req *CheckRequest) (*CheckResult, error) {
	result := &CheckResult{
		Score:    0,
		Reasons:  []string{},
		Decision: DecisionAccept,
	}

	// 1. 速率限制检查
	if e.ratelimit != nil {
		if !e.ratelimit.CheckIP(req.IP.String(), 100, 1*time.Minute) {
			result.Score += 50
			result.Reasons = append(result.Reasons, "速率限制：IP 发送频率过高")
			result.Decision = DecisionReject
			return result, nil
		}
	}

	// 2. 灰名单检查
	if e.greylist != nil {
		allowed, err := e.greylist.Check(ctx, req.IP.String(), req.From, req.To)
		if err != nil {
			logger.Warn().Err(err).Msg("灰名单检查失败")
		} else if !allowed {
			result.Score += 30
			result.Reasons = append(result.Reasons, "灰名单：首次发送，需要延迟")
			result.Decision = DecisionTempReject
			return result, nil
		}
	}

	// 3. SPF 检查
	spfResult := ResultNone
	if e.spf != nil && req.Domain != "" {
		var err error
		spfResult, err = e.spf.Check(req.IP, req.Domain, req.HELO)
		if err != nil {
			logger.Warn().Err(err).Msg("SPF 检查失败")
		} else {
			switch spfResult {
			case ResultFail:
				result.Score += 40
				result.Reasons = append(result.Reasons, "SPF 验证失败")
			case ResultSoftFail:
				result.Score += 20
				result.Reasons = append(result.Reasons, "SPF 软失败")
			case ResultPass:
				result.Score -= 10
				result.Reasons = append(result.Reasons, "SPF 验证通过")
			}
		}
	}

	// 4. DKIM 检查
	dkimValid := false
	if e.dkim != nil && req.DKIMSignature != "" {
		var err error
		dkimValid, err = e.dkim.Verify(req.Headers, req.Body, req.DKIMSignature)
		if err != nil {
			logger.Warn().Err(err).Msg("DKIM 验证失败")
		} else if !dkimValid {
			result.Score += 30
			result.Reasons = append(result.Reasons, "DKIM 验证失败")
		} else {
			result.Score -= 15
			result.Reasons = append(result.Reasons, "DKIM 验证通过")
		}
	}

	// 5. DMARC 检查
	if e.dmarc != nil && req.Domain != "" {
		dmarcPolicy, err := e.dmarc.Check(req.Domain, spfResult, dkimValid)
		if err != nil {
			logger.Warn().Err(err).Msg("DMARC 检查失败")
		} else {
			switch dmarcPolicy {
			case PolicyReject:
				result.Score += 50
				result.Reasons = append(result.Reasons, "DMARC 策略：拒绝")
				result.Decision = DecisionReject
			case PolicyQuarantine:
				result.Score += 30
				result.Reasons = append(result.Reasons, "DMARC 策略：隔离")
			}
		}
	}

	// 6. HELO 检查
	if req.HELO != "" {
		if err := e.checkHELO(req.HELO, req.IP); err != nil {
			result.Score += 10
			result.Reasons = append(result.Reasons, fmt.Sprintf("HELO 检查失败: %v", err))
		}
	}

	// 根据分数决定
	if result.Score >= 100 {
		result.Decision = DecisionReject
	} else if result.Score >= 50 {
		result.Decision = DecisionQuarantine
	} else if result.Score >= 30 {
		result.Decision = DecisionTempReject
	}

	return result, nil
}

// checkHELO 检查 HELO 主机名
func (e *Engine) checkHELO(helo string, ip net.IP) error {
	// 检查 HELO 是否与 IP 匹配
	// 简化实现：检查是否为有效的域名格式
	if helo == "" || helo == "localhost" {
		return fmt.Errorf("无效的 HELO 主机名")
	}
	return nil
}

// CheckRequest 检查请求
type CheckRequest struct {
	IP            net.IP
	From          string
	To            string
	Domain        string
	HELO          string
	Headers       map[string]string
	Body          []byte
	DKIMSignature string
}

// CheckResult 检查结果
type CheckResult struct {
	Score    int      // 垃圾邮件分数（0-100）
	Reasons  []string // 原因列表
	Decision Decision // 决策
}

// Decision 决策
type Decision int

const (
	DecisionAccept     Decision = iota // 接受
	DecisionQuarantine                // 隔离
	DecisionTempReject                // 临时拒绝
	DecisionReject                    // 拒绝
)

// String 返回决策的字符串表示
func (d Decision) String() string {
	switch d {
	case DecisionAccept:
		return "accept"
	case DecisionQuarantine:
		return "quarantine"
	case DecisionTempReject:
		return "temp_reject"
	case DecisionReject:
		return "reject"
	default:
		return "unknown"
	}
}

