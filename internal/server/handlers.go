package server

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"xfile/internal/domain"
	appstore "xfile/internal/store"
)

const maxTextEditBytes = 2 << 20
const maxFileDescriptionBytes = 8 << 10

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
	loggedIn := s.isAuthenticated(r)
	data, err := s.store.PublicSite(loggedIn)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if loggedIn {
		user, err := s.currentUser(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		sources, err := s.store.StorageSourcesForUser(user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		data.Sources = sources
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) storageSources(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	sources, err := s.store.StorageSourcesForUser(user)
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
	files, err := s.store.ListSourceFilesWithPassword(storageKey, rel, true, r.URL.Query().Get("directoryPassword"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("public-list", storageKey+":"+rel, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) publicStorageDownload(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, downloadOperation(r)) {
		return
	}
	storageKey := r.PathValue("key")
	rel := r.URL.Query().Get("path")
	download, err := s.store.SourceDownloadWithPassword(storageKey, rel, true, r.URL.Query().Get("directoryPassword"))
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

func (s *Server) publicStorageArchive(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "download") {
		return
	}
	var req struct {
		Paths             []string `json:"paths"`
		DirectoryPassword string   `json:"directoryPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	paths, err := normalizeArchivePaths(req.Paths)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := r.PathValue("key")
	if !s.enforceDownloadLimit(w, r, storageKey+":public-archive") {
		return
	}

	filename := archiveFilename(paths)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	writer := zip.NewWriter(w)
	defer writer.Close()

	added := make(map[string]bool)
	for _, rel := range paths {
		if err := s.addPublicPathToArchive(writer, storageKey, rel, path.Base(rel), req.DirectoryPassword, added); err != nil {
			return
		}
	}
	_ = s.store.LogAccess("public-archive-download", storageKey+":"+strconv.Itoa(len(paths)), clientIP(r), r.UserAgent())
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	storageKey := storageKeyFromRequest(r)
	if !s.requireStorageListAccess(w, r, storageKey, path) {
		return
	}
	files, err := s.store.ListSourceFilesForAdmin(storageKey, path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	files = s.filterStorageFilesForUser(r, storageKey, files, true)
	_ = s.store.LogAccess("list", storageKey+":"+path, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) searchFiles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit := queryInt(r, "limit", 50)
	storageKey := storageKeyFromRequest(r)
	if !s.requireStorageAccess(w, r, storageKey) {
		return
	}
	files, err := s.store.SearchSourceFiles(storageKey, query, limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	files = s.filterStorageFilesForUser(r, storageKey, files, false)
	if strings.TrimSpace(query) != "" {
		_ = s.store.LogAccess("search", storageKey+":"+query, clientIP(r), r.UserAgent())
	}
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "upload") {
		return
	}
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
	if !s.requireStoragePathAccess(w, r, storageKey, requestTargetPath(r.FormValue("path"), name)) {
		return
	}
	rel, err := s.store.UploadSourceFile(storageKey, r.FormValue("path"), name, file, header.Size)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("upload", storageKey+":"+rel, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, map[string]string{"path": rel})
}

func (s *Server) createFolder(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "upload") {
		return
	}
	var req struct {
		StorageKey string `json:"storageKey"`
		Path       string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	if !s.requireStoragePathAccess(w, r, storageKey, req.Path) {
		return
	}
	folder, err := s.store.CreateSourceFolder(storageKey, req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("mkdir", storageKey+":"+req.Path, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, folder)
}

func (s *Server) createEmptyFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "upload") {
		return
	}
	var req struct {
		StorageKey string `json:"storageKey"`
		Path       string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	if !s.requireStoragePathAccess(w, r, storageKey, req.Path) {
		return
	}
	file, err := s.store.CreateSourceEmptyFile(storageKey, req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("touch", storageKey+":"+req.Path, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, file)
}

func (s *Server) saveTextFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "upload") {
		return
	}
	if s.store.SettingValue("allowUpload", "enabled") != "enabled" {
		writeError(w, http.StatusForbidden, os.ErrPermission)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxTextEditBytes*4+16*1024)
	var req struct {
		StorageKey string `json:"storageKey"`
		Path       string `json:"path"`
		Content    string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if int64(len(req.Content)) > maxTextEditBytes {
		writeError(w, http.StatusRequestEntityTooLarge, errors.New("text file exceeds 2 MB"))
		return
	}
	clean, err := cleanSharePath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if clean == "" {
		writeError(w, http.StatusBadRequest, errors.New("file path is required"))
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	if !s.requireStoragePathAccess(w, r, storageKey, clean) {
		return
	}
	entry, err := s.store.SaveSourceTextFile(storageKey, clean, req.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("text-save", storageKey+":"+clean, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) saveFileMetadata(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "rename") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFileDescriptionBytes)
	var req struct {
		StorageKey  string `json:"storageKey"`
		Path        string `json:"path"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	clean, err := cleanSharePath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if clean == "" {
		writeError(w, http.StatusBadRequest, errors.New("file path is required"))
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	if !s.requireStoragePathAccess(w, r, storageKey, clean) {
		return
	}
	entry, err := s.store.SaveFileDescription(storageKey, clean, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("metadata-save", storageKey+":"+clean, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, downloadOperation(r)) {
		return
	}
	rel := r.URL.Query().Get("path")
	storageKey := storageKeyFromRequest(r)
	if !s.requireStoragePathAccess(w, r, storageKey, rel) {
		return
	}
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

func (s *Server) downloadArchive(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "download") {
		return
	}
	var req struct {
		StorageKey string   `json:"storageKey"`
		Paths      []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	paths, err := normalizeArchivePaths(req.Paths)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	if !s.requireStoragePathAccess(w, r, storageKey, paths...) {
		return
	}
	if !s.enforceDownloadLimit(w, r, storageKey+":archive") {
		return
	}

	filename := archiveFilename(paths)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	writer := zip.NewWriter(w)
	defer writer.Close()

	added := make(map[string]bool)
	for _, rel := range paths {
		if err := s.addPathToArchive(writer, storageKey, rel, path.Base(rel), added); err != nil {
			return
		}
	}
	_ = s.store.LogAccess("archive-download", storageKey+":"+strconv.Itoa(len(paths)), clientIP(r), r.UserAgent())
}

func (s *Server) addPathToArchive(writer *zip.Writer, storageKey, rel, archivePath string, added map[string]bool) error {
	download, err := s.store.SourceDownload(storageKey, rel, false)
	if err == nil {
		defer download.Reader.Close()
		return addDownloadToArchive(writer, download.Entry, download.Reader, archivePath, added)
	}
	files, listErr := s.store.ListSourceFilesForAdmin(storageKey, rel)
	if listErr != nil {
		return err
	}
	folderPath := strings.Trim(archivePath, "/")
	if folderPath != "" && !strings.HasSuffix(folderPath, "/") {
		folderPath += "/"
	}
	if folderPath != "" && !added[folderPath] {
		header := &zip.FileHeader{Name: folderPath, Method: zip.Store}
		header.SetModTime(time.Now())
		if _, err := writer.CreateHeader(header); err != nil {
			return err
		}
		added[folderPath] = true
	}
	for _, file := range files {
		childArchivePath := strings.Trim(strings.Trim(archivePath, "/")+"/"+file.Name, "/")
		if err := s.addPathToArchive(writer, storageKey, file.Path, childArchivePath, added); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) addPublicPathToArchive(writer *zip.Writer, storageKey, rel, archivePath, directoryPassword string, added map[string]bool) error {
	download, err := s.store.SourceDownloadWithPassword(storageKey, rel, true, directoryPassword)
	if err == nil {
		defer download.Reader.Close()
		return addDownloadToArchive(writer, download.Entry, download.Reader, archivePath, added)
	}
	files, listErr := s.store.ListSourceFilesWithPassword(storageKey, rel, true, directoryPassword)
	if listErr != nil {
		return err
	}
	folderPath := strings.Trim(archivePath, "/")
	if folderPath != "" && !strings.HasSuffix(folderPath, "/") {
		folderPath += "/"
	}
	if folderPath != "" && !added[folderPath] {
		header := &zip.FileHeader{Name: folderPath, Method: zip.Store}
		header.SetModTime(time.Now())
		if _, err := writer.CreateHeader(header); err != nil {
			return err
		}
		added[folderPath] = true
	}
	for _, file := range files {
		childArchivePath := strings.Trim(strings.Trim(archivePath, "/")+"/"+file.Name, "/")
		if err := s.addPublicPathToArchive(writer, storageKey, file.Path, childArchivePath, directoryPassword, added); err != nil {
			return err
		}
	}
	return nil
}

func addDownloadToArchive(writer *zip.Writer, entry domain.FileEntry, reader io.Reader, archivePath string, added map[string]bool) error {
	name := strings.Trim(archivePath, "/")
	if name == "" {
		name = entry.Name
	}
	if added[name] {
		return nil
	}
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	header.SetModTime(parseRFC3339(entry.ModifiedAt))
	header.SetMode(0o644)
	target, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, reader); err != nil {
		return err
	}
	added[name] = true
	return nil
}

func normalizeArchivePaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, errors.New("at least one path is required")
	}
	normalized := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, value := range paths {
		clean, err := cleanSharePath(value)
		if err != nil {
			return nil, err
		}
		if clean == "" {
			return nil, errors.New("archive path is required")
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		normalized = append(normalized, clean)
	}
	if len(normalized) == 0 {
		return nil, errors.New("at least one path is required")
	}
	return normalized, nil
}

func archiveFilename(paths []string) string {
	if len(paths) == 1 {
		name := strings.Trim(path.Base(paths[0]), ".")
		if name != "" {
			return name + ".zip"
		}
	}
	return "xfile-archive.zip"
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "delete") {
		return
	}
	rel := r.URL.Query().Get("path")
	storageKey := storageKeyFromRequest(r)
	if !s.requireStoragePathAccess(w, r, storageKey, rel) {
		return
	}
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
	operation := "move"
	if sameDirectory(req.From, req.To) {
		operation = "rename"
	}
	if !s.requireOperation(w, r, operation) {
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	if !s.requireStoragePathAccess(w, r, storageKey, req.From, req.To) {
		return
	}
	entry, err := s.store.MoveSourceFile(storageKey, req.From, req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("move", storageKey+":"+req.From+" -> "+req.To, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) batchMoveFiles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StorageKey string   `json:"storageKey"`
		Paths      []string `json:"paths"`
		TargetDir  string   `json:"targetDir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.requireOperation(w, r, "move") {
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	targets, err := batchMoveTargets(req.Paths, req.TargetDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	checkPaths := make([]string, 0, len(req.Paths)+len(targets))
	checkPaths = append(checkPaths, req.Paths...)
	checkPaths = append(checkPaths, targets...)
	if !s.requireStoragePathAccess(w, r, storageKey, checkPaths...) {
		return
	}
	entries := make([]domain.FileEntry, 0, len(req.Paths))
	for index, from := range req.Paths {
		entry, err := s.store.MoveSourceFile(storageKey, from, targets[index])
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		entries = append(entries, entry)
	}
	_ = s.store.LogAccess("batch-move", storageKey+":"+strconv.Itoa(len(entries))+" -> "+strings.Trim(targetDirForLog(req.TargetDir), "/"), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) batchCopyFiles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StorageKey string   `json:"storageKey"`
		Paths      []string `json:"paths"`
		TargetDir  string   `json:"targetDir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.requireOperation(w, r, "copy") {
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	targets, err := batchMoveTargets(req.Paths, req.TargetDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	checkPaths := make([]string, 0, len(req.Paths)+len(targets))
	checkPaths = append(checkPaths, req.Paths...)
	checkPaths = append(checkPaths, targets...)
	if !s.requireStoragePathAccess(w, r, storageKey, checkPaths...) {
		return
	}
	entries := make([]domain.FileEntry, 0, len(req.Paths))
	for index, from := range req.Paths {
		entry, err := s.store.CopySourceFile(storageKey, from, targets[index])
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		entries = append(entries, entry)
	}
	_ = s.store.LogAccess("batch-copy", storageKey+":"+strconv.Itoa(len(entries))+" -> "+strings.Trim(targetDirForLog(req.TargetDir), "/"), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, entries)
}

func batchMoveTargets(paths []string, targetDir string) ([]string, error) {
	targetDir = strings.Trim(strings.ReplaceAll(targetDir, "\\", "/"), "/")
	if len(paths) == 0 {
		return nil, errors.New("at least one path is required")
	}
	targets := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, source := range paths {
		source = strings.Trim(strings.ReplaceAll(source, "\\", "/"), "/")
		if source == "" {
			return nil, errors.New("source path is required")
		}
		name := path.Base(source)
		if name == "." || name == "/" {
			return nil, errors.New("source path is invalid")
		}
		target := strings.Trim(strings.Trim(targetDir+"/"+name, "/"), "/")
		if target == "" || target == source {
			return nil, errors.New("target path must be different from source path")
		}
		if strings.HasPrefix(target+"/", source+"/") {
			return nil, errors.New("cannot move a directory into itself")
		}
		if seen[target] {
			return nil, errors.New("duplicate target path")
		}
		seen[target] = true
		targets = append(targets, target)
	}
	return targets, nil
}

func targetDirForLog(targetDir string) string {
	targetDir = strings.Trim(strings.ReplaceAll(targetDir, "\\", "/"), "/")
	if targetDir == "" {
		return "."
	}
	return targetDir
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
	if !s.requireOperation(w, r, "share") {
		return
	}
	var req struct {
		StorageKey string `json:"storageKey"`
		Path       string `json:"path"`
		ExpiresAt  string `json:"expiresAt"`
		Password   string `json:"password"`
		Token      string `json:"token"`
		CustomKey  string `json:"customKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	if !s.requireStoragePathAccess(w, r, storageKey, req.Path) {
		return
	}
	customKey := req.Token
	if strings.TrimSpace(customKey) == "" {
		customKey = req.CustomKey
	}
	share, err := s.store.CreateSourceShareWithToken(storageKey, req.Path, req.ExpiresAt, req.Password, customKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, share)
}

func (s *Server) batchCreateShares(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "share") {
		return
	}
	var req struct {
		StorageKey string   `json:"storageKey"`
		Paths      []string `json:"paths"`
		ExpiresAt  string   `json:"expiresAt"`
		Password   string   `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	paths, err := normalizeBatchSharePaths(req.Paths)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	if !s.requireStoragePathAccess(w, r, storageKey, paths...) {
		return
	}
	share, err := s.store.CreateSourceBundleShare(storageKey, paths, req.ExpiresAt, req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("batch-share", strconv.Itoa(len(paths)), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, share)
}

func normalizeBatchSharePaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, errors.New("at least one path is required")
	}
	normalized := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, value := range paths {
		clean, err := cleanSharePath(value)
		if err != nil {
			return nil, err
		}
		if clean == "" {
			return nil, errors.New("share path is required")
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		normalized = append(normalized, clean)
	}
	if len(normalized) == 0 {
		return nil, errors.New("at least one path is required")
	}
	return normalized, nil
}

func cleanSharePath(value string) (string, error) {
	value = strings.TrimPrefix(filepath.ToSlash(value), "/")
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." {
		return "", nil
	}
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", errors.New("invalid path")
	}
	return filepath.ToSlash(clean), nil
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

func (s *Server) deleteExpiredShares(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.store.DeleteExpiredShares()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = s.store.LogAccess("shares-expired-cleanup", strconv.FormatInt(deleted, 10), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": deleted})
}

func (s *Server) sharePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.cfg.StaticDir, "index.html"))
}

func (s *Server) publicShare(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	password := r.URL.Query().Get("password")
	if !s.allowSharePasswordAttempt(w, r, token, password) {
		return
	}
	detail, err := s.store.ShareDetail(token, password, r.URL.Query().Get("path"))
	if err != nil {
		s.recordSharePasswordFailure(r, token, password, err)
		writeError(w, http.StatusForbidden, err)
		return
	}
	s.clearSharePasswordFailures(r, token)
	if share, err := s.store.ResolveShare(token, password); err == nil {
		_ = s.store.RecordShareView(share.ID)
	}
	_ = s.store.LogAccess("share-view", detail.CurrentPath, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) downloadShare(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, downloadOperation(r)) {
		return
	}
	token := r.PathValue("token")
	password := r.URL.Query().Get("password")
	if !s.allowSharePasswordAttempt(w, r, token, password) {
		return
	}
	share, download, err := s.store.SharedDownload(token, password, r.URL.Query().Get("path"))
	if err != nil {
		s.recordSharePasswordFailure(r, token, password, err)
		writeError(w, http.StatusForbidden, err)
		return
	}
	s.clearSharePasswordFailures(r, token)
	defer download.Reader.Close()
	downloadPath := share.StorageKey + ":" + share.Path
	if child := strings.TrimSpace(r.URL.Query().Get("path")); child != "" {
		downloadPath = strings.Trim(downloadPath+"/"+child, "/")
	}
	if !s.enforceDownloadLimit(w, r, downloadPath) {
		return
	}
	_ = s.store.RecordShareDownload(share.ID)
	_ = s.store.LogAccess("share-download", downloadPath, clientIP(r), r.UserAgent())
	http.ServeContent(w, r, download.Entry.Name, parseRFC3339(download.Entry.ModifiedAt), download.Reader)
}

func (s *Server) openShare(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "download") {
		return
	}
	token := r.PathValue("token")
	password := r.URL.Query().Get("password")
	if !s.allowSharePasswordAttempt(w, r, token, password) {
		return
	}
	share, err := s.store.ResolveShare(token, password)
	if err != nil {
		s.recordSharePasswordFailure(r, token, password, err)
		writeError(w, http.StatusForbidden, err)
		return
	}
	s.clearSharePasswordFailures(r, token)
	path, err := s.store.FilePath(share.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("share", share.Path, clientIP(r), r.UserAgent())
	http.ServeFile(w, r, path)
}

func (s *Server) allowSharePasswordAttempt(w http.ResponseWriter, r *http.Request, token, password string) bool {
	if strings.TrimSpace(password) == "" {
		return true
	}
	limit, err := strconv.Atoi(s.store.SettingValue("sharePasswordLimitPerMinute", "5"))
	if err != nil || limit < 1 {
		return true
	}
	key := sharePasswordAttemptKey(token, clientIP(r))
	if !s.sharePasswords.tooMany(key, limit, time.Now()) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	_ = s.store.LogAccess("share-password-rate-limited", token, clientIP(r), r.UserAgent())
	writeError(w, http.StatusTooManyRequests, errors.New("share password attempts exceeded"))
	return false
}

func (s *Server) recordSharePasswordFailure(r *http.Request, token, password string, err error) {
	if strings.TrimSpace(password) == "" || !errors.Is(err, appstore.ErrInvalidSharePassword) {
		return
	}
	limit, parseErr := strconv.Atoi(s.store.SettingValue("sharePasswordLimitPerMinute", "5"))
	if parseErr != nil || limit < 1 {
		return
	}
	s.sharePasswords.record(sharePasswordAttemptKey(token, clientIP(r)), time.Now())
	_ = s.store.LogAccess("share-password-failed", token, clientIP(r), r.UserAgent())
}

func (s *Server) clearSharePasswordFailures(r *http.Request, token string) {
	s.sharePasswords.reset(sharePasswordAttemptKey(token, clientIP(r)))
}

func sharePasswordAttemptKey(token, ip string) string {
	return strings.TrimSpace(token) + ":" + strings.TrimSpace(ip)
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
	if !s.requireOperation(w, r, "directLinks") {
		return
	}
	var req struct {
		StorageKey string `json:"storageKey"`
		Path       string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	if !s.requireStoragePathAccess(w, r, storageKey, req.Path) {
		return
	}
	link, err := s.store.CreateSourceDirectLink(storageKey, req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (s *Server) batchCreateDirectLinks(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "directLinks") {
		return
	}
	var req struct {
		StorageKey string   `json:"storageKey"`
		Paths      []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	paths, err := normalizeBatchSharePaths(req.Paths)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	if !s.requireStoragePathAccess(w, r, storageKey, paths...) {
		return
	}
	links := make([]domain.DirectLink, 0, len(paths))
	for _, path := range paths {
		link, err := s.store.CreateSourceDirectLink(storageKey, path)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		links = append(links, link)
	}
	_ = s.store.LogAccess("batch-direct-link", strconv.Itoa(len(links)), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, links)
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

func (s *Server) listFavorites(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	favorites, err := s.store.Favorites(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, favorites)
}

func (s *Server) createFavorite(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var req struct {
		StorageKey string `json:"storageKey"`
		Path       string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	storageKey := storageKeyOrDefault(req.StorageKey)
	if !s.requireStoragePathAccess(w, r, storageKey, req.Path) {
		return
	}
	favorite, err := s.store.CreateFavorite(user.ID, storageKey, req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, favorite)
}

func (s *Server) deleteFavorite(w http.ResponseWriter, r *http.Request) {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.DeleteFavorite(user.ID, id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) openDirectLink(w http.ResponseWriter, r *http.Request) {
	if !s.requireOperation(w, r, "download") {
		return
	}
	link, err := s.store.ResolveDirectLink(r.PathValue("token"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	download, err := s.store.SourceDownload(link.StorageKey, link.Path, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer download.Reader.Close()
	downloadPath := link.StorageKey + ":" + link.Path
	if !s.enforceDownloadLimit(w, r, downloadPath) {
		return
	}
	_ = s.store.RecordDirectLinkAccess(link.ID)
	_ = s.store.LogAccess("direct", downloadPath, clientIP(r), r.UserAgent())
	http.ServeContent(w, r, download.Entry.Name, parseRFC3339(download.Entry.ModifiedAt), download.Reader)
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

func (s *Server) linkAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics, err := s.store.LinkAnalytics()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, analytics)
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

func (s *Server) listUserSessions(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sessions, err := s.store.SessionsForUser(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	current, hasCurrent := s.currentSession(r)
	for index := range sessions {
		sessions[index].Current = hasCurrent && sessions[index].ID == current.ID
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) revokeUserSession(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	current, hasCurrent := s.currentSession(r)
	session, err := s.store.RevokeSession(id, sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("session not found"))
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("session-revoke", session.Username+":"+strconv.FormatInt(session.ID, 10), clientIP(r), r.UserAgent())
	if hasCurrent && current.ID == sessionID {
		clearSessionCookie(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) revokeUserSessions(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	current, hasCurrent := s.currentSession(r)
	revoked, err := s.store.RevokeUserSessions(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("sessions-revoke", strconv.FormatInt(id, 10)+":"+strconv.FormatInt(revoked, 10), clientIP(r), r.UserAgent())
	if hasCurrent && current.UserID == id {
		clearSessionCookie(w)
	}
	writeJSON(w, http.StatusOK, map[string]int64{"revoked": revoked})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username           string              `json:"username"`
		Password           string              `json:"password"`
		Role               string              `json:"role"`
		Enabled            *bool               `json:"enabled"`
		StorageSourceKeys  []string            `json:"storageSourceKeys"`
		StorageSourceRoots map[string][]string `json:"storageSourceRoots"`
		DisabledOperations []string            `json:"disabledOperations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	user, err := s.store.CreateUserWithPolicyStatus(req.Username, req.Password, req.Role, req.StorageSourceKeys, req.StorageSourceRoots, req.DisabledOperations, enabled)
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
		Username           string              `json:"username"`
		Password           string              `json:"password"`
		Role               string              `json:"role"`
		Enabled            *bool               `json:"enabled"`
		StorageSourceKeys  []string            `json:"storageSourceKeys"`
		StorageSourceRoots map[string][]string `json:"storageSourceRoots"`
		DisabledOperations []string            `json:"disabledOperations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	enabled := true
	if req.Enabled == nil {
		existing, err := s.store.UserByID(id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		enabled = existing.Enabled
	} else {
		enabled = *req.Enabled
	}
	user, err := s.store.UpdateUserWithPolicyStatus(id, req.Username, req.Password, req.Role, req.StorageSourceKeys, req.StorageSourceRoots, req.DisabledOperations, enabled)
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
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if user.Role != "super_admin" {
		settings = map[string]string{
			"rootName":                settings["rootName"],
			"disabledOperations":      settings["disabledOperations"],
			"externalPreviewProvider": settings["externalPreviewProvider"],
			"externalPreviewBaseUrl":  settings["externalPreviewBaseUrl"],
			"externalPreviewTemplate": settings["externalPreviewTemplate"],
		}
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

func sameDirectory(from, to string) bool {
	fromDir := filepath.ToSlash(filepath.Dir(strings.TrimSpace(from)))
	toDir := filepath.ToSlash(filepath.Dir(strings.TrimSpace(to)))
	if fromDir == "." {
		fromDir = ""
	}
	if toDir == "." {
		toDir = ""
	}
	return fromDir == toDir
}

func parseRFC3339(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
