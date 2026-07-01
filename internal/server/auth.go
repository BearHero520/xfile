package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"xfile/internal/domain"
)

const sessionCookieName = "xfile_session"

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.store.IsInitialized()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !initialized {
		writeError(w, http.StatusConflict, errors.New("system is not initialized"))
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.store.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	expires := time.Now().Add(24 * time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.signSession(user.Username, expires),
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": user})
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.store.CreateSuperAdmin(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	expires := time.Now().Add(24 * time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.signSession(user.Username, expires),
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusCreated, map[string]any{"authenticated": true, "user": user})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.store.IsInitialized()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	username, authenticated := s.sessionUsername(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"initialized":    initialized,
		"authenticated":  authenticated,
		"username":       username,
		"sessionSeconds": int((24 * time.Hour).Seconds()),
	})
}

func (s *Server) private(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.ipAllowed(r) {
			ip := clientIP(r)
			_ = s.store.LogAccess("ip-blocked", r.URL.Path, ip, r.UserAgent())
			writeError(w, http.StatusForbidden, errors.New("ip address is not allowed"))
			return
		}
		if !s.isAuthenticated(r) {
			writeError(w, http.StatusUnauthorized, errors.New("authentication required"))
			return
		}
		next(w, r)
	}
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	_, ok := s.sessionUsername(r)
	return ok
}

func (s *Server) currentUser(r *http.Request) (domain.User, error) {
	username, ok := s.sessionUsername(r)
	if !ok {
		return domain.User{}, errors.New("authentication required")
	}
	return s.store.UserByUsername(username)
}

func (s *Server) requireStorageAccess(w http.ResponseWriter, r *http.Request, storageKey string) bool {
	return s.requireStorageListAccess(w, r, storageKey, "")
}

func (s *Server) requireStorageListAccess(w http.ResponseWriter, r *http.Request, storageKey, rel string) bool {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return false
	}
	storageKey = storageKeyOrDefault(storageKey)
	if s.store.UserCanListStoragePath(user, storageKey, rel) {
		return true
	}
	_ = s.store.LogAccess("storage-access-blocked", storageKey+":"+rel, clientIP(r), r.UserAgent())
	writeError(w, http.StatusForbidden, errors.New("storage path is not assigned to user"))
	return false
}

func (s *Server) requireStoragePathAccess(w http.ResponseWriter, r *http.Request, storageKey string, paths ...string) bool {
	user, err := s.currentUser(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return false
	}
	storageKey = storageKeyOrDefault(storageKey)
	for _, path := range paths {
		if !s.store.UserCanAccessStoragePath(user, storageKey, path) {
			_ = s.store.LogAccess("storage-access-blocked", storageKey+":"+path, clientIP(r), r.UserAgent())
			writeError(w, http.StatusForbidden, errors.New("storage path is not assigned to user"))
			return false
		}
	}
	return true
}

func (s *Server) filterStorageFilesForUser(r *http.Request, storageKey string, files []domain.FileEntry, includeAncestors bool) []domain.FileEntry {
	user, err := s.currentUser(r)
	if err != nil {
		return files
	}
	return s.store.FilterStorageFilesForUser(user, storageKeyOrDefault(storageKey), files, includeAncestors)
}

func requestTargetPath(parts ...string) string {
	joined := filepath.ToSlash(filepath.Join(parts...))
	if joined == "." {
		return ""
	}
	return strings.TrimPrefix(joined, "/")
}

func (s *Server) sessionUsername(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	return s.verifySession(cookie.Value)
}

func (s *Server) signSession(username string, expires time.Time) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(username)) + "." + strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(s.sessionSecret))
	mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature
}

func (s *Server) verifySession(value string) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return "", false
	}
	payload := parts[0] + "." + parts[1]
	signature := parts[2]
	expiresUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().After(time.Unix(expiresUnix, 0)) {
		return "", false
	}
	mac := hmac.New(sha256.New, []byte(s.sessionSecret))
	mac.Write([]byte(payload))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) != 1 {
		return "", false
	}
	username, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	return string(username), true
}

func newSessionSecret(configured string) string {
	if strings.TrimSpace(configured) != "" {
		return configured
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buf)
}
