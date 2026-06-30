package server

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"xfile/internal/domain"
)

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Dashboard()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) publicSite(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.PublicSite(s.isAuthenticated(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) storageSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.store.StorageSources(false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sources)
}

func (s *Server) createStorageSource(w http.ResponseWriter, r *http.Request) {
	var req domain.StorageSourceInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	source, err := s.store.CreateStorageSource(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("storage-create", source.Key, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, source)
}

func (s *Server) updateStorageSource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req domain.StorageSourceInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	source, err := s.store.UpdateStorageSource(id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("storage-update", source.Key, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, source)
}

func (s *Server) deleteStorageSource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.DeleteStorageSource(id); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("storage-delete", strconv.FormatInt(id, 10), clientIP(r), r.UserAgent())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) publicStorageFiles(w http.ResponseWriter, r *http.Request) {
	storageKey := r.PathValue("key")
	rel := r.URL.Query().Get("path")
	files, err := s.store.ListSourceFiles(storageKey, rel, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("public-list", storageKey+":"+rel, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) publicStorageDownload(w http.ResponseWriter, r *http.Request) {
	storageKey := r.PathValue("key")
	rel := r.URL.Query().Get("path")
	download, err := s.store.SourceDownload(storageKey, rel, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer download.Reader.Close()
	downloadPath := storageKey + ":" + rel
	if !s.enforceDownloadLimit(w, r, downloadPath) {
		return
	}
	_ = s.store.LogAccess("public-download", downloadPath, clientIP(r), r.UserAgent())
	http.ServeContent(w, r, download.Entry.Name, parseRFC3339(download.Entry.ModifiedAt), download.Reader)
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	storageKey := storageKeyFromRequest(r)
	files, err := s.store.ListSourceFilesForAdmin(storageKey, path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("list", storageKey+":"+path, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) searchFiles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit := queryInt(r, "limit", 50)
	storageKey := storageKeyFromRequest(r)
	files, err := s.store.SearchSourceFiles(storageKey, query, limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(query) != "" {
		_ = s.store.LogAccess("search", storageKey+":"+query, clientIP(r), r.UserAgent())
	}
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	if s.store.SettingValue("allowUpload", "enabled") != "enabled" {
		writeError(w, http.StatusForbidden, os.ErrPermission)
		return
	}
	limitMB, err := strconv.ParseInt(s.store.SettingValue("maxUploadMB", "512"), 10, 64)
	if err != nil || limitMB < 1 {
		limitMB = 512
	}
	limit := limitMB << 20
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseMultipartForm(limit); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyFromRequest(r)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	name := filepath.Base(header.Filename)
	rel, err := s.store.UploadSourceFile(storageKey, r.FormValue("path"), name, file, header.Size)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("upload", storageKey+":"+rel, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, map[string]string{"path": rel})
}

func (s *Server) createFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StorageKey string `json:"storageKey"`
		Path       string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	folder, err := s.store.CreateSourceFolder(storageKey, req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("mkdir", storageKey+":"+req.Path, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, folder)
}

func (s *Server) createEmptyFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StorageKey string `json:"storageKey"`
		Path       string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	file, err := s.store.CreateSourceEmptyFile(storageKey, req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("touch", storageKey+":"+req.Path, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, file)
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	storageKey := storageKeyFromRequest(r)
	download, err := s.store.SourceDownload(storageKey, rel, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer download.Reader.Close()
	downloadPath := storageKey + ":" + rel
	if !s.enforceDownloadLimit(w, r, downloadPath) {
		return
	}
	_ = s.store.LogAccess("download", downloadPath, clientIP(r), r.UserAgent())
	http.ServeContent(w, r, download.Entry.Name, parseRFC3339(download.Entry.ModifiedAt), download.Reader)
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	storageKey := storageKeyFromRequest(r)
	if err := s.store.DeleteSourceFile(storageKey, rel); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("delete", storageKey+":"+rel, clientIP(r), r.UserAgent())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) moveFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StorageKey string `json:"storageKey"`
		From       string `json:"from"`
		To         string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	entry, err := s.store.MoveSourceFile(storageKey, req.From, req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("move", storageKey+":"+req.From+" -> "+req.To, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) listShares(w http.ResponseWriter, r *http.Request) {
	shares, err := s.store.Shares()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, shares)
}

func (s *Server) createShare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path      string `json:"path"`
		ExpiresAt string `json:"expiresAt"`
		Password  string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	share, err := s.store.CreateShare(req.Path, req.ExpiresAt, req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, share)
}

func (s *Server) deleteShare(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.DeleteShare(id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) sharePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.cfg.StaticDir, "index.html"))
}

func (s *Server) publicShare(w http.ResponseWriter, r *http.Request) {
	detail, err := s.store.ShareDetail(r.PathValue("token"), r.URL.Query().Get("password"), r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	if share, err := s.store.ResolveShare(r.PathValue("token"), r.URL.Query().Get("password")); err == nil {
		_ = s.store.RecordShareView(share.ID)
	}
	_ = s.store.LogAccess("share-view", detail.CurrentPath, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) downloadShare(w http.ResponseWriter, r *http.Request) {
	share, path, err := s.store.SharedFilePath(r.PathValue("token"), r.URL.Query().Get("password"), r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	downloadPath := share.Path
	if child := strings.TrimSpace(r.URL.Query().Get("path")); child != "" {
		downloadPath = strings.Trim(downloadPath+"/"+child, "/")
	}
	if !s.enforceDownloadLimit(w, r, downloadPath) {
		return
	}
	_ = s.store.RecordShareDownload(share.ID)
	_ = s.store.LogAccess("share-download", downloadPath, clientIP(r), r.UserAgent())
	http.ServeFile(w, r, path)
}

func (s *Server) openShare(w http.ResponseWriter, r *http.Request) {
	share, err := s.store.ResolveShare(r.PathValue("token"), r.URL.Query().Get("password"))
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	path, err := s.store.FilePath(share.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("share", share.Path, clientIP(r), r.UserAgent())
	http.ServeFile(w, r, path)
}

func (s *Server) listDirectLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.store.DirectLinks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, links)
}

func (s *Server) createDirectLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	link, err := s.store.CreateDirectLink(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (s *Server) deleteDirectLink(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.DeleteDirectLink(id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) updateDirectLink(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.UpdateDirectLink(id, req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": req.Enabled})
}

func (s *Server) openDirectLink(w http.ResponseWriter, r *http.Request) {
	link, err := s.store.ResolveDirectLink(r.PathValue("token"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	path, err := s.store.FilePath(link.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.enforceDownloadLimit(w, r, link.Path) {
		return
	}
	_ = s.store.RecordDirectLinkAccess(link.ID)
	_ = s.store.LogAccess("direct", link.Path, clientIP(r), r.UserAgent())
	http.ServeFile(w, r, path)
}

func (s *Server) accessLogs(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	pageSize := queryInt(r, "pageSize", 20)
	logs, err := s.store.SearchAccessLogs(
		page,
		pageSize,
		r.URL.Query().Get("action"),
		r.URL.Query().Get("path"),
		r.URL.Query().Get("ip"),
		r.URL.Query().Get("userAgent"),
		r.URL.Query().Get("startTime"),
		r.URL.Query().Get("endTime"),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) deleteAccessLogs(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.store.DeleteAccessLogs(queryInt(r, "olderThanDays", 0), queryBool(r, "all", false))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("logs-cleanup", strconv.FormatInt(deleted, 10), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": deleted})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.Users()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.store.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("user-create", user.Username, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.store.UpdateUser(id, req.Username, req.Password, req.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("user-update", user.Username, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.DeleteUser(id); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("user-delete", strconv.FormatInt(id, 10), clientIP(r), r.UserAgent())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.Settings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := validateAccessSettings(req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.SaveSettings(req); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		if first, _, ok := strings.Cut(forwarded, ","); ok {
			return strings.TrimSpace(first)
		}
		return forwarded
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func queryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func queryBool(r *http.Request, key string, fallback bool) bool {
	value, err := strconv.ParseBool(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func storageKeyFromRequest(r *http.Request) string {
	if r.Method == http.MethodPost {
		if value := strings.TrimSpace(r.FormValue("storageKey")); value != "" {
			return value
		}
	}
	return storageKeyOrDefault(r.URL.Query().Get("storageKey"))
}

func storageKeyOrDefault(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return "local"
}

func parseRFC3339(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
