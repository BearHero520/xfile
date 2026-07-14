package server

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadArchiveRouteIncludesFoldersAndSelectedFiles(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	root := appStoreRoot(t, appStore)
	if err := os.MkdirAll(filepath.Join(root, "docs", "manuals"), 0o755); err != nil {
		t.Fatalf("mkdir manuals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "manuals", "guide.txt"), []byte("guide"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("readme"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	res := performJSONRequestAs(s, "root", http.MethodPost, "/api/v1/workspace/archives", `{"storageKey":"local","paths":["docs","readme.txt"]}`)
	if res.Code != http.StatusOK {
		t.Fatalf("archive download failed: %d %s", res.Code, res.Body.String())
	}
	if contentType := res.Header().Get("Content-Type"); contentType != "application/zip" {
		t.Fatalf("content type = %q", contentType)
	}

	reader, err := zip.NewReader(bytes.NewReader(res.Body.Bytes()), int64(res.Body.Len()))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	names := make(map[string]bool)
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, name := range []string{"docs/", "docs/manuals/", "docs/manuals/guide.txt", "readme.txt"} {
		if !names[name] {
			t.Fatalf("zip missing %s, got %#v", name, names)
		}
	}
}

func TestPublicStorageArchiveRouteUsesPublicRules(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	root := appStoreRoot(t, appStore)
	if err := os.MkdirAll(filepath.Join(root, "docs", "manuals"), 0o755); err != nil {
		t.Fatalf("mkdir manuals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "manuals", "guide.txt"), []byte("guide"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("readme"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/public/drives/local/archives", bytes.NewBufferString(`{"paths":["docs","readme.txt"]}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("public archive download failed: %d %s", res.Code, res.Body.String())
	}
	if contentType := res.Header().Get("Content-Type"); contentType != "application/zip" {
		t.Fatalf("content type = %q", contentType)
	}

	reader, err := zip.NewReader(bytes.NewReader(res.Body.Bytes()), int64(res.Body.Len()))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	names := make(map[string]bool)
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, name := range []string{"docs/", "docs/manuals/", "docs/manuals/guide.txt", "readme.txt"} {
		if !names[name] {
			t.Fatalf("zip missing %s, got %#v", name, names)
		}
	}
}
