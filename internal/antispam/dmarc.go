package antispam

import (
	"fmt"
	"strings"
)

// DMARC DMARC 验证器
type DMARC struct {
	dnsResolver DNSResolver
}

// NewDMARC 创建 DMARC 验证器
func NewDMARC(resolver DNSResolver) *DMARC {
	return &DMARC{
		dnsResolver: resolver,
	}
}

// Check 检查 DMARC 记录
func (d *DMARC) Check(domain string, spfResult Result, dkimResult bool) (Policy, error) {
	// 获取 DMARC 记录
	dmarcRecord, err := d.getDMARCRecord(domain)
	if err != nil {
		return PolicyNone, nil // DMARC 记录不存在不算失败
	}

	// 解析 DMARC 记录
	policy, err := d.parseDMARCRecord(dmarcRecord)
	if err != nil {
		return PolicyNone, fmt.Errorf("解析 DMARC 记录失败: %w", err)
	}

	// 评估策略
	return d.evaluatePolicy(policy, spfResult, dkimResult), nil
}

// getDMARCRecord 获取 DMARC 记录
func (d *DMARC) getDMARCRecord(domain string) (string, error) {
	dmarcDomain := "_dmarc." + domain
	txtRecords, err := d.dnsResolver.LookupTXT(dmarcDomain)
	if err != nil {
		return "", fmt.Errorf("DNS 查询失败: %w", err)
	}

	// 查找 DMARC 记录
	for _, record := range txtRecords {
		if strings.HasPrefix(record, "v=DMARC1") {
			return record, nil
		}
	}

	return "", fmt.Errorf("未找到 DMARC 记录")
}

// parseDMARCRecord 解析 DMARC 记录
func (d *DMARC) parseDMARCRecord(record string) (map[string]string, error) {
	params := make(map[string]string)
	parts := strings.Split(record, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "v=DMARC1") {
			continue
		}
		if idx := strings.Index(part, "="); idx > 0 {
			key := strings.TrimSpace(part[:idx])
			val := strings.TrimSpace(part[idx+1:])
			params[key] = val
		}
	}

	return params, nil
}

// evaluatePolicy 评估 DMARC 策略
func (d *DMARC) evaluatePolicy(params map[string]string, spfResult Result, dkimResult bool) Policy {
	// 获取策略
	policyStr := params["p"]
	if policyStr == "" {
		policyStr = params["sp"] // 子域策略
	}

	// SPF 和 DKIM 都失败
	if spfResult != ResultPass && !dkimResult {
		// 根据策略返回结果
		switch strings.ToLower(policyStr) {
		case "reject":
			return PolicyReject
		case "quarantine":
			return PolicyQuarantine
		case "none":
			return PolicyNone
		default:
			return PolicyNone
		}
	}

	// SPF 或 DKIM 通过
	return PolicyNone
}

// Policy DMARC 策略
type Policy int

const (
	PolicyNone       Policy = iota // 无策略
	PolicyQuarantine               // 隔离
	PolicyReject                   // 拒绝
)

// String 返回策略的字符串表示
func (p Policy) String() string {
	switch p {
	case PolicyQuarantine:
		return "quarantine"
	case PolicyReject:
		return "reject"
	default:
		return "none"
	}
}
