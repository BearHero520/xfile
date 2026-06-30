package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"path"
	"strings"

	"golang.org/x/net/webdav"
)

func (s *Server) webDAVOrSPA() http.Handler {
	spa := spaHandler(s.cfg.StaticDir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := s.store.Settings()
		if err == nil && settings["webdav"] == "enabled" {
			mountPath := normalizeDAVMountPath(settings["webdavMountPath"])
			if davRequestMatches(r.URL.Path, mountPath) {
				s.serveWebDAV(w, r, settings, mountPath)
				return
			}
		}
		spa.ServeHTTP(w, r)
	})
}

func (s *Server) serveWebDAV(w http.ResponseWriter, r *http.Request, settings map[string]string, mountPath string) {
	if !s.ipAllowed(r) {
		ip := clientIP(r)
		_ = s.store.LogAccess("webdav-ip-blocked", r.URL.Path, ip, r.UserAgent())
		writeError(w, http.StatusForbidden, errors.New("ip address is not allowed"))
		return
	}
	if settings["webdavAllowAnonymous"] != "enabled" && !webDAVCredentialsAllowed(r, settings) {
		w.Header().Set("WWW-Authenticate", `Basic realm="XFile WebDAV"`)
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	root, err := s.store.AdminSourceFilePath("local", "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	fileSystem := webdav.FileSystem(webdav.Dir(root))
	if settings["webdavReadOnly"] == "enabled" {
		fileSystem = readOnlyDAVFileSystem{fs: fileSystem}
	}
	handler := &webdav.Handler{
		Prefix:     mountPath,
		FileSystem: fileSystem,
		LockSystem: s.davLocks,
		Logger: func(r *http.Request, err error) {
			action := "webdav-" + strings.ToLower(r.Method)
			if err != nil {
				action += "-error"
			}
			_ = s.store.LogAccess(action, r.URL.Path, clientIP(r), r.UserAgent())
		},
	}
	handler.ServeHTTP(w, r)
}

func webDAVCredentialsAllowed(r *http.Request, settings map[string]string) bool {
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	expectedUsername := strings.TrimSpace(settings["webdavUsername"])
	expectedPassword := settings["webdavPassword"]
	if expectedUsername == "" || expectedPassword == "" {
		return false
	}
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(expectedUsername)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1
	return usernameMatch && passwordMatch
}

func davRequestMatches(requestPath, mountPath string) bool {
	cleanRequest := path.Clean("/" + strings.TrimPrefix(requestPath, "/"))
	if cleanRequest == mountPath {
		return true
	}
	return strings.HasPrefix(cleanRequest, strings.TrimRight(mountPath, "/")+"/")
}

func normalizeDAVMountPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "/dav"
	}
	value = "/" + strings.Trim(value, "/")
	clean := path.Clean(value)
	if clean == "." || clean == "/" {
		return "/dav"
	}
	return clean
}

type readOnlyDAVFileSystem struct {
	fs webdav.FileSystem
}

func (r readOnlyDAVFileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	return os.ErrPermission
}

func (r readOnlyDAVFileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0 {
		return nil, os.ErrPermission
	}
	return r.fs.OpenFile(ctx, name, flag, perm)
}

func (r readOnlyDAVFileSystem) RemoveAll(ctx context.Context, name string) error {
	return os.ErrPermission
}

func (r readOnlyDAVFileSystem) Rename(ctx context.Context, oldName, newName string) error {
	return os.ErrPermission
}

func (r readOnlyDAVFileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	return r.fs.Stat(ctx, name)
}
