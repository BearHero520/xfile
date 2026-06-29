package main

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	appName          = "xfile"
	maxSearchResults = 200
	maxAccessLogs    = 2000
	sessionCookie    = "xfile_session"
)

var errSearchLimit = errors.New("search result limit reached")

type server struct {
	dataDir       string
	webDir        string
	adminUser     string
	adminPassword string
	trustProxy    bool
	sessionTTL    time.Duration
	db            *sql.DB
	shares        map[string]share
	directLinks   map[string]directLink
	sessions      map[string]session
	mu            sync.RWMutex
	authMu        sync.RWMutex
}

type fileItem struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Type        string    `json:"type"`
	Size        int64     `json:"size"`
	Modified    time.Time `json:"modified"`
	Mime        string    `json:"mime"`
	PreviewType string    `json:"previewType"`
}

type share struct {
	Key          string    `json:"key"`
	Path         string    `json:"path"`
	PasswordHash string    `json:"passwordHash,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt"`
	CreatedAt    time.Time `json:"createdAt"`
}

type shareView struct {
	Key         string    `json:"key"`
	Path        string    `json:"path"`
	HasPassword bool      `json:"hasPassword"`
	ExpiresAt   time.Time `json:"expiresAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

type directLink struct {
	Key             string     `json:"key"`
	Path            string     `json:"path"`
	Name            string     `json:"name"`
	CreatedAt       time.Time  `json:"createdAt"`
	ExpiresAt       *time.Time `json:"expiresAt"`
	AllowedReferers []string   `json:"allowedReferers,omitempty"`
	AllowedIPs      []string   `json:"allowedIPs,omitempty"`
	RateLimitKBps   int        `json:"rateLimitKBps,omitempty"`
	DownloadCount   int        `json:"downloadCount"`
	LastAccessAt    *time.Time `json:"lastAccessAt"`
}

type session struct {
	Username  string
	ExpiresAt time.Time
}

type storageStats struct {
	FileCount   int   `json:"fileCount"`
	FolderCount int   `json:"folderCount"`
	TotalSize   int64 `json:"totalSize"`
	ShareCount  int   `json:"shareCount"`
	DirectCount int   `json:"directCount"`
	LogCount    int   `json:"logCount"`
}

type accessLog struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Key        string    `json:"key"`
	Path       string    `json:"path"`
	IP         string    `json:"ip"`
	Referer    string    `json:"referer"`
	UserAgent  string    `json:"userAgent"`
	Status     int       `json:"status"`
	Bytes      int64     `json:"bytes"`
	DurationMs int64     `json:"durationMs"`
	Message    string    `json:"message,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type captureResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *captureResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *captureResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err
}

type throttleResponseWriter struct {
	http.ResponseWriter
	bytesPerSecond int64
	written        int64
	started        time.Time
}

func (w *throttleResponseWriter) Write(p []byte) (int, error) {
	if w.bytesPerSecond <= 0 {
		return w.ResponseWriter.Write(p)
	}

	total := 0
	for len(p) > 0 {
		chunkSize := len(p)
		if chunkSize > 32*1024 {
			chunkSize = 32 * 1024
		}
		n, err := w.ResponseWriter.Write(p[:chunkSize])
		total += n
		w.written += int64(n)
		w.throttle()
		if err != nil || n == 0 {
			return total, err
		}
		p = p[n:]
	}
	return total, nil
}

func (w *throttleResponseWriter) throttle() {
	expected := time.Duration(w.written*int64(time.Second)) / time.Duration(w.bytesPerSecond)
	if sleep := w.started.Add(expected).Sub(time.Now()); sleep > 0 {
		time.Sleep(sleep)
	}
}

