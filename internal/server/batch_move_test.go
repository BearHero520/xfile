package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestBatchMoveTargets(t *testing.T) {
	targets, err := batchMoveTargets([]string{"docs/a.txt", "docs/team"}, "archive")
	if err != nil {
		t.Fatalf("batch move targets: %v", err)
	}
	if len(targets) != 2 || targets[0] != "archive/a.txt" || targets[1] != "archive/team" {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}

func TestBatchMoveTargetsRejectsUnsafeMoves(t *testing.T) {
	tests := []struct {
		name      string
		paths     []string
		targetDir string
	}{
		{name: "empty", paths: nil, targetDir: "archive"},
		{name: "same target", paths: []string{"docs/a.txt"}, targetDir: "docs"},
		{name: "folder into itself", paths: []string{"docs"}, targetDir: "docs/archive"},
		{name: "duplicate target", paths: []string{"first/a.txt", "second/a.txt"}, targetDir: "archive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := batchMoveTargets(tt.paths, tt.targetDir); err == nil {
				t.Fatal("expected batch move target validation to fail")
			}
		})
	}
}

func TestBatchMoveFilesRouteMovesSelectedFiles(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	sourceRoot := filepath.Join(appStoreRoot(t, appStore), "docs")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	res := performJSONRequestAs(s, "root", http.MethodPost, "/api/v1/workspace/actions/move", `{"storageKey":"local","paths":["docs/a.txt","docs/b.txt"],"targetDir":"archive"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("batch move failed: %d %s", res.Code, res.Body.String())
	}
	root := appStoreRoot(t, appStore)
	for _, rel := range []string{"archive/a.txt", "archive/b.txt"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected moved file %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "a.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected source to be moved, stat err: %v", err)
	}
}

func TestBatchCopyFilesRouteCopiesSelectedFiles(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	sourceRoot := filepath.Join(appStoreRoot(t, appStore), "docs")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	res := performJSONRequestAs(s, "root", http.MethodPost, "/api/v1/workspace/actions/copy", `{"storageKey":"local","paths":["docs/a.txt","docs/b.txt"],"targetDir":"copies"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("batch copy failed: %d %s", res.Code, res.Body.String())
	}
	root := appStoreRoot(t, appStore)
	for _, rel := range []string{"docs/a.txt", "docs/b.txt", "copies/a.txt", "copies/b.txt"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected file %s: %v", rel, err)
		}
	}
}

func appStoreRoot(t *testing.T, appStore interface{ FilePath(string) (string, error) }) string {
	t.Helper()
	root, err := appStore.FilePath("")
	if err != nil {
		t.Fatalf("store root: %v", err)
	}
	return root
}
