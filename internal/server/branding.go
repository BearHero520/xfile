package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxBrandLogoBytes    = 2 << 20
	maxBrandFaviconBytes = 512 << 10
	maxBrandingBodyBytes = 4 << 20
)

var allowedThemePresets = map[string]struct{}{
	"ocean":     {},
	"violet":    {},
	"emerald":   {},
	"sunset":    {},
	"graphite":  {},
	"sky":       {},
	"rose":      {},
	"sunflower": {},
}

var allowedBrandImageTypes = map[string]struct{}{
	"image/png":                {},
	"image/jpeg":               {},
	"image/webp":               {},
	"image/gif":                {},
	"image/x-icon":             {},
	"image/vnd.microsoft.icon": {},
}

var brandingSettingKeys = []string{
	"themePreset",
	"brandLogo",
	"brandFavicon",
	"brandingVersion",
}

func (s *Server) getThemeSettings(w http.ResponseWriter, _ *http.Request) {
	settings, err := s.store.Settings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, themeSettingsResponse(settings))
}

func (s *Server) saveThemeSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBrandingBodyBytes)
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	settings, err := s.store.Settings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	preset := strings.TrimSpace(req["themePreset"])
	if preset == "" {
		preset = settings["themePreset"]
	}
	if _, ok := allowedThemePresets[preset]; !ok {
		writeError(w, http.StatusBadRequest, errors.New("不支持的主题预设"))
		return
	}
	logo := strings.TrimSpace(req["brandLogo"])
	favicon := strings.TrimSpace(req["brandFavicon"])
	if err := validateBrandSource("Logo", logo, maxBrandLogoBytes); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := validateBrandSource("favicon", favicon, maxBrandFaviconBytes); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	next := map[string]string{
		"themePreset":     preset,
		"brandLogo":       logo,
		"brandFavicon":    favicon,
		"brandingVersion": strconv.FormatInt(time.Now().UnixNano(), 10),
	}
	if err := s.store.SaveSettings(next); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, themeSettingsResponse(next))
}

func themeSettingsResponse(settings map[string]string) map[string]string {
	preset := strings.TrimSpace(settings["themePreset"])
	if _, ok := allowedThemePresets[preset]; !ok {
		preset = "ocean"
	}
	return map[string]string{
		"themePreset":     preset,
		"brandLogo":       settings["brandLogo"],
		"brandFavicon":    settings["brandFavicon"],
		"brandingVersion": settings["brandingVersion"],
	}
}

func removeBrandingSettings(settings map[string]string) {
	for _, key := range brandingSettingKeys {
		delete(settings, key)
	}
}

func validateBrandSource(label, value string, maxBytes int) error {
	if value == "" {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		_, _, err := decodeBrandDataURL(value, maxBytes)
		if err != nil {
			return fmt.Errorf("%s 图片无效: %w", label, err)
		}
		return nil
	}
	if len(value) > 2048 {
		return fmt.Errorf("%s 地址过长", label)
	}
	if strings.HasPrefix(value, "/api/v1/public/branding/") {
		return fmt.Errorf("%s 地址不能指向品牌资源接口自身", label)
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%s 请输入站内绝对路径或 http/https 地址", label)
	}
	return nil
}

func decodeBrandDataURL(value string, maxBytes int) (string, []byte, error) {
	header, encoded, ok := strings.Cut(value, ",")
	if !ok || !strings.HasPrefix(strings.ToLower(header), "data:") || !strings.HasSuffix(strings.ToLower(header), ";base64") {
		return "", nil, errors.New("仅支持 base64 图片")
	}
	mimeType := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64"))
	if _, ok := allowedBrandImageTypes[mimeType]; !ok {
		return "", nil, errors.New("仅支持 PNG、JPEG、WebP、GIF 或 ICO")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", nil, errors.New("base64 内容损坏")
	}
	if len(data) == 0 {
		return "", nil, errors.New("图片内容为空")
	}
	if len(data) > maxBytes {
		return "", nil, fmt.Errorf("图片不能超过 %d KB", maxBytes>>10)
	}
	return mimeType, data, nil
}

func (s *Server) brandLogo(w http.ResponseWriter, r *http.Request) {
	s.serveBrandAsset(w, r, "brandLogo", maxBrandLogoBytes)
}

func (s *Server) brandFavicon(w http.ResponseWriter, r *http.Request) {
	s.serveBrandAsset(w, r, "brandFavicon", maxBrandFaviconBytes)
}

func (s *Server) serveBrandAsset(w http.ResponseWriter, r *http.Request, key string, maxBytes int) {
	settings, err := s.store.Settings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	source := strings.TrimSpace(settings[key])
	if source == "" {
		http.NotFound(w, r)
		return
	}
	if !strings.HasPrefix(strings.ToLower(source), "data:") {
		http.Redirect(w, r, source, http.StatusFound)
		return
	}
	mimeType, data, err := decodeBrandDataURL(source, maxBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
