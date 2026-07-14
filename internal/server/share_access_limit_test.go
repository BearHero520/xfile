package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestShareAccessLimitCountsEachRootOpen(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if err := os.MkdirAll(s.cfg.FilesDir, 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.cfg.FilesDir, "report.txt"), []byte("report"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	share, err := appStore.CreateShareWithLimit("report.txt", "", "", 1)
	if err != nil {
		t.Fatalf("create share: %v", err)
	}

	first := performShareAccessRequest(s, "/api/v1/public/shares/"+share.Token, nil)
	if first.Code != http.StatusOK {
		t.Fatalf("first access failed: %d %s", first.Code, first.Body.String())
	}
	var grant *http.Cookie
	for _, cookie := range first.Result().Cookies() {
		if cookie.Name == shareAccessCookieName {
			grant = cookie
			break
		}
	}
	if grant == nil || grant.Value == "" || !grant.HttpOnly {
		t.Fatalf("missing secure share access grant: %#v", grant)
	}

	blocked := performShareAccessRequest(s, "/api/v1/public/shares/"+share.Token, nil)
	if blocked.Code != http.StatusGone {
		t.Fatalf("new visitor should be blocked: %d %s", blocked.Code, blocked.Body.String())
	}

	repeat := performShareAccessRequest(s, "/api/v1/public/shares/"+share.Token, grant)
	if repeat.Code != http.StatusGone {
		t.Fatalf("reopening the share should consume another access and be blocked: %d %s", repeat.Code, repeat.Body.String())
	}
	download := performShareAccessRequest(s, "/api/v1/public/shares/"+share.Token+"/content", grant)
	if download.Code != http.StatusOK || download.Body.String() != "report" {
		t.Fatalf("granted visitor download failed: %d %q", download.Code, download.Body.String())
	}

	shares, err := appStore.Shares()
	if err != nil {
		t.Fatalf("shares: %v", err)
	}
	if len(shares) != 1 || shares[0].ViewCount != 1 || shares[0].DownloadCount != 1 {
		t.Fatalf("unexpected share counters: %#v", shares)
	}
}

func TestUpdateShareLimitsRoute(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := os.MkdirAll(s.cfg.FilesDir, 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.cfg.FilesDir, "editable.txt"), []byte("editable"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	share, err := appStore.CreateShare("editable.txt", "", "")
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	expiresAt := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	target := "/api/v1/collaboration/shares/" + strconv.FormatInt(share.ID, 10)
	body := `{"expiresAt":"` + expiresAt + `","maxAccessCount":2}`
	res := performJSONRequestAs(s, "root", http.MethodPatch, target, body)
	if res.Code != http.StatusOK {
		t.Fatalf("update limits failed: %d %s", res.Code, res.Body.String())
	}

	shares, err := appStore.Shares()
	if err != nil {
		t.Fatalf("shares: %v", err)
	}
	if len(shares) != 1 || shares[0].MaxAccessCount != 2 || shares[0].ExpiresAt == "" {
		t.Fatalf("limits were not persisted: %#v", shares)
	}
}

func performShareAccessRequest(s *Server, target string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	return res
}
