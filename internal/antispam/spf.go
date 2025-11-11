package antispam

import (
	"fmt"
	"net"
	"strings"

	"github.com/gomailzero/gmz/internal/logger"
)

// SPF SPF 验证器
type SPF struct {
	dnsResolver DNSResolver
}

// DNSResolver DNS 解析器接口
type DNSResolver interface {
	LookupTXT(domain string) ([]string, error)
}

// NewSPF 创建 SPF 验证器
func NewSPF(resolver DNSResolver) *SPF {
	return &SPF{
		dnsResolver: resolver,
	}
}

// Check 检查 SPF 记录
func (s *SPF) Check(ip net.IP, domain string, helo string) (Result, error) {
	// 获取 SPF 记录
	spfRecord, err := s.getSPFRecord(domain)
	if err != nil {
		logger.Debug().Err(err).Str("domain", domain).Msg("获取 SPF 记录失败")
		return ResultNone, nil // SPF 记录不存在不算失败
	}

	// 解析 SPF 记录
	mechanisms, err := s.parseSPFRecord(spfRecord)
	if err != nil {
		return ResultFail, fmt.Errorf("解析 SPF 记录失败: %w", err)
	}

	// 检查机制
	for _, mech := range mechanisms {
		result, err := s.checkMechanism(mech, ip, domain, helo)
		if err != nil {
			continue
		}
		if result != ResultNone {
			return result, nil
		}
	}

	// 默认结果（如果没有匹配的机制）
	return ResultNeutral, nil
}

// getSPFRecord 获取 SPF 记录
func (s *SPF) getSPFRecord(domain string) (string, error) {
	txtRecords, err := s.dnsResolver.LookupTXT(domain)
	if err != nil {
		return "", fmt.Errorf("DNS 查询失败: %w", err)
	}

	// 查找 SPF 记录
	for _, record := range txtRecords {
		if strings.HasPrefix(record, "v=spf1") {
			return record, nil
		}
	}

	return "", fmt.Errorf("未找到 SPF 记录")
}

// parseSPFRecord 解析 SPF 记录
func (s *SPF) parseSPFRecord(record string) ([]string, error) {
	// 简化实现：仅提取机制
	parts := strings.Fields(record)
	if len(parts) < 2 {
		return nil, fmt.Errorf("无效的 SPF 记录格式")
	}

	if parts[0] != "v=spf1" {
		return nil, fmt.Errorf("无效的 SPF 版本")
	}

	return parts[1:], nil
}

// checkMechanism 检查 SPF 机制
func (s *SPF) checkMechanism(mech string, ip net.IP, domain string, helo string) (Result, error) {
	// 处理限定符
	qualifier := ResultPass
	if strings.HasPrefix(mech, "+") {
		qualifier = ResultPass
		mech = mech[1:]
	} else if strings.HasPrefix(mech, "-") {
		qualifier = ResultFail
		mech = mech[1:]
	} else if strings.HasPrefix(mech, "~") {
		qualifier = ResultSoftFail
		mech = mech[1:]
	} else if strings.HasPrefix(mech, "?") {
		qualifier = ResultNeutral
		mech = mech[1:]
	}

	// 检查机制类型
	switch {
	case mech == "all":
		return qualifier, nil
	case mech == "ip4" || strings.HasPrefix(mech, "ip4:"):
		return s.checkIP4(mech, ip, qualifier)
	case mech == "ip6" || strings.HasPrefix(mech, "ip6:"):
		return s.checkIP6(mech, ip, qualifier)
	case mech == "a" || strings.HasPrefix(mech, "a:"):
		return s.checkA(mech, ip, domain, qualifier)
	case mech == "mx" || strings.HasPrefix(mech, "mx:"):
		return s.checkMX(mech, ip, domain, qualifier)
	case strings.HasPrefix(mech, "include:"):
		return s.checkInclude(mech, ip, qualifier)
	default:
		return ResultNone, nil
	}
}

// checkIP4 检查 IPv4 机制
func (s *SPF) checkIP4(mech string, ip net.IP, qualifier Result) (Result, error) {
	if ip.To4() == nil {
		return ResultNone, nil
	}

	if mech == "ip4" {
		// 需要从 HELO 获取 IP（简化实现）
		return ResultNone, nil
	}

	// 解析 IP 地址或 CIDR
	ipStr := strings.TrimPrefix(mech, "ip4:")
	if strings.Contains(ipStr, "/") {
		_, ipNet, err := net.ParseCIDR(ipStr)
		if err != nil {
			return ResultNone, err
		}
		if ipNet.Contains(ip) {
			return qualifier, nil
		}
	} else {
		expectedIP := net.ParseIP(ipStr)
		if expectedIP != nil && expectedIP.Equal(ip) {
			return qualifier, nil
		}
	}

	return ResultNone, nil
}

// checkIP6 检查 IPv6 机制
func (s *SPF) checkIP6(mech string, ip net.IP, qualifier Result) (Result, error) {
	if ip.To4() != nil {
		return ResultNone, nil
	}

	// 类似 IPv4 检查
	return ResultNone, nil
}

// checkA 检查 A 记录机制
func (s *SPF) checkA(mech string, ip net.IP, domain string, qualifier Result) (Result, error) {
	// TODO: 实现 A 记录检查
	return ResultNone, nil
}

// checkMX 检查 MX 记录机制
func (s *SPF) checkMX(mech string, ip net.IP, domain string, qualifier Result) (Result, error) {
	// TODO: 实现 MX 记录检查
	return ResultNone, nil
}

// checkInclude 检查 include 机制
func (s *SPF) checkInclude(mech string, ip net.IP, qualifier Result) (Result, error) {
	// TODO: 实现 include 检查
	return ResultNone, nil
}

// Result SPF 检查结果
type Result int

const (
	ResultNone      Result = iota // 无结果
	ResultPass                     // 通过
	ResultFail                     // 失败
	ResultSoftFail                 // 软失败
	ResultNeutral                  // 中性
	ResultTempError                // 临时错误
	ResultPermError                // 永久错误
)

// String 返回结果的字符串表示
func (r Result) String() string {
	switch r {
	case ResultPass:
		return "pass"
	case ResultFail:
		return "fail"
	case ResultSoftFail:
		return "softfail"
	case ResultNeutral:
		return "neutral"
	case ResultTempError:
		return "temperror"
	case ResultPermError:
		return "permerror"
	default:
		return "none"
	}
}

