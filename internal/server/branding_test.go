package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestThemeSettingsPersistAndPublishBrandAssets(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	logoBytes := []byte("persisted-logo-bytes")
	logoData := "data:image/png;base64," + base64.StdEncoding.EncodeToString(logoBytes)
	body, err := json.Marshal(map[string]string{
		"themePreset":  "sky",
		"brandLogo":    logoData,
		"brandFavicon": "/custom-favicon.png",
	})
	if err != nil {
		t.Fatalf("marshal theme settings: %v", err)
	}
	res := performJSONRequestAs(s, "root", http.MethodPut, "/api/v1/admin/theme", string(body))
	if res.Code != http.StatusOK {
		t.Fatalf("save theme status = %d: %s", res.Code, res.Body.String())
	}

	// A normal settings save must not be able to overwrite branding with a
	// stale form that was opened before the theme changed.
	res = performJSONRequestAs(s, "root", http.MethodPut, "/api/v1/preferences", `{"themePreset":"ocean","brandLogo":""}`)
	if res.Code != http.StatusOK {
		t.Fatalf("save preferences status = %d: %s", res.Code, res.Body.String())
	}

	nextServer := New(s.cfg, appStore)
	bootstrap := httptest.NewRecorder()
	nextServer.mux.ServeHTTP(bootstrap, httptest.NewRequest(http.MethodGet, "/api/v1/public/bootstrap", nil))
	if bootstrap.Code != http.StatusOK {
		t.Fatalf("public bootstrap status = %d: %s", bootstrap.Code, bootstrap.Body.String())
	}
	var public struct {
		Preferences map[string]string `json:"preferences"`
	}
	if err := json.NewDecoder(bootstrap.Body).Decode(&public); err != nil {
		t.Fatalf("decode public bootstrap: %v", err)
	}
	if public.Preferences["themePreset"] != "sky" {
		t.Fatalf("theme preset = %q, want sky", public.Preferences["themePreset"])
	}
	if !strings.Contains(public.Preferences["brandLogoUrl"], "/api/v1/public/branding/logo?v=") {
		t.Fatalf("unexpected logo url: %q", public.Preferences["brandLogoUrl"])
	}
	if !strings.Contains(public.Preferences["brandFaviconUrl"], "/api/v1/public/branding/favicon?v=") {
		t.Fatalf("unexpected favicon url: %q", public.Preferences["brandFaviconUrl"])
	}

	logo := httptest.NewRecorder()
	nextServer.mux.ServeHTTP(logo, httptest.NewRequest(http.MethodGet, public.Preferences["brandLogoUrl"], nil))
	if logo.Code != http.StatusOK || logo.Header().Get("Content-Type") != "image/png" || !bytes.Equal(logo.Body.Bytes(), logoBytes) {
		t.Fatalf("unexpected logo response: status=%d type=%q body=%q", logo.Code, logo.Header().Get("Content-Type"), logo.Body.Bytes())
	}
	if !strings.Contains(logo.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("logo cache control = %q", logo.Header().Get("Cache-Control"))
	}

	favicon := httptest.NewRecorder()
	nextServer.mux.ServeHTTP(favicon, httptest.NewRequest(http.MethodGet, public.Preferences["brandFaviconUrl"], nil))
	if favicon.Code != http.StatusFound || favicon.Header().Get("Location") != "/custom-favicon.png" {
		t.Fatalf("unexpected favicon redirect: status=%d location=%q", favicon.Code, favicon.Header().Get("Location"))
	}
}

func TestThemeSettingsValidation(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	tests := []string{
		`{"themePreset":"unknown","brandLogo":"","brandFavicon":""}`,
		`{"themePreset":"ocean","brandLogo":"javascript:alert(1)","brandFavicon":""}`,
		`{"themePreset":"ocean","brandLogo":"data:text/html;base64,SGVsbG8=","brandFavicon":""}`,
	}
	for _, body := range tests {
		res := performJSONRequestAs(s, "root", http.MethodPut, "/api/v1/admin/theme", body)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("invalid theme body %s returned %d: %s", body, res.Code, res.Body.String())
		}
	}
}
