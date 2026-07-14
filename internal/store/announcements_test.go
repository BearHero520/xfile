package store

import (
	"testing"

	"xfile/internal/domain"
)

func TestAnnouncementManagement(t *testing.T) {
	s := newTestStore(t)

	published, err := s.CreateAnnouncement(domain.AnnouncementInput{
		Title:     "维护通知",
		Content:   "今晚 22:00 进行例行维护。",
		Published: true,
	})
	if err != nil {
		t.Fatalf("create published announcement: %v", err)
	}
	draft, err := s.CreateAnnouncement(domain.AnnouncementInput{
		Title:     "草稿",
		Content:   "暂不公开。",
		Published: false,
	})
	if err != nil {
		t.Fatalf("create draft announcement: %v", err)
	}

	publicItems, err := s.Announcements(true)
	if err != nil {
		t.Fatalf("list public announcements: %v", err)
	}
	if len(publicItems) != 1 || publicItems[0].ID != published.ID {
		t.Fatalf("unexpected public announcements: %#v", publicItems)
	}

	updated, err := s.UpdateAnnouncement(draft.ID, domain.AnnouncementInput{
		Title:     "草稿已发布",
		Content:   "现在公开。",
		Published: true,
	})
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}
	if !updated.Published {
		t.Fatalf("expected updated announcement to be published: %#v", updated)
	}

	if err := s.DeleteAnnouncement(published.ID); err != nil {
		t.Fatalf("delete announcement: %v", err)
	}
	items, err := s.Announcements(false)
	if err != nil {
		t.Fatalf("list announcements: %v", err)
	}
	if len(items) != 1 || items[0].ID != draft.ID {
		t.Fatalf("unexpected remaining announcements: %#v", items)
	}
}
