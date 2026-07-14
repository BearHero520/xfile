package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSharePasswordAttemptLimit(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	sharePath := "docs/report.txt"
	if err := os.MkdirAll(filepath.Join(s.cfg.FilesDir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.cfg.FilesDir, sharePath), []byte("report"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	share, err := appStore.CreateShare(sharePath, "", "secret")
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	if err := appStore.SaveSettings(map[string]string{"sharePasswordLimitPerMinute": "2"}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	if res := performPublicShareRequest(s, "198.51.100.12", share.Token, ""); res.Code != http.StatusForbidden {
		t.Fatalf("empty password prompt request should be forbidden but not counted, got %d: %s", res.Code, res.Body.String())
	}
	if res := performPublicShareRequest(s, "198.51.100.12", share.Token, "bad-1"); res.Code != http.StatusForbidden {
		t.Fatalf("first wrong password should be forbidden, got %d: %s", res.Code, res.Body.String())
	}
	if res := performPublicShareRequest(s, "198.51.100.12", share.Token, "secret"); res.Code != http.StatusOK {
		t.Fatalf("correct password should clear failures, got %d: %s", res.Code, res.Body.String())
	}
	if res := performPublicShareRequest(s, "198.51.100.12", share.Token, "bad-2"); res.Code != http.StatusForbidden {
		t.Fatalf("wrong password after clear should be forbidden, got %d: %s", res.Code, res.Body.String())
	}
	if res := performPublicShareRequest(s, "198.51.100.12", share.Token, "bad-3"); res.Code != http.StatusForbidden {
		t.Fatalf("second wrong password after clear should be forbidden, got %d: %s", res.Code, res.Body.String())
	}

	limited := performPublicShareRequest(s, "198.51.100.12", share.Token, "secret")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("third attempt inside window should be rate limited, got %d: %s", limited.Code, limited.Body.String())
	}
	if limited.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected retry-after header, got %q", limited.Header().Get("Retry-After"))
	}

	otherIP := performPublicShareRequest(s, "203.0.113.12", share.Token, "secret")
	if otherIP.Code != http.StatusOK {
		t.Fatalf("different IP should still access share, got %d: %s", otherIP.Code, otherIP.Body.String())
	}
}

func TestSharePasswordAttemptLimitCanBeDisabled(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	sharePath := "report.txt"
	if err := os.MkdirAll(s.cfg.FilesDir, 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.cfg.FilesDir, sharePath), []byte("report"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	share, err := appStore.CreateShare(sharePath, "", "secret")
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	if err := appStore.SaveSettings(map[string]string{"sharePasswordLimitPerMinute": "0"}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	for attempt := 1; attempt <= 3; attempt++ {
		res := performPublicShareRequest(s, "198.51.100.13", share.Token, "bad")
		if res.Code != http.StatusForbidden {
			t.Fatalf("attempt %d should be forbidden with limiter disabled, got %d: %s", attempt, res.Code, res.Body.String())
		}
	}
}

func performPublicShareRequest(s *Server, ip, token, password string) *httptest.ResponseRecorder {
	target := "/api/v1/public/shares/" + token + "?password=" + password
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("X-Forwarded-For", ip)
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	return res
}
