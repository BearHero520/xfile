package store

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xfile/internal/config"
	"xfile/internal/domain"
)

type Store struct {
	db  *sql.DB
	cfg config.Config
}

func New(db *sql.DB, cfg config.Config) *Store {
	_ = os.MkdirAll(cfg.FilesDir, 0o755)
	return &Store{db: db, cfg: cfg}
}

func (s *Store) Settings() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := map[string]string{
		"siteName":    s.cfg.SiteName,
		"rootName":    "首页",
		"webdav":      "disabled",
		"publicIndex": "disabled",
		"allowUpload": "enabled",
		"maxUploadMB": "512",
	}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		settings[key] = value
	}
	return settings, rows.Err()
}

func (s *Store) SettingValue(key, fallback string) string {
	settings, err := s.Settings()
	if err != nil {
		return fallback
	}
	if value := strings.TrimSpace(settings[key]); value != "" {
		return value
	}
	return fallback
}

func (s *Store) SaveSettings(settings map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for key, value := range settings {
		if _, err := tx.Exec(`INSERT INTO settings(key, value, updated_at) VALUES(?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, key, value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListFiles(rel string) ([]domain.FileEntry, error) {
	root, err := s.safePath(rel)
	if err != nil {
		return nil, err
	}
	infos, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	files := make([]domain.FileEntry, 0, len(infos))
	for _, entry := range infos {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		entryType := "file"
		size := info.Size()
		if entry.IsDir() {
			entryType = "folder"
			size = 0
		}
		files = append(files, domain.FileEntry{
			Name:       entry.Name(),
			Path:       cleanJoin(rel, entry.Name()),
			Type:       entryType,
			Size:       size,
			ModifiedAt: info.ModTime().Format(time.RFC3339),
		})
	}
	return files, nil
}

func (s *Store) FilePath(rel string) (string, error) {
	return s.safePath(rel)
}

func (s *Store) CreateFolder(rel string) (domain.FileEntry, error) {
	path, err := s.safePath(rel)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if strings.TrimSpace(rel) == "" {
		return domain.FileEntry{}, errors.New("folder path is required")
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return domain.FileEntry{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return domain.FileEntry{}, err
	}
	return domain.FileEntry{
		Name:       filepath.Base(path),
		Path:       strings.Trim(filepath.ToSlash(rel), "/"),
		Type:       "folder",
		Size:       0,
		ModifiedAt: info.ModTime().Format(time.RFC3339),
	}, nil
}

func (s *Store) MoveFile(from, to string) (domain.FileEntry, error) {
	source, err := s.safePath(from)
	if err != nil {
		return domain.FileEntry{}, err
	}
	target, err := s.safePath(to)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return domain.FileEntry{}, errors.New("source and target paths are required")
	}
	if _, err := os.Stat(source); err != nil {
		return domain.FileEntry{}, err
	}
	if _, err := os.Stat(target); err == nil {
		return domain.FileEntry{}, errors.New("target already exists")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return domain.FileEntry{}, err
	}
	if err := os.Rename(source, target); err != nil {
		return domain.FileEntry{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return domain.FileEntry{}, err
	}
	entryType := "file"
	size := info.Size()
	if info.IsDir() {
		entryType = "folder"
		size = 0
	}
	return domain.FileEntry{
		Name:       filepath.Base(target),
		Path:       strings.Trim(filepath.ToSlash(to), "/"),
		Type:       entryType,
		Size:       size,
		ModifiedAt: info.ModTime().Format(time.RFC3339),
	}, nil
}

func (s *Store) DeleteFile(rel string) error {
	path, err := s.safePath(rel)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(s.cfg.FilesDir)
	if err != nil {
		return err
	}
	if path == root {
		return errors.New("refuse to delete storage root")
	}
	return os.RemoveAll(path)
}

func (s *Store) Dashboard() (domain.Dashboard, error) {
	var fileCount, folderCount int
	var totalBytes int64
	recent := make([]domain.FileEntry, 0)

	err := filepath.WalkDir(s.cfg.FilesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == s.cfg.FilesDir {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(s.cfg.FilesDir, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			folderCount++
			return nil
		}
		fileCount++
		totalBytes += info.Size()
		entry := domain.FileEntry{Name: d.Name(), Path: rel, Type: "file", Size: info.Size(), ModifiedAt: info.ModTime().Format(time.RFC3339)}
		recent = append(recent, entry)
		if len(recent) > 8 {
			recent = recent[len(recent)-8:]
		}
		return nil
	})
	if err != nil {
		return domain.Dashboard{}, err
	}

	shareCount, err := s.ShareCount()
	if err != nil {
		return domain.Dashboard{}, err
	}
	logs, err := s.AccessLogs(6)
	if err != nil {
		return domain.Dashboard{}, err
	}
	settings, err := s.Settings()
	if err != nil {
		return domain.Dashboard{}, err
	}

	return domain.Dashboard{
		SiteName:       settings["siteName"],
		StorageRoot:    s.cfg.FilesDir,
		FileCount:      fileCount,
		FolderCount:    folderCount,
		TotalBytes:     totalBytes,
		ShareCount:     shareCount,
		RecentFiles:    recent,
		RecentLogs:     logs,
		StorageSources: []string{"Local storage", "S3 / WebDAV planned", "Offline download planned"},
	}, nil
}

func (s *Store) CreateShare(path string, expiresAt string, password string) (domain.Share, error) {
	if _, err := s.safePath(path); err != nil {
		return domain.Share{}, err
	}
	token, err := randomToken()
	if err != nil {
		return domain.Share{}, err
	}
	res, err := s.db.Exec(`INSERT INTO shares(token, path, password, expires_at) VALUES(?, ?, ?, ?)`, token, path, nullable(hashSharePassword(password)), nullable(expiresAt))
	if err != nil {
		return domain.Share{}, err
	}
	id, _ := res.LastInsertId()
	return domain.Share{ID: id, Token: token, Path: path, URL: "/s/" + token, Protected: strings.TrimSpace(password) != "", ExpiresAt: expiresAt, CreatedAt: time.Now().Format(time.RFC3339)}, nil
}

func (s *Store) Shares() ([]domain.Share, error) {
	rows, err := s.db.Query(`SELECT id, token, path, COALESCE(password, ''), COALESCE(expires_at, ''), created_at FROM shares ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	shares := make([]domain.Share, 0)
	for rows.Next() {
		var share domain.Share
		var password string
		if err := rows.Scan(&share.ID, &share.Token, &share.Path, &password, &share.ExpiresAt, &share.CreatedAt); err != nil {
			return nil, err
		}
		share.URL = "/s/" + share.Token
		share.Protected = password != ""
		shares = append(shares, share)
	}
	return shares, rows.Err()
}

func (s *Store) DeleteShare(id int64) error {
	_, err := s.db.Exec(`DELETE FROM shares WHERE id = ?`, id)
	return err
}

func (s *Store) ResolveShare(token string, password string) (domain.Share, error) {
	var share domain.Share
	var storedPassword string
	err := s.db.QueryRow(`SELECT id, token, path, COALESCE(password, ''), COALESCE(expires_at, ''), created_at
		FROM shares
		WHERE token = ? AND (expires_at IS NULL OR expires_at = '' OR expires_at > CURRENT_TIMESTAMP)`, token).
		Scan(&share.ID, &share.Token, &share.Path, &storedPassword, &share.ExpiresAt, &share.CreatedAt)
	if err != nil {
		return domain.Share{}, err
	}
	if storedPassword != "" && !verifySharePassword(storedPassword, password) {
		return domain.Share{}, errors.New("invalid share password")
	}
	share.URL = "/s/" + share.Token
	share.Protected = storedPassword != ""
	return share, nil
}

func (s *Store) ShareCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM shares`).Scan(&count)
	return count, err
}

func (s *Store) CreateDirectLink(path string) (domain.DirectLink, error) {
	if _, err := s.safePath(path); err != nil {
		return domain.DirectLink{}, err
	}
	token, err := randomToken()
	if err != nil {
		return domain.DirectLink{}, err
	}
	res, err := s.db.Exec(`INSERT INTO direct_links(token, path) VALUES(?, ?)`, token, path)
	if err != nil {
		return domain.DirectLink{}, err
	}
	id, _ := res.LastInsertId()
	return domain.DirectLink{ID: id, Token: token, Path: path, URL: "/d/" + token, Enabled: true, CreatedAt: time.Now().Format(time.RFC3339)}, nil
}

func (s *Store) DirectLinks() ([]domain.DirectLink, error) {
	rows, err := s.db.Query(`SELECT id, token, path, enabled, created_at FROM direct_links ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := make([]domain.DirectLink, 0)
	for rows.Next() {
		var link domain.DirectLink
		var enabled int
		if err := rows.Scan(&link.ID, &link.Token, &link.Path, &enabled, &link.CreatedAt); err != nil {
			return nil, err
		}
		link.URL = "/d/" + link.Token
		link.Enabled = enabled == 1
		links = append(links, link)
	}
	return links, rows.Err()
}

func (s *Store) DeleteDirectLink(id int64) error {
	_, err := s.db.Exec(`DELETE FROM direct_links WHERE id = ?`, id)
	return err
}

func (s *Store) UpdateDirectLink(id int64, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	_, err := s.db.Exec(`UPDATE direct_links SET enabled = ? WHERE id = ?`, value, id)
	return err
}

func (s *Store) ResolveDirectLink(token string) (domain.DirectLink, error) {
	var link domain.DirectLink
	var enabled int
	err := s.db.QueryRow(`SELECT id, token, path, enabled, created_at FROM direct_links WHERE token = ? AND enabled = 1`, token).
		Scan(&link.ID, &link.Token, &link.Path, &enabled, &link.CreatedAt)
	if err != nil {
		return domain.DirectLink{}, err
	}
	link.URL = "/d/" + link.Token
	link.Enabled = enabled == 1
	return link, nil
}

func (s *Store) LogAccess(action, path, ip, userAgent string) error {
	_, err := s.db.Exec(`INSERT INTO access_logs(action, path, ip, user_agent) VALUES(?, ?, ?, ?)`, action, path, ip, userAgent)
	return err
}

func (s *Store) AccessLogs(limit int) ([]domain.AccessLog, error) {
	rows, err := s.db.Query(`SELECT id, action, path, ip, user_agent, created_at FROM access_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := make([]domain.AccessLog, 0)
	for rows.Next() {
		var log domain.AccessLog
		if err := rows.Scan(&log.ID, &log.Action, &log.Path, &log.IP, &log.UserAgent, &log.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (s *Store) safePath(rel string) (string, error) {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." {
		clean = ""
	}
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", errors.New("invalid path")
	}
	root, err := filepath.Abs(s.cfg.FilesDir)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, clean))
	if err != nil {
		return "", err
	}
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return "", errors.New("path escapes storage root")
	}
	return target, nil
}

func cleanJoin(parts ...string) string {
	joined := filepath.ToSlash(filepath.Join(parts...))
	if joined == "." {
		return ""
	}
	return strings.TrimPrefix(joined, "/")
}

func nullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func hashSharePassword(password string) string {
	password = strings.TrimSpace(password)
	if password == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(password))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func verifySharePassword(stored, password string) bool {
	password = strings.TrimSpace(password)
	if strings.HasPrefix(stored, "sha256:") {
		expected := hashSharePassword(password)
		return subtle.ConstantTimeCompare([]byte(stored), []byte(expected)) == 1
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(password)) == 1
}

func randomToken() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
