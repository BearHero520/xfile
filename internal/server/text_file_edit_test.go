package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveTextFileRouteUpdatesExistingTextFile(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	root := appStoreRoot(t, appStore)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(root, "docs", "note.txt")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	res := performJSONRequestAs(s, "root", http.MethodPut, "/api/v1/workspace/text-content", `{"storageKey":"local","path":"docs/note.txt","content":"updated\nbody"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("save text failed: %d %s", res.Code, res.Body.String())
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(body) != "updated\nbody" {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestSaveTextFileRouteRequiresUploadOperation(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	if err := appStore.SaveSettings(map[string]string{"disabledOperations": "upload"}); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	root := appStoreRoot(t, appStore)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(root, "docs", "note.txt")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	res := performJSONRequestAs(s, "root", http.MethodPut, "/api/v1/workspace/text-content", `{"storageKey":"local","path":"docs/note.txt","content":"updated"}`)
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected upload operation to be blocked, got %d %s", res.Code, res.Body.String())
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(body) != "old" {
		t.Fatalf("blocked save changed body: %q", string(body))
	}
}
