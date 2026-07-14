package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xfile/internal/config"
	"xfile/internal/database"
	"xfile/internal/store"
)

func newAuthzTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "xfile.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	cfg := config.Config{
		DataDir:      dir,
		FilesDir:     filepath.Join(dir, "files"),
		DatabasePath: filepath.Join(dir, "xfile.db"),
		StaticDir:    filepath.Join(dir, "dist"),
		SiteName:     "XFile",
	}
	appStore := store.New(db, cfg)
	return New(cfg, appStore), appStore
}

func TestSuperAdminOnlyManagementRoutes(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	if _, err := appStore.CreateUserWithPolicy("operator", "password123", "admin", []string{"local"}, nil, nil); err != nil {
		t.Fatalf("create operator: %v", err)
	}

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "create storage source", method: http.MethodPost, target: "/api/v1/admin/storage-nodes", body: `{"name":"Alt","key":"alt","type":"local","rootPath":"` + filepath.ToSlash(t.TempDir()) + `","enabled":true}`},
		{name: "list users", method: http.MethodGet, target: "/api/v1/admin/accounts"},
		{name: "save settings", method: http.MethodPut, target: "/api/v1/preferences", body: `{"siteName":"Changed"}`},
		{name: "delete logs", method: http.MethodDelete, target: "/api/v1/audit/events?all=true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := performJSONRequestAs(s, "operator", tt.method, tt.target, tt.body)
			if res.Code != http.StatusForbidden {
				t.Fatalf("normal admin should be forbidden, got %d: %s", res.Code, res.Body.String())
			}
		})
	}

	if res := performJSONRequestAs(s, "operator", http.MethodGet, "/api/v1/admin/storage-nodes", ""); res.Code != http.StatusOK {
		t.Fatalf("normal admin should read assigned storage sources, got %d: %s", res.Code, res.Body.String())
	}
	if res := performJSONRequestAs(s, "operator", http.MethodGet, "/api/v1/preferences", ""); res.Code != http.StatusOK {
		t.Fatalf("normal admin should read operation settings, got %d: %s", res.Code, res.Body.String())
	}
	if res := performJSONRequestAs(s, "root", http.MethodGet, "/api/v1/admin/accounts", ""); res.Code != http.StatusOK {
		t.Fatalf("super admin should manage users, got %d: %s", res.Code, res.Body.String())
	}
}

func performJSONRequestAs(s *Server, username, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	sessionValue := s.signSession(username, time.Now().Add(time.Hour))
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: sessionValue,
	})
	if csrfRequired(method) {
		req.Header.Set(csrfHeaderName, s.csrfToken(sessionValue))
	}
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	return res
}
