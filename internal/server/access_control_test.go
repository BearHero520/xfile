package server

import "testing"

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
