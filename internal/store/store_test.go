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
