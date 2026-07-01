package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/crypto/bcrypt"
	"xfile/internal/config"
	"xfile/internal/domain"
)

type Store struct {
	db  *sql.DB
	cfg config.Config
}

type readSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type sourceDownload struct {
	Entry  domain.FileEntry
	Reader readSeekCloser
}

type tempReadSeekCloser struct {
	*os.File
	path string
}

func (t tempReadSeekCloser) Close() error {
	err := t.File.Close()
	_ = os.Remove(t.path)
	return err
}

type s3SourceConfig struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	Prefix    string `json:"prefix"`
	Secure    bool   `json:"secure"`
}

type webDAVSourceConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Root     string `json:"root"`
}

type davMultiStatus struct {
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href     string        `xml:"href"`
	PropStat []davPropStat `xml:"propstat"`
	Status   string        `xml:"status"`
}

type davPropStat struct {
	Prop   davProp `xml:"prop"`
	Status string  `xml:"status"`
}

type davProp struct {
	ResourceType davResourceType `xml:"resourcetype"`
	Length       string          `xml:"getcontentlength"`
	Modified     string          `xml:"getlastmodified"`
}

type davResourceType struct {
	Collection *struct{} `xml:"collection"`
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
		if _, err := s.db.Exec(`INSERT INTO storage_sources(name, key, type, root_path, hidden_paths, blocked_paths, public, enabled, order_num)
			VALUES(?, ?, ?, ?, '', '', ?, ?, ?)`, source.name, source.key, source.sourceType, source.root, source.public, source.enabled, index); err != nil {
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
		"webdavAllowAnonymous":    "disabled",
		"webdavUsername":          "",
		"webdavPassword":          "",
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
		"directoryPasswordRules":  "",
		"disabledOperations":      "",
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
	var disabledOperations string
	err := s.db.QueryRow(`SELECT id, username, password_hash, role, COALESCE(disabled_operations, ''), created_at FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &passwordHash, &user.Role, &disabledOperations, &user.CreatedAt)
	if err != nil {
		return domain.User{}, errors.New("invalid username or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return domain.User{}, errors.New("invalid username or password")
	}
	if err := s.loadUserStorageSourceKeys(&user); err != nil {
		return domain.User{}, err
	}
	user.DisabledOperations = splitOperationRules(disabledOperations)
	return user, nil
}

func (s *Store) UserByUsername(username string) (domain.User, error) {
	username = strings.TrimSpace(username)
	var user domain.User
	var disabledOperations string
	err := s.db.QueryRow(`SELECT id, username, role, COALESCE(disabled_operations, ''), created_at FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.Role, &disabledOperations, &user.CreatedAt)
	if err != nil {
		return domain.User{}, err
	}
	if err := s.loadUserStorageSourceKeys(&user); err != nil {
		return domain.User{}, err
	}
	user.DisabledOperations = splitOperationRules(disabledOperations)
	return user, nil
}

func (s *Store) Users() ([]domain.User, error) {
	rows, err := s.db.Query(`SELECT id, username, role, COALESCE(disabled_operations, ''), created_at FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		var disabledOperations string
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &disabledOperations, &user.CreatedAt); err != nil {
			return nil, err
		}
		user.DisabledOperations = splitOperationRules(disabledOperations)
		if err := s.loadUserStorageSourceKeys(&user); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) CreateUser(username, password, role string) (domain.User, error) {
	return s.CreateUserWithStorageSources(username, password, role, nil)
}

func (s *Store) CreateUserWithStorageSources(username, password, role string, storageSourceKeys []string) (domain.User, error) {
	return s.CreateUserWithStorageAccess(username, password, role, storageSourceKeys, nil)
}

func (s *Store) CreateUserWithStorageAccess(username, password, role string, storageSourceKeys []string, storageSourceRoots map[string][]string) (domain.User, error) {
	return s.CreateUserWithPolicy(username, password, role, storageSourceKeys, storageSourceRoots, nil)
}

func (s *Store) CreateUserWithPolicy(username, password, role string, storageSourceKeys []string, storageSourceRoots map[string][]string, disabledOperations []string) (domain.User, error) {
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
	disabledOperationText, err := normalizeOperationRules(disabledOperations)
	if err != nil {
		return domain.User{}, err
	}
	res, err := s.db.Exec(`INSERT INTO users(username, password_hash, role, disabled_operations) VALUES(?, ?, ?, ?)`, username, string(hash), role, disabledOperationText)
	if err != nil {
		return domain.User{}, err
	}
	id, _ := res.LastInsertId()
	if err := s.setUserStorageSources(id, storageSourceKeys, storageSourceRoots); err != nil {
		return domain.User{}, err
	}
	return s.userByID(id)
}

func (s *Store) UpdateUser(id int64, username, password, role string) (domain.User, error) {
	return s.UpdateUserWithStorageSources(id, username, password, role, nil)
}

func (s *Store) UpdateUserWithStorageSources(id int64, username, password, role string, storageSourceKeys []string) (domain.User, error) {
	return s.UpdateUserWithStorageAccess(id, username, password, role, storageSourceKeys, nil)
}

func (s *Store) UpdateUserWithStorageAccess(id int64, username, password, role string, storageSourceKeys []string, storageSourceRoots map[string][]string) (domain.User, error) {
	return s.UpdateUserWithPolicy(id, username, password, role, storageSourceKeys, storageSourceRoots, nil)
}

func (s *Store) UpdateUserWithPolicy(id int64, username, password, role string, storageSourceKeys []string, storageSourceRoots map[string][]string, disabledOperations []string) (domain.User, error) {
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
	disabledOperationText, err := normalizeOperationRules(disabledOperations)
	if err != nil {
		return domain.User{}, err
	}

	if strings.TrimSpace(password) == "" {
		if _, err := s.db.Exec(`UPDATE users SET username = ?, role = ?, disabled_operations = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, username, role, disabledOperationText, id); err != nil {
			return domain.User{}, err
		}
	} else {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return domain.User{}, err
		}
		if _, err := s.db.Exec(`UPDATE users SET username = ?, password_hash = ?, role = ?, disabled_operations = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, username, string(hash), role, disabledOperationText, id); err != nil {
			return domain.User{}, err
		}
	}

	if err := s.setUserStorageSources(id, storageSourceKeys, storageSourceRoots); err != nil {
		return domain.User{}, err
	}
	return s.userByID(id)
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

func (s *Store) userByID(id int64) (domain.User, error) {
	var user domain.User
	var disabledOperations string
	err := s.db.QueryRow(`SELECT id, username, role, COALESCE(disabled_operations, ''), created_at FROM users WHERE id = ?`, id).Scan(&user.ID, &user.Username, &user.Role, &disabledOperations, &user.CreatedAt)
	if err != nil {
		return domain.User{}, err
	}
	if err := s.loadUserStorageSourceKeys(&user); err != nil {
		return domain.User{}, err
	}
	user.DisabledOperations = splitOperationRules(disabledOperations)
	return user, nil
}

func (s *Store) loadUserStorageSourceKeys(user *domain.User) error {
	rows, err := s.db.Query(`SELECT ss.key, uss.root_paths
		FROM user_storage_sources uss
		JOIN storage_sources ss ON ss.id = uss.storage_source_id
		WHERE uss.user_id = ?
		ORDER BY ss.order_num ASC, ss.id ASC`, user.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	keys := make([]string, 0)
	roots := make(map[string][]string)
	for rows.Next() {
		var key, rootPaths string
		if err := rows.Scan(&key, &rootPaths); err != nil {
			return err
		}
		keys = append(keys, key)
		roots[key] = splitPathRules(rootPaths)
	}
	user.StorageSourceKeys = keys
	user.StorageSourceRoots = roots
	return rows.Err()
}

func (s *Store) setUserStorageSources(userID int64, keys []string, roots map[string][]string) error {
	if userID < 1 {
		return errors.New("invalid user id")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM user_storage_sources WHERE user_id = ?`, userID); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		var sourceID int64
		if err := tx.QueryRow(`SELECT id FROM storage_sources WHERE key = ?`, key).Scan(&sourceID); err != nil {
			return err
		}
		rootPaths, err := normalizeUserStorageRoots(roots[key])
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO user_storage_sources(user_id, storage_source_id, root_paths) VALUES(?, ?, ?)`, userID, sourceID, rootPaths); err != nil {
			return err
		}
	}
	return tx.Commit()
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
	query := `SELECT id, name, key, type, root_path, hidden_paths, blocked_paths, public, enabled, order_num, created_at FROM storage_sources`
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
		if publicOnly {
			source.RootPath = ""
			source.HiddenPaths = ""
			source.BlockedPaths = ""
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *Store) StorageSourcesForUser(user domain.User) ([]domain.StorageSource, error) {
	if user.Role == "super_admin" {
		return s.StorageSources(false)
	}
	rows, err := s.db.Query(`SELECT ss.id, ss.name, ss.key, ss.type, ss.root_path, ss.hidden_paths, ss.blocked_paths, ss.public, ss.enabled, ss.order_num, ss.created_at
		FROM storage_sources ss
		JOIN user_storage_sources uss ON uss.storage_source_id = ss.id
		WHERE uss.user_id = ? AND ss.enabled = 1
		ORDER BY ss.order_num ASC, ss.id ASC`, user.ID)
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
	row := s.db.QueryRow(`SELECT id, name, key, type, root_path, hidden_paths, blocked_paths, public, enabled, order_num, created_at FROM storage_sources WHERE key = ?`, key)
	return scanStorageSource(row)
}

func (s *Store) UserCanAccessStorageSource(user domain.User, key string) bool {
	if user.Role == "super_admin" {
		return true
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "local"
	}
	for _, allowed := range user.StorageSourceKeys {
		if allowed == key {
			return true
		}
	}
	return false
}

func (s *Store) UserCanListStoragePath(user domain.User, key, rel string) bool {
	if user.Role == "super_admin" {
		return true
	}
	if !s.UserCanAccessStorageSource(user, key) {
		return false
	}
	roots := user.StorageSourceRoots[strings.TrimSpace(key)]
	if len(roots) == 0 {
		return true
	}
	clean, err := cleanRelative(rel)
	if err != nil {
		return false
	}
	if clean == "" {
		return true
	}
	for _, root := range roots {
		if pathWithinRoot(clean, root) || pathWithinRoot(root, clean) {
			return true
		}
	}
	return false
}

func (s *Store) UserCanAccessStoragePath(user domain.User, key, rel string) bool {
	if user.Role == "super_admin" {
		return true
	}
	if !s.UserCanAccessStorageSource(user, key) {
		return false
	}
	roots := user.StorageSourceRoots[strings.TrimSpace(key)]
	if len(roots) == 0 {
		return true
	}
	clean, err := cleanRelative(rel)
	if err != nil {
		return false
	}
	for _, root := range roots {
		if pathWithinRoot(clean, root) {
			return true
		}
	}
	return false
}

func (s *Store) FilterStorageFilesForUser(user domain.User, key string, files []domain.FileEntry, includeAncestors bool) []domain.FileEntry {
	if user.Role == "super_admin" {
		return files
	}
	roots := user.StorageSourceRoots[strings.TrimSpace(key)]
	if len(roots) == 0 {
		return files
	}
	filtered := files[:0]
	for _, file := range files {
		if storagePathAllowedByRoots(file.Path, roots, includeAncestors) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func (s *Store) CreateStorageSource(input domain.StorageSourceInput) (domain.StorageSource, error) {
	normalized, err := s.normalizeStorageSourceInput(input, 0)
	if err != nil {
		return domain.StorageSource{}, err
	}
	res, err := s.db.Exec(`INSERT INTO storage_sources(name, key, type, root_path, hidden_paths, blocked_paths, public, enabled, order_num)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		normalized.Name,
		normalized.Key,
		normalized.Type,
		normalized.RootPath,
		normalized.HiddenPaths,
		normalized.BlockedPaths,
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
		SET name = ?, key = ?, type = ?, root_path = ?, hidden_paths = ?, blocked_paths = ?, public = ?, enabled = ?, order_num = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		normalized.Name,
		normalized.Key,
		normalized.Type,
		normalized.RootPath,
		normalized.HiddenPaths,
		normalized.BlockedPaths,
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
	row := s.db.QueryRow(`SELECT id, name, key, type, root_path, hidden_paths, blocked_paths, public, enabled, order_num, created_at FROM storage_sources WHERE id = ?`, id)
	return scanStorageSource(row)
}

