package server

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type downloadRateLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

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

func (s *Server) enforceDownloadLimit(w http.ResponseWriter, r *http.Request, path string) bool {
	limit, err := strconv.Atoi(s.store.SettingValue("downloadLimitPerMinute", "0"))
	if err != nil || limit < 1 {
		return true
	}
	if s.downloads.allow(clientIP(r), limit, time.Now()) {
		return true
	}
	_ = s.store.LogAccess("download-rate-limited", path, clientIP(r), r.UserAgent())
	writeError(w, http.StatusTooManyRequests, errors.New("download rate limit exceeded"))
	return false
}

func (l *downloadRateLimiter) allow(key string, limit int, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.hits == nil {
		l.hits = make(map[string][]time.Time)
	}
	windowStart := now.Add(-time.Minute)
	hits := l.hits[key]
	kept := hits[:0]
	for _, hit := range hits {
		if hit.After(windowStart) {
			kept = append(kept, hit)
		}
	}
	if len(kept) >= limit {
		l.hits[key] = kept
		return false
	}
	kept = append(kept, now)
	l.hits[key] = kept
	return true
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

func validateAccessSettings(settings map[string]string) error {
	if value, ok := settings["storageProvider"]; ok {
		if err := validateStorageProvider(value); err != nil {
			return err
		}
	}
	if value, ok := settings["allowUpload"]; ok {
		if err := validateSwitch(value, "上传开关"); err != nil {
			return err
		}
	}
	if value, ok := settings["uploadOverwrite"]; ok {
		if err := validateSwitch(value, "上传覆盖策略"); err != nil {
			return err
		}
	}
	if value, ok := settings["maxUploadMB"]; ok {
		limit, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || limit < 1 {
			return errors.New("上传上限必须是大于 0 的整数")
		}
	}
	if value, ok := settings["uploadAllowExtensions"]; ok {
		if err := validateExtensionRuleList(value, "允许扩展名"); err != nil {
			return err
		}
	}
	if value, ok := settings["uploadDenyExtensions"]; ok {
		if err := validateExtensionRuleList(value, "禁止扩展名"); err != nil {
			return err
		}
	}
	if value, ok := settings["uploadPathAllowList"]; ok {
		if err := validatePathRuleList(value, "允许上传路径"); err != nil {
			return err
		}
	}
	if value, ok := settings["uploadPathDenyList"]; ok {
		if err := validatePathRuleList(value, "禁止上传路径"); err != nil {
			return err
		}
	}
	if value, ok := settings["ipAllowList"]; ok {
		if err := validateIPRuleList(value, "IP 白名单"); err != nil {
			return err
		}
	}
	if value, ok := settings["ipDenyList"]; ok {
		if err := validateIPRuleList(value, "IP 黑名单"); err != nil {
			return err
		}
	}
	if value, ok := settings["refererProtection"]; ok {
		value = strings.TrimSpace(value)
		if value != "enabled" && value != "disabled" {
			return errors.New("Referer 防盗链只能是 enabled 或 disabled")
		}
	}
	if value, ok := settings["refererAllowList"]; ok {
		if err := validateRefererRuleList(value); err != nil {
			return err
		}
	}
	if value, ok := settings["downloadLimitPerMinute"]; ok {
		limit, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || limit < 0 {
			return errors.New("下载限频必须是大于或等于 0 的整数")
		}
	}
	if value, ok := settings["webdav"]; ok {
		if err := validateSwitch(value, "WebDAV 开关"); err != nil {
			return err
		}
	}
	if value, ok := settings["webdavReadOnly"]; ok {
		if err := validateSwitch(value, "WebDAV 只读模式"); err != nil {
			return err
		}
	}
	if value, ok := settings["webdavMountPath"]; ok {
		value = strings.TrimSpace(value)
		if value == "" || !strings.HasPrefix(value, "/") || strings.Contains(value, "..") || strings.ContainsAny(value, "?#") {
			return errors.New("WebDAV 挂载路径必须是有效的绝对路径")
		}
	}
	return nil
}

func validateSwitch(value, label string) error {
	value = strings.TrimSpace(value)
	if value != "enabled" && value != "disabled" {
		return fmt.Errorf("%s只能是 enabled 或 disabled", label)
	}
	return nil
}

func validateStorageProvider(value string) error {
	switch strings.TrimSpace(value) {
	case "local", "s3", "webdav", "aliyun_oss", "tencent_cos":
		return nil
	default:
		return errors.New("存储源类型无效")
	}
}

func validateIPRuleList(value, label string) error {
	for _, rule := range splitIPRules(value) {
		if strings.Contains(rule, "/") {
			if _, _, err := net.ParseCIDR(rule); err != nil {
				return fmt.Errorf("%s包含无效 CIDR：%s", label, rule)
			}
			continue
		}
		if net.ParseIP(rule) == nil {
			return fmt.Errorf("%s包含无效 IP：%s", label, rule)
		}
	}
	return nil
}

func validateRefererRuleList(value string) error {
	for _, rule := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		if strings.Contains(rule, "://") {
			parsed, err := url.Parse(rule)
			if err != nil || parsed.Host == "" {
				return fmt.Errorf("允许来源域名包含无效 URL：%s", rule)
			}
			rule = parsed.Host
		}
		host := cleanHost(strings.TrimPrefix(rule, "*."))
		if host == "" || strings.ContainsAny(host, "/?#") {
			return fmt.Errorf("允许来源域名包含无效规则：%s", rule)
		}
	}
	return nil
}

func validateExtensionRuleList(value, label string) error {
	for _, rule := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		if rule == "(none)" {
			continue
		}
		rule = strings.TrimPrefix(rule, "*")
		if !strings.HasPrefix(rule, ".") {
			rule = "." + rule
		}
		if len(rule) < 2 || strings.ContainsAny(rule, `/\:*?"<>|`) {
			return fmt.Errorf("%s包含无效扩展名：%s", label, rule)
		}
	}
	return nil
}

func validatePathRuleList(value, label string) error {
	for _, rule := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	}) {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		clean := strings.TrimPrefix(strings.ReplaceAll(rule, `\`, `/`), "/")
		if clean == "" || strings.Contains(clean, ":") || strings.HasPrefix(clean, "../") || clean == ".." || strings.Contains(clean, "/../") {
			return fmt.Errorf("%s包含无效路径：%s", label, rule)
		}
	}
	return nil
}