func main() {
	port := getenv("XFILE_PORT", "3008")
	dataDir := getenv("XFILE_DATA_DIR", "data")
	webDir := getenv("XFILE_WEB_DIR", "web/dist")
	dbPath := getenv("XFILE_DB_PATH", filepath.Join(dataDir, "xfile.db"))
	adminUser := getenv("XFILE_ADMIN_USER", "admin")
	adminPassword := getenv("XFILE_ADMIN_PASSWORD", "xfile-admin")
	trustProxy := getenvBool("XFILE_TRUST_PROXY", false)

	if err := os.MkdirAll(filepath.Join(dataDir, "files"), 0755); err != nil {
		log.Fatalf("create data directory: %v", err)
	}
	db, err := openDatabase(dbPath)
	if err != nil {
		log.Fatalf("open sqlite database: %v", err)
	}
	defer db.Close()

	s := &server{
		dataDir:       dataDir,
		webDir:        webDir,
		adminUser:     adminUser,
		adminPassword: adminPassword,
		trustProxy:    trustProxy,
		sessionTTL:    24 * time.Hour,
		db:            db,
		shares:        map[string]share{},
		directLinks:   map[string]directLink{},
		sessions:      map[string]session{},
	}
	if adminPassword == "xfile-admin" {
		log.Printf("warning: using default admin password, set XFILE_ADMIN_PASSWORD before exposing xfile")
	}
	if err := s.loadShares(); err != nil {
		log.Printf("load shares: %v", err)
	}
	if err := s.loadDirectLinks(); err != nil {
		log.Printf("load direct links: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/config", s.config)
	mux.HandleFunc("GET /api/auth/me", s.me)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.Handle("GET /api/stats", s.requireAuth(http.HandlerFunc(s.stats)))
	mux.Handle("GET /api/files", s.requireAuth(http.HandlerFunc(s.listFiles)))
	mux.Handle("GET /api/search", s.requireAuth(http.HandlerFunc(s.searchFiles)))
	mux.Handle("POST /api/folders", s.requireAuth(http.HandlerFunc(s.createFolder)))
	mux.Handle("POST /api/upload", s.requireAuth(http.HandlerFunc(s.uploadFile)))
	mux.Handle("GET /api/download", s.requireAuth(http.HandlerFunc(s.downloadFile)))
	mux.Handle("GET /api/preview", s.requireAuth(http.HandlerFunc(s.previewFile)))
	mux.Handle("DELETE /api/files", s.requireAuth(http.HandlerFunc(s.deleteFile)))
	mux.Handle("POST /api/rename", s.requireAuth(http.HandlerFunc(s.renameFile)))
	mux.Handle("POST /api/share", s.requireAuth(http.HandlerFunc(s.createShare)))
	mux.Handle("GET /api/shares", s.requireAuth(http.HandlerFunc(s.listShares)))
	mux.Handle("DELETE /api/shares/{key}", s.requireAuth(http.HandlerFunc(s.deleteShare)))
	mux.Handle("POST /api/direct-links", s.requireAuth(http.HandlerFunc(s.createDirectLink)))
	mux.Handle("GET /api/direct-links", s.requireAuth(http.HandlerFunc(s.listDirectLinks)))
	mux.Handle("DELETE /api/direct-links/{key}", s.requireAuth(http.HandlerFunc(s.deleteDirectLink)))
	mux.Handle("GET /api/access-logs", s.requireAuth(http.HandlerFunc(s.listAccessLogs)))
	mux.Handle("DELETE /api/access-logs", s.requireAuth(http.HandlerFunc(s.clearAccessLogs)))
	mux.HandleFunc("GET /s/", s.openShare)
	mux.HandleFunc("POST /s/", s.openShare)
	mux.HandleFunc("GET /dl/", s.openDirectLink)
	mux.HandleFunc("/", s.frontend)

	addr := ":" + port
	log.Printf("%s listening on http://localhost%s", appName, addr)
	if err := http.ListenAndServe(addr, logRequest(mux)); err != nil {
		log.Fatal(err)
	}
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":    appName,
		"status":  "ok",
		"version": "0.2.0",
	})
}

func (s *server) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":     appName,
		"features": []string{"admin-auth", "sqlite", "local-storage", "upload", "download", "folder-zip", "direct-link", "share-link", "preview", "search", "access-log", "file-management"},
	})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !secureCompare(req.Username, s.adminUser) || !secureCompare(req.Password, s.adminPassword) {
		writeError(w, http.StatusUnauthorized, errors.New("invalid username or password"))
		return
	}

	token, err := randomHex(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	expiresAt := time.Now().Add(s.sessionTTL)

	s.authMu.Lock()
	s.sessions[token] = session{Username: s.adminUser, ExpiresAt: expiresAt}
	s.authMu.Unlock()

	http.SetCookie(w, s.sessionCookie(r, token, int(s.sessionTTL.Seconds())))
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"username":      s.adminUser,
		"expiresAt":     expiresAt,
	})
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	username, expiresAt, ok := s.currentSession(r)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"username":      username,
		"expiresAt":     expiresAt,
	})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		s.authMu.Lock()
		delete(s.sessions, cookie.Value)
		s.authMu.Unlock()
	}
	http.SetCookie(w, s.sessionCookie(r, "", -1))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.storageStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *server) listFiles(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	dir, cleanRel, err := s.safePath(rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	items := make([]fileItem, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, itemFromInfo(joinRel(cleanRel, entry.Name()), info))
	}

	sortItems(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"path":  cleanRel,
		"items": items,
	})
}

