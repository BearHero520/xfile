package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCSRFProtectsMutatingPrivateRoutes(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	sessionValue := s.signSession("root", time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodPost, "/api/files/folders", strings.NewReader(`{"storageKey":"local","path":"docs"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionValue})
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("mutating request without CSRF should be forbidden, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/files/folders", strings.NewReader(`{"storageKey":"local","path":"docs"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeaderName, s.csrfToken(sessionValue))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionValue})
	res = httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("mutating request with CSRF should succeed, got %d: %s", res.Code, res.Body.String())
	}
}

func TestCSRFAllowsSafePrivateRoutes(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: s.signSession("root", time.Now().Add(time.Hour))})
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("safe request should not need CSRF, got %d: %s", res.Code, res.Body.String())
	}
}

func TestCSRFTokenReturnedForAuthenticatedSession(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	sessionValue := s.signSession("root", time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionValue})
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("auth/me failed: %d: %s", res.Code, res.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["csrfToken"] != s.csrfToken(sessionValue) {
		t.Fatalf("unexpected CSRF token: %#v", body["csrfToken"])
	}
}

func TestLogoutRequiresCSRFForAuthenticatedSession(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	sessionValue := s.signSession("root", time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionValue})
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF should be forbidden, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set(csrfHeaderName, s.csrfToken(sessionValue))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionValue})
	res = httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("logout with CSRF should succeed, got %d: %s", res.Code, res.Body.String())
	}
}
