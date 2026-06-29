package server

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	files, err := s.store.ListFiles(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("list", path, clientIP(r), r.UserAgent())
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
	dir, err := s.store.FilePath(r.FormValue("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	name := filepath.Base(header.Filename)
	target := filepath.Join(dir, name)
	out, err := os.Create(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer out.Close()
	if _, err := out.ReadFrom(file); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	rel := strings.Trim(filepath.ToSlash(filepath.Join(r.FormValue("path"), name)), "/")
	_ = s.store.LogAccess("upload", rel, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, map[string]string{"path": rel})
}

func (s *Server) createFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	folder, err := s.store.CreateFolder(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("mkdir", req.Path, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, folder)
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	path, err := s.store.FilePath(rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("download", rel, clientIP(r), r.UserAgent())
	http.ServeFile(w, r, path)
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	if err := s.store.DeleteFile(rel); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("delete", rel, clientIP(r), r.UserAgent())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) moveFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	entry, err := s.store.MoveFile(req.From, req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("move", req.From+" -> "+req.To, clientIP(r), r.UserAgent())
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
	_ = s.store.LogAccess("direct", link.Path, clientIP(r), r.UserAgent())
	http.ServeFile(w, r, path)
}

func (s *Server) accessLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.store.AccessLogs(50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
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
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