func (s *server) searchFiles(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if query == "" {
		writeError(w, http.StatusBadRequest, errors.New("search keyword is required"))
		return
	}
	root, _, err := s.safePath(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := os.Stat(root); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	filesRoot, err := filepath.Abs(s.filesRoot())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	items := make([]fileItem, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || path == root {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(filesRoot, path)
		if err != nil {
			return nil
		}
		cleanRel := filepath.ToSlash(rel)
		if strings.Contains(strings.ToLower(cleanRel), query) {
			items = append(items, itemFromInfo(cleanRel, info))
		}
		if len(items) >= maxSearchResults {
			return errSearchLimit
		}
		return nil
	})
	if err != nil && !errors.Is(err, errSearchLimit) {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	sortItems(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"query": query,
		"items": items,
		"limit": maxSearchResults,
	})
}

func (s *server) createFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !validName(req.Name) {
		writeError(w, http.StatusBadRequest, errors.New("invalid folder name"))
		return
	}
	target, _, err := s.safePath(filepath.ToSlash(filepath.Join(req.Path, req.Name)))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *server) uploadFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	dir, _, err := s.safePath(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("files are required"))
		return
	}
	for _, header := range files {
		name := filepath.Base(header.Filename)
		if !validName(name) {
			writeError(w, http.StatusBadRequest, errors.New("invalid file name"))
			return
		}

		src, err := header.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		dstPath := filepath.Join(dir, name)
		dst, err := os.Create(dstPath)
		if err != nil {
			_ = src.Close()
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = src.Close()
			_ = dst.Close()
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		_ = src.Close()
		_ = dst.Close()
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "count": len(files)})
}

func (s *server) downloadFile(w http.ResponseWriter, r *http.Request) {
	s.serveFilePath(w, r, r.URL.Query().Get("path"), true)
}

func (s *server) previewFile(w http.ResponseWriter, r *http.Request) {
	s.serveFilePath(w, r, r.URL.Query().Get("path"), false)
}

