package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xfile/internal/domain"
)

func TestShareAndDirectLinkRoutesUseRequestedStorageSource(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	root := filepath.Join(t.TempDir(), "team-drive")
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir secondary source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "guide.txt"), []byte("team drive content"), 0o644); err != nil {
		t.Fatalf("write secondary file: %v", err)
	}
	if _, err := appStore.CreateStorageSource(domain.StorageSourceInput{
		Name:     "Team drive",
		Key:      "team-drive",
		Type:     "local",
		RootPath: root,
		Public:   false,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("create secondary source: %v", err)
	}

	shareResponse := performJSONRequestAs(s, "root", http.MethodPost, "/api/v1/collaboration/shares", `{"storageKey":"team-drive","path":"docs/guide.txt"}`)
	if shareResponse.Code != http.StatusCreated {
		t.Fatalf("create source share: %d %s", shareResponse.Code, shareResponse.Body.String())
	}
	var share domain.Share
	if err := json.Unmarshal(shareResponse.Body.Bytes(), &share); err != nil {
		t.Fatalf("decode source share: %v", err)
	}
	if share.StorageKey != "team-drive" {
		t.Fatalf("unexpected share source: %#v", share)
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/public/shares/"+share.Token, nil)
	detailResponse := httptest.NewRecorder()
	s.mux.ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK || !strings.Contains(detailResponse.Body.String(), `"storageKey":"team-drive"`) {
		t.Fatalf("source share detail failed: %d %s", detailResponse.Code, detailResponse.Body.String())
	}

	contentRequest := httptest.NewRequest(http.MethodGet, "/api/v1/public/shares/"+share.Token+"/content", nil)
	contentResponse := httptest.NewRecorder()
	s.mux.ServeHTTP(contentResponse, contentRequest)
	if contentResponse.Code != http.StatusOK || contentResponse.Body.String() != "team drive content" {
		t.Fatalf("source share content failed: %d %q", contentResponse.Code, contentResponse.Body.String())
	}

	linkResponse := performJSONRequestAs(s, "root", http.MethodPost, "/api/v1/delivery/links", `{"storageKey":"team-drive","path":"docs/guide.txt"}`)
	if linkResponse.Code != http.StatusCreated {
		t.Fatalf("create source direct link: %d %s", linkResponse.Code, linkResponse.Body.String())
	}
	var link domain.DirectLink
	if err := json.Unmarshal(linkResponse.Body.Bytes(), &link); err != nil {
		t.Fatalf("decode source direct link: %v", err)
	}
	if link.StorageKey != "team-drive" {
		t.Fatalf("unexpected direct link source: %#v", link)
	}

	openRequest := httptest.NewRequest(http.MethodGet, "/open/"+link.Token, nil)
	openResponse := httptest.NewRecorder()
	s.mux.ServeHTTP(openResponse, openRequest)
	if openResponse.Code != http.StatusOK || openResponse.Body.String() != "team drive content" {
		t.Fatalf("source direct link failed: %d %q", openResponse.Code, openResponse.Body.String())
	}
}
