package server

import (
	"errors"
	"net"
	"net/http"
	"net/url"
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

func (s *Server) publicLinkControlled(next http.HandlerFunc) http.HandlerFunc {
	return s.accessControlled(func(w http.ResponseWriter, r *http.Request) {
		if !s.refererAllowed(r) {
			_ = s.store.LogAccess("referer-blocked", r.URL.Path, clientIP(r), r.UserAgent())
			writeError(w, http.StatusForbidden, errors.New("referer is not allowed"))
			return
		}
		next(w, r)
	})
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

func (s *Server) refererAllowed(r *http.Request) bool {
	if s.store.SettingValue("refererProtection", "disabled") != "enabled" {
		return true
	}
	referer := strings.TrimSpace(r.Referer())
	if referer == "" {
		return true
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed.Host == "" {
		return false
	}
	return refererHostAllowed(parsed.Host, r.Host, s.store.SettingValue("refererAllowList", ""))
}

func refererHostAllowed(refererHost, requestHost, rulesText string) bool {
	host := cleanHost(refererHost)
	if host == "" {
		return false
	}
	if host == cleanHost(requestHost) {
		return true
	}
	for _, rule := range splitRefererRules(rulesText) {
		if hostMatchesRule(host, rule) {
			return true
		}
	}
	return false
}

func splitRefererRules(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	rules := make([]string, 0, len(fields))
	for _, field := range fields {
		rule := strings.TrimSpace(field)
		if rule == "" {
			continue
		}
		if parsed, err := url.Parse(rule); err == nil && parsed.Host != "" {
			rule = parsed.Host
		}
		if host := cleanHost(rule); host != "" {
			rules = append(rules, host)
		}
	}
	return rules
}

func hostMatchesRule(host, rule string) bool {
	rule = strings.TrimPrefix(rule, "*.")
	if strings.HasPrefix(rule, ".") {
		rule = strings.TrimPrefix(rule, ".")
	}
	return host == rule || strings.HasSuffix(host, "."+rule)
}

func cleanHost(value string) string {
	host := strings.ToLower(strings.TrimSpace(value))
	if host == "" {
		return ""
	}
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		return splitHost
	}
	return strings.Trim(host, "[]")
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