func (s *server) deleteFile(w http.ResponseWriter, r *http.Request) {
	target, cleanRel, err := s.safePath(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if cleanRel == "" {
		writeError(w, http.StatusBadRequest, errors.New("cannot delete root"))
		return
	}
	if err := os.RemoveAll(target); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) renameFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		NewName string `json:"newName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !validName(req.NewName) {
		writeError(w, http.StatusBadRequest, errors.New("invalid new name"))
		return
	}
	src, cleanRel, err := s.safePath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if cleanRel == "" {
		writeError(w, http.StatusBadRequest, errors.New("cannot rename root"))
		return
	}
	dstRel := filepath.ToSlash(filepath.Join(filepath.Dir(cleanRel), req.NewName))
	dst, _, err := s.safePath(dstRel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := os.Rename(src, dst); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) createShare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path           string `json:"path"`
		ExpiresInHours int    `json:"expiresInHours"`
		Password       string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	target, cleanRel, err := s.safePath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := os.Stat(target); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if req.ExpiresInHours <= 0 {
		req.ExpiresInHours = 24
	}
	key, err := randomKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	now := time.Now()
	item := share{
		Key:          key,
		Path:         cleanRel,
		PasswordHash: passwordHash(req.Password),
		ExpiresAt:    now.Add(time.Duration(req.ExpiresInHours) * time.Hour),
		CreatedAt:    now,
	}

	s.mu.Lock()
	s.shares[key] = item
	err = s.saveSharesLocked()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, shareToView(item))
}

func (s *server) listShares(w http.ResponseWriter, r *http.Request) {
	if err := s.pruneExpiredShares(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]shareView, 0, len(s.shares))
	for _, item := range s.shares {
		items = append(items, shareToView(item))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) deleteShare(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.PathValue("key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, errors.New("share key is required"))
		return
	}

	s.mu.Lock()
	delete(s.shares, key)
	err := s.saveSharesLocked()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) createDirectLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path            string   `json:"path"`
		ExpiresInHours  int      `json:"expiresInHours"`
		AllowedReferers []string `json:"allowedReferers"`
		AllowedIPs      []string `json:"allowedIPs"`
		RateLimitKBps   int      `json:"rateLimitKBps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	target, cleanRel, err := s.safePath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if cleanRel == "" {
		writeError(w, http.StatusBadRequest, errors.New("cannot create a direct link for root"))
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	key, err := randomKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	now := time.Now()
	var expiresAt *time.Time
	if req.ExpiresInHours > 0 {
		expiry := now.Add(time.Duration(req.ExpiresInHours) * time.Hour)
		expiresAt = &expiry
	}
	if req.RateLimitKBps < 0 {
		writeError(w, http.StatusBadRequest, errors.New("rate limit must be greater than or equal to 0"))
		return
	}
	item := directLink{
		Key:             key,
		Path:            cleanRel,
		Name:            directLinkName(cleanRel, info.IsDir()),
		CreatedAt:       now,
		ExpiresAt:       expiresAt,
		AllowedReferers: cleanRules(req.AllowedReferers),
		AllowedIPs:      cleanRules(req.AllowedIPs),
		RateLimitKBps:   req.RateLimitKBps,
	}

	s.mu.Lock()
	s.directLinks[key] = item
	err = s.saveDirectLinksLocked()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *server) listDirectLinks(w http.ResponseWriter, r *http.Request) {
	if err := s.pruneExpiredDirectLinks(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]directLink, 0, len(s.directLinks))
	for _, item := range s.directLinks {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) deleteDirectLink(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.PathValue("key"))
	if key == "" {
		writeError(w, http.StatusBadRequest, errors.New("direct link key is required"))
		return
	}

	s.mu.Lock()
	delete(s.directLinks, key)
	err := s.saveDirectLinksLocked()
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) listAccessLogs(w http.ResponseWriter, r *http.Request) {
	limit := parsePositiveInt(r.URL.Query().Get("limit"), 200)
	if limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.Query(`SELECT id, type, link_key, path, ip, referer, user_agent, status, bytes, duration_ms, message, created_at FROM access_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	items := make([]accessLog, 0)
	for rows.Next() {
		var item accessLog
		var createdAt string
		if err := rows.Scan(&item.ID, &item.Type, &item.Key, &item.Path, &item.IP, &item.Referer, &item.UserAgent, &item.Status, &item.Bytes, &item.DurationMs, &item.Message, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		parsedCreatedAt, err := parseDBTime(createdAt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		item.CreatedAt = parsedCreatedAt
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) clearAccessLogs(w http.ResponseWriter, r *http.Request) {
	if _, err := s.db.Exec(`DELETE FROM access_logs`); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *server) openShare(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	capture := &captureResponseWriter{ResponseWriter: w}
	key := strings.TrimPrefix(r.URL.Path, "/s/")
	path := ""
	message := "ok"
	defer func() {
		s.recordAccessLog("share", key, path, r, capture, start, message)
	}()

	s.mu.RLock()
	item, ok := s.shares[key]
	s.mu.RUnlock()
	if !ok || time.Now().After(item.ExpiresAt) {
		message = "not found or expired"
		writeError(capture, http.StatusNotFound, errors.New("share link not found or expired"))
		return
	}
	path = item.Path
	if item.PasswordHash != "" && !s.authorizeShare(capture, r, item) {
		message = "password required"
		return
	}
	s.serveFilePath(capture, r, item.Path, false)
}

func (s *server) openDirectLink(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	capture := &captureResponseWriter{ResponseWriter: w}
	key := strings.TrimPrefix(r.URL.Path, "/dl/")
	if idx := strings.Index(key, "/"); idx >= 0 {
		key = key[:idx]
	}
	key = strings.TrimSpace(key)
	path := ""
	message := "ok"
	defer func() {
		s.recordAccessLog("direct", key, path, r, capture, start, message)
	}()

	if key == "" {
		message = "missing key"
		writeError(capture, http.StatusNotFound, errors.New("direct link not found"))
		return
	}

	s.mu.RLock()
	item, ok := s.directLinks[key]
	s.mu.RUnlock()
	if !ok || directLinkExpired(item, time.Now()) {
		if ok {
			_ = s.removeExpiredDirectLink(key)
		}
		message = "not found or expired"
		writeError(capture, http.StatusNotFound, errors.New("direct link not found or expired"))
		return
	}
	path = item.Path
	if !s.directLinkAllowed(item, r) {
		message = "access denied"
		writeError(capture, http.StatusForbidden, errors.New("direct link access denied"))
		return
	}

	now := time.Now()
	s.mu.Lock()
	current, stillExists := s.directLinks[key]
	if stillExists {
		current.DownloadCount++
		current.LastAccessAt = &now
		s.directLinks[key] = current
		item = current
	}
	s.mu.Unlock()
	if !stillExists {
		message = "not found or expired"
		writeError(capture, http.StatusNotFound, errors.New("direct link not found or expired"))
		return
	}
	if err := s.updateDirectLinkAccess(key, item.DownloadCount, now); err != nil {
		log.Printf("update direct link access: %v", err)
	}

	capture.Header().Set("X-Content-Type-Options", "nosniff")
	out := http.ResponseWriter(capture)
	if item.RateLimitKBps > 0 {
		out = &throttleResponseWriter{
			ResponseWriter: capture,
			bytesPerSecond: int64(item.RateLimitKBps) * 1024,
			started:        time.Now(),
		}
	}
	if target, _, err := s.safePath(item.Path); err == nil {
		if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
			s.serveFilePath(out, r, item.Path, true)
			return
		}
	}
	s.serveFilePath(out, r, item.Path, false)
}

func (s *server) serveFilePath(w http.ResponseWriter, r *http.Request, rel string, attachment bool) {
	target, cleanRel, err := s.safePath(rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if info.IsDir() {
		if attachment {
			s.serveZip(w, target, downloadName(cleanRel, "xfile")+".zip")
			return
		}
		s.renderSharedDirectory(w, cleanRel, target)
		return
	}

	if attachment {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", downloadName(info.Name(), "download")))
	} else {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", downloadName(info.Name(), "preview")))
	}
	if ctype := mime.TypeByExtension(filepath.Ext(info.Name())); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	http.ServeFile(w, r, target)
}

func (s *server) serveZip(w http.ResponseWriter, target string, name string) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))

	zipWriter := zip.NewWriter(w)
	defer func() {
		if err := zipWriter.Close(); err != nil {
			log.Printf("close zip: %v", err)
		}
	}()

	base := strings.TrimSuffix(name, ".zip")
	if err := filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == target {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(target, path)
		if err != nil {
			return err
		}
		zipPath := filepath.ToSlash(filepath.Join(base, rel))
		if entry.IsDir() {
			_, err := zipWriter.Create(zipPath + "/")
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = zipPath
		header.Method = zip.Deflate
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}); err != nil {
		log.Printf("zip folder: %v", err)
	}
}

func (s *server) renderSharedDirectory(w http.ResponseWriter, rel string, target string) {
	entries, err := os.ReadDir(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!doctype html><title>xfile share</title><body><h1>xfile share: /%s</h1><ul>", html.EscapeString(rel))
	for _, entry := range entries {
		label := entry.Name()
		if entry.IsDir() {
			label += "/"
		}
		_, _ = fmt.Fprintf(w, "<li>%s</li>", html.EscapeString(label))
	}
	_, _ = fmt.Fprint(w, "</ul></body>")
}

func (s *server) authorizeShare(w http.ResponseWriter, r *http.Request, item share) bool {
	if cookie, err := r.Cookie(shareAuthCookieName(item.Key)); err == nil && secureCompare(cookie.Value, item.PasswordHash) {
		return true
	}

	password := r.URL.Query().Get("password")
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err == nil {
			password = r.FormValue("password")
		}
	}
	if password == "" {
		renderSharePasswordForm(w, item, "")
		return false
	}
	if !secureCompare(passwordHash(password), item.PasswordHash) {
		renderSharePasswordForm(w, item, "密码不正确")
		return false
	}

	maxAge := int(time.Until(item.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     shareAuthCookieName(item.Key),
		Value:    item.PasswordHash,
		Path:     "/s/" + item.Key,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
	})
	return true
}

func renderSharePasswordForm(w http.ResponseWriter, item share, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	notice := ""
	if message != "" {
		notice = fmt.Sprintf("<p class=\"error\">%s</p>", html.EscapeString(message))
	}
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>xfile share</title>
  <style>
    body{margin:0;min-height:100vh;display:grid;place-items:center;background:#eef2f7;font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#111827}
    form{width:min(380px,calc(100vw - 40px));padding:28px;border:1px solid #e5e7eb;border-radius:8px;background:#fff;box-shadow:0 18px 48px rgba(15,23,42,.12)}
    h1{margin:0 0 8px;font-size:24px}.path{margin:0 0 18px;color:#64748b;word-break:break-all}.error{color:#dc2626}
    input{width:100%%;height:38px;padding:0 10px;border:1px solid #d1d5db;border-radius:6px;box-sizing:border-box}
    button{width:100%%;height:38px;margin-top:12px;border:0;border-radius:6px;background:#2563eb;color:#fff;font-weight:600}
  </style>
</head>
<body>
  <form method="post">
    <h1>xfile 分享</h1>
    <p class="path">/%s</p>
    %s
    <input name="password" type="password" placeholder="请输入分享密码" autofocus>
    <button type="submit">访问</button>
  </form>
</body>
</html>`, html.EscapeString(item.Path), notice)
}