func (s *Store) normalizeStorageSourceInput(input domain.StorageSourceInput, currentID int64) (domain.StorageSourceInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Key = strings.TrimSpace(input.Key)
	input.Type = strings.TrimSpace(input.Type)
	input.RootPath = strings.TrimSpace(input.RootPath)
	input.HiddenPaths = normalizePathRulesText(input.HiddenPaths)
	input.BlockedPaths = normalizePathRulesText(input.BlockedPaths)
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
	switch input.Type {
	case "local":
		if input.RootPath == "" {
			input.RootPath = s.cfg.FilesDir
		}
		if err := os.MkdirAll(input.RootPath, 0o755); err != nil {
			return domain.StorageSourceInput{}, err
		}
	case "s3", "aliyun_oss", "tencent_cos":
		normalizedConfig, err := normalizeS3SourceConfig(input.RootPath, input.Enabled)
		if err != nil {
			return domain.StorageSourceInput{}, err
		}
		input.RootPath = normalizedConfig
	case "webdav":
		normalizedConfig, err := normalizeWebDAVSourceConfig(input.RootPath, input.Enabled)
		if err != nil {
			return domain.StorageSourceInput{}, err
		}
		input.RootPath = normalizedConfig
	default:
		if input.Enabled {
			return domain.StorageSourceInput{}, errors.New("storage adapter is not implemented yet")
		}
	}
	if !storageTypeReady(input.Type) && input.Enabled {
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
	if err := row.Scan(&source.ID, &source.Name, &source.Key, &source.Type, &source.RootPath, &source.HiddenPaths, &source.BlockedPaths, &public, &enabled, &source.OrderNum, &source.CreatedAt); err != nil {
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
	return s.ListSourceFilesWithPassword(storageKey, rel, publicOnly, "")
}

func (s *Store) ListSourceFilesWithPassword(storageKey, rel string, publicOnly bool, directoryPassword string) ([]domain.FileEntry, error) {
	source, err := s.availableSource(storageKey, publicOnly)
	if err != nil {
		return nil, err
	}
	if publicOnly && s.IsSourceBlockedPath(source, rel) {
		return nil, errors.New("path is blocked")
	}
	if publicOnly && !s.DirectoryPasswordAllowed(rel, directoryPassword) {
		return nil, errors.New("directory password is required")
	}
	if objectStorageType(source.Type) {
		files, err := s.listS3Files(source, rel)
		if err != nil {
			return nil, err
		}
		return s.applyFileListRules(fileListRuleContext{Source: source, PublicOnly: publicOnly}, files), nil
	}
	if source.Type == "webdav" {
		files, err := s.listWebDAVFiles(source, rel)
		if err != nil {
			return nil, err
		}
		return s.applyFileListRules(fileListRuleContext{Source: source, PublicOnly: publicOnly}, files), nil
	}
	_, root, err := s.sourceRoot(storageKey, publicOnly)
	if err != nil {
		return nil, err
	}
	files, err := s.listFilesInRoot(root, rel)
	if err != nil {
		return nil, err
	}
	return s.applyFileListRules(fileListRuleContext{Source: source, PublicOnly: publicOnly}, files), nil
}

func (s *Store) SourceFilePath(storageKey, rel string, publicOnly bool) (string, error) {
	source, err := s.availableSource(storageKey, publicOnly)
	if err != nil {
		return "", err
	}
	if source.Type != "local" {
		return "", errors.New("storage source does not expose a local file path")
	}
	if publicOnly && s.IsSourceBlockedPath(source, rel) {
		return "", errors.New("path is blocked")
	}
	return s.sourceSafePath(source, rel)
}

func (s *Store) SourceDownload(storageKey, rel string, publicOnly bool) (sourceDownload, error) {
	return s.SourceDownloadWithPassword(storageKey, rel, publicOnly, "")
}

func (s *Store) SourceDownloadWithPassword(storageKey, rel string, publicOnly bool, directoryPassword string) (sourceDownload, error) {
	source, err := s.availableSource(storageKey, publicOnly)
	if err != nil {
		return sourceDownload{}, err
	}
	if publicOnly && s.IsSourceBlockedPath(source, rel) {
		return sourceDownload{}, errors.New("path is blocked")
	}
	if publicOnly && !s.DirectoryPasswordAllowed(rel, directoryPassword) {
		return sourceDownload{}, errors.New("directory password is required")
	}
	switch source.Type {
	case "local":
		path, err := s.sourceSafePath(source, rel)
		if err != nil {
			return sourceDownload{}, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return sourceDownload{}, err
		}
		if info.IsDir() {
			return sourceDownload{}, errors.New("folder download is not implemented yet")
		}
		file, err := os.Open(path)
		if err != nil {
			return sourceDownload{}, err
		}
		return sourceDownload{Entry: fileInfoEntry(rel, info), Reader: file}, nil
	case "s3":
		return s.s3Download(source, rel)
	case "aliyun_oss", "tencent_cos":
		return s.s3Download(source, rel)
	case "webdav":
		return s.webDAVDownload(source, rel)
	default:
		return sourceDownload{}, errors.New("storage adapter is not implemented yet")
	}
}

func (s *Store) sourceRoot(storageKey string, publicOnly bool) (domain.StorageSource, string, error) {
	source, err := s.availableSource(storageKey, publicOnly)
	if err != nil {
		return domain.StorageSource{}, "", err
	}
	if source.Type != "local" {
		return domain.StorageSource{}, "", errors.New("storage source does not expose a local root")
	}
	root, err := s.sourceSafePath(source, "")
	if err != nil {
		return domain.StorageSource{}, "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return domain.StorageSource{}, "", err
	}
	return source, root, nil
}

func (s *Store) availableSource(storageKey string, publicOnly bool) (domain.StorageSource, error) {
	source, err := s.StorageSource(storageKey)
	if err != nil {
		return domain.StorageSource{}, err
	}
	if !source.Enabled || (publicOnly && !source.Public) {
		return domain.StorageSource{}, errors.New("storage source is not available")
	}
	return source, nil
}

func (s *Store) UploadAllowed(dirRel, filename string) error {
	return s.uploadAllowedInRoot(s.cfg.FilesDir, dirRel, filename)
}

func (s *Store) SourceUploadAllowed(storageKey, dirRel, filename string) error {
	source, err := s.availableSource(storageKey, false)
	if err != nil {
		return err
	}
	if objectStorageType(source.Type) {
		return s.s3UploadAllowed(source, dirRel, filename)
	}
	if source.Type == "webdav" {
		return s.webDAVUploadAllowed(source, dirRel, filename)
	}
	_, root, err := s.sourceRoot(storageKey, false)
	if err != nil {
		return err
	}
	return s.uploadAllowedInRoot(root, dirRel, filename)
}

func (s *Store) uploadAllowedInRoot(root, dirRel, filename string) error {
	filename = filepath.Base(filename)
	if strings.TrimSpace(filename) == "" || filename == "." {
		return errors.New("filename is required")
	}
	targetRel := cleanJoin(dirRel, filename)
	if _, err := safePathInRoot(root, targetRel); err != nil {
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
		target, err := safePathInRoot(root, targetRel)
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
	root, err := s.safePath("")
	if err != nil {
		return nil, err
	}
	return s.listFilesInRoot(root, rel)
}

func (s *Store) ListSourceFilesForAdmin(storageKey, rel string) ([]domain.FileEntry, error) {
	source, err := s.availableSource(storageKey, false)
	if err != nil {
		return nil, err
	}
	if objectStorageType(source.Type) {
		files, err := s.listS3Files(source, rel)
		if err != nil {
			return nil, err
		}
		return s.applyFileListRules(fileListRuleContext{Source: source}, files), nil
	}
	if source.Type == "webdav" {
		files, err := s.listWebDAVFiles(source, rel)
		if err != nil {
			return nil, err
		}
		return s.applyFileListRules(fileListRuleContext{Source: source}, files), nil
	}
	_, root, err := s.sourceRoot(storageKey, false)
	if err != nil {
		return nil, err
	}
	files, err := s.listFilesInRoot(root, rel)
	if err != nil {
		return nil, err
	}
	return s.applyFileListRules(fileListRuleContext{Source: source}, files), nil
}

func (s *Store) listFilesInRoot(rootPath, rel string) ([]domain.FileEntry, error) {
	root, err := safePathInRoot(rootPath, rel)
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
	return s.searchFilesInRoot(s.cfg.FilesDir, query, limit)
}

func (s *Store) SearchSourceFiles(storageKey, query string, limit int) ([]domain.FileEntry, error) {
	source, err := s.availableSource(storageKey, false)
	if err != nil {
		return nil, err
	}
	if objectStorageType(source.Type) {
		return s.searchS3Files(source, query, limit)
	}
	if source.Type == "webdav" {
		return s.searchWebDAVFiles(source, query, limit)
	}
	_, root, err := s.sourceRoot(storageKey, false)
	if err != nil {
		return nil, err
	}
	return s.searchFilesInRoot(root, query, limit)
}

func (s *Store) searchFilesInRoot(rootText, query string, limit int) ([]domain.FileEntry, error) {
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

	root, err := filepath.Abs(rootText)
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

func (s *Store) AdminSourceFilePath(storageKey, rel string) (string, error) {
	_, root, err := s.sourceRoot(storageKey, false)
	if err != nil {
		return "", err
	}
	return safePathInRoot(root, rel)
}

func (s *Store) UploadSourceFile(storageKey, dirRel, filename string, reader io.Reader, size int64) (string, error) {
	source, err := s.availableSource(storageKey, false)
	if err != nil {
		return "", err
	}
	name := filepath.Base(filename)
	if err := s.SourceUploadAllowed(storageKey, dirRel, name); err != nil {
		return "", err
	}
	rel := cleanJoin(dirRel, name)
	switch source.Type {
	case "local":
		root, err := s.sourceSafePath(source, "")
		if err != nil {
			return "", err
		}
		dir, err := safePathInRoot(root, dirRel)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		target := filepath.Join(dir, name)
		out, err := os.Create(target)
		if err != nil {
			return "", err
		}
		defer out.Close()
		if _, err := io.Copy(out, reader); err != nil {
			return "", err
		}
		return rel, nil
	case "s3", "aliyun_oss", "tencent_cos":
		return rel, s.putS3Object(source, rel, reader, size)
	case "webdav":
		return rel, s.putWebDAVFile(source, rel, reader)
	default:
		return "", errors.New("storage adapter is not implemented yet")
	}
}

func (s *Store) CreateFolder(rel string) (domain.FileEntry, error) {
	root, err := s.safePath("")
	if err != nil {
		return domain.FileEntry{}, err
	}
	return s.createFolderInRoot(root, rel)
}

func (s *Store) CreateSourceFolder(storageKey, rel string) (domain.FileEntry, error) {
	source, err := s.availableSource(storageKey, false)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if objectStorageType(source.Type) {
		return s.createS3Folder(source, rel)
	}
	if source.Type == "webdav" {
		return s.createWebDAVFolder(source, rel)
	}
	_, root, err := s.sourceRoot(storageKey, false)
	if err != nil {
		return domain.FileEntry{}, err
	}
	return s.createFolderInRoot(root, rel)
}

func (s *Store) createFolderInRoot(root, rel string) (domain.FileEntry, error) {
	path, err := safePathInRoot(root, rel)
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
	root, err := s.safePath("")
	if err != nil {
		return domain.FileEntry{}, err
	}
	return s.createEmptyFileInRoot(root, rel)
}

func (s *Store) CreateSourceEmptyFile(storageKey, rel string) (domain.FileEntry, error) {
	source, err := s.availableSource(storageKey, false)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if objectStorageType(source.Type) {
		return s.createS3EmptyFile(source, rel)
	}
	if source.Type == "webdav" {
		return s.createWebDAVEmptyFile(source, rel)
	}
	_, root, err := s.sourceRoot(storageKey, false)
	if err != nil {
		return domain.FileEntry{}, err
	}
	return s.createEmptyFileInRoot(root, rel)
}

func (s *Store) createEmptyFileInRoot(root, rel string) (domain.FileEntry, error) {
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
	if err := s.uploadAllowedInRoot(root, dirRel, name); err != nil {
		return domain.FileEntry{}, err
	}
	path, err := safePathInRoot(root, rel)
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
	root, err := s.safePath("")
	if err != nil {
		return domain.FileEntry{}, err
	}
	return s.moveFileInRoot(root, from, to)
}

func (s *Store) MoveSourceFile(storageKey, from, to string) (domain.FileEntry, error) {
	source, err := s.availableSource(storageKey, false)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if objectStorageType(source.Type) {
		return s.moveS3Object(source, from, to)
	}
	if source.Type == "webdav" {
		return s.moveWebDAVPath(source, from, to)
	}
	_, root, err := s.sourceRoot(storageKey, false)
	if err != nil {
		return domain.FileEntry{}, err
	}
	return s.moveFileInRoot(root, from, to)
}

func (s *Store) moveFileInRoot(root, from, to string) (domain.FileEntry, error) {
	source, err := safePathInRoot(root, from)
	if err != nil {
		return domain.FileEntry{}, err
	}
	target, err := safePathInRoot(root, to)
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
	root, err := s.safePath("")
	if err != nil {
		return err
	}
	return s.deleteFileInRoot(root, rel)
}

func (s *Store) DeleteSourceFile(storageKey, rel string) error {
	source, err := s.availableSource(storageKey, false)
	if err != nil {
		return err
	}
	if objectStorageType(source.Type) {
		return s.deleteS3Object(source, rel)
	}
	if source.Type == "webdav" {
		return s.deleteWebDAVPath(source, rel)
	}
	_, root, err := s.sourceRoot(storageKey, false)
	if err != nil {
		return err
	}
	return s.deleteFileInRoot(root, rel)
}

func (s *Store) deleteFileInRoot(root, rel string) error {
	path, err := safePathInRoot(root, rel)
	if err != nil {
		return err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if path == absRoot {
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
		files = s.applyFileListRules(fileListRuleContext{PublicOnly: true}, files)
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

func (s *Store) DirectoryPasswordAllowed(rel, password string) bool {
	rule, ok := matchingDirectoryPasswordRule(rel, s.SettingValue("directoryPasswordRules", ""))
	if !ok {
		return true
	}
	return verifySharePassword(hashSharePassword(rule.Password), password)
}

func (s *Store) IsSourceHiddenPath(source domain.StorageSource, rel string) bool {
	return s.IsPrivatePath(rel) || pathMatchesRules(rel, source.HiddenPaths)
}

func (s *Store) IsSourceBlockedPath(source domain.StorageSource, rel string) bool {
	return s.IsPrivatePath(rel) || pathMatchesRules(rel, source.BlockedPaths)
}

type fileListRuleContext struct {
	Source     domain.StorageSource
	PublicOnly bool
}

func (s *Store) applyFileListRules(ctx fileListRuleContext, files []domain.FileEntry) []domain.FileEntry {
	if !ctx.PublicOnly {
		return files
	}
	filtered := files[:0]
	for _, file := range files {
		if s.fileHiddenByRules(ctx, file.Path) || s.fileBlockedByRules(ctx, file.Path) {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

func (s *Store) fileHiddenByRules(ctx fileListRuleContext, rel string) bool {
	if s.IsPrivatePath(rel) {
		return true
	}
	if ctx.Source.ID == 0 {
		return false
	}
	return pathMatchesRules(rel, ctx.Source.HiddenPaths)
}

func (s *Store) fileBlockedByRules(ctx fileListRuleContext, rel string) bool {
	if s.IsPrivatePath(rel) {
		return true
	}
	if ctx.Source.ID == 0 {
		return false
	}
	return pathMatchesRules(rel, ctx.Source.BlockedPaths)
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

type directoryPasswordRule struct {
	Path     string
	Password string
}

func matchingDirectoryPasswordRule(rel, rulesText string) (directoryPasswordRule, bool) {
	path, err := cleanRelative(rel)
	if err != nil {
		return directoryPasswordRule{}, false
	}
	var matched directoryPasswordRule
	for _, rule := range splitDirectoryPasswordRules(rulesText) {
		if rule.Path == path || strings.HasPrefix(path, rule.Path+"/") {
			if len(rule.Path) > len(matched.Path) {
				matched = rule
			}
		}
	}
	return matched, matched.Path != ""
}

func splitDirectoryPasswordRules(value string) []directoryPasswordRule {
	lines := strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	rules := make([]directoryPasswordRule, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pathText, password, ok := strings.Cut(line, "=")
		if !ok {
			pathText, password, ok = strings.Cut(line, ":")
		}
		if !ok {
			continue
		}
		pathText = strings.TrimSpace(pathText)
		password = strings.TrimSpace(password)
		if pathText == "" || password == "" {
			continue
		}
		path, err := cleanRelative(pathText)
		if err != nil || path == "" {
			continue
		}
		rules = append(rules, directoryPasswordRule{Path: path, Password: password})
	}
	return rules
}

func normalizeUserStorageRoots(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}
	seen := make(map[string]bool)
	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		clean, err := cleanRelative(path)
		if err != nil {
			return "", err
		}
		if clean == "" || seen[clean] {
			continue
		}
		seen[clean] = true
		normalized = append(normalized, clean)
	}
	return strings.Join(normalized, "\n"), nil
}

func storagePathAllowedByRoots(rel string, roots []string, includeAncestors bool) bool {
	clean, err := cleanRelative(rel)
	if err != nil {
		return false
	}
	for _, root := range roots {
		if pathWithinRoot(clean, root) || (includeAncestors && pathWithinRoot(root, clean)) {
			return true
		}
	}
	return false
}

func pathWithinRoot(rel, root string) bool {
	rel, relErr := cleanRelative(rel)
	root, rootErr := cleanRelative(root)
	if relErr != nil || rootErr != nil {
		return false
	}
	return root == "" || rel == root || strings.HasPrefix(rel, root+"/")
}

var validOperationRules = map[string]bool{
	"preview":     true,
	"download":    true,
	"upload":      true,
	"rename":      true,
	"move":        true,
	"copy":        true,
	"delete":      true,
	"share":       true,
	"directLinks": true,
}

func splitOperationRules(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	rules := make([]string, 0, len(fields))
	for _, field := range fields {
		rule := strings.TrimSpace(field)
		if validOperationRules[rule] {
			rules = append(rules, rule)
		}
	}
	return rules
}

func normalizeOperationRules(rules []string) (string, error) {
	if len(rules) == 0 {
		return "", nil
	}
	seen := make(map[string]bool)
	normalized := make([]string, 0, len(rules))
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" || seen[rule] {
			continue
		}
		if !validOperationRules[rule] {
			return "", errors.New("operation permission rule is invalid")
		}
		seen[rule] = true
		normalized = append(normalized, rule)
	}
	return strings.Join(normalized, ","), nil
}

func normalizePathRulesText(value string) string {
	rules := splitPathRules(value)
	if len(rules) == 0 {
		return ""
	}
	return strings.Join(rules, "\n")
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

func storageTypeReady(sourceType string) bool {
	return sourceType == "local" || sourceType == "webdav" || objectStorageType(sourceType)
}

func objectStorageType(sourceType string) bool {
	return sourceType == "s3" || sourceType == "aliyun_oss" || sourceType == "tencent_cos"
}

func normalizeS3SourceConfig(raw string, enabled bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if enabled {
			return "", errors.New("S3 config is required")
		}
		return "", nil
	}
	var cfg s3SourceConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return "", errors.New("S3 config must be valid JSON")
	}
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.AccessKey = strings.TrimSpace(cfg.AccessKey)
	cfg.SecretKey = strings.TrimSpace(cfg.SecretKey)
	cfg.Prefix = strings.Trim(strings.TrimSpace(filepath.ToSlash(cfg.Prefix)), "/")
	if parsed, err := url.Parse(cfg.Endpoint); err == nil && parsed.Host != "" {
		cfg.Secure = parsed.Scheme == "https"
		cfg.Endpoint = parsed.Host
	}
	if enabled && (cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "") {
		return "", errors.New("S3 endpoint, bucket, access key, and secret key are required")
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func normalizeWebDAVSourceConfig(raw string, enabled bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if enabled {
			return "", errors.New("WebDAV config is required")
		}
		return "", nil
	}
	var cfg webDAVSourceConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return "", errors.New("WebDAV config must be valid JSON")
	}
	cfg.URL = strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.Password = strings.TrimSpace(cfg.Password)
	cfg.Root = strings.Trim(strings.TrimSpace(filepath.ToSlash(cfg.Root)), "/")
	if enabled && cfg.URL == "" {
		return "", errors.New("WebDAV URL is required")
	}
	if cfg.URL != "" {
		parsed, err := url.Parse(cfg.URL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", errors.New("WebDAV URL is invalid")
		}
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func s3ConfigFromSource(source domain.StorageSource) (s3SourceConfig, error) {
	var cfg s3SourceConfig
	if err := json.Unmarshal([]byte(source.RootPath), &cfg); err != nil {
		return s3SourceConfig{}, errors.New("S3 config is invalid")
	}
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.AccessKey = strings.TrimSpace(cfg.AccessKey)
	cfg.SecretKey = strings.TrimSpace(cfg.SecretKey)
	cfg.Prefix = strings.Trim(strings.TrimSpace(filepath.ToSlash(cfg.Prefix)), "/")
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return s3SourceConfig{}, errors.New("S3 config is incomplete")
	}
	return cfg, nil
}

func webDAVConfigFromSource(source domain.StorageSource) (webDAVSourceConfig, error) {
	var cfg webDAVSourceConfig
	if err := json.Unmarshal([]byte(source.RootPath), &cfg); err != nil {
		return webDAVSourceConfig{}, errors.New("WebDAV config is invalid")
	}
	cfg.URL = strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.Password = strings.TrimSpace(cfg.Password)
	cfg.Root = strings.Trim(strings.TrimSpace(filepath.ToSlash(cfg.Root)), "/")
	if cfg.URL == "" {
		return webDAVSourceConfig{}, errors.New("WebDAV config is incomplete")
	}
	return cfg, nil
}

func (s *Store) s3Client(source domain.StorageSource) (*minio.Client, s3SourceConfig, error) {
	cfg, err := s3ConfigFromSource(source)
	if err != nil {
		return nil, s3SourceConfig{}, err
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, s3SourceConfig{}, err
	}
	return client, cfg, nil
}

func s3ObjectKey(cfg s3SourceConfig, rel string) (string, error) {
	clean, err := cleanRelative(rel)
	if err != nil {
		return "", err
	}
	return cleanJoin(cfg.Prefix, clean), nil
}

func s3ListPrefix(cfg s3SourceConfig, rel string) (string, error) {
	key, err := s3ObjectKey(cfg, rel)
	if err != nil {
		return "", err
	}
	if key != "" && !strings.HasSuffix(key, "/") {
		key += "/"
	}
	return key, nil
}

func s3EntryFromObject(cfg s3SourceConfig, object minio.ObjectInfo) domain.FileEntry {
	key := strings.TrimPrefix(object.Key, strings.TrimSuffix(cfg.Prefix, "/"))
	key = strings.TrimPrefix(key, "/")
	entryType := "file"
	size := object.Size
	if strings.HasSuffix(key, "/") {
		entryType = "folder"
		size = 0
		key = strings.TrimSuffix(key, "/")
	}
	name := pathBase(key)
	return domain.FileEntry{
		Name:       name,
		Path:       key,
		Type:       entryType,
		Size:       size,
		ModifiedAt: object.LastModified.Format(time.RFC3339),
	}
}

func fileInfoEntry(rel string, info os.FileInfo) domain.FileEntry {
	entryType := "file"
	size := info.Size()
	if info.IsDir() {
		entryType = "folder"
		size = 0
	}
	return domain.FileEntry{
		Name:       filepath.Base(rel),
		Path:       strings.Trim(filepath.ToSlash(rel), "/"),
		Type:       entryType,
		Size:       size,
		ModifiedAt: info.ModTime().Format(time.RFC3339),
	}
}

func pathBase(path string) string {
	path = strings.Trim(strings.TrimSpace(filepath.ToSlash(path)), "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func (s *Store) listS3Files(source domain.StorageSource, rel string) ([]domain.FileEntry, error) {
	client, cfg, err := s.s3Client(source)
	if err != nil {
		return nil, err
	}
	prefix, err := s3ListPrefix(cfg, rel)
	if err != nil {
		return nil, err
	}
	files := make([]domain.FileEntry, 0)
	for object := range client.ListObjects(context.Background(), cfg.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: false}) {
		if object.Err != nil {
			return nil, object.Err
		}
		if object.Key == prefix {
			continue
		}
		entry := s3EntryFromObject(cfg, object)
		if entry.Path == "" {
			continue
		}
		files = append(files, entry)
	}
	return files, nil
}

func (s *Store) searchS3Files(source domain.StorageSource, query string, limit int) ([]domain.FileEntry, error) {
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
	client, cfg, err := s.s3Client(source)
	if err != nil {
		return nil, err
	}
	prefix, err := s3ListPrefix(cfg, "")
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(query)
	results := make([]domain.FileEntry, 0)
	for object := range client.ListObjects(context.Background(), cfg.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return nil, object.Err
		}
		entry := s3EntryFromObject(cfg, object)
		if entry.Path == "" || !strings.Contains(strings.ToLower(entry.Path), needle) {
			continue
		}
		results = append(results, entry)
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (s *Store) s3Download(source domain.StorageSource, rel string) (sourceDownload, error) {
	client, cfg, err := s.s3Client(source)
	if err != nil {
		return sourceDownload{}, err
	}
	key, err := s3ObjectKey(cfg, rel)
	if err != nil {
		return sourceDownload{}, err
	}
	object, err := client.GetObject(context.Background(), cfg.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return sourceDownload{}, err
	}
	info, err := object.Stat()
	if err != nil {
		object.Close()
		return sourceDownload{}, err
	}
	return sourceDownload{Entry: s3EntryFromObject(cfg, info), Reader: object}, nil
}

func (s *Store) s3UploadAllowed(source domain.StorageSource, dirRel, filename string) error {
	filename = filepath.Base(filename)
	if strings.TrimSpace(filename) == "" || filename == "." {
		return errors.New("filename is required")
	}
	targetRel := cleanJoin(dirRel, filename)
	if _, err := cleanRelative(targetRel); err != nil {
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
	if s.SettingValue("uploadOverwrite", "enabled") == "enabled" {
		return nil
	}
	client, cfg, err := s.s3Client(source)
	if err != nil {
		return err
	}
	key, err := s3ObjectKey(cfg, targetRel)
	if err != nil {
		return err
	}
	_, err = client.StatObject(context.Background(), cfg.Bucket, key, minio.StatObjectOptions{})
	if err == nil {
		return errors.New("target already exists")
	}
	response := minio.ToErrorResponse(err)
	if response.Code == "NoSuchKey" || response.Code == "NotFound" {
		return nil
	}
	return err
}

func (s *Store) putS3Object(source domain.StorageSource, rel string, reader io.Reader, size int64) error {
	client, cfg, err := s.s3Client(source)
	if err != nil {
		return err
	}
	key, err := s3ObjectKey(cfg, rel)
	if err != nil {
		return err
	}
	_, err = client.PutObject(context.Background(), cfg.Bucket, key, reader, size, minio.PutObjectOptions{})
	return err
}

func (s *Store) createS3Folder(source domain.StorageSource, rel string) (domain.FileEntry, error) {
	rel = strings.Trim(strings.TrimSpace(filepath.ToSlash(rel)), "/")
	if rel == "" {
		return domain.FileEntry{}, errors.New("folder path is required")
	}
	folderRel, err := cleanRelative(rel)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if err := s.putS3Object(source, folderRel+"/", strings.NewReader(""), 0); err != nil {
		return domain.FileEntry{}, err
	}
	now := time.Now().Format(time.RFC3339)
	return domain.FileEntry{Name: pathBase(folderRel), Path: folderRel, Type: "folder", Size: 0, ModifiedAt: now}, nil
}

func (s *Store) createS3EmptyFile(source domain.StorageSource, rel string) (domain.FileEntry, error) {
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
	if err := s.s3UploadAllowed(source, dirRel, name); err != nil {
		return domain.FileEntry{}, err
	}
	clean, err := cleanRelative(rel)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if err := s.putS3Object(source, clean, strings.NewReader(""), 0); err != nil {
		return domain.FileEntry{}, err
	}
	now := time.Now().Format(time.RFC3339)
	return domain.FileEntry{Name: pathBase(clean), Path: clean, Type: "file", Size: 0, ModifiedAt: now}, nil
}

func (s *Store) moveS3Object(source domain.StorageSource, from, to string) (domain.FileEntry, error) {
	client, cfg, err := s.s3Client(source)
	if err != nil {
		return domain.FileEntry{}, err
	}
	fromKey, err := s3ObjectKey(cfg, from)
	if err != nil {
		return domain.FileEntry{}, err
	}
	toClean, err := cleanRelative(to)
	if err != nil {
		return domain.FileEntry{}, err
	}
	toKey, err := s3ObjectKey(cfg, toClean)
	if err != nil {
		return domain.FileEntry{}, err
	}
	info, err := client.StatObject(context.Background(), cfg.Bucket, fromKey, minio.StatObjectOptions{})
	if err != nil {
		return s.moveS3Prefix(client, cfg, from, to)
	}
	_, err = client.CopyObject(
		context.Background(),
		minio.CopyDestOptions{Bucket: cfg.Bucket, Object: toKey},
		minio.CopySrcOptions{Bucket: cfg.Bucket, Object: fromKey},
	)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if err := client.RemoveObject(context.Background(), cfg.Bucket, fromKey, minio.RemoveObjectOptions{}); err != nil {
		return domain.FileEntry{}, err
	}
	info.Key = toKey
	return s3EntryFromObject(cfg, info), nil
}

func (s *Store) moveS3Prefix(client *minio.Client, cfg s3SourceConfig, from, to string) (domain.FileEntry, error) {
	fromPrefix, err := s3ListPrefix(cfg, from)
	if err != nil {
		return domain.FileEntry{}, err
	}
	toClean, err := cleanRelative(to)
	if err != nil {
		return domain.FileEntry{}, err
	}
	toPrefix, err := s3ListPrefix(cfg, toClean)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if fromPrefix == "" || toPrefix == "" {
		return domain.FileEntry{}, errors.New("source and target paths are required")
	}
	moved := 0
	for object := range client.ListObjects(context.Background(), cfg.Bucket, minio.ListObjectsOptions{Prefix: fromPrefix, Recursive: true}) {
		if object.Err != nil {
			return domain.FileEntry{}, object.Err
		}
		targetKey := toPrefix + strings.TrimPrefix(object.Key, fromPrefix)
		if _, err := client.CopyObject(
			context.Background(),
			minio.CopyDestOptions{Bucket: cfg.Bucket, Object: targetKey},
			minio.CopySrcOptions{Bucket: cfg.Bucket, Object: object.Key},
		); err != nil {
			return domain.FileEntry{}, err
		}
		if err := client.RemoveObject(context.Background(), cfg.Bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			return domain.FileEntry{}, err
		}
		moved++
	}
	if moved == 0 {
		return domain.FileEntry{}, errors.New("source does not exist")
	}
	now := time.Now().Format(time.RFC3339)
	return domain.FileEntry{Name: pathBase(toClean), Path: toClean, Type: "folder", Size: 0, ModifiedAt: now}, nil
}

func (s *Store) deleteS3Object(source domain.StorageSource, rel string) error {
	client, cfg, err := s.s3Client(source)
	if err != nil {
		return err
	}
	key, err := s3ObjectKey(cfg, rel)
	if err != nil {
		return err
	}
	if key == "" {
		return errors.New("refuse to delete storage root")
	}
	prefix := key
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	removed := 0
	for object := range client.ListObjects(context.Background(), cfg.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return object.Err
		}
		if err := client.RemoveObject(context.Background(), cfg.Bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
		removed++
	}
	if removed > 0 {
		return client.RemoveObject(context.Background(), cfg.Bucket, key, minio.RemoveObjectOptions{})
	}
	return client.RemoveObject(context.Background(), cfg.Bucket, key, minio.RemoveObjectOptions{})
}

func (s *Store) webDAVURL(source domain.StorageSource, rel string) (string, webDAVSourceConfig, error) {
	cfg, err := webDAVConfigFromSource(source)
	if err != nil {
		return "", webDAVSourceConfig{}, err
	}
	clean, err := cleanRelative(rel)
	if err != nil {
		return "", webDAVSourceConfig{}, err
	}
	parsed, err := url.Parse(cfg.URL)
	if err != nil {
		return "", webDAVSourceConfig{}, err
	}
	parts := []string{strings.Trim(parsed.Path, "/")}
	if cfg.Root != "" {
		parts = append(parts, cfg.Root)
	}
	if clean != "" {
		parts = append(parts, clean)
	}
	parsed.Path = escapePath(strings.Join(parts, "/"))
	return parsed.String(), cfg, nil
}

func (s *Store) webDAVRequest(source domain.StorageSource, method, rel string, body io.Reader, headers map[string]string) (*http.Response, error) {
	target, cfg, err := s.webDAVURL(source, rel)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, target, body)
	if err != nil {
		return nil, err
	}
	if cfg.Username != "" || cfg.Password != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return http.DefaultClient.Do(req)
}

func escapePath(value string) string {
	value = strings.Trim(value, "/")
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return "/" + strings.Join(parts, "/")
}

func webDAVSuccess(status int) bool {
	return status >= 200 && status < 300
}

func webDAVEntryFromResponse(cfg webDAVSourceConfig, response davResponse) (domain.FileEntry, bool) {
	rel := webDAVRelFromHref(cfg, response.Href)
	if rel == "" {
		return domain.FileEntry{}, false
	}
	prop := davProp{}
	status := response.Status
	for _, propStat := range response.PropStat {
		if strings.Contains(propStat.Status, " 200 ") || propStat.Status == "" {
			prop = propStat.Prop
			status = propStat.Status
			break
		}
	}
	if status != "" && !strings.Contains(status, " 200 ") {
		return domain.FileEntry{}, false
	}
	entryType := "file"
	size, _ := strconv.ParseInt(strings.TrimSpace(prop.Length), 10, 64)
	path := strings.TrimSuffix(rel, "/")
	if prop.ResourceType.Collection != nil || strings.HasSuffix(rel, "/") {
		entryType = "folder"
		size = 0
	}
	modifiedAt := time.Now().Format(time.RFC3339)
	if modified, err := http.ParseTime(strings.TrimSpace(prop.Modified)); err == nil {
		modifiedAt = modified.Format(time.RFC3339)
	}
	return domain.FileEntry{Name: pathBase(path), Path: path, Type: entryType, Size: size, ModifiedAt: modifiedAt}, true
}

func webDAVRelFromHref(cfg webDAVSourceConfig, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	var pathText string
	if parsed, err := url.Parse(href); err == nil {
		pathText = parsed.Path
	} else {
		pathText = href
	}
	unescaped, err := url.PathUnescape(pathText)
	if err == nil {
		pathText = unescaped
	}
	baseParsed, _ := url.Parse(cfg.URL)
	prefix := strings.Trim(strings.Trim(baseParsed.Path, "/")+"/"+cfg.Root, "/")
	pathText = strings.Trim(pathText, "/")
	if prefix != "" {
		pathText = strings.TrimPrefix(pathText, prefix)
	}
	return strings.Trim(pathText, "/")
}

func (s *Store) listWebDAVFiles(source domain.StorageSource, rel string) ([]domain.FileEntry, error) {
	body := `<?xml version="1.0" encoding="utf-8"?><propfind xmlns="DAV:"><prop><resourcetype/><getcontentlength/><getlastmodified/></prop></propfind>`
	res, err := s.webDAVRequest(source, "PROPFIND", rel, strings.NewReader(body), map[string]string{
		"Depth":        "1",
		"Content-Type": "application/xml; charset=utf-8",
	})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return []domain.FileEntry{}, nil
	}
	if !webDAVSuccess(res.StatusCode) && res.StatusCode != 207 {
		return nil, errors.New(res.Status)
	}
	var multistatus davMultiStatus
	if err := xml.NewDecoder(res.Body).Decode(&multistatus); err != nil {
		return nil, err
	}
	cfg, err := webDAVConfigFromSource(source)
	if err != nil {
		return nil, err
	}
	current, _ := cleanRelative(rel)
	current = strings.Trim(current, "/")
	files := make([]domain.FileEntry, 0)
	for _, response := range multistatus.Responses {
		entry, ok := webDAVEntryFromResponse(cfg, response)
		if !ok || entry.Path == current {
			continue
		}
		files = append(files, entry)
	}
	return files, nil
}

func (s *Store) searchWebDAVFiles(source domain.StorageSource, query string, limit int) ([]domain.FileEntry, error) {
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
	body := `<?xml version="1.0" encoding="utf-8"?><propfind xmlns="DAV:"><prop><resourcetype/><getcontentlength/><getlastmodified/></prop></propfind>`
	res, err := s.webDAVRequest(source, "PROPFIND", "", strings.NewReader(body), map[string]string{
		"Depth":        "infinity",
		"Content-Type": "application/xml; charset=utf-8",
	})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if !webDAVSuccess(res.StatusCode) && res.StatusCode != 207 {
		return nil, errors.New(res.Status)
	}
	var multistatus davMultiStatus
	if err := xml.NewDecoder(res.Body).Decode(&multistatus); err != nil {
		return nil, err
	}
	cfg, err := webDAVConfigFromSource(source)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(query)
	results := make([]domain.FileEntry, 0)
	for _, response := range multistatus.Responses {
		entry, ok := webDAVEntryFromResponse(cfg, response)
		if !ok || entry.Path == "" || !strings.Contains(strings.ToLower(entry.Path), needle) {
			continue
		}
		results = append(results, entry)
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (s *Store) webDAVDownload(source domain.StorageSource, rel string) (sourceDownload, error) {
	res, err := s.webDAVRequest(source, http.MethodGet, rel, nil, nil)
	if err != nil {
		return sourceDownload{}, err
	}
	defer res.Body.Close()
	if !webDAVSuccess(res.StatusCode) {
		return sourceDownload{}, errors.New(res.Status)
	}
	tmp, err := os.CreateTemp("", "xfile-webdav-*")
	if err != nil {
		return sourceDownload{}, err
	}
	if _, err := io.Copy(tmp, res.Body); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return sourceDownload{}, err
	}
	info, err := tmp.Stat()
	if err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return sourceDownload{}, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return sourceDownload{}, err
	}
	return sourceDownload{Entry: fileInfoEntry(rel, info), Reader: tempReadSeekCloser{File: tmp, path: tmp.Name()}}, nil
}

func (s *Store) webDAVUploadAllowed(source domain.StorageSource, dirRel, filename string) error {
	filename = filepath.Base(filename)
	if strings.TrimSpace(filename) == "" || filename == "." {
		return errors.New("filename is required")
	}
	targetRel := cleanJoin(dirRel, filename)
	if _, err := cleanRelative(targetRel); err != nil {
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
	if s.SettingValue("uploadOverwrite", "enabled") == "enabled" {
		return nil
	}
	res, err := s.webDAVRequest(source, http.MethodHead, targetRel, nil, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if webDAVSuccess(res.StatusCode) {
		return errors.New("target already exists")
	}
	if res.StatusCode == http.StatusNotFound {
		return nil
	}
	return errors.New(res.Status)
}

func (s *Store) putWebDAVFile(source domain.StorageSource, rel string, reader io.Reader) error {
	res, err := s.webDAVRequest(source, http.MethodPut, rel, reader, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if !webDAVSuccess(res.StatusCode) {
		return errors.New(res.Status)
	}
	return nil
}

func (s *Store) createWebDAVFolder(source domain.StorageSource, rel string) (domain.FileEntry, error) {
	clean, err := cleanRelative(rel)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if clean == "" {
		return domain.FileEntry{}, errors.New("folder path is required")
	}
	res, err := s.webDAVRequest(source, "MKCOL", clean, nil, nil)
	if err != nil {
		return domain.FileEntry{}, err
	}
	defer res.Body.Close()
	if !webDAVSuccess(res.StatusCode) && res.StatusCode != http.StatusMethodNotAllowed {
		return domain.FileEntry{}, errors.New(res.Status)
	}
	return domain.FileEntry{Name: pathBase(clean), Path: clean, Type: "folder", Size: 0, ModifiedAt: time.Now().Format(time.RFC3339)}, nil
}

func (s *Store) createWebDAVEmptyFile(source domain.StorageSource, rel string) (domain.FileEntry, error) {
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
	if err := s.webDAVUploadAllowed(source, dirRel, name); err != nil {
		return domain.FileEntry{}, err
	}
	clean, err := cleanRelative(rel)
	if err != nil {
		return domain.FileEntry{}, err
	}
	if err := s.putWebDAVFile(source, clean, strings.NewReader("")); err != nil {
		return domain.FileEntry{}, err
	}
	return domain.FileEntry{Name: pathBase(clean), Path: clean, Type: "file", Size: 0, ModifiedAt: time.Now().Format(time.RFC3339)}, nil
}

func (s *Store) moveWebDAVPath(source domain.StorageSource, from, to string) (domain.FileEntry, error) {
	fromClean, err := cleanRelative(from)
	if err != nil {
		return domain.FileEntry{}, err
	}
	toClean, err := cleanRelative(to)
	if err != nil {
		return domain.FileEntry{}, err
	}
	destination, _, err := s.webDAVURL(source, toClean)
	if err != nil {
		return domain.FileEntry{}, err
	}
	res, err := s.webDAVRequest(source, "MOVE", fromClean, nil, map[string]string{
		"Destination": destination,
		"Overwrite":   "F",
	})
	if err != nil {
		return domain.FileEntry{}, err
	}
	defer res.Body.Close()
	if !webDAVSuccess(res.StatusCode) {
		return domain.FileEntry{}, errors.New(res.Status)
	}
	return domain.FileEntry{Name: pathBase(toClean), Path: toClean, Type: "file", Size: 0, ModifiedAt: time.Now().Format(time.RFC3339)}, nil
}

func (s *Store) deleteWebDAVPath(source domain.StorageSource, rel string) error {
	clean, err := cleanRelative(rel)
	if err != nil {
		return err
	}
	if clean == "" {
		return errors.New("refuse to delete storage root")
	}
	res, err := s.webDAVRequest(source, http.MethodDelete, clean, nil, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if !webDAVSuccess(res.StatusCode) && res.StatusCode != http.StatusNotFound {
		return errors.New(res.Status)
	}
	return nil
}

func (s *Store) safePath(rel string) (string, error) {
	return safePathInRoot(s.cfg.FilesDir, rel)
}

func (s *Store) sourceSafePath(source domain.StorageSource, rel string) (string, error) {
	if source.Type != "local" {
		return "", errors.New("storage adapter is not implemented yet")
	}
	rootText := strings.TrimSpace(source.RootPath)
	if rootText == "" {
		rootText = s.cfg.FilesDir
	}
	return safePathInRoot(rootText, rel)
}

func safePathInRoot(rootText, rel string) (string, error) {
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
