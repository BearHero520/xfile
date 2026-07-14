package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBatchCreateDirectLinksRoute(t *testing.T) {
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

	res := performJSONRequestAs(s, "root", http.MethodPost, "/api/v1/delivery/links/batch", `{"paths":["docs/a.txt","docs/b.txt"]}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("batch direct link failed: %d %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"path":"docs/a.txt"`) || !strings.Contains(res.Body.String(), `"path":"docs/b.txt"`) {
		t.Fatalf("batch direct link response missing paths: %s", res.Body.String())
	}

	links, err := appStore.DirectLinks()
	if err != nil {
		t.Fatalf("direct links: %v", err)
	}
	if len(links) != 2 || !links[0].Enabled || !links[1].Enabled {
		t.Fatalf("unexpected direct links: %#v", links)
	}
}

func TestCreateDirectLinkReusesExistingPath(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	root := appStoreRoot(t, appStore)
	if err := os.WriteFile(filepath.Join(root, "same.txt"), []byte("same"), 0o644); err != nil {
		t.Fatalf("write same file: %v", err)
	}

	first := performJSONRequestAs(s, "root", http.MethodPost, "/api/v1/delivery/links", `{"path":"same.txt"}`)
	second := performJSONRequestAs(s, "root", http.MethodPost, "/api/v1/delivery/links", `{"path":"same.txt"}`)
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("create direct links failed: %d %d", first.Code, second.Code)
	}
	links, err := appStore.DirectLinks()
	if err != nil {
		t.Fatalf("direct links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected one reused direct link, got %#v", links)
	}
}