func (s *server) frontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	path, err := s.safeWebPath(r.URL.Path)
	if err == nil {
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
	}
	http.ServeFile(w, r, filepath.Join(s.webDir, "index.html"))
}

func openDatabase(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrateDatabase(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func migrateDatabase(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS shares (
			key TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			password_hash TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_shares_expires_at ON shares(expires_at);`,
		`CREATE TABLE IF NOT EXISTS direct_links (
			key TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT,
			allowed_referers TEXT NOT NULL DEFAULT '[]',
			allowed_ips TEXT NOT NULL DEFAULT '[]',
			rate_limit_kbps INTEGER NOT NULL DEFAULT 0,
			download_count INTEGER NOT NULL DEFAULT 0,
			last_access_at TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_direct_links_expires_at ON direct_links(expires_at);`,
		`CREATE TABLE IF NOT EXISTS access_logs (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			link_key TEXT NOT NULL,
			path TEXT NOT NULL,
			ip TEXT NOT NULL,
			referer TEXT NOT NULL,
			user_agent TEXT NOT NULL,
			status INTEGER NOT NULL,
			bytes INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_access_logs_created_at ON access_logs(created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_access_logs_type_key ON access_logs(type, link_key);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) loadShares() error {
	if err := s.deleteExpiredShares(time.Now()); err != nil {
		return err
	}
	rows, err := s.db.Query(`SELECT key, path, password_hash, expires_at, created_at FROM shares`)
	if err != nil {
		return err
	}
	defer rows.Close()

	now := time.Now()
	for rows.Next() {
		var item share
		var expiresAt, createdAt string
		if err := rows.Scan(&item.Key, &item.Path, &item.PasswordHash, &expiresAt, &createdAt); err != nil {
			return err
		}
		parsedExpiresAt, err := parseDBTime(expiresAt)
		if err != nil {
			return err
		}
		parsedCreatedAt, err := parseDBTime(createdAt)
		if err != nil {
			return err
		}
		item.ExpiresAt = parsedExpiresAt
		item.CreatedAt = parsedCreatedAt
		if item.Key != "" && item.ExpiresAt.After(now) {
			s.shares[item.Key] = item
		}
	}
	return rows.Err()
}

func (s *server) loadDirectLinks() error {
	if err := s.deleteExpiredDirectLinks(time.Now()); err != nil {
		return err
	}
	rows, err := s.db.Query(`SELECT key, path, name, created_at, expires_at, allowed_referers, allowed_ips, rate_limit_kbps, download_count, last_access_at FROM direct_links`)
	if err != nil {
		return err
	}
	defer rows.Close()

	now := time.Now()
	for rows.Next() {
		var item directLink
		var createdAt, referers, ips string
		var expiresAt, lastAccessAt sql.NullString
		if err := rows.Scan(&item.Key, &item.Path, &item.Name, &createdAt, &expiresAt, &referers, &ips, &item.RateLimitKBps, &item.DownloadCount, &lastAccessAt); err != nil {
			return err
		}
		parsedCreatedAt, err := parseDBTime(createdAt)
		if err != nil {
			return err
		}
		item.CreatedAt = parsedCreatedAt
		item.ExpiresAt, err = parseNullableDBTime(expiresAt)
		if err != nil {
			return err
		}
		item.LastAccessAt, err = parseNullableDBTime(lastAccessAt)
		if err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(referers), &item.AllowedReferers); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(ips), &item.AllowedIPs); err != nil {
			return err
		}
		if item.Key != "" && !directLinkExpired(item, now) {
			s.directLinks[item.Key] = item
		}
	}
	return rows.Err()
}

func (s *server) pruneExpiredShares() error {
	now := time.Now()
	if err := s.deleteExpiredShares(now); err != nil {
		return err
	}
	changed := false

	s.mu.Lock()
	for key, item := range s.shares {
		if now.After(item.ExpiresAt) {
			delete(s.shares, key)
			changed = true
		}
	}
	var err error
	if changed {
		err = s.saveSharesLocked()
	}
	s.mu.Unlock()
	return err
}

func (s *server) pruneExpiredDirectLinks() error {
	now := time.Now()
	if err := s.deleteExpiredDirectLinks(now); err != nil {
		return err
	}
	changed := false

	s.mu.Lock()
	for key, item := range s.directLinks {
		if directLinkExpired(item, now) {
			delete(s.directLinks, key)
			changed = true
		}
	}
	var err error
	if changed {
		err = s.saveDirectLinksLocked()
	}
	s.mu.Unlock()
	return err
}

func (s *server) removeExpiredDirectLink(key string) error {
	s.mu.Lock()
	delete(s.directLinks, key)
	err := s.saveDirectLinksLocked()
	s.mu.Unlock()
	return err
}

func (s *server) deleteExpiredShares(now time.Time) error {
	_, err := s.db.Exec(`DELETE FROM shares WHERE expires_at <= ?`, formatDBTime(now))
	return err
}

func (s *server) deleteExpiredDirectLinks(now time.Time) error {
	_, err := s.db.Exec(`DELETE FROM direct_links WHERE expires_at IS NOT NULL AND expires_at <= ?`, formatDBTime(now))
	return err
}

func (s *server) updateDirectLinkAccess(key string, count int, accessedAt time.Time) error {
	_, err := s.db.Exec(`UPDATE direct_links SET download_count = ?, last_access_at = ? WHERE key = ?`, count, formatDBTime(accessedAt), key)
	return err
}

func (s *server) recordAccessLog(kind string, key string, path string, r *http.Request, capture *captureResponseWriter, started time.Time, message string) {
	if s.db == nil {
		return
	}
	status := capture.status
	if status == 0 {
		status = http.StatusOK
	}
	id, err := randomHex(12)
	if err != nil {
		log.Printf("access log id: %v", err)
		return
	}
	item := accessLog{
		ID:         id,
		Type:       kind,
		Key:        key,
		Path:       path,
		IP:         clientIP(r, s.trustProxy),
		Referer:    r.Referer(),
		UserAgent:  r.UserAgent(),
		Status:     status,
		Bytes:      capture.bytes,
		DurationMs: time.Since(started).Milliseconds(),
		Message:    message,
		CreatedAt:  time.Now(),
	}
	_, err = s.db.Exec(
		`INSERT INTO access_logs (id, type, link_key, path, ip, referer, user_agent, status, bytes, duration_ms, message, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID,
		item.Type,
		item.Key,
		item.Path,
		item.IP,
		item.Referer,
		item.UserAgent,
		item.Status,
		item.Bytes,
		item.DurationMs,
		item.Message,
		formatDBTime(item.CreatedAt),
	)
	if err != nil {
		log.Printf("insert access log: %v", err)
		return
	}
	if err := s.pruneAccessLogs(); err != nil {
		log.Printf("prune access logs: %v", err)
	}
}

func (s *server) pruneAccessLogs() error {
	_, err := s.db.Exec(`DELETE FROM access_logs WHERE id NOT IN (SELECT id FROM access_logs ORDER BY created_at DESC LIMIT ?)`, maxAccessLogs)
	return err
}

func (s *server) saveSharesLocked() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM shares`); err != nil {
		_ = tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO shares (key, path, password_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, item := range s.shares {
		if _, err := stmt.Exec(item.Key, item.Path, item.PasswordHash, formatDBTime(item.ExpiresAt), formatDBTime(item.CreatedAt)); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *server) saveDirectLinksLocked() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM direct_links`); err != nil {
		_ = tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO direct_links (key, path, name, created_at, expires_at, allowed_referers, allowed_ips, rate_limit_kbps, download_count, last_access_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, item := range s.directLinks {
		referers, err := json.Marshal(item.AllowedReferers)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		ips, err := json.Marshal(item.AllowedIPs)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := stmt.Exec(item.Key, item.Path, item.Name, formatDBTime(item.CreatedAt), formatNullableDBTime(item.ExpiresAt), string(referers), string(ips), item.RateLimitKBps, item.DownloadCount, formatNullableDBTime(item.LastAccessAt)); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *server) storageStats() (storageStats, error) {
	stats := storageStats{}
	if err := filepath.WalkDir(s.filesRoot(), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || path == s.filesRoot() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if info.IsDir() {
			stats.FolderCount++
		} else {
			stats.FileCount++
			stats.TotalSize += info.Size()
		}
		return nil
	}); err != nil {
		return stats, err
	}

	if err := s.pruneExpiredShares(); err != nil {
		return stats, err
	}
	if err := s.pruneExpiredDirectLinks(); err != nil {
		return stats, err
	}
	s.mu.RLock()
	stats.ShareCount = len(s.shares)
	stats.DirectCount = len(s.directLinks)
	s.mu.RUnlock()
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM access_logs`).Scan(&stats.LogCount); err != nil {
		return stats, err
	}
	return stats, nil
}

func (s *server) safePath(rel string) (string, string, error) {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	rel = strings.TrimPrefix(rel, "/")
	clean := filepath.Clean(rel)
	if clean == "." {
		clean = ""
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
		return "", "", errors.New("invalid path")
	}
	target := filepath.Join(s.filesRoot(), clean)
	root, err := filepath.Abs(s.filesRoot())
	if err != nil {
		return "", "", err
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", "", err
	}
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", "", errors.New("path escapes storage root")
	}
	return abs, filepath.ToSlash(clean), nil
}

func (s *server) safeWebPath(path string) (string, error) {
	clean := filepath.Clean(strings.TrimPrefix(path, "/"))
	if clean == "." {
		clean = "index.html"
	}
	root, err := filepath.Abs(s.webDir)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, clean))
	if err != nil {
		return "", err
	}
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return "", errors.New("path escapes web root")
	}
	return target, nil
}

func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := s.currentSession(r); !ok {
			writeError(w, http.StatusUnauthorized, errors.New("authentication required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) currentSession(r *http.Request) (string, time.Time, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return "", time.Time{}, false
	}

	now := time.Now()
	s.authMu.RLock()
	item, ok := s.sessions[cookie.Value]
	s.authMu.RUnlock()
	if !ok {
		return "", time.Time{}, false
	}
	if now.After(item.ExpiresAt) {
		s.authMu.Lock()
		delete(s.sessions, cookie.Value)
		s.authMu.Unlock()
		return "", time.Time{}, false
	}
	return item.Username, item.ExpiresAt, true
}

func (s *server) sessionCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
	}
}

