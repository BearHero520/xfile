package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xfile/internal/config"
	"xfile/internal/database"
	"xfile/internal/store"
)

func newWebDAVTestServer(t *testing.T) (*Server, *store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	filesDir := filepath.Join(dir, "files")
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
		FilesDir:     filesDir,
		DatabasePath: filepath.Join(dir, "xfile.db"),
		StaticDir:    filepath.Join(dir, "dist"),
		SiteName:     "XFile",
	}
	appStore := store.New(db, cfg)
	return New(cfg, appStore), appStore, filesDir
}

func TestWebDAVRequiresConfiguredCredentials(t *testing.T) {
	s, appStore, filesDir := newWebDAVTestServer(t)
	if err := appStore.SaveSettings(map[string]string{
		"webdav":          "enabled",
		"webdavMountPath": "/dav",
		"webdavUsername":  "dav-user",
		"webdavPassword":  "dav-pass",
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	if err := writeTestFile(filesDir, "hello.txt", "hello"); err != nil {
		t.Fatalf("write file: %v", err)
	}

	res := performWebDAVRequest(s, "PROPFIND", "/dav", "", "", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized without credentials, got %d", res.Code)
	}

	res = performWebDAVRequest(s, "PROPFIND", "/dav", "dav-user", "wrong", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized with bad credentials, got %d", res.Code)
	}

	res = performWebDAVRequest(s, "PROPFIND", "/dav", "dav-user", "dav-pass", "")
	if res.Code != http.StatusMultiStatus {
		t.Fatalf("expected multistatus with credentials, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "hello.txt") {
		t.Fatalf("expected WebDAV listing to include file, got %s", res.Body.String())
	}
}

func TestWebDAVAnonymousAndReadOnlyPolicies(t *testing.T) {
	s, appStore, filesDir := newWebDAVTestServer(t)
	if err := appStore.SaveSettings(map[string]string{
		"webdav":               "enabled",
		"webdavMountPath":      "/files",
		"webdavReadOnly":       "enabled",
		"webdavAllowAnonymous": "enabled",
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	res := performWebDAVRequest(s, "PROPFIND", "/files", "", "", "")
	if res.Code != http.StatusMultiStatus {
		t.Fatalf("expected anonymous PROPFIND to work, got %d", res.Code)
	}

	if err := writeTestFile(filesDir, "readonly.txt", "content"); err != nil {
		t.Fatalf("write file: %v", err)
	}
	res = performWebDAVRequest(s, http.MethodDelete, "/files/readonly.txt", "", "", "")
	if res.Code != http.StatusForbidden && res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected readonly DELETE to be forbidden, got %d", res.Code)
	}
	if _, err := os.Stat(filepath.Join(filesDir, "readonly.txt")); err != nil {
		t.Fatalf("expected readonly file to remain: %v", err)
	}
}

func TestValidateWebDAVMountPathRejectsRouteConflicts(t *testing.T) {
	for _, mountPath := range []string{"/", "/api", "/api/v1/dav", "/share", "/open/file"} {
		if err := validateAccessSettings(map[string]string{"webdavMountPath": mountPath}); err == nil {
			t.Fatalf("expected %q to be rejected", mountPath)
		}
	}
}

func performWebDAVRequest(s *Server, method, target, username, password, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Depth", "1")
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	return res
}

func writeTestFile(root, rel, content string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
