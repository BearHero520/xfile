package database

import (
	"path/filepath"
	"testing"
)

func TestMigrateMovesLegacyAnnouncementIntoAnnouncementTable(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "xfile.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatalf("initial migrate: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO settings(key, value) VALUES('announcement', '旧版公告内容')`,
	); err != nil {
		t.Fatalf("insert legacy announcement: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate legacy announcement: %v", err)
	}

	var title, content string
	if err := db.QueryRow(
		`SELECT title, content FROM announcements ORDER BY id LIMIT 1`,
	).Scan(&title, &content); err != nil {
		t.Fatalf("read migrated announcement: %v", err)
	}
	if title != "网站公告" || content != "旧版公告内容" {
		t.Fatalf("unexpected migrated announcement: %q %q", title, content)
	}

	var legacyCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM settings WHERE key = 'announcement'`,
	).Scan(&legacyCount); err != nil {
		t.Fatalf("count legacy settings: %v", err)
	}
	if legacyCount != 0 {
		t.Fatalf("legacy announcement setting should be removed, count = %d", legacyCount)
	}
}
