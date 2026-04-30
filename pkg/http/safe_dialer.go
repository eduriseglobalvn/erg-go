package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

var ErrUnsafeAddress = errors.New("http: unsafe private or local address")

type SafeDialerConfig struct {
	AllowPrivateNetworks bool
	Timeout              time.Duration
	KeepAlive            time.Duration
}

func SafeDialContext(cfg SafeDialerConfig) func(context.Context, string, string) (net.Conn, error) {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	keepAlive := cfg.KeepAlive
	if keepAlive <= 0 {
		keepAlive = 30 * time.Second
	}
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: keepAlive,
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("http.safe_dialer: split host/port: %w", err)
		}
		if strings.TrimSpace(host) == "" {
			return nil, fmt.Errorf("http.safe_dialer: empty host")
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("http.safe_dialer: lookup %s: %w", host, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("http.safe_dialer: no addresses for %s", host)
		}
		for _, addr := range ips {
			if !cfg.AllowPrivateNetworks && IsUnsafeAddress(addr.IP) {
				return nil, fmt.Errorf("%w: %s resolved to %s", ErrUnsafeAddress, host, addr.IP.String())
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
	}
}

func IsUnsafeAddress(ip net.IP) bool {
	if ip == nil {
		return true
	}
	ip = ip.To16()
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}
