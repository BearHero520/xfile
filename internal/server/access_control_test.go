package server

import (
	"testing"
	"time"
)

func TestIPMatchesRules(t *testing.T) {
	tests := []struct {
		name  string
		ip    string
		rules string
		want  bool
	}{
		{name: "exact", ip: "192.168.1.10", rules: "192.168.1.10", want: true},
		{name: "cidr", ip: "10.8.1.4", rules: "192.168.1.1, 10.0.0.0/8", want: true},
		{name: "ipv6", ip: "2001:db8::2", rules: "2001:db8::/32", want: true},
		{name: "miss", ip: "172.16.0.2", rules: "10.0.0.0/8", want: false},
		{name: "invalid ignored", ip: "127.0.0.1", rules: "not-an-ip", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ipMatchesRules(tt.ip, tt.rules); got != tt.want {
				t.Fatalf("ipMatchesRules(%q, %q) = %v, want %v", tt.ip, tt.rules, got, tt.want)
			}
		})
	}
}

func TestSplitIPRules(t *testing.T) {
	rules := splitIPRules("127.0.0.1, 10.0.0.0/8\n2001:db8::/32")
	if len(rules) != 3 {
		t.Fatalf("rules = %#v", rules)
	}
}

func TestRefererHostAllowed(t *testing.T) {
	tests := []struct {
		name        string
		refererHost string
		requestHost string
		rules       string
		want        bool
	}{
		{name: "same host", refererHost: "xfile.example.com", requestHost: "xfile.example.com", want: true},
		{name: "same host with port", refererHost: "xfile.example.com:443", requestHost: "xfile.example.com", want: true},
		{name: "allowed domain", refererHost: "cdn.example.com", requestHost: "xfile.example.com", rules: "example.com", want: true},
		{name: "allowed wildcard", refererHost: "assets.example.net", requestHost: "xfile.example.com", rules: "*.example.net", want: true},
		{name: "blocked", refererHost: "other.test", requestHost: "xfile.example.com", rules: "example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := refererHostAllowed(tt.refererHost, tt.requestHost, tt.rules); got != tt.want {
				t.Fatalf("refererHostAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDownloadRateLimiter(t *testing.T) {
	limiter := downloadRateLimiter{}
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)

	if !limiter.allow("127.0.0.1", 2, now) {
		t.Fatal("first download should be allowed")
	}
	if !limiter.allow("127.0.0.1", 2, now.Add(10*time.Second)) {
		t.Fatal("second download should be allowed")
	}
	if limiter.allow("127.0.0.1", 2, now.Add(20*time.Second)) {
		t.Fatal("third download inside window should be blocked")
	}
	if !limiter.allow("127.0.0.1", 2, now.Add(61*time.Second)) {
		t.Fatal("download after window should be allowed")
	}
}

func TestValidateAccessSettings(t *testing.T) {
	valid := map[string]string{
		"ipAllowList":            "127.0.0.1\n10.0.0.0/8",
		"ipDenyList":             "2001:db8::/32",
		"refererProtection":      "enabled",
		"refererAllowList":       "example.com\n*.cdn.example.com\nhttps://static.example.net",
		"downloadLimitPerMinute": "12",
		"webdav":                 "enabled",
		"webdavReadOnly":         "disabled",
		"webdavMountPath":        "/dav",
	}
	if err := validateAccessSettings(valid); err != nil {
		t.Fatalf("valid access settings failed: %v", err)
	}

	tests := []struct {
		name     string
		settings map[string]string
	}{
		{name: "invalid allow ip", settings: map[string]string{"ipAllowList": "bad-ip"}},
		{name: "invalid cidr", settings: map[string]string{"ipDenyList": "10.0.0.0/99"}},
		{name: "invalid referer switch", settings: map[string]string{"refererProtection": "yes"}},
		{name: "invalid referer url", settings: map[string]string{"refererAllowList": "https://"}},
		{name: "negative download limit", settings: map[string]string{"downloadLimitPerMinute": "-1"}},
		{name: "text download limit", settings: map[string]string{"downloadLimitPerMinute": "many"}},
		{name: "invalid webdav switch", settings: map[string]string{"webdav": "on"}},
		{name: "invalid webdav readonly switch", settings: map[string]string{"webdavReadOnly": "off"}},
		{name: "invalid webdav mount path", settings: map[string]string{"webdavMountPath": "dav"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateAccessSettings(tt.settings); err == nil {
				t.Fatal("expected validation to fail")
			}
		})
	}
}
