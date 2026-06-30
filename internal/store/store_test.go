package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestSearchFiles(t *testing.T) {
	s := newTestStore(t)
	docs, err := s.FilePath("docs/manuals")
	if err != nil {
		t.Fatalf("docs path: %v", err)
	}
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docs, "setup-guide.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docs, "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	results, err := s.SearchFiles("guide", 20)
	if err != nil {
		t.Fatalf("search files: %v", err)
	}
	if len(results) != 1 || results[0].Path != "docs/manuals/setup-guide.txt" {
		t.Fatalf("unexpected search results: %#v", results)
	}

	folders, err := s.SearchFiles("manuals", 20)
	if err != nil {
		t.Fatalf("search folders: %v", err)
	}
	if len(folders) < 1 || folders[0].Path != "docs/manuals" {
		t.Fatalf("expected matching folder first, got %#v", folders)
	}

	empty, err := s.SearchFiles("guide", 0)
	if err != nil {
		t.Fatalf("search with fallback limit: %v", err)
	}
	if len(empty) != 1 {
		t.Fatalf("fallback limit search = %#v", empty)
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

func TestShareAndDirectLinkStats(t *testing.T) {
	s := newTestStore(t)
	file, err := s.FilePath("report.txt")
	if err != nil {
		t.Fatalf("file path: %v", err)
	}
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	share, err := s.CreateShare("report.txt", "", "")
	if err != nil {
		t.Fatalf("create share: %v", err)
	}
	if err := s.RecordShareView(share.ID); err != nil {
		t.Fatalf("record share view: %v", err)
	}
	if err := s.RecordShareDownload(share.ID); err != nil {
		t.Fatalf("record share download: %v", err)
	}
	shares, err := s.Shares()
	if err != nil {
		t.Fatalf("shares: %v", err)
	}
	if shares[0].ViewCount != 1 || shares[0].DownloadCount != 1 || shares[0].LastAccessAt == "" {
		t.Fatalf("unexpected share stats: %#v", shares[0])
	}

	link, err := s.CreateDirectLink("report.txt")
	if err != nil {
		t.Fatalf("create direct link: %v", err)
	}
	if err := s.RecordDirectLinkAccess(link.ID); err != nil {
		t.Fatalf("record direct link access: %v", err)
	}
	links, err := s.DirectLinks()
	if err != nil {
		t.Fatalf("direct links: %v", err)
	}
	if links[0].AccessCount != 1 || links[0].LastAccessAt == "" {
		t.Fatalf("unexpected direct link stats: %#v", links[0])
	}
}

func TestPrivatePathRules(t *testing.T) {
	s := newTestStore(t)
	privateDir, err := s.FilePath("secret")
	if err != nil {
		t.Fatalf("private path: %v", err)
	}
	if err := os.MkdirAll(privateDir, 0o755); err != nil {
		t.Fatalf("mkdir private: %v", err)
	}
	if err := os.WriteFile(filepath.Join(privateDir, "hidden.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write private file: %v", err)
	}

	publicDir, err := s.FilePath("public")
	if err != nil {
		t.Fatalf("public path: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(publicDir, "private"), 0o755); err != nil {
		t.Fatalf("mkdir nested private: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "readme.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write public file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "private", "hidden.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write nested private file: %v", err)
	}

	existingShare, err := s.CreateShare("secret/hidden.txt", "", "")
	if err != nil {
		t.Fatalf("create existing share before rule: %v", err)
	}
	existingLink, err := s.CreateDirectLink("secret/hidden.txt")
	if err != nil {
		t.Fatalf("create existing direct link before rule: %v", err)
	}
	parentShare, err := s.CreateShare("public", "", "")
	if err != nil {
		t.Fatalf("create parent share: %v", err)
	}
	if err := s.SaveSettings(map[string]string{"privatePathList": "secret\npublic/private"}); err != nil {
		t.Fatalf("save private path rules: %v", err)
	}

	if _, err := s.CreateShare("secret/hidden.txt", "", ""); err == nil {
		t.Fatal("expected private share creation to fail")
	}
	if _, err := s.CreateDirectLink("secret/hidden.txt"); err == nil {
		t.Fatal("expected private direct link creation to fail")
	}
	if _, err := s.ResolveShare(existingShare.Token, ""); err == nil {
		t.Fatal("expected existing private share to be blocked")
	}
	if _, err := s.ResolveDirectLink(existingLink.Token); err == nil {
		t.Fatal("expected existing private direct link to be blocked")
	}

	detail, err := s.ShareDetail(parentShare.Token, "", "")
	if err != nil {
		t.Fatalf("parent share detail: %v", err)
	}
	if len(detail.Files) != 1 || detail.Files[0].Path != "public/readme.txt" {
		t.Fatalf("private child was not filtered: %#v", detail.Files)
	}
	if _, err := s.ShareDetail(parentShare.Token, "", "private"); err == nil {
		t.Fatal("expected private child browsing to fail")
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

	filtered, err := s.SearchAccessLogs(1, 1, "download", "docs", "127.", "", "", "")
	if err != nil {
		t.Fatalf("search logs: %v", err)
	}
	if filtered.Total != 1 || len(filtered.Items) != 1 {
		t.Fatalf("filtered logs = %#v", filtered)
	}
	if filtered.Items[0].Path != "docs/a.txt" {
		t.Fatalf("unexpected filtered path: %s", filtered.Items[0].Path)
	}

	startTime := time.Now().AddDate(0, 0, -1).UTC().Format("2006-01-02 15:04:05")
	endTime := time.Now().AddDate(0, 0, 1).UTC().Format("2006-01-02 15:04:05")
	agentFiltered, err := s.SearchAccessLogs(1, 10, "", "", "", "test-agent", startTime, endTime)
	if err != nil {
		t.Fatalf("search logs by agent and time: %v", err)
	}
	if agentFiltered.Total != 3 {
		t.Fatalf("agent/time filtered logs = %#v", agentFiltered)
	}

	page, err := s.SearchAccessLogs(2, 2, "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("paged logs: %v", err)
	}
	if page.Total != 3 || page.Page != 2 || page.PageSize != 2 || len(page.Items) != 1 {
		t.Fatalf("paged logs = %#v", page)
	}

	capped, err := s.SearchAccessLogs(1, 500, "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("capped logs: %v", err)
	}
	if capped.PageSize != 200 {
		t.Fatalf("page size was not capped: %d", capped.PageSize)
	}
}

func TestDeleteAccessLogs(t *testing.T) {
	s := newTestStore(t)
	oldTime := time.Now().AddDate(0, 0, -45).UTC().Format("2006-01-02 15:04:05")
	newTime := time.Now().UTC().Format("2006-01-02 15:04:05")
	if _, err := s.db.Exec(`INSERT INTO access_logs(action, path, ip, user_agent, created_at) VALUES
		('download', 'old.txt', '127.0.0.1', 'test', ?),
		('download', 'new.txt', '127.0.0.1', 'test', ?)`, oldTime, newTime); err != nil {
		t.Fatalf("insert logs: %v", err)
	}

	deleted, err := s.DeleteAccessLogs(30, false)
	if err != nil {
		t.Fatalf("delete old logs: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted old logs = %d", deleted)
	}
	page, err := s.SearchAccessLogs(1, 10, "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("search logs: %v", err)
	}
	if page.Total != 1 || page.Items[0].Path != "new.txt" {
		t.Fatalf("remaining logs = %#v", page)
	}

	deleted, err = s.DeleteAccessLogs(0, true)
	if err != nil {
		t.Fatalf("delete all logs: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted all logs = %d", deleted)
	}
}