func (s *server) filesRoot() string {
	return filepath.Join(s.dataDir, "files")
}

func formatDBTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseDBTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func formatNullableDBTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatDBTime(*value)
}

func parseNullableDBTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	parsed, err := parseDBTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func itemFromInfo(rel string, info os.FileInfo) fileItem {
	itemType := "file"
	if info.IsDir() {
		itemType = "folder"
	}
	mimeType := mime.TypeByExtension(filepath.Ext(info.Name()))
	return fileItem{
		Name:        info.Name(),
		Path:        filepath.ToSlash(rel),
		Type:        itemType,
		Size:        info.Size(),
		Modified:    info.ModTime(),
		Mime:        mimeType,
		PreviewType: previewType(info.Name(), info.IsDir(), mimeType),
	}
}

func previewType(name string, isDir bool, mimeType string) string {
	if isDir {
		return "folder"
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case mimeType == "application/pdf":
		return "pdf"
	}
	switch ext {
	case "txt", "md", "json", "yaml", "yml", "xml", "csv", "log", "go", "js", "ts", "tsx", "jsx", "css", "html", "sh", "sql":
		return "text"
	default:
		return "download"
	}
}

func directLinkExpired(item directLink, now time.Time) bool {
	return item.ExpiresAt != nil && now.After(*item.ExpiresAt)
}

