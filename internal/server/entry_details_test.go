package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"xfile/internal/domain"
)

func TestEntryDetailsCalculatesFolderSizeAndCounts(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	root := appStoreRoot(t, appStore)
	if err := os.MkdirAll(filepath.Join(root, "docs", "manuals"), 0o755); err != nil {
		t.Fatalf("mkdir manuals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "readme.txt"), []byte("readme"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "manuals", "guide.txt"), []byte("guide"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}

	res := performJSONRequestAs(s, "root", http.MethodGet, "/api/v1/workspace/entries/details?storageKey=local&path=docs", "")
	if res.Code != http.StatusOK {
		t.Fatalf("entry details failed: %d %s", res.Code, res.Body.String())
	}
	var details domain.EntryDetails
	if err := json.NewDecoder(res.Body).Decode(&details); err != nil {
		t.Fatalf("decode details: %v", err)
	}
	if details.TotalSize != 11 || details.FileCount != 2 || details.FolderCount != 1 {
		t.Fatalf("unexpected folder stats: %+v", details)
	}
}

func TestPublicEntryDetailsCalculatesVisibleFolderStats(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	root := appStoreRoot(t, appStore)
	if err := os.MkdirAll(filepath.Join(root, "public", "nested"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "public", "a.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "public", "nested", "b.txt"), []byte("12345"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/drives/local/entries/details?path=public", nil)
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("public entry details failed: %d %s", res.Code, res.Body.String())
	}
	var details domain.EntryDetails
	if err := json.NewDecoder(res.Body).Decode(&details); err != nil {
		t.Fatalf("decode details: %v", err)
	}
	if details.TotalSize != 8 || details.FileCount != 2 || details.FolderCount != 1 {
		t.Fatalf("unexpected public folder stats: %+v", details)
	}
}
