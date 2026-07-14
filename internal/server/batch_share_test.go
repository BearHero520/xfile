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

	for _, bad := range [][]string{nil, []string{""}, []string{"../secret.txt"}, []string{`..\secret.txt`}} {
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

	res := performJSONRequestAs(s, "root", http.MethodPost, "/api/v1/collaboration/shares/batch", `{"paths":["docs/a.txt","docs/b.txt"],"password":"secret","maxAccessCount":3}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("batch share failed: %d %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"itemCount":2`) || !strings.Contains(res.Body.String(), `"docs/a.txt"`) || !strings.Contains(res.Body.String(), `"docs/b.txt"`) {
		t.Fatalf("batch share response missing paths: %s", res.Body.String())
	}

	shares, err := appStore.Shares()
	if err != nil {
		t.Fatalf("shares: %v", err)
	}
	if len(shares) != 1 || !shares[0].Protected || shares[0].ItemCount != 2 || shares[0].MaxAccessCount != 3 {
		t.Fatalf("unexpected shares: %#v", shares)
	}
	detail, err := appStore.ShareDetail(shares[0].Token, "secret", "")
	if err != nil {
		t.Fatalf("bundle share detail: %v", err)
	}
	if detail.ItemCount != 2 || len(detail.Files) != 2 || detail.Files[0].Path != "0" || detail.Files[1].Path != "1" {
		t.Fatalf("unexpected bundle detail: %#v", detail)
	}
}

func TestBundleShareBrowsesSelectedFolder(t *testing.T) {
	_, appStore := newAuthzTestServer(t)
	root := appStoreRoot(t, appStore)
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "guide.txt"), []byte("guide"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("readme"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	share, err := appStore.CreateSourceBundleShare("local", []string{"docs", "readme.txt"}, "", "")
	if err != nil {
		t.Fatalf("create bundle share: %v", err)
	}
	folder, err := appStore.ShareDetail(share.Token, "", "0")
	if err != nil {
		t.Fatalf("open bundled folder: %v", err)
	}
	if folder.Name != "docs" || len(folder.Files) != 1 || folder.Files[0].Path != "0/guide.txt" {
		t.Fatalf("unexpected bundled folder detail: %#v", folder)
	}
	_, download, err := appStore.SharedDownload(share.Token, "", "0/guide.txt")
	if err != nil {
		t.Fatalf("download bundled child: %v", err)
	}
	defer download.Reader.Close()
}
