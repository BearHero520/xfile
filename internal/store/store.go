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
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"xfile/internal/config"
	"xfile/internal/domain"
)

type Store struct {
	db  *sql.DB
	cfg config.Config
}

func New(db *sql.DB, cfg config.Config) *Store {
	_ = os.MkdirAll(cfg.FilesDir, 0o755)
	s := &Store{db: db, cfg: cfg}
	_ = s.ensureDefaultStorageSources()
	return s
}

var supportedStorageTypes = map[string]string{
	"local":       "本地存储",
	"s3":          "S3 / MinIO",
	"webdav":      "WebDAV",
	"aliyun_oss":  "阿里云 OSS",
	"tencent_cos": "腾讯云 COS",
}

var storageKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{2,64}$`)

func (s *Store) ensureDefaultStorageSources() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM storage_sources`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	defaults := []struct {
		name, key, sourceType, root string
		public, enabled             int
	}{
		{name: "本地文件", key: "local", sourceType: "local", root: s.cfg.FilesDir, public: 1, enabled: 1},
		{name: "S3 / MinIO", key: "s3", sourceType: "s3", public: 0, enabled: 0},
		{name: "WebDAV", key: "webdav", sourceType: "webdav", public: 0, enabled: 0},
		{name: "阿里云 OSS", key: "aliyun", sourceType: "aliyun_oss", public: 0, enabled: 0},
		{name: "腾讯云 COS", key: "tencent", sourceType: "tencent_cos", public: 0, enabled: 0},
	}
	for index, source := range defaults {
		if _, err := s.db.Exec(`INSERT INTO storage_sources(name, key, type, root_path, public, enabled, order_num)
			VALUES(?, ?, ?, ?, ?, ?, ?)`, source.name, source.key, source.sourceType, source.root, source.public, source.enabled, index); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Settings() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := map[string]string{
		"siteName":                s.cfg.SiteName,
		"rootName":                "首页",
		"webdav":                  "disabled",
		"webdavMountPath":         "/dav",
		"webdavReadOnly":          "disabled",
		"webdavUsername":          "",
		"publicIndex":             "disabled",
		"storageProvider":         "local",
		"storageLocalRoot":        s.cfg.FilesDir,
		"storageS3Endpoint":       "",
		"storageS3Bucket":         "",
		"storageS3Region":         "",
		"storageS3AccessKey":      "",
		"storageS3SecretKey":      "",
		"storageS3Prefix":         "",
		"storageAliyunEndpoint":   "",
		"storageAliyunBucket":     "",
		"storageAliyunAccessKey":  "",
		"storageAliyunSecretKey":  "",
		"storageAliyunPrefix":     "",
		"storageWebDAVURL":        "",
		"storageWebDAVUsername":   "",
		"storageWebDAVPassword":   "",
		"storageWebDAVRoot":       "",
		"storageTencentEndpoint":  "",
		"storageTencentBucket":    "",
		"storageTencentSecretID":  "",
		"storageTencentSecretKey": "",
		"storageTencentPrefix":    "",
		"allowUpload":             "enabled",
		"maxUploadMB":             "512",
		"uploadAllowExtensions":   "",
		"uploadDenyExtensions":    "",
		"uploadPathAllowList":     "",
		"uploadPathDenyList":      "",
		"uploadOverwrite":         "enabled",
		"ipAllowList":             "",
		"ipDenyList":              "",
		"privatePathList":         "",
		"refererProtection":       "disabled",
		"refererAllowList":        "",
		"downloadLimitPerMinute":  "0",
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

func (s *Store) IsInitialized() (bool, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) CreateSuperAdmin(username, password string) (domain.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return domain.User{}, errors.New("username is required")
	}
	if len(password) < 8 {
		return domain.User{}, errors.New("password must be at least 8 characters")
	}
	initialized, err := s.IsInitialized()
	if err != nil {
		return domain.User{}, err
	}
	if initialized {
		return domain.User{}, errors.New("system is already initialized")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}
	res, err := s.db.Exec(`INSERT INTO users(username, password_hash, role) VALUES(?, ?, 'super_admin')`, username, string(hash))
	if err != nil {
		return domain.User{}, err
	}
	id, _ := res.LastInsertId()
	return domain.User{ID: id, Username: username, Role: "super_admin", CreatedAt: time.Now().Format(time.RFC3339)}, nil
}

func (s *Store) AuthenticateUser(username, password string) (domain.User, error) {
	username = strings.TrimSpace(username)
	var user domain.User
	var passwordHash string
	err := s.db.QueryRow(`SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &passwordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		return domain.User{}, errors.New("invalid username or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return domain.User{}, errors.New("invalid username or password")
	}
	return user, nil
}

func (s *Store) Users() ([]domain.User, error) {
	rows, err := s.db.Query(`SELECT id, username, role, created_at FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) CreateUser(username, password, role string) (domain.User, error) {
	username = strings.TrimSpace(username)
	role = normalizeUserRole(role)
	if username == "" {
		return domain.User{}, errors.New("username is required")
	}
	if len(password) < 8 {
		return domain.User{}, errors.New("password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}
	res, err := s.db.Exec(`INSERT INTO users(username, password_hash, role) VALUES(?, ?, ?)`, username, string(hash), role)
	if err != nil {
		return domain.User{}, err
	}
	id, _ := res.LastInsertId()
	return domain.User{ID: id, Username: username, Role: role, CreatedAt: time.Now().Format(time.RFC3339)}, nil
}

func (s *Store) UpdateUser(id int64, username, password, role string) (domain.User, error) {
	username = strings.TrimSpace(username)
	role = normalizeUserRole(role)
	if id < 1 {
		return domain.User{}, errors.New("invalid user id")
	}
	if username == "" {
		return domain.User{}, errors.New("username is required")
	}
	if strings.TrimSpace(password) != "" && len(password) < 8 {
		return domain.User{}, errors.New("password must be at least 8 characters")
	}

	if strings.TrimSpace(password) == "" {
		if _, err := s.db.Exec(`UPDATE users SET username = ?, role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, username, role, id); err != nil {
			return domain.User{}, err
		}
	} else {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return domain.User{}, err
		}
		if _, err := s.db.Exec(`UPDATE users SET username = ?, password_hash = ?, role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, username, string(hash), role, id); err != nil {
			return domain.User{}, err
		}
	}

	var user domain.User
	err := s.db.QueryRow(`SELECT id, username, role, created_at FROM users WHERE id = ?`, id).Scan(&user.ID, &user.Username, &user.Role, &user.CreatedAt)
	return user, err
}

func (s *Store) DeleteUser(id int64) error {
	if id < 1 {
		return errors.New("invalid user id")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count <= 1 {
		return errors.New("at least one user is required")
	}
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
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
	if err := tx.Commit(); err != nil {
		return err
	}
	if root, ok := settings["storageLocalRoot"]; ok {
		_, err = s.db.Exec(`UPDATE storage_sources SET root_path = ?, updated_at = CURRENT_TIMESTAMP WHERE key = 'local'`, strings.TrimSpace(root))
	}
	return err
}

func (s *Store) PublicSite(loggedIn bool) (domain.PublicSite, error) {
	settings, err := s.Settings()
	if err != nil {
		return domain.PublicSite{}, err
	}
	initialized, err := s.IsInitialized()
	if err != nil {
		return domain.PublicSite{}, err
	}
	sources, err := s.StorageSources(!loggedIn)
	if err != nil {
		return domain.PublicSite{}, err
	}
	return domain.PublicSite{
		SiteName:    settings["siteName"],
		RootName:    settings["rootName"],
		Initialized: initialized,
		LoggedIn:    loggedIn,
		Sources:     sources,
	}, nil
}

func (s *Store) StorageSources(publicOnly bool) ([]domain.StorageSource, error) {
	query := `SELECT id, name, key, type, root_path, public, enabled, order_num, created_at FROM storage_sources`
	if publicOnly {
		query += ` WHERE public = 1 AND enabled = 1`
	}
	query += ` ORDER BY order_num ASC, id ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := make([]domain.StorageSource, 0)
	for rows.Next() {
		source, err := scanStorageSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *Store) StorageSource(key string) (domain.StorageSource, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.StorageSource{}, errors.New("storage source key is required")
	}
	row := s.db.QueryRow(`SELECT id, name, key, type, root_path, public, enabled, order_num, created_at FROM storage_sources WHERE key = ?`, key)
	return scanStorageSource(row)
}

func (s *Store) CreateStorageSource(input domain.StorageSourceInput) (domain.StorageSource, error) {
	normalized, err := s.normalizeStorageSourceInput(input, 0)
	if err != nil {
		return domain.StorageSource{}, err
	}
	res, err := s.db.Exec(`INSERT INTO storage_sources(name, key, type, root_path, public, enabled, order_num)
		VALUES(?, ?, ?, ?, ?, ?, ?)`,
		normalized.Name,
		normalized.Key,
		normalized.Type,
		normalized.RootPath,
		boolInt(normalized.Public),
		boolInt(normalized.Enabled),
		normalized.OrderNum,
	)
	if err != nil {
		return domain.StorageSource{}, err
	}
	id, _ := res.LastInsertId()
	return s.storageSourceByID(id)
}

func (s *Store) UpdateStorageSource(id int64, input domain.StorageSourceInput) (domain.StorageSource, error) {
	if id < 1 {
		return domain.StorageSource{}, errors.New("invalid storage source id")
	}
	normalized, err := s.normalizeStorageSourceInput(input, id)
	if err != nil {
		return domain.StorageSource{}, err
	}
	res, err := s.db.Exec(`UPDATE storage_sources
		SET name = ?, key = ?, type = ?, root_path = ?, public = ?, enabled = ?, order_num = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		normalized.Name,
		normalized.Key,
		normalized.Type,
		normalized.RootPath,
		boolInt(normalized.Public),
		boolInt(normalized.Enabled),
		normalized.OrderNum,
		id,
	)
	if err != nil {
		return domain.StorageSource{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return domain.StorageSource{}, err
	}
	if affected == 0 {
		return domain.StorageSource{}, sql.ErrNoRows
	}
	return s.storageSourceByID(id)
}

func (s *Store) DeleteStorageSource(id int64) error {
	if id < 1 {
		return errors.New("invalid storage source id")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM storage_sources`).Scan(&count); err != nil {
		return err
	}
	if count <= 1 {
		return errors.New("at least one storage source is required")
	}
	res, err := s.db.Exec(`DELETE FROM storage_sources WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) storageSourceByID(id int64) (domain.StorageSource, error) {
	row := s.db.QueryRow(`SELECT id, name, key, type, root_path, public, enabled, order_num, created_at FROM storage_sources WHERE id = ?`, id)
	return scanStorageSource(row)
}

func (s *Store) normalizeStorageSourceInput(input domain.StorageSourceInput, currentID int64) (domain.StorageSourceInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Key = strings.TrimSpace(input.Key)
	input.Type = strings.TrimSpace(input.Type)
	input.RootPath = strings.TrimSpace(input.RootPath)
	if input.Name == "" {
		return domain.StorageSourceInput{}, errors.New("storage source name is required")
	}
	if !storageKeyPattern.MatchString(input.Key) {
		return domain.StorageSourceInput{}, errors.New("storage source key must be 2-64 letters, numbers, underscores, or dashes")
	}
	if _, ok := supportedStorageTypes[input.Type]; !ok {
		return domain.StorageSourceInput{}, errors.New("storage source type is not supported")
	}
	var existingID int64
	err := s.db.QueryRow(`SELECT id FROM storage_sources WHERE key = ?`, input.Key).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.StorageSourceInput{}, err
	}
	if err == nil && existingID != currentID {
		return domain.StorageSourceInput{}, errors.New("storage source key already exists")
	}
	if input.Type == "local" {
		if input.RootPath == "" {
			input.RootPath = s.cfg.FilesDir
		}
		if err := os.MkdirAll(input.RootPath, 0o755); err != nil {
			return domain.StorageSourceInput{}, err
		}
	} else if input.Enabled {
		return domain.StorageSourceInput{}, errors.New("storage adapter is not implemented yet")
	}
	return input, nil
}

type storageSourceScanner interface {
	Scan(dest ...any) error
}

func scanStorageSource(row storageSourceScanner) (domain.StorageSource, error) {
	var source domain.StorageSource
	var public, enabled int
	if err := row.Scan(&source.ID, &source.Name, &source.Key, &source.Type, &source.RootPath, &public, &enabled, &source.OrderNum, &source.CreatedAt); err != nil {
		return domain.StorageSource{}, err
	}
	source.Public = public == 1
	source.Enabled = enabled == 1
	source.TypeLabel = supportedStorageTypes[source.Type]
	if source.TypeLabel == "" {
		source.TypeLabel = source.Type
	}
	return source, nil
}

func (s *Store) ListSourceFiles(storageKey, rel string, publicOnly bool) ([]domain.FileEntry, error) {
	source, err := s.StorageSource(storageKey)
	if err != nil {
		return nil, err
	}
	if !source.Enabled || (publicOnly && !source.Public) {
		return nil, errors.New("storage source is not available")
	}
	root, err := s.sourceSafePath(source, rel)
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
		item := domain.FileEntry{
			Name:       entry.Name(),
			Path:       cleanJoin(rel, entry.Name()),
			Type:       entryType,
			Size:       size,
			ModifiedAt: info.ModTime().Format(time.RFC3339),
		}
		if !publicOnly || !s.IsPrivatePath(item.Path) {
			files = append(files, item)
		}
	}
	return files, nil
}

func (s *Store) SourceFilePath(storageKey, rel string, publicOnly bool) (string, error) {
	source, err := s.StorageSource(storageKey)
	if err != nil {
		return "", err
	}
	if !source.Enabled || (publicOnly && !source.Public) {
		return "", errors.New("storage source is not available")
	}
	if publicOnly && s.IsPrivatePath(rel) {
		return "", errors.New("path is private")
	}
	return s.sourceSafePath(source, rel)
}

func (s *Store) UploadAllowed(dirRel, filename string) error {
	filename = filepath.Base(filename)
	if strings.TrimSpace(filename) == "" || filename == "." {
		return errors.New("filename is required")
	}
	targetRel := cleanJoin(dirRel, filename)
	if _, err := s.safePath(targetRel); err != nil {
		return err
	}

	if denyList := s.SettingValue("uploadPathDenyList", ""); pathMatchesRules(targetRel, denyList) {
		return errors.New("upload path is denied")
	}
	if allowList := strings.TrimSpace(s.SettingValue("uploadPathAllowList", "")); allowList != "" && !pathMatchesRules(targetRel, allowList) {
		return errors.New("upload path is not allowed")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = "(none)"
	}
	if extensionInList(ext, s.SettingValue("uploadDenyExtensions", "")) {
		return errors.New("file extension is denied")
	}
	if allowList := strings.TrimSpace(s.SettingValue("uploadAllowExtensions", "")); allowList != "" && !extensionInList(ext, allowList) {
		return errors.New("file extension is not allowed")
	}
	if s.SettingValue("uploadOverwrite", "enabled") != "enabled" {
		target, err := s.safePath(targetRel)
		if err != nil {
			return err
		}
		if _, err := os.Stat(target); err == nil {
			return errors.New("target already exists")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
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

func (s *Store) SearchFiles(query string, limit int) ([]domain.FileEntry, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []domain.FileEntry{}, nil
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	root, err := filepath.Abs(s.cfg.FilesDir)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(query)
	results := make([]domain.FileEntry, 0)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !strings.Contains(strings.ToLower(d.Name()), needle) && !strings.Contains(strings.ToLower(rel), needle) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		entryType := "file"
		size := info.Size()
		if d.IsDir() {
			entryType = "folder"
			size = 0
		}
		results = append(results, domain.FileEntry{
			Name:       d.Name(),
			Path:       rel,
			Type:       entryType,
			Size:       size,
			ModifiedAt: info.ModTime().Format(time.RFC3339),
		})
		if len(results) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
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

func (s *Store) CreateEmptyFile(rel string) (domain.FileEntry, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return domain.FileEntry{}, errors.New("file path is required")
	}
	if strings.HasSuffix(filepath.ToSlash(rel), "/") {
		return domain.FileEntry{}, errors.New("file name is required")
	}
	dirRel := filepath.ToSlash(filepath.Dir(rel))
	if dirRel == "." {
		dirRel = ""
	}
	name := filepath.Base(rel)
	if err := s.UploadAllowed(dirRel, name); err != nil {
		return domain.FileEntry{}, err
	}
	path, err := s.safePath(rel)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return domain.FileEntry{}, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if err := file.Close(); err != nil {
		return domain.FileEntry{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return domain.FileEntry{}, err
	}
	return domain.FileEntry{
		Name:       filepath.Base(path),
		Path:       strings.Trim(filepath.ToSlash(rel), "/"),
		Type:       "file",
		Size:       info.Size(),
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
		StorageSources: storageSourceSummary(settings),
	}, nil
}

func (s *Store) CreateShare(path string, expiresAt string, password string) (domain.Share, error) {
	if _, err := s.safePath(path); err != nil {
		return domain.Share{}, err
	}
	if err := s.ensurePublicPath(path); err != nil {
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
	rows, err := s.db.Query(`SELECT id, token, path, COALESCE(password, ''), COALESCE(expires_at, ''), view_count, download_count, COALESCE(last_access_at, ''), created_at FROM shares ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	shares := make([]domain.Share, 0)
	for rows.Next() {
		var share domain.Share
		var password string
		if err := rows.Scan(&share.ID, &share.Token, &share.Path, &password, &share.ExpiresAt, &share.ViewCount, &share.DownloadCount, &share.LastAccessAt, &share.CreatedAt); err != nil {
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
	err := s.db.QueryRow(`SELECT id, token, path, COALESCE(password, ''), COALESCE(expires_at, ''), view_count, download_count, COALESCE(last_access_at, ''), created_at
		FROM shares
		WHERE token = ? AND (expires_at IS NULL OR expires_at = '' OR expires_at > CURRENT_TIMESTAMP)`, token).
		Scan(&share.ID, &share.Token, &share.Path, &storedPassword, &share.ExpiresAt, &share.ViewCount, &share.DownloadCount, &share.LastAccessAt, &share.CreatedAt)
	if err != nil {
		return domain.Share{}, err
	}
	if err := s.ensurePublicPath(share.Path); err != nil {
		return domain.Share{}, err
	}
	if storedPassword != "" && !verifySharePassword(storedPassword, password) {
		return domain.Share{}, errors.New("invalid share password")
	}
	share.URL = "/s/" + share.Token
	share.Protected = storedPassword != ""
	return share, nil
}

func (s *Store) RecordShareView(id int64) error {
	_, err := s.db.Exec(`UPDATE shares SET view_count = view_count + 1, last_access_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (s *Store) RecordShareDownload(id int64) error {
	_, err := s.db.Exec(`UPDATE shares SET download_count = download_count + 1, last_access_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (s *Store) ShareDetail(token string, password string, child string) (domain.ShareDetail, error) {
	share, err := s.ResolveShare(token, password)
	if err != nil {
		return domain.ShareDetail{}, err
	}
	base, err := s.safePath(share.Path)
	if err != nil {
		return domain.ShareDetail{}, err
	}
	baseInfo, err := os.Stat(base)
	if err != nil {
		return domain.ShareDetail{}, err
	}
	if !baseInfo.IsDir() && strings.TrimSpace(child) != "" {
		return domain.ShareDetail{}, errors.New("shared file does not contain child paths")
	}

	cleanChild, err := cleanRelative(child)
	if err != nil {
		return domain.ShareDetail{}, err
	}
	currentRel := cleanJoin(share.Path, cleanChild)
	if err := s.ensurePublicPath(currentRel); err != nil {
		return domain.ShareDetail{}, err
	}
	path, err := s.safePath(currentRel)
	if err != nil {
		return domain.ShareDetail{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return domain.ShareDetail{}, err
	}
	entryType := "file"
	size := info.Size()
	files := make([]domain.FileEntry, 0)
	if info.IsDir() {
		entryType = "folder"
		size = 0
		files, err = s.ListFiles(currentRel)
		if err != nil {
			return domain.ShareDetail{}, err
		}
		files = s.filterPublicFiles(files)
	}
	return domain.ShareDetail{
		Token:       share.Token,
		Path:        share.Path,
		CurrentPath: currentRel,
		Name:        info.Name(),
		Type:        entryType,
		Size:        size,
		Protected:   share.Protected,
		ExpiresAt:   share.ExpiresAt,
		CreatedAt:   share.CreatedAt,
		Files:       files,
	}, nil
}

func (s *Store) SharedFilePath(token, password, child string) (domain.Share, string, error) {
	share, err := s.ResolveShare(token, password)
	if err != nil {
		return domain.Share{}, "", err
	}
	base, err := s.safePath(share.Path)
	if err != nil {
		return domain.Share{}, "", err
	}
	info, err := os.Stat(base)
	if err != nil {
		return domain.Share{}, "", err
	}
	if !info.IsDir() {
		if strings.TrimSpace(child) != "" {
			return domain.Share{}, "", errors.New("shared file does not contain child paths")
		}
		return share, base, nil
	}
	cleanChild, err := cleanRelative(child)
	if err != nil {
		return domain.Share{}, "", err
	}
	targetRel := cleanJoin(share.Path, cleanChild)
	if err := s.ensurePublicPath(targetRel); err != nil {
		return domain.Share{}, "", err
	}
	target, err := s.safePath(targetRel)
	if err != nil {
		return domain.Share{}, "", err
	}
	return share, target, nil
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
	if err := s.ensurePublicPath(path); err != nil {
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
	rows, err := s.db.Query(`SELECT id, token, path, enabled, access_count, COALESCE(last_access_at, ''), created_at FROM direct_links ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := make([]domain.DirectLink, 0)
	for rows.Next() {
		var link domain.DirectLink
		var enabled int
		if err := rows.Scan(&link.ID, &link.Token, &link.Path, &enabled, &link.AccessCount, &link.LastAccessAt, &link.CreatedAt); err != nil {
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
	err := s.db.QueryRow(`SELECT id, token, path, enabled, access_count, COALESCE(last_access_at, ''), created_at FROM direct_links WHERE token = ? AND enabled = 1`, token).
		Scan(&link.ID, &link.Token, &link.Path, &enabled, &link.AccessCount, &link.LastAccessAt, &link.CreatedAt)
	if err != nil {
		return domain.DirectLink{}, err
	}
	if err := s.ensurePublicPath(link.Path); err != nil {
		return domain.DirectLink{}, err
	}
	link.URL = "/d/" + link.Token
	link.Enabled = enabled == 1
	return link, nil
}

func (s *Store) RecordDirectLinkAccess(id int64) error {
	_, err := s.db.Exec(`UPDATE direct_links SET access_count = access_count + 1, last_access_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
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

func (s *Store) SearchAccessLogs(page, pageSize int, action, path, ip, userAgent, startTime, endTime string) (domain.AccessLogPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	where, args := accessLogFilters(action, path, ip, userAgent, startTime, endTime)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM access_logs`+where, args...).Scan(&total); err != nil {
		return domain.AccessLogPage{}, err
	}

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(`SELECT id, action, path, ip, user_agent, created_at FROM access_logs`+where+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return domain.AccessLogPage{}, err
	}
	defer rows.Close()

	logs := make([]domain.AccessLog, 0)
	for rows.Next() {
		var log domain.AccessLog
		if err := rows.Scan(&log.ID, &log.Action, &log.Path, &log.IP, &log.UserAgent, &log.CreatedAt); err != nil {
			return domain.AccessLogPage{}, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return domain.AccessLogPage{}, err
	}
	return domain.AccessLogPage{Items: logs, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Store) DeleteAccessLogs(olderThanDays int, all bool) (int64, error) {
	if all {
		res, err := s.db.Exec(`DELETE FROM access_logs`)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}
	if olderThanDays < 1 {
		return 0, errors.New("olderThanDays must be at least 1")
	}
	threshold := time.Now().AddDate(0, 0, -olderThanDays).UTC().Format("2006-01-02 15:04:05")
	res, err := s.db.Exec(`DELETE FROM access_logs WHERE created_at < ?`, threshold)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) ensurePublicPath(rel string) error {
	if s.IsPrivatePath(rel) {
		return errors.New("path is private")
	}
	return nil
}

func (s *Store) IsPrivatePath(rel string) bool {
	return pathMatchesRules(rel, s.SettingValue("privatePathList", ""))
}

func (s *Store) filterPublicFiles(files []domain.FileEntry) []domain.FileEntry {
	publicFiles := files[:0]
	for _, file := range files {
		if !s.IsPrivatePath(file.Path) {
			publicFiles = append(publicFiles, file)
		}
	}
	return publicFiles
}

func accessLogFilters(action, path, ip, userAgent, startTime, endTime string) (string, []any) {
	conditions := make([]string, 0, 6)
	args := make([]any, 0, 6)
	if action = strings.TrimSpace(action); action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, action)
	}
	if path = strings.TrimSpace(path); path != "" {
		conditions = append(conditions, "path LIKE ?")
		args = append(args, "%"+path+"%")
	}
	if ip = strings.TrimSpace(ip); ip != "" {
		conditions = append(conditions, "ip LIKE ?")
		args = append(args, "%"+ip+"%")
	}
	if userAgent = strings.TrimSpace(userAgent); userAgent != "" {
		conditions = append(conditions, "user_agent LIKE ?")
		args = append(args, "%"+userAgent+"%")
	}
	if startTime = strings.TrimSpace(startTime); startTime != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, startTime)
	}
	if endTime = strings.TrimSpace(endTime); endTime != "" {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, endTime)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func pathMatchesRules(rel, rulesText string) bool {
	path, err := cleanRelative(rel)
	if err != nil {
		return false
	}
	for _, rule := range splitPathRules(rulesText) {
		if rule == "" || path == rule || strings.HasPrefix(path, rule+"/") {
			return true
		}
	}
	return false
}

func splitPathRules(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})
	rules := make([]string, 0, len(fields))
	for _, field := range fields {
		rawRule := strings.TrimSpace(field)
		if rawRule == "" {
			continue
		}
		rule, err := cleanRelative(rawRule)
		if err == nil {
			rules = append(rules, rule)
		}
	}
	return rules
}

func extensionInList(ext, rulesText string) bool {
	ext = normalizeExtension(ext)
	if ext == "" {
		return false
	}
	for _, rule := range splitExtensionRules(rulesText) {
		if rule == ext {
			return true
		}
	}
	return false
}

func splitExtensionRules(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	rules := make([]string, 0, len(fields))
	for _, field := range fields {
		if rule := normalizeExtension(field); rule != "" {
			rules = append(rules, rule)
		}
	}
	return rules
}

func normalizeExtension(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if value == "(none)" {
		return value
	}
	value = strings.TrimPrefix(value, "*")
	if !strings.HasPrefix(value, ".") {
		value = "." + value
	}
	return value
}

func normalizeUserRole(role string) string {
	role = strings.TrimSpace(role)
	if role == "super_admin" {
		return role
	}
	return "admin"
}

func storageSourceSummary(settings map[string]string) []string {
	provider := strings.TrimSpace(settings["storageProvider"])
	labels := map[string]string{
		"local":       "本地存储",
		"s3":          "S3 / MinIO",
		"aliyun_oss":  "阿里云 OSS",
		"webdav":      "WebDAV",
		"tencent_cos": "腾讯云 COS",
	}
	label := labels[provider]
	if label == "" {
		label = labels["local"]
	}
	sources := []string{label + " / 当前使用"}
	for key, name := range labels {
		if key != provider {
			sources = append(sources, name)
		}
	}
	return sources
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

func (s *Store) sourceSafePath(source domain.StorageSource, rel string) (string, error) {
	if source.Type != "local" {
		return "", errors.New("storage adapter is not implemented yet")
	}
	rootText := strings.TrimSpace(source.RootPath)
	if rootText == "" {
		rootText = s.cfg.FilesDir
	}
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." {
		clean = ""
	}
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", errors.New("invalid path")
	}
	root, err := filepath.Abs(rootText)
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

func cleanRelative(rel string) (string, error) {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." {
		return "", nil
	}
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", errors.New("invalid path")
	}
	return filepath.ToSlash(clean), nil
}

func nullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
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
