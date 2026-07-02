package store

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xfile/internal/config"
	"xfile/internal/database"
	"xfile/internal/domain"
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

func TestStorageSourceManagement(t *testing.T) {
	s := newTestStore(t)
	root := filepath.Join(t.TempDir(), "alt")
	source, err := s.CreateStorageSource(domain.StorageSourceInput{
		Name:     "Alt local",
		Key:      "alt-local",
		Type:     "local",
		RootPath: root,
		Public:   true,
		Enabled:  true,
		OrderNum: 10,
	})
	if err != nil {
		t.Fatalf("create storage source: %v", err)
	}
	if source.ID == 0 || source.Key != "alt-local" || source.TypeLabel != "本地存储" {
		t.Fatalf("unexpected source: %#v", source)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("expected local root to be created: %v", err)
	}

	updated, err := s.UpdateStorageSource(source.ID, domain.StorageSourceInput{
		Name:     "Alt private",
		Key:      "alt-private",
		Type:     "local",
		RootPath: root,
		Public:   false,
		Enabled:  true,
		OrderNum: 2,
	})
	if err != nil {
		t.Fatalf("update storage source: %v", err)
	}
	if updated.Key != "alt-private" || updated.Public || updated.OrderNum != 2 {
		t.Fatalf("unexpected updated source: %#v", updated)
	}

	if _, err := s.CreateStorageSource(domain.StorageSourceInput{Name: "dup", Key: "alt-private", Type: "local", RootPath: root}); err == nil {
		t.Fatal("expected duplicate key to fail")
	}
	if _, err := s.CreateStorageSource(domain.StorageSourceInput{Name: "remote", Key: "remote", Type: "webdav", Enabled: true}); err == nil {
		t.Fatal("expected enabled WebDAV without config to fail")
	}
	if err := s.DeleteStorageSource(source.ID); err != nil {
		t.Fatalf("delete storage source: %v", err)
	}
	remaining, err := s.StorageSources(false)
	if err != nil {
		t.Fatalf("list remaining storage sources: %v", err)
	}
	for _, item := range remaining {
		if err := s.DeleteStorageSource(item.ID); err != nil {
			t.Fatalf("delete extra storage source: %v", err)
		}
	}
	empty, err := s.StorageSources(false)
	if err != nil {
		t.Fatalf("list empty storage sources: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected all storage sources to be deleted: %#v", empty)
	}
}

func TestS3StorageSourceConfigAndPublicSanitization(t *testing.T) {
	s := newTestStore(t)
	cases := []struct {
		sourceType string
		key        string
	}{
		{"s3", "object-store"},
		{"aliyun_oss", "aliyun-store"},
		{"tencent_cos", "tencent-store"},
	}
	for _, tc := range cases {
		source, err := s.CreateStorageSource(domain.StorageSourceInput{
			Name:     tc.key,
			Key:      tc.key,
			Type:     tc.sourceType,
			RootPath: `{"endpoint":"https://s3.example.com","bucket":"files","region":"us-east-1","accessKey":"ak","secretKey":"sk","prefix":"root","secure":false}`,
			Public:   true,
			Enabled:  true,
			OrderNum: 20,
		})
		if err != nil {
			t.Fatalf("create %s source: %v", tc.sourceType, err)
		}
		if source.Type != tc.sourceType || source.RootPath == "" {
			t.Fatalf("unexpected %s source: %#v", tc.sourceType, source)
		}
		if !strings.Contains(source.RootPath, `"endpoint":"s3.example.com"`) || !strings.Contains(source.RootPath, `"secure":true`) {
			t.Fatalf("%s config was not normalized from endpoint scheme: %s", tc.sourceType, source.RootPath)
		}
	}

	publicSources, err := s.StorageSources(true)
	if err != nil {
		t.Fatalf("public storage sources: %v", err)
	}
	for _, item := range publicSources {
		if (item.Type == "s3" || item.Type == "aliyun_oss" || item.Type == "tencent_cos") && item.RootPath != "" {
			t.Fatalf("public source leaked root config: %#v", item)
		}
	}
}

