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
