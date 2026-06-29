package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const appName = "xfile"

type server struct {
	dataDir string
	webDir  string
	shares  map[string]share
	mu      sync.RWMutex
}

type fileItem struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Type     string    `json:"type"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

type share struct {
	Key       string    `json:"key"`
	Path      string    `json:"path"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

func main() {
	port := getenv("XFILE_PORT", "3008")
	dataDir := getenv("XFILE_DATA_DIR", "data")
	webDir := getenv("XFILE_WEB_DIR", "web/dist")

	if err := os.MkdirAll(filepath.Join(dataDir, "files"), 0755); err != nil {
		log.Fatalf("create data directory: %v", err)
	}

	s := &server{
		dataDir: dataDir,
		webDir:  webDir,
		shares:  map[string]share{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/config", s.config)
	mux.HandleFunc("GET /api/files", s.listFiles)
	mux.HandleFunc("POST /api/folders", s.createFolder)
	mux.HandleFunc("POST /api/upload", s.uploadFile)
	mux.HandleFunc("GET /api/download", s.downloadFile)
	mux.HandleFunc("DELETE /api/files", s.deleteFile)
	mux.HandleFunc("POST /api/rename", s.renameFile)
	mux.HandleFunc("POST /api/share", s.createShare)
	mux.HandleFunc("GET /api/shares", s.listShares)
	mux.HandleFunc("GET /s/", s.openShare)
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
		"version": "0.1.0",
	})
}

func (s *server) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":     appName,
		"features": []string{"local-storage", "upload", "download", "share-link", "preview", "file-management"},
	})
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
		itemType := "file"
		if entry.IsDir() {
			itemType = "folder"
		}
		itemPath := strings.Trim(strings.Trim(cleanRel, "/")+"/"+entry.Name(), "/")
		items = append(items, fileItem{
			Name:     entry.Name(),
			Path:     itemPath,
			Type:     itemType,
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			return items[i].Type == "folder"
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"path":  cleanRel,
		"items": items,
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
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, errors.New("folder name is required"))
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
		src, err := header.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		defer src.Close()

		dstPath := filepath.Join(dir, filepath.Base(header.Filename))
		dst, err := os.Create(dstPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = dst.Close()
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		_ = dst.Close()
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "count": len(files)})
}

func (s *server) downloadFile(w http.ResponseWriter, r *http.Request) {
	s.serveFilePath(w, r, r.URL.Query().Get("path"), true)
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
	if strings.TrimSpace(req.NewName) == "" || strings.Contains(req.NewName, "/") || strings.Contains(req.NewName, "\\") {
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
	item := share{
		Key:       key,
		Path:      cleanRel,
		ExpiresAt: time.Now().Add(time.Duration(req.ExpiresInHours) * time.Hour),
		CreatedAt: time.Now(),
	}
	s.mu.Lock()
	s.shares[key] = item
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, item)
}

func (s *server) listShares(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]share, 0, len(s.shares))
	for _, item := range s.shares {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) openShare(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/s/")
	s.mu.RLock()
	item, ok := s.shares[key]
	s.mu.RUnlock()
	if !ok || time.Now().After(item.ExpiresAt) {
		writeError(w, http.StatusNotFound, errors.New("share link not found or expired"))
		return
	}
	s.serveFilePath(w, r, item.Path, false)
}

func (s *server) serveFilePath(w http.ResponseWriter, r *http.Request, rel string, attachment bool) {
	target, _, err := s.safePath(rel)
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
		if !attachment {
			s.renderSharedDirectory(w, rel, target)
			return
		}
		writeError(w, http.StatusBadRequest, errors.New("folders cannot be downloaded yet"))
		return
	}
	if attachment {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
	}
	if ctype := mime.TypeByExtension(filepath.Ext(info.Name())); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	http.ServeFile(w, r, target)
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
		name := entry.Name()
		label := name
		if entry.IsDir() {
			label += "/"
		}
		_, _ = fmt.Fprintf(w, "<li>%s</li>", html.EscapeString(label))
	}
	_, _ = fmt.Fprint(w, "</ul></body>")
}

func (s *server) frontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.webDir, filepath.Clean(r.URL.Path))
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.webDir, "index.html"))
}

func (s *server) safePath(rel string) (string, string, error) {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	rel = strings.TrimPrefix(rel, "/")
	clean := filepath.Clean(rel)
	if clean == "." {
		clean = ""
	}
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
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

func (s *server) filesRoot() string {
	return filepath.Join(s.dataDir, "files")
}

func randomKey() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
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

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
