package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"time"

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

// NewHTTPClient 返回带 SSRF 防护的 HTTP 客户端：
// 每次连接在拨号时重新校验解析到的 IP（覆盖 DNS 重绑定与解析时间差），
// 重定向目标同样按策略校验。
func NewHTTPClient(policy config.URLPolicyConfig, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	maxRedirects := policy.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 5
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: NewCheckedTransport(policy),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("重定向次数超过上限 %d", maxRedirects)
			}
			_, err := ValidateExternalURL(req.URL.String(), policy)
			return err
		},
	}
}

// NewCheckedTransport 返回在拨号时校验目标 IP 的 Transport。
func NewCheckedTransport(policy config.URLPolicyConfig) *http.Transport {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           checkedDialContext(dialer, policy),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

// checkedDialContext 在每次拨号前解析主机并逐个校验 IP，
// 只连接通过策略的地址，防止校验与连接之间的 DNS 结果变化（rebinding）。
func checkedDialContext(dialer *net.Dialer, policy config.URLPolicyConfig) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("地址非法: %w", err)
		}
		if policy.AllowPrivateNetworks {
			return dialer.DialContext(ctx, network, addr)
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("解析目标地址失败: %w", err)
		}
		var lastErr error
		for _, ip := range ips {
			netipAddr, ok := netip.AddrFromSlice(ip)
			if !ok {
				continue
			}
			if isPrivateOrLocal(netipAddr.Unmap()) {
				lastErr = fmt.Errorf("目标地址 %s 属于私有或本地网络，已拒绝访问", ip)
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err != nil {
				lastErr = err
				continue
			}
			return conn, nil
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("目标主机 %s 没有可用地址", host)
		}
		return nil, lastErr
	}
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
