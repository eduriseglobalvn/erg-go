package service

import "testing"

func TestContinentForCountryCode(t *testing.T) {
	if got := continentForCountryCode("VN"); got != "AS" {
		t.Fatalf("VN continent = %q, want AS", got)
	}
	if got := continentForCountryCode("US"); got != "" {
		t.Fatalf("US continent = %q, want empty/non-Asia", got)
	}
}

func TestIPMatchesAllowlistEntry(t *testing.T) {
	cases := []struct {
		ip    string
		entry string
		want  bool
	}{
		{ip: "203.0.113.10", entry: "203.0.113.10", want: true},
		{ip: "203.0.113.10", entry: "203.0.113.0/24", want: true},
		{ip: "203.0.114.10", entry: "203.0.113.0/24", want: false},
		{ip: "not-ip", entry: "203.0.113.0/24", want: false},
	}
	for _, tc := range cases {
		if got := ipMatchesAllowlistEntry(tc.ip, tc.entry); got != tc.want {
			t.Fatalf("ipMatchesAllowlistEntry(%q, %q) = %v, want %v", tc.ip, tc.entry, got, tc.want)
		}
	}
}

func TestPrivateOrLoopbackIPSkipsGeoBlock(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1", "10.0.0.8", "192.168.1.20"} {
		if !isPrivateOrLoopbackIP(ip) {
			t.Fatalf("%s should be private/loopback", ip)
		}
	}
}

func TestAllowedContinent(t *testing.T) {
	if !isAllowedContinent("as", []string{"AS"}) {
		t.Fatal("AS should be allowed")
	}
	if isAllowedContinent("EU", []string{"AS"}) {
		t.Fatal("EU should not be allowed when only AS is configured")
	}
}
