package server

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type requestRateLimiter struct {
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

func (s *Server) operationAllowed(operation string) bool {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		return true
	}
	for _, disabled := range splitOperationRules(s.store.SettingValue("disabledOperations", "")) {
		if disabled == operation {
			return false
		}
	}
	return true
}

func (s *Server) requireOperation(w http.ResponseWriter, r *http.Request, operation string) bool {
	if s.operationAllowed(operation) && s.userOperationAllowed(r, operation) {
		return true
	}
	_ = s.store.LogAccess("operation-blocked", operation+":"+r.URL.Path, clientIP(r), r.UserAgent())
	writeError(w, http.StatusForbidden, fmt.Errorf("%s operation is disabled", operation))
	return false
}

func (s *Server) userOperationAllowed(r *http.Request, operation string) bool {
	user, err := s.currentUser(r)
	if err != nil || user.Role == "super_admin" {
		return true
	}
	for _, disabled := range user.DisabledOperations {
		if disabled == operation {
			return false
		}
	}
	return true
}

func (l *requestRateLimiter) allow(key string, limit int, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	kept := l.prunedLocked(key, now, time.Minute)
	if len(kept) >= limit {
		l.hits[key] = kept
		return false
	}
	kept = append(kept, now)
	l.hits[key] = kept
	return true
}

func (l *requestRateLimiter) tooMany(key string, limit int, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	kept := l.prunedLocked(key, now, time.Minute)
	l.hits[key] = kept
	return len(kept) >= limit
}

func (l *requestRateLimiter) record(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	kept := l.prunedLocked(key, now, time.Minute)
	kept = append(kept, now)
	l.hits[key] = kept
}

func (l *requestRateLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.hits != nil {
		delete(l.hits, key)
	}
}

func (l *requestRateLimiter) prunedLocked(key string, now time.Time, window time.Duration) []time.Time {
	if l.hits == nil {
		l.hits = make(map[string][]time.Time)
	}
	windowStart := now.Add(-window)
	hits := l.hits[key]
	kept := hits[:0]
	for _, hit := range hits {
		if hit.After(windowStart) {
			kept = append(kept, hit)
		}
	}
	return kept
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

var validOperationRules = map[string]bool{
	"preview":     true,
	"download":    true,
	"upload":      true,
	"rename":      true,
	"move":        true,
	"copy":        true,
	"delete":      true,
	"share":       true,
	"directLinks": true,
}

func splitOperationRules(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	rules := make([]string, 0, len(fields))
	for _, field := range fields {
		rule := strings.TrimSpace(field)
		if validOperationRules[rule] {
			rules = append(rules, rule)
		}
	}
	return rules
}

func downloadOperation(r *http.Request) string {
	if queryBool(r, "preview", false) {
		return "preview"
	}
	return "download"
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
	if value, ok := settings["directoryPasswordRules"]; ok {
		if err := validateDirectoryPasswordRules(value); err != nil {
			return err
		}
	}
	if value, ok := settings["disabledOperations"]; ok {
		if err := validateOperationRuleList(value); err != nil {
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
	if value, ok := settings["loginLimitPerMinute"]; ok {
		limit, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || limit < 0 {
			return errors.New("登录限频必须是大于或等于 0 的整数")
		}
	}
	if value, ok := settings["loginCaptcha"]; ok {
		if err := validateSwitch(value, "登录验证码"); err != nil {
			return err
		}
	}
	if value, ok := settings["sharePasswordLimitPerMinute"]; ok {
		limit, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || limit < 0 {
			return errors.New("分享密码限频必须是大于或等于 0 的整数")
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
	if value, ok := settings["webdavAllowAnonymous"]; ok {
		if err := validateSwitch(value, "WebDAV 匿名访问"); err != nil {
			return err
		}
	}
	if value, ok := settings["webdavMountPath"]; ok {
		mountPath, err := validateWebDAVMountPath(value)
		if err != nil {
			return err
		}
		if mountPath == "/" || mountPath == "/api" || strings.HasPrefix(mountPath, "/api/") || mountPath == "/s" || strings.HasPrefix(mountPath, "/s/") || mountPath == "/d" || strings.HasPrefix(mountPath, "/d/") {
			return errors.New("WebDAV 挂载路径必须是有效的绝对路径")
		}
	}
	if err := validateExternalPreviewSettings(settings); err != nil {
		return err
	}
	return nil
}

func validateWebDAVMountPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.Contains(value, "..") || strings.ContainsAny(value, "?#") {
		return "", errors.New("WebDAV 挂载路径必须是有效的绝对路径")
	}
	clean := path.Clean(value)
	if clean == "." {
		return "", errors.New("WebDAV 挂载路径必须是有效的绝对路径")
	}
	return clean, nil
}

func validateExternalPreviewSettings(settings map[string]string) error {
	provider, hasProvider := settings["externalPreviewProvider"]
	baseURL, hasBaseURL := settings["externalPreviewBaseUrl"]
	template, hasTemplate := settings["externalPreviewTemplate"]
	if !hasProvider && !hasBaseURL && !hasTemplate {
		return nil
	}

	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "disabled"
	}
	if err := validateExternalPreviewProvider(provider); err != nil {
		return err
	}
	if err := validateExternalPreviewBaseURL(baseURL); err != nil {
		return err
	}
	if err := validateExternalPreviewTemplate(template); err != nil {
		return err
	}
	if provider != "disabled" && strings.TrimSpace(baseURL) == "" && strings.TrimSpace(template) == "" {
		return errors.New("外部预览启用时必须配置服务地址或 URL 模板")
	}
	return nil
}

func validateExternalPreviewProvider(value string) error {
	switch strings.TrimSpace(value) {
	case "", "disabled", "kkfileview", "onlyoffice":
		return nil
	default:
		return errors.New("外部预览服务只能是 disabled、kkfileview 或 onlyoffice")
	}
}

func validateExternalPreviewBaseURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, " \r\n\t") {
		return errors.New("外部预览服务地址必须是有效的 http 或 https URL")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("外部预览服务地址必须是有效的 http 或 https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("外部预览服务地址必须是有效的 http 或 https URL")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("外部预览服务地址不能包含查询参数或片段")
	}
	return nil
}

func validateExternalPreviewTemplate(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) > 1000 || strings.ContainsAny(value, "\r\n") {
		return errors.New("外部预览 URL 模板格式无效")
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "/") && !strings.Contains(value, "{server}") {
		return errors.New("外部预览 URL 模板必须以 http、https、/ 或 {server} 开头")
	}
	if !strings.Contains(value, "{url}") && !strings.Contains(value, "{encodedUrl}") && !strings.Contains(value, "{base64Url}") {
		return errors.New("外部预览 URL 模板必须包含文件 URL 占位符")
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

func validateDirectoryPasswordRules(value string) error {
	for _, line := range strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == '\r'
	}) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pathText, password, ok := strings.Cut(line, "=")
		if !ok {
			pathText, password, ok = strings.Cut(line, ":")
		}
		if !ok || strings.TrimSpace(pathText) == "" || strings.TrimSpace(password) == "" {
			return fmt.Errorf("directory password rule is invalid: %s", line)
		}
		if err := validatePathRuleList(pathText, "directory password path"); err != nil {
			return err
		}
	}
	return nil
}

func validateOperationRuleList(value string) error {
	for _, rule := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		if !validOperationRules[rule] {
			return fmt.Errorf("operation permission rule is invalid: %s", rule)
		}
	}
	return nil
}