func (s *server) directLinkAllowed(item directLink, r *http.Request) bool {
	return refererAllowed(item.AllowedReferers, r.Referer()) && ipAllowed(item.AllowedIPs, clientIP(r, s.trustProxy))
}

func refererAllowed(rules []string, referer string) bool {
	if len(rules) == 0 {
		return true
	}
	referer = strings.ToLower(strings.TrimSpace(referer))
	if referer == "" {
		return false
	}
	for _, rule := range rules {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == "" {
			continue
		}
		if strings.Contains(referer, rule) {
			return true
		}
	}
	return false
}

func ipAllowed(rules []string, ipText string) bool {
	if len(rules) == 0 {
		return true
	}
	ip := net.ParseIP(ipText)
	if ip == nil {
		return false
	}
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		if strings.Contains(rule, "/") {
			_, network, err := net.ParseCIDR(rule)
			if err == nil && network.Contains(ip) {
				return true
			}
			continue
		}
		if allowed := net.ParseIP(rule); allowed != nil && allowed.Equal(ip) {
			return true
		}
	}
	return false
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		return forwardedClientIP(r)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func forwardedClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func directLinkName(rel string, isDir bool) string {
	name := downloadName(rel, "xfile")
	if isDir && !strings.HasSuffix(strings.ToLower(name), ".zip") {
		name += ".zip"
	}
	return name
}

