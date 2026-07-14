package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"xfile/internal/domain"
)

func TestSaveFileMetadataRouteUpdatesDescription(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	root := appStoreRoot(t, appStore)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.txt"), []byte("note"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	res := performJSONRequestAs(s, "root", http.MethodPut, "/api/v1/workspace/metadata", `{"storageKey":"local","path":"docs/note.txt","description":"Ops handoff"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("save metadata failed: %d %s", res.Code, res.Body.String())
	}
	var entry domain.FileEntry
	if err := json.NewDecoder(res.Body).Decode(&entry); err != nil {
		t.Fatalf("decode entry: %v", err)
	}
	if entry.Description != "Ops handoff" || entry.MetadataUpdatedAt == "" {
		t.Fatalf("unexpected metadata response: %#v", entry)
	}

	res = performJSONRequestAs(s, "root", http.MethodGet, "/api/v1/workspace/entries?storageKey=local&path=docs", "")
	if res.Code != http.StatusOK {
		t.Fatalf("list files failed: %d %s", res.Code, res.Body.String())
	}
	var files []domain.FileEntry
	if err := json.NewDecoder(res.Body).Decode(&files); err != nil {
		t.Fatalf("decode files: %v", err)
	}
	if len(files) != 1 || files[0].Description != "Ops handoff" {
		t.Fatalf("expected metadata in list: %#v", files)
	}
}

func TestSaveFileMetadataRouteRequiresRenameOperation(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	if err := appStore.SaveSettings(map[string]string{"disabledOperations": "rename"}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	root := appStoreRoot(t, appStore)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.txt"), []byte("note"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	res := performJSONRequestAs(s, "root", http.MethodPut, "/api/v1/workspace/metadata", `{"storageKey":"local","path":"docs/note.txt","description":"Blocked"}`)
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected rename operation to be blocked, got %d %s", res.Code, res.Body.String())
	}
	files, err := appStore.ListSourceFilesForAdmin("local", "docs")
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	if len(files) != 1 || files[0].Description != "" {
		t.Fatalf("blocked save wrote metadata: %#v", files)
	}
}
