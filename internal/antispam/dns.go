package antispam

import (
	"fmt"
	"net"
)

// DefaultDNSResolver 默认 DNS 解析器
type DefaultDNSResolver struct{}

// NewDefaultDNSResolver 创建默认 DNS 解析器
func NewDefaultDNSResolver() *DefaultDNSResolver {
	return &DefaultDNSResolver{}
}

// LookupTXT 查询 TXT 记录
func (r *DefaultDNSResolver) LookupTXT(domain string) ([]string, error) {
	txtRecords, err := net.LookupTXT(domain)
	if err != nil {
		return nil, fmt.Errorf("DNS TXT 查询失败: %w", err)
	}
	return txtRecords, nil
}

// LookupMX 查询 MX 记录
func (r *DefaultDNSResolver) LookupMX(domain string) ([]*net.MX, error) {
	mxRecords, err := net.LookupMX(domain)
	if err != nil {
		return nil, fmt.Errorf("DNS MX 查询失败: %w", err)
	}
	return mxRecords, nil
}

// LookupA 查询 A 记录
func (r *DefaultDNSResolver) LookupA(domain string) ([]net.IP, error) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return nil, fmt.Errorf("DNS A 查询失败: %w", err)
	}
	return ips, nil
}

