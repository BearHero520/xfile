package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginRateLimit(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	if err := appStore.SaveSettings(map[string]string{"loginLimitPerMinute": "2"}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		res := performLoginRequest(s, "198.51.100.8", `{"username":"root","password":"wrong-password"}`)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d should be unauthorized, got %d: %s", attempt, res.Code, res.Body.String())
		}
	}

	limited := performLoginRequest(s, "198.51.100.8", `{"username":"root","password":"password123"}`)
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("third login should be rate limited, got %d: %s", limited.Code, limited.Body.String())
	}
	if limited.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected retry-after header, got %q", limited.Header().Get("Retry-After"))
	}

	otherIP := performLoginRequest(s, "203.0.113.10", `{"username":"root","password":"password123"}`)
	if otherIP.Code != http.StatusOK {
		t.Fatalf("different IP should still log in, got %d: %s", otherIP.Code, otherIP.Body.String())
	}
}

func TestLoginRateLimitCanBeDisabled(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	if err := appStore.SaveSettings(map[string]string{"loginLimitPerMinute": "0"}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	for attempt := 1; attempt <= 3; attempt++ {
		res := performLoginRequest(s, "198.51.100.9", `{"username":"root","password":"wrong-password"}`)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d should be unauthorized with limiter disabled, got %d: %s", attempt, res.Code, res.Body.String())
		}
	}
}

func performLoginRequest(s *Server, ip, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/session", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", ip)
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	return res
}