func TestWebDAVStorageSourceConfigAndPublicSanitization(t *testing.T) {
	s := newTestStore(t)
	source, err := s.CreateStorageSource(domain.StorageSourceInput{
		Name:     "DAV",
		Key:      "dav",
		Type:     "webdav",
		RootPath: `{"url":"https://dav.example.com/root/","username":"user","password":"pass","root":"docs/"}`,
		Public:   true,
		Enabled:  true,
		OrderNum: 21,
	})
	if err != nil {
		t.Fatalf("create WebDAV source: %v", err)
	}
	if !strings.Contains(source.RootPath, `"url":"https://dav.example.com/root"`) || !strings.Contains(source.RootPath, `"root":"docs"`) {
		t.Fatalf("WebDAV config was not normalized: %s", source.RootPath)
	}
	publicSources, err := s.StorageSources(true)
	if err != nil {
		t.Fatalf("public storage sources: %v", err)
	}
	for _, item := range publicSources {
		if item.Key == source.Key && item.RootPath != "" {
			t.Fatalf("public WebDAV source leaked root config: %#v", item)
		}
	}
}

func TestLocalSourceFileOperationsUseSourceRoot(t *testing.T) {
	s := newTestStore(t)
	firstRoot := filepath.Join(t.TempDir(), "first")
	secondRoot := filepath.Join(t.TempDir(), "second")

	first, err := s.CreateStorageSource(domain.StorageSourceInput{
		Name:     "First local",
		Key:      "first-local",
		Type:     "local",
		RootPath: firstRoot,
		Public:   true,
		Enabled:  true,
		OrderNum: 11,
	})
	if err != nil {
		t.Fatalf("create first source: %v", err)
	}
	second, err := s.CreateStorageSource(domain.StorageSourceInput{
		Name:     "Second local",
		Key:      "second-local",
		Type:     "local",
		RootPath: secondRoot,
		Public:   true,
		Enabled:  true,
		OrderNum: 12,
	})
	if err != nil {
		t.Fatalf("create second source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(firstRoot, "first.txt"), []byte("first"), 0o644); err != nil {
		t.Fatalf("write first root file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secondRoot, "second.txt"), []byte("second"), 0o644); err != nil {
		t.Fatalf("write second root file: %v", err)
	}

	firstFiles, err := s.ListSourceFilesForAdmin(first.Key, "")
	if err != nil {
		t.Fatalf("list first source: %v", err)
	}
	if len(firstFiles) != 1 || firstFiles[0].Path != "first.txt" {
		t.Fatalf("unexpected first source files: %#v", firstFiles)
	}
	secondFiles, err := s.ListSourceFilesForAdmin(second.Key, "")
	if err != nil {
		t.Fatalf("list second source: %v", err)
	}
	if len(secondFiles) != 1 || secondFiles[0].Path != "second.txt" {
		t.Fatalf("unexpected second source files: %#v", secondFiles)
	}

	if _, err := s.CreateSourceFolder(second.Key, "docs"); err != nil {
		t.Fatalf("create source folder: %v", err)
	}
	if _, err := s.CreateSourceEmptyFile(second.Key, "docs/note.txt"); err != nil {
		t.Fatalf("create source empty file: %v", err)
	}
	if _, err := s.MoveSourceFile(second.Key, "docs/note.txt", "archive/note.txt"); err != nil {
		t.Fatalf("move source file: %v", err)
	}
	if err := s.DeleteSourceFile(second.Key, "archive/note.txt"); err != nil {
		t.Fatalf("delete source file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(secondRoot, "archive", "note.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected moved file to be deleted, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(firstRoot, "docs")); !os.IsNotExist(err) {
		t.Fatalf("first root should not receive second source operations, stat err: %v", err)
	}
}

func TestPublicLocalSourceCreatesMissingRoot(t *testing.T) {
	s := newTestStore(t)
	root := filepath.Join(t.TempDir(), "missing-root")
	source, err := s.CreateStorageSource(domain.StorageSourceInput{
		Name:     "Missing root",
		Key:      "missing-root",
		Type:     "local",
		RootPath: root,
		Public:   true,
		Enabled:  true,
		OrderNum: 30,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove source root: %v", err)
	}

	files, err := s.ListSourceFiles(source.Key, "", true)
	if err != nil {
		t.Fatalf("list public files: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected empty files: %#v", files)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatalf("expected source root to be recreated, info=%#v err=%v", info, err)
	}
}

func TestPerStorageHiddenPathsFilterPublicListing(t *testing.T) {
	s := newTestStore(t)
	firstRoot := filepath.Join(t.TempDir(), "first")
	secondRoot := filepath.Join(t.TempDir(), "second")
	for _, root := range []string{firstRoot, secondRoot} {
		if err := os.MkdirAll(filepath.Join(root, "private"), 0o755); err != nil {
			t.Fatalf("mkdir private: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "private", "hidden.txt"), []byte("secret"), 0o644); err != nil {
			t.Fatalf("write hidden: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("hello"), 0o644); err != nil {
			t.Fatalf("write readme: %v", err)
		}
	}
	first, err := s.CreateStorageSource(domain.StorageSourceInput{
		Name:        "First",
		Key:         "first-hidden",
		Type:        "local",
		RootPath:    firstRoot,
		HiddenPaths: "private",
		Public:      true,
		Enabled:     true,
		OrderNum:    31,
	})
	if err != nil {
		t.Fatalf("create first source: %v", err)
	}
	second, err := s.CreateStorageSource(domain.StorageSourceInput{
		Name:     "Second",
		Key:      "second-visible",
		Type:     "local",
		RootPath: secondRoot,
		Public:   true,
		Enabled:  true,
		OrderNum: 32,
	})
	if err != nil {
		t.Fatalf("create second source: %v", err)
	}

	firstFiles, err := s.ListSourceFiles(first.Key, "", true)
	if err != nil {
		t.Fatalf("list first public files: %v", err)
	}
	if len(firstFiles) != 1 || firstFiles[0].Path != "readme.txt" {
		t.Fatalf("hidden path was not filtered for first source: %#v", firstFiles)
	}
	secondFiles, err := s.ListSourceFiles(second.Key, "", true)
	if err != nil {
		t.Fatalf("list second public files: %v", err)
	}
	if len(secondFiles) != 2 {
		t.Fatalf("second source should not inherit first hidden rules: %#v", secondFiles)
	}
}

func TestPerStorageBlockedPathsRejectPublicAccess(t *testing.T) {
	s := newTestStore(t)
	firstRoot := filepath.Join(t.TempDir(), "first-blocked")
	secondRoot := filepath.Join(t.TempDir(), "second-blocked")
	for _, root := range []string{firstRoot, secondRoot} {
		if err := os.MkdirAll(filepath.Join(root, "secret"), 0o755); err != nil {
			t.Fatalf("mkdir secret: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "secret", "blocked.txt"), []byte("secret"), 0o644); err != nil {
			t.Fatalf("write blocked: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("hello"), 0o644); err != nil {
			t.Fatalf("write readme: %v", err)
		}
	}
	first, err := s.CreateStorageSource(domain.StorageSourceInput{
		Name:         "First blocked",
		Key:          "first-blocked",
		Type:         "local",
		RootPath:     firstRoot,
		BlockedPaths: "secret",
		Public:       true,
		Enabled:      true,
		OrderNum:     33,
	})
	if err != nil {
		t.Fatalf("create first source: %v", err)
	}
	second, err := s.CreateStorageSource(domain.StorageSourceInput{
		Name:     "Second blocked",
		Key:      "second-blocked",
		Type:     "local",
		RootPath: secondRoot,
		Public:   true,
		Enabled:  true,
		OrderNum: 34,
	})
	if err != nil {
		t.Fatalf("create second source: %v", err)
	}

	firstFiles, err := s.ListSourceFiles(first.Key, "", true)
	if err != nil {
		t.Fatalf("list first public files: %v", err)
	}
	if len(firstFiles) != 1 || firstFiles[0].Path != "readme.txt" {
		t.Fatalf("blocked path was not filtered for first source: %#v", firstFiles)
	}
	if _, err := s.ListSourceFiles(first.Key, "secret", true); err == nil {
		t.Fatal("expected public listing blocked path to fail")
	}
	if _, err := s.SourceDownload(first.Key, "secret/blocked.txt", true); err == nil {
		t.Fatal("expected public download blocked path to fail")
	}
	secondFiles, err := s.ListSourceFiles(second.Key, "", true)
	if err != nil {
		t.Fatalf("list second public files: %v", err)
	}
	if len(secondFiles) != 2 {
		t.Fatalf("second source should not inherit first blocked rules: %#v", secondFiles)
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

func TestCopySourceFile(t *testing.T) {
	s := newTestStore(t)
	root, err := s.FilePath("")
	if err != nil {
		t.Fatalf("root path: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "folder"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "readme.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "folder", "note.txt"), []byte("note"), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	fileEntry, err := s.CopySourceFile("local", "docs/readme.txt", "archive/readme.txt")
	if err != nil {
		t.Fatalf("copy file: %v", err)
	}
	if fileEntry.Path != "archive/readme.txt" {
		t.Fatalf("file entry path = %q", fileEntry.Path)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "readme.txt")); err != nil {
		t.Fatalf("source file should remain: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(root, "archive", "readme.txt")); err != nil || string(content) != "hello" {
		t.Fatalf("copied file content = %q, %v", string(content), err)
	}

	folderEntry, err := s.CopySourceFile("local", "docs/folder", "archive/folder")
	if err != nil {
		t.Fatalf("copy folder: %v", err)
	}
	if folderEntry.Type != "folder" || folderEntry.Path != "archive/folder" {
		t.Fatalf("folder entry = %#v", folderEntry)
	}
	if content, err := os.ReadFile(filepath.Join(root, "archive", "folder", "note.txt")); err != nil || string(content) != "note" {
		t.Fatalf("copied folder content = %q, %v", string(content), err)
	}
	if _, err := s.CopySourceFile("local", "docs/folder", "docs/folder/nested"); err == nil {
		t.Fatal("expected copying folder into itself to fail")
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

func TestCustomShareKey(t *testing.T) {
	s := newTestStore(t)
	path, err := s.FilePath("report.txt")
	if err != nil {
		t.Fatalf("file path: %v", err)
	}
	if err := os.WriteFile(path, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	share, err := s.CreateShareWithToken("report.txt", "", "", "project-docs")
	if err != nil {
		t.Fatalf("create custom share: %v", err)
	}
	if share.Token != "project-docs" || share.URL != "/s/project-docs" {
		t.Fatalf("unexpected custom share: %#v", share)
	}
	if resolved, err := s.ResolveShare("project-docs", ""); err != nil || resolved.Path != "report.txt" {
		t.Fatalf("resolve custom share = %#v, %v", resolved, err)
	}
	if _, err := s.CreateShareWithToken("report.txt", "", "", "project-docs"); err == nil {
		t.Fatal("expected duplicate custom share key to fail")
	}
	if _, err := s.CreateShareWithToken("report.txt", "", "", "../bad"); err == nil {
		t.Fatal("expected invalid custom share key to fail")
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

func TestManageUsers(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateSuperAdmin("admin", "password123"); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	user, err := s.CreateUserWithPolicy("operator", "password123", "admin", []string{"local"}, map[string][]string{"local": []string{"docs/team-a"}}, []string{"delete", "share"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if user.Username != "operator" || user.Role != "admin" || !user.Enabled {
		t.Fatalf("unexpected user: %#v", user)
	}
	if len(user.StorageSourceKeys) != 1 || user.StorageSourceKeys[0] != "local" {
		t.Fatalf("unexpected storage assignments: %#v", user.StorageSourceKeys)
	}
	if roots := user.StorageSourceRoots["local"]; len(roots) != 1 || roots[0] != "docs/team-a" {
		t.Fatalf("unexpected storage roots: %#v", user.StorageSourceRoots)
	}
	if len(user.DisabledOperations) != 2 || user.DisabledOperations[0] != "delete" || user.DisabledOperations[1] != "share" {
		t.Fatalf("unexpected disabled operations: %#v", user.DisabledOperations)
	}
	if !s.UserCanAccessStorageSource(user, "local") || s.UserCanAccessStorageSource(user, "s3") {
		t.Fatalf("unexpected storage access for user: %#v", user)
	}
	if !s.UserCanListStoragePath(user, "local", "") || !s.UserCanListStoragePath(user, "local", "docs") {
		t.Fatal("expected user to list ancestors of assigned root")
	}
	if !s.UserCanAccessStoragePath(user, "local", "docs/team-a/report.txt") {
		t.Fatal("expected user to access assigned root child")
	}
	if s.UserCanAccessStoragePath(user, "local", "docs/team-b/report.txt") {
		t.Fatal("expected user to be blocked outside assigned root")
	}
	filtered := s.FilterStorageFilesForUser(user, "local", []domain.FileEntry{
		{Name: "docs", Path: "docs", Type: "folder"},
		{Name: "tmp", Path: "tmp", Type: "folder"},
	}, true)
	if len(filtered) != 1 || filtered[0].Path != "docs" {
		t.Fatalf("unexpected ancestor filtered list: %#v", filtered)
	}
	searchFiltered := s.FilterStorageFilesForUser(user, "local", []domain.FileEntry{
		{Name: "a.txt", Path: "docs/team-a/a.txt", Type: "file"},
		{Name: "b.txt", Path: "docs/team-b/b.txt", Type: "file"},
	}, false)
	if len(searchFiltered) != 1 || searchFiltered[0].Path != "docs/team-a/a.txt" {
		t.Fatalf("unexpected search filtered list: %#v", searchFiltered)
	}
	assignedSources, err := s.StorageSourcesForUser(user)
	if err != nil {
		t.Fatalf("storage sources for user: %v", err)
	}
	if len(assignedSources) != 1 || assignedSources[0].Key != "local" {
		t.Fatalf("unexpected assigned sources: %#v", assignedSources)
	}
	updated, err := s.UpdateUserWithPolicy(user.ID, "ops", "newpass123", "super_admin", nil, nil, []string{"download"})
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updated.Username != "ops" || updated.Role != "super_admin" {
		t.Fatalf("unexpected updated user: %#v", updated)
	}
	if len(updated.DisabledOperations) != 1 || updated.DisabledOperations[0] != "download" {
		t.Fatalf("unexpected updated disabled operations: %#v", updated.DisabledOperations)
	}
	if _, err := s.AuthenticateUser("ops", "newpass123"); err != nil {
		t.Fatalf("authenticate updated user: %v", err)
	}
	if _, err := s.UpdateUserWithPolicyStatus(updated.ID, "ops", "", "super_admin", nil, nil, nil, false); err != nil {
		t.Fatalf("disable updated user: %v", err)
	}
	if _, err := s.AuthenticateUser("ops", "newpass123"); err == nil {
		t.Fatal("expected disabled user authentication to fail")
	}
	if _, err := s.UpdateUserWithPolicyStatus(usersMustFindID(t, s, "admin"), "admin", "", "super_admin", nil, nil, nil, false); err == nil {
		t.Fatal("expected disabling the last enabled super admin to fail")
	}
	users, err := s.Users()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("users = %#v", users)
	}
	if err := s.DeleteUser(user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if err := s.DeleteUser(users[0].ID); err == nil {
		t.Fatal("expected deleting last user to fail")
	}
}

func TestManageSessions(t *testing.T) {
	s := newTestStore(t)
	user, err := s.CreateSuperAdmin("admin", "password123")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	session, token, err := s.CreateSession(user, "198.51.100.20", "test-agent", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	resolved, err := s.SessionByToken(token)
	if err != nil {
		t.Fatalf("resolve session: %v", err)
	}
	if resolved.ID != session.ID || resolved.Username != "admin" || resolved.IP != "198.51.100.20" {
		t.Fatalf("unexpected session: %#v", resolved)
	}
	users, err := s.Users()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 1 || users[0].ActiveSessionCount != 1 {
		t.Fatalf("unexpected active session count: %#v", users)
	}
	if _, err := s.RevokeSession(user.ID, session.ID); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if _, err := s.SessionByToken(token); err == nil {
		t.Fatal("expected revoked session to be rejected")
	}
	users, err = s.Users()
	if err != nil {
		t.Fatalf("list users after revoke: %v", err)
	}
	if users[0].ActiveSessionCount != 0 {
		t.Fatalf("expected no active sessions: %#v", users[0])
	}
}

func TestPasswordChangeRevokesUserSessions(t *testing.T) {
	s := newTestStore(t)
	user, err := s.CreateSuperAdmin("admin", "password123")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	_, token, err := s.CreateSession(user, "198.51.100.21", "test-agent", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := s.UpdateUserWithPolicyStatus(user.ID, "admin", "newpass123", "super_admin", nil, nil, nil, true); err != nil {
		t.Fatalf("update password: %v", err)
	}
	if _, err := s.SessionByToken(token); err == nil {
		t.Fatal("expected password change to revoke active sessions")
	}
}

func usersMustFindID(t *testing.T, s *Store, username string) int64 {
	t.Helper()
	users, err := s.Users()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	for _, user := range users {
		if user.Username == username {
			return user.ID
		}
	}
	t.Fatalf("user %q not found", username)
	return 0
}

func TestUploadRules(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveSettings(map[string]string{
		"uploadAllowExtensions": ".txt\n.md",
		"uploadDenyExtensions":  ".exe",
		"uploadPathAllowList":   "incoming",
		"uploadPathDenyList":    "incoming/private",
		"uploadOverwrite":       "disabled",
	}); err != nil {
		t.Fatalf("save upload rules: %v", err)
	}

	if err := s.UploadAllowed("incoming", "readme.txt"); err != nil {
		t.Fatalf("expected upload to be allowed: %v", err)
	}
	if err := s.UploadAllowed("other", "readme.txt"); err == nil {
		t.Fatal("expected path allow list to block upload")
	}
	if err := s.UploadAllowed("incoming/private", "readme.txt"); err == nil {
		t.Fatal("expected path deny list to block upload")
	}
	if err := s.UploadAllowed("incoming", "archive.zip"); err == nil {
		t.Fatal("expected extension allow list to block upload")
	}
	if err := s.UploadAllowed("incoming", "tool.exe"); err == nil {
		t.Fatal("expected extension deny list to block upload")
	}

	target, err := s.FilePath("incoming/readme.txt")
	if err != nil {
		t.Fatalf("target path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := s.UploadAllowed("incoming", "readme.txt"); err == nil {
		t.Fatal("expected overwrite rule to block existing target")
	}
}

func TestSaveSourceTextFileUpdatesExistingFile(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveSettings(map[string]string{
		"uploadAllowExtensions": ".txt",
		"uploadPathAllowList":   "editable",
		"uploadOverwrite":       "disabled",
	}); err != nil {
		t.Fatalf("save upload rules: %v", err)
	}
	target, err := s.FilePath("editable/note.txt")
	if err != nil {
		t.Fatalf("target path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	entry, err := s.SaveSourceTextFile("local", "editable/note.txt", "updated")
	if err != nil {
		t.Fatalf("save text file: %v", err)
	}
	if entry.Path != "editable/note.txt" || entry.Size != int64(len("updated")) {
		t.Fatalf("unexpected entry: %#v", entry)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(body) != "updated" {
		t.Fatalf("unexpected body: %q", string(body))
	}
}

func TestSaveSourceTextFileRequiresExistingEditableAllowedPath(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveSettings(map[string]string{
		"uploadAllowExtensions": ".txt",
		"uploadPathAllowList":   "editable",
	}); err != nil {
		t.Fatalf("save upload rules: %v", err)
	}
	root, err := s.FilePath("editable")
	if err != nil {
		t.Fatalf("root path: %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "note.md"), []byte("old"), 0o644); err != nil {
		t.Fatalf("write md: %v", err)
	}

	if _, err := s.SaveSourceTextFile("local", "editable/missing.txt", "new"); err == nil {
		t.Fatal("expected missing file to fail")
	}
	if _, err := s.SaveSourceTextFile("local", "blocked/note.txt", "new"); err == nil {
		t.Fatal("expected path allow list to block save")
	}
	if _, err := s.SaveSourceTextFile("local", "editable/note.md", "new"); err == nil {
		t.Fatal("expected extension allow list to block save")
	}
	if _, err := s.SaveSourceTextFile("local", "editable/image.png", "new"); err == nil {
		t.Fatal("expected non-text extension to block save")
	}
}

func TestFileDescriptionsAreReturnedAndSearchable(t *testing.T) {
	s := newTestStore(t)
	target, err := s.FilePath("docs/readme.txt")
	if err != nil {
		t.Fatalf("target path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	entry, err := s.SaveFileDescription("local", "docs/readme.txt", "Release notes for operators")
	if err != nil {
		t.Fatalf("save description: %v", err)
	}
	if entry.Description != "Release notes for operators" || entry.MetadataUpdatedAt == "" {
		t.Fatalf("unexpected metadata entry: %#v", entry)
	}
	files, err := s.ListSourceFilesForAdmin("local", "docs")
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	if len(files) != 1 || files[0].Description != "Release notes for operators" {
		t.Fatalf("expected description in list: %#v", files)
	}
	results, err := s.SearchSourceFiles("local", "operators", 10)
	if err != nil {
		t.Fatalf("search files: %v", err)
	}
	if len(results) != 1 || results[0].Path != "docs/readme.txt" || results[0].Description == "" {
		t.Fatalf("expected description search result: %#v", results)
	}
	entry, err = s.SaveFileDescription("local", "docs/readme.txt", "")
	if err != nil {
		t.Fatalf("clear description: %v", err)
	}
	if entry.Description != "" || entry.MetadataUpdatedAt != "" {
		t.Fatalf("expected metadata to be cleared: %#v", entry)
	}
}

func TestFileDescriptionsFollowMoveCopyAndDelete(t *testing.T) {
	s := newTestStore(t)
	root, err := s.FilePath("docs")
	if err != nil {
		t.Fatalf("root path: %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "guide.txt"), []byte("guide"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	if _, err := s.SaveFileDescription("local", "docs", "Folder note"); err != nil {
		t.Fatalf("save folder description: %v", err)
	}
	if _, err := s.SaveFileDescription("local", "docs/guide.txt", "Guide note"); err != nil {
		t.Fatalf("save file description: %v", err)
	}
	if _, err := s.MoveSourceFile("local", "docs", "archive/docs"); err != nil {
		t.Fatalf("move docs: %v", err)
	}
	moved, err := s.ListSourceFilesForAdmin("local", "archive")
	if err != nil {
		t.Fatalf("list archive: %v", err)
	}
	if len(moved) != 1 || moved[0].Description != "Folder note" {
		t.Fatalf("expected moved folder description: %#v", moved)
	}
	children, err := s.ListSourceFilesForAdmin("local", "archive/docs")
	if err != nil {
		t.Fatalf("list moved docs: %v", err)
	}
	if len(children) != 1 || children[0].Description != "Guide note" {
		t.Fatalf("expected moved child description: %#v", children)
	}
	if _, err := s.CopySourceFile("local", "archive/docs", "copies/docs"); err != nil {
		t.Fatalf("copy docs: %v", err)
	}
	copied, err := s.ListSourceFilesForAdmin("local", "copies/docs")
	if err != nil {
		t.Fatalf("list copied docs: %v", err)
	}
	if len(copied) != 1 || copied[0].Description != "Guide note" {
		t.Fatalf("expected copied child description: %#v", copied)
	}
	if err := s.DeleteSourceFile("local", "archive/docs"); err != nil {
		t.Fatalf("delete moved docs: %v", err)
	}
	results, err := s.SearchSourceFiles("local", "Guide note", 10)
	if err != nil {
		t.Fatalf("search description: %v", err)
	}
	if len(results) != 1 || results[0].Path != "copies/docs/guide.txt" {
		t.Fatalf("expected only copied metadata after delete: %#v", results)
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

	if err := s.LogAccess("share-view", "report.txt", "192.0.2.10", "browser"); err != nil {
		t.Fatalf("log share view: %v", err)
	}
	if err := s.LogAccess("share-download", "report.txt", "192.0.2.10", "browser"); err != nil {
		t.Fatalf("log share download: %v", err)
	}
	if err := s.LogAccess("direct", "report.txt", "192.0.2.11", "client"); err != nil {
		t.Fatalf("log direct access: %v", err)
	}
	analytics, err := s.LinkAnalytics()
	if err != nil {
		t.Fatalf("link analytics: %v", err)
	}
	if len(analytics.ShareVisits) != 2 {
		t.Fatalf("share visits = %d, want 2", len(analytics.ShareVisits))
	}
	if len(analytics.DirectLinkAccesses) != 1 || analytics.DirectLinkAccesses[0].Path != "report.txt" {
		t.Fatalf("unexpected direct link accesses: %#v", analytics.DirectLinkAccesses)
	}
	if len(analytics.DownloadRanking) != 1 || analytics.DownloadRanking[0].Path != "report.txt" || analytics.DownloadRanking[0].Count != 2 {
		t.Fatalf("unexpected download ranking: %#v", analytics.DownloadRanking)
	}
}

func TestDeleteExpiredShares(t *testing.T) {
	s := newTestStore(t)
	file, err := s.FilePath("report.txt")
	if err != nil {
		t.Fatalf("file path: %v", err)
	}
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	expired, err := s.CreateShare("report.txt", time.Now().Add(-time.Hour).Format(time.RFC3339), "")
	if err != nil {
		t.Fatalf("create expired share: %v", err)
	}
	active, err := s.CreateShare("report.txt", time.Now().Add(time.Hour).Format(time.RFC3339), "")
	if err != nil {
		t.Fatalf("create active share: %v", err)
	}
	permanent, err := s.CreateShare("report.txt", "", "")
	if err != nil {
		t.Fatalf("create permanent share: %v", err)
	}

	deleted, err := s.DeleteExpiredShares()
	if err != nil {
		t.Fatalf("delete expired shares: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, err := s.ResolveShare(expired.Token, ""); err == nil {
		t.Fatal("expected expired share to be deleted")
	}
	if _, err := s.ResolveShare(active.Token, ""); err != nil {
		t.Fatalf("active share should remain: %v", err)
	}
	if _, err := s.ResolveShare(permanent.Token, ""); err != nil {
		t.Fatalf("permanent share should remain: %v", err)
	}
	count, err := s.ShareCount()
	if err != nil {
		t.Fatalf("share count: %v", err)
	}
	if count != 2 {
		t.Fatalf("share count = %d, want 2", count)
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

func TestApplyFileListRules(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveSettings(map[string]string{"privatePathList": "secret"}); err != nil {
		t.Fatalf("save private path rules: %v", err)
	}
	files := []domain.FileEntry{
		{Name: "readme.txt", Path: "public/readme.txt", Type: "file"},
		{Name: "hidden.txt", Path: "source-hidden/hidden.txt", Type: "file"},
		{Name: "blocked.txt", Path: "source-blocked/blocked.txt", Type: "file"},
		{Name: "secret.txt", Path: "secret/secret.txt", Type: "file"},
	}
	source := domain.StorageSource{ID: 1, HiddenPaths: "source-hidden", BlockedPaths: "source-blocked"}

	adminFiles := s.applyFileListRules(fileListRuleContext{Source: source}, append([]domain.FileEntry(nil), files...))
	if len(adminFiles) != len(files) {
		t.Fatalf("admin files were filtered: %#v", adminFiles)
	}
	publicFiles := s.applyFileListRules(fileListRuleContext{Source: source, PublicOnly: true}, append([]domain.FileEntry(nil), files...))
	if len(publicFiles) != 1 || publicFiles[0].Path != "public/readme.txt" {
		t.Fatalf("unexpected public files: %#v", publicFiles)
	}
}

func TestDirectoryPasswordRules(t *testing.T) {
	s := newTestStore(t)
	vault, err := s.FilePath("vault/private")
	if err != nil {
		t.Fatalf("vault path: %v", err)
	}
	if err := os.MkdirAll(vault, 0o755); err != nil {
		t.Fatalf("mkdir vault: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vault, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := s.SaveSettings(map[string]string{"directoryPasswordRules": "vault = open\nvault/private = deeper"}); err != nil {
		t.Fatalf("save directory password rules: %v", err)
	}

	rootFiles, err := s.ListSourceFilesWithPassword("local", "", true, "")
	if err != nil {
		t.Fatalf("list root: %v", err)
	}
	if len(rootFiles) != 1 || rootFiles[0].Path != "vault" {
		t.Fatalf("unexpected root files: %#v", rootFiles)
	}
	if _, err := s.ListSourceFilesWithPassword("local", "vault", true, "wrong"); err == nil {
		t.Fatal("expected wrong password to block protected directory")
	}
	if _, err := s.ListSourceFilesWithPassword("local", "vault", true, "open"); err != nil {
		t.Fatalf("expected parent password to open protected directory: %v", err)
	}
	if _, err := s.SourceDownloadWithPassword("local", "vault/private/secret.txt", true, "open"); err == nil {
		t.Fatal("expected more specific directory password to win")
	}
	download, err := s.SourceDownloadWithPassword("local", "vault/private/secret.txt", true, "deeper")
	if err != nil {
		t.Fatalf("download with password: %v", err)
	}
	defer download.Reader.Close()
	body, err := io.ReadAll(download.Reader)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if string(body) != "secret" {
		t.Fatalf("unexpected download body: %q", string(body))
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