func shareToView(item share) shareView {
	return shareView{
		Key:         item.Key,
		Path:        item.Path,
		HasPassword: item.PasswordHash != "",
		ExpiresAt:   item.ExpiresAt,
		CreatedAt:   item.CreatedAt,
	}
}

func passwordHash(password string) string {
	password = strings.TrimSpace(password)
	if password == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func shareAuthCookieName(key string) string {
	return "xfile_share_" + key
}

func cleanRules(values []string) []string {
	rules := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		rules = append(rules, value)
	}
	return rules
}

func parsePositiveInt(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	n := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return fallback
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 {
		return fallback
	}
	return n
}

func sortItems(items []fileItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			return items[i].Type == "folder"
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
}

func joinRel(parent string, name string) string {
	return strings.Trim(strings.Trim(parent, "/")+"/"+name, "/")
}

func validName(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && name != "." && name != ".." && !strings.Contains(name, "/") && !strings.Contains(name, "\\")
}

func downloadName(name string, fallback string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Trim(name, ".")
	if name == "" || name == string(os.PathSeparator) {
		name = fallback
	}
	return strings.NewReplacer("\r", "", "\n", "", "\"", "").Replace(name)
}

func secureCompare(a string, b string) bool {
	aHash := sha256.Sum256([]byte(a))
	bHash := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(aHash[:], bHash[:]) == 1
}

func randomKey() (string, error) {
	return randomHex(8)
}

func randomHex(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
