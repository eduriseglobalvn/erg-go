package http

import (
	"net"
	"testing"
)

func TestIsUnsafeAddressBlocksLocalAndPrivateRanges(t *testing.T) {
	for _, ip := range []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.1.1",
		"::1",
		"fc00::1",
	} {
		if !IsUnsafeAddress(net.ParseIP(ip)) {
			t.Fatalf("expected %s to be unsafe", ip)
		}
	}
}

func TestIsUnsafeAddressAllowsPublicAddress(t *testing.T) {
	if IsUnsafeAddress(net.ParseIP("8.8.8.8")) {
		t.Fatal("expected public IP to be allowed")
	}
}
