package server

import (
	"errors"
	"net"
	"net/http"
	"strings"
)

func (s *Server) accessControlled(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.ipAllowed(r) {
			ip := clientIP(r)
			_ = s.store.LogAccess("ip-blocked", r.URL.Path, ip, r.UserAgent())
			writeError(w, http.StatusForbidden, errors.New("ip address is not allowed"))
			return
		}
		next(w, r)
	}
}

func (s *Server) ipAllowed(r *http.Request) bool {
	ip := clientIP(r)
	if ipMatchesRules(ip, s.store.SettingValue("ipDenyList", "")) {
		return false
	}
	allowList := s.store.SettingValue("ipAllowList", "")
	rules := splitIPRules(allowList)
	if len(rules) == 0 {
		return true
	}
	return ipMatchesParsedRules(ip, rules)
}

func ipMatchesRules(ipText, rulesText string) bool {
	return ipMatchesParsedRules(ipText, splitIPRules(rulesText))
}

func ipMatchesParsedRules(ipText string, rules []string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipText))
	if ip == nil {
		return false
	}
	for _, rule := range rules {
		if strings.Contains(rule, "/") {
			_, network, err := net.ParseCIDR(rule)
			if err == nil && network.Contains(ip) {
				return true
			}
			continue
		}
		if ruleIP := net.ParseIP(rule); ruleIP != nil && ruleIP.Equal(ip) {
			return true
		}
	}
	return false
}

func splitIPRules(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	rules := make([]string, 0, len(fields))
	for _, field := range fields {
		if rule := strings.TrimSpace(field); rule != "" {
			rules = append(rules, rule)
		}
	}
	return rules
}
