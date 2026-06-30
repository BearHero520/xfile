package store

import (
	"os"
	"path/filepath"
	"testing"

	"xfile/internal/config"
	"xfile/internal/database"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "xfile.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return New(db, config.Config{
		DataDir:      dir,
		FilesDir:     filepath.Join(dir, "files"),
		DatabasePath: filepath.Join(dir, "xfile.db"),
		SiteName:     "XFile",
	})
}

func TestSafePathRejectsTraversal(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.FilePath("../secret.txt"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestMoveFile(t *testing.T) {
	s := newTestStore(t)
	source, err := s.FilePath("docs/readme.txt")
	if err != nil {
		t.Fatalf("source path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(source, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	entry, err := s.MoveFile("docs/readme.txt", "archive/readme.txt")
	if err != nil {
		t.Fatalf("move file: %v", err)
	}
	if entry.Path != "archive/readme.txt" {
		t.Fatalf("entry path = %q", entry.Path)
	}
	if _, err := s.FilePath("archive/readme.txt"); err != nil {
		t.Fatalf("target path: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists or stat failed unexpectedly: %v", err)
	}
}

func TestSharePassword(t *testing.T) {
	s := newTestStore(t)
	path, err := s.FilePath("report.txt")
	if err != nil {
		t.Fatalf("file path: %v", err)
	}
	if err := os.WriteFile(path, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	share, err := s.CreateShare("report.txt", "", "pass123")
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	if !share.Protected {
		t.Fatal("expected protected share")
	}
	if _, err := s.ResolveShare(share.Token, "wrong"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
	if resolved, err := s.ResolveShare(share.Token, "pass123"); err != nil || resolved.Path != "report.txt" {
		t.Fatalf("resolve share = %#v, %v", resolved, err)
	}
}

func TestCreateSuperAdminAndAuthenticate(t *testing.T) {
	s := newTestStore(t)

	initialized, err := s.IsInitialized()
	if err != nil {
		t.Fatalf("initialized: %v", err)
	}
	if initialized {
		t.Fatal("new store should not be initialized")
	}

	user, err := s.CreateSuperAdmin("admin", "password123")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if user.Username != "admin" || user.Role != "super_admin" {
		t.Fatalf("unexpected user: %#v", user)
	}
	if _, err := s.CreateSuperAdmin("other", "password123"); err == nil {
		t.Fatal("expected second super admin initialization to fail")
	}
	if _, err := s.AuthenticateUser("admin", "wrong"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
	if authenticated, err := s.AuthenticateUser("admin", "password123"); err != nil || authenticated.Username != "admin" {
		t.Fatalf("authenticate = %#v, %v", authenticated, err)
	}
}

func TestShareDetailAndSharedFilePath(t *testing.T) {
	s := newTestStore(t)
	folder, err := s.FilePath("docs")
	if err != nil {
		t.Fatalf("folder path: %v", err)
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(folder, "guide.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	share, err := s.CreateShare("docs", "", "")
	if err != nil {
		t.Fatalf("create folder share: %v", err)
	}
	detail, err := s.ShareDetail(share.Token, "", "")
	if err != nil {
		t.Fatalf("share detail: %v", err)
	}
	if detail.Type != "folder" || len(detail.Files) != 1 {
		t.Fatalf("unexpected detail: %#v", detail)
	}
	if _, _, err := s.SharedFilePath(share.Token, "", "../x.txt"); err == nil {
		t.Fatal("expected shared child traversal to fail")
	}
	_, path, err := s.SharedFilePath(share.Token, "", "guide.txt")
	if err != nil {
		t.Fatalf("shared file path: %v", err)
	}
	if filepath.Base(path) != "guide.txt" {
		t.Fatalf("unexpected shared path: %s", path)
	}
}

func TestShareDetailNestedFolder(t *testing.T) {
	s := newTestStore(t)
	nested, err := s.FilePath("docs/manuals")
	if err != nil {
		t.Fatalf("nested path: %v", err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "setup.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	share, err := s.CreateShare("docs", "", "")
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	detail, err := s.ShareDetail(share.Token, "", "manuals")
	if err != nil {
		t.Fatalf("share nested detail: %v", err)
	}
	if detail.Path != "docs" || detail.CurrentPath != "docs/manuals" || detail.Name != "manuals" {
		t.Fatalf("unexpected nested detail: %#v", detail)
	}
	if len(detail.Files) != 1 || detail.Files[0].Path != "docs/manuals/setup.txt" {
		t.Fatalf("unexpected nested files: %#v", detail.Files)
	}
	if _, err := s.ShareDetail(share.Token, "", "../outside"); err == nil {
		t.Fatal("expected traversal child path to fail")
	}
}

func TestSearchAccessLogs(t *testing.T) {
	s := newTestStore(t)
	entries := []struct {
		action string
		path   string
		ip     string
	}{
		{"download", "docs/a.txt", "127.0.0.1"},
		{"upload", "docs/b.txt", "10.0.0.2"},
		{"download", "images/a.png", "127.0.0.1"},
	}
	for _, entry := range entries {
		if err := s.LogAccess(entry.action, entry.path, entry.ip, "test-agent"); err != nil {
			t.Fatalf("log access: %v", err)
		}
	}

	filtered, err := s.SearchAccessLogs(1, 1, "download", "docs", "127.")
	if err != nil {
		t.Fatalf("search logs: %v", err)
	}
	if filtered.Total != 1 || len(filtered.Items) != 1 {
		t.Fatalf("filtered logs = %#v", filtered)
	}
	if filtered.Items[0].Path != "docs/a.txt" {
		t.Fatalf("unexpected filtered path: %s", filtered.Items[0].Path)
	}

	page, err := s.SearchAccessLogs(2, 2, "", "", "")
	if err != nil {
		t.Fatalf("paged logs: %v", err)
	}
	if page.Total != 3 || page.Page != 2 || page.PageSize != 2 || len(page.Items) != 1 {
		t.Fatalf("paged logs = %#v", page)
	}

	capped, err := s.SearchAccessLogs(1, 500, "", "", "")
	if err != nil {
		t.Fatalf("capped logs: %v", err)
	}
	if capped.PageSize != 200 {
		t.Fatalf("page size was not capped: %d", capped.PageSize)
	}
}
