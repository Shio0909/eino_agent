package security

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"eino_agent/internal/config"
)

// ValidateExternalURL 校验 URL 是否允许被知识库导入抓取。
func ValidateExternalURL(raw string, cfg config.URLPolicyConfig) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("url 非法: %w", err)
	}

	if !containsFold(cfg.AllowedSchemes, parsed.Scheme) {
		return nil, fmt.Errorf("url 非法，仅支持 %s", strings.Join(cfg.AllowedSchemes, "/"))
	}

	host := parsed.Hostname()
	if host == "" {
		return nil, fmt.Errorf("url 缺少主机名")
	}
	if containsFold(cfg.BlockedHosts, host) {
		return nil, fmt.Errorf("目标主机不允许访问")
	}
	if len(cfg.AllowedDomains) > 0 && !matchesDomainList(host, cfg.AllowedDomains) {
		return nil, fmt.Errorf("目标域名不在允许列表中")
	}
	if matchesDomainList(host, cfg.BlockedDomains) {
		return nil, fmt.Errorf("目标域名在阻止列表中")
	}

	if cfg.AllowPrivateNetworks {
		return parsed, nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("解析目标地址失败: %w", err)
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			continue
		}
		if isPrivateOrLocal(addr.Unmap()) {
			return nil, fmt.Errorf("目标地址属于私有或本地网络，已拒绝访问")
		}
	}

	return parsed, nil
}

func containsFold(items []string, want string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), want) {
			return true
		}
	}
	return false
}

func matchesDomainList(host string, domains []string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" {
			continue
		}
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func isPrivateOrLocal(addr netip.Addr) bool {
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return true
	}
	if addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	if addr.Is6() {
		if addr.String() == "::1" {
			return true
		}
		return addr.IsInterfaceLocalMulticast()
	}
	reserved := []netip.Prefix{
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("224.0.0.0/4"),
	}
	return slices.ContainsFunc(reserved, func(prefix netip.Prefix) bool { return prefix.Contains(addr) })
}
