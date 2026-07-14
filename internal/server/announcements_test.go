package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xfile/internal/domain"
)

func TestAnnouncementRoutesExposeOnlyPublishedItems(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	created := performJSONRequestAs(s, "root", http.MethodPost, "/api/v1/admin/announcements", `{"title":"公开公告","content":"访客可见","published":true}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create announcement status = %d: %s", created.Code, created.Body.String())
	}
	if _, err := appStore.CreateAnnouncement(domain.AnnouncementInput{Title: "草稿", Content: "访客不可见", Published: false}); err != nil {
		t.Fatalf("create draft: %v", err)
	}

	public := httptest.NewRecorder()
	s.mux.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/api/v1/public/announcements", nil))
	if public.Code != http.StatusOK {
		t.Fatalf("public announcements status = %d: %s", public.Code, public.Body.String())
	}
	var items []domain.Announcement
	if err := json.Unmarshal(public.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode public announcements: %v", err)
	}
	if len(items) != 1 || items[0].Title != "公开公告" || !items[0].Published {
		t.Fatalf("unexpected public announcements: %#v", items)
	}

	bootstrap := httptest.NewRecorder()
	s.mux.ServeHTTP(bootstrap, httptest.NewRequest(http.MethodGet, "/api/v1/public/bootstrap", nil))
	if bootstrap.Code != http.StatusOK {
		t.Fatalf("public bootstrap status = %d: %s", bootstrap.Code, bootstrap.Body.String())
	}
	var site domain.PublicSite
	if err := json.Unmarshal(bootstrap.Body.Bytes(), &site); err != nil {
		t.Fatalf("decode public bootstrap: %v", err)
	}
	if len(site.Announcements) != 1 || site.Announcements[0].Title != "公开公告" {
		t.Fatalf("unexpected bootstrap announcements: %#v", site.Announcements)
	}
}
