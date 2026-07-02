package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeBatchSharePaths(t *testing.T) {
	paths, err := normalizeBatchSharePaths([]string{"docs/a.txt", "docs/a.txt", `docs\b.txt`})
	if err != nil {
		t.Fatalf("normalize batch share paths: %v", err)
	}
	if len(paths) != 2 || paths[0] != "docs/a.txt" || paths[1] != "docs/b.txt" {
		t.Fatalf("unexpected paths: %#v", paths)
	}

	for _, bad := range [][]string{nil, []string{""}, []string{"../secret.txt"}} {
		if _, err := normalizeBatchSharePaths(bad); err == nil {
			t.Fatalf("expected invalid paths to fail: %#v", bad)
		}
	}
}

func TestBatchCreateSharesRoute(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	root := appStoreRoot(t, appStore)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, "docs", name), []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	res := performJSONRequestAs(s, "root", http.MethodPost, "/api/shares/batch", `{"paths":["docs/a.txt","docs/b.txt"],"password":"secret"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("batch share failed: %d %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"path":"docs/a.txt"`) || !strings.Contains(res.Body.String(), `"path":"docs/b.txt"`) {
		t.Fatalf("batch share response missing paths: %s", res.Body.String())
	}

	shares, err := appStore.Shares()
	if err != nil {
		t.Fatalf("shares: %v", err)
	}
	if len(shares) != 2 || !shares[0].Protected || !shares[1].Protected {
		t.Fatalf("unexpected shares: %#v", shares)
	}
}
