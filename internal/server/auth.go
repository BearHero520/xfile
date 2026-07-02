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

const (
	sessionCookieName = "xfile_session"
	csrfHeaderName    = "X-CSRF-Token"
)

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
		Username      string `json:"username"`
		Password      string `json:"password"`
		CaptchaID     string `json:"captchaID"`
		CaptchaAnswer string `json:"captchaAnswer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.enforceLoginLimit(w, r, req.Username) {
		return
	}
	if !s.verifyLoginCaptcha(w, r, req.Username, req.CaptchaID, req.CaptchaAnswer) {
		return
	}
	user, err := s.store.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	expires := time.Now().Add(24 * time.Hour)
	sessionValue, err := s.createSessionValue(user, expires, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionValue,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": user, "csrfToken": s.csrfToken(sessionValue)})
}

func (s *Server) captcha(w http.ResponseWriter, r *http.Request) {
	if !s.loginCaptchaEnabled() {
		writeJSON(w, http.StatusOK, map[string]bool{"required": false})
		return
	}
	id, question, err := s.newCaptchaChallenge(time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"required": true,
		"id":       id,
		"question": question,
	})
}

func (s *Server) verifyLoginCaptcha(w http.ResponseWriter, r *http.Request, username, id, answer string) bool {
	if !s.loginCaptchaEnabled() {
		return true
	}
	if s.captchas.verify(id, answer, time.Now()) {
		return true
	}
	target := strings.TrimSpace(username)
	if target == "" {
		target = "login"
	}
	_ = s.store.LogAccess("login-captcha-failed", target, clientIP(r), r.UserAgent())
	writeError(w, http.StatusForbidden, errors.New("captcha is invalid"))
	return false
}

func (s *Server) enforceLoginLimit(w http.ResponseWriter, r *http.Request, username string) bool {
	limit, err := strconv.Atoi(s.store.SettingValue("loginLimitPerMinute", "5"))
	if err != nil || limit < 1 {
		return true
	}
	ip := clientIP(r)
	if s.logins.allow(ip, limit, time.Now()) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	target := strings.TrimSpace(username)
	if target == "" {
		target = "login"
	}
	_ = s.store.LogAccess("login-rate-limited", target, ip, r.UserAgent())
	writeError(w, http.StatusTooManyRequests, errors.New("login rate limit exceeded"))
	return false
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
	sessionValue, err := s.createSessionValue(user, expires, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionValue,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusCreated, map[string]any{"authenticated": true, "user": user, "csrfToken": s.csrfToken(sessionValue)})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	token, hasToken := s.sessionToken(r)
	if _, authenticated := s.verifySession(token); authenticated && !s.requireCSRF(w, r) {
		return
	}
	if hasToken {
		_ = s.store.RevokeSessionToken(token)
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.store.IsInitialized()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	session, authenticated := s.currentSession(r)
	username := ""
	var user domain.User
	if authenticated {
		username = session.Username
		user, err = s.store.UserByID(session.UserID)
		if err != nil {
			username = ""
			authenticated = false
		}
		if !user.Enabled {
			username = ""
			authenticated = false
		}
	}
	response := map[string]any{
		"initialized":     initialized,
		"authenticated":   authenticated,
		"captchaRequired": initialized && !authenticated && s.loginCaptchaEnabled(),
		"username":        username,
		"sessionSeconds":  int((24 * time.Hour).Seconds()),
	}
	if authenticated {
		session.Current = true
		response["user"] = user
		response["session"] = session
		if token, ok := s.csrfTokenForRequest(r); ok {
			response["csrfToken"] = token
		}
	}
	writeJSON(w, http.StatusOK, response)
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
		if !s.requireCSRF(w, r) {
			return
		}
		next(w, r)
	}
}

func (s *Server) superAdminOnly(next http.HandlerFunc) http.HandlerFunc {
	return s.private(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.currentUser(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		if user.Role != "super_admin" {
			_ = s.store.LogAccess("admin-role-blocked", r.URL.Path, clientIP(r), r.UserAgent())
			writeError(w, http.StatusForbidden, errors.New("super admin role is required"))
			return
		}
		next(w, r)
	})
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	_, err := s.currentUser(r)
	return err == nil
}

func (s *Server) currentUser(r *http.Request) (domain.User, error) {
	session, ok := s.currentSession(r)
	if !ok {
		return domain.User{}, errors.New("authentication required")
	}
	user, err := s.store.UserByID(session.UserID)
	if err != nil {
		return domain.User{}, err
	}
	if !user.Enabled {
		return domain.User{}, errors.New("user is disabled")
	}
	return user, nil
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
	session, ok := s.currentSession(r)
	if !ok {
		return "", false
	}
	return session.Username, true
}

func (s *Server) signSession(username string, expires time.Time) string {
	user, err := s.store.UserByUsername(username)
	if err != nil {
		return ""
	}
	value, err := s.createSessionValue(user, expires, nil)
	if err != nil {
		return ""
	}
	return value
}

func (s *Server) verifySession(value string) (string, bool) {
	session, err := s.store.SessionByToken(value)
	if err != nil {
		return "", false
	}
	return session.Username, true
}

func (s *Server) currentSession(r *http.Request) (domain.Session, bool) {
	token, ok := s.sessionToken(r)
	if !ok {
		return domain.Session{}, false
	}
	session, err := s.store.SessionByToken(token)
	if err != nil {
		return domain.Session{}, false
	}
	return session, true
}

func (s *Server) sessionToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	return cookie.Value, true
}

func (s *Server) createSessionValue(user domain.User, expires time.Time, r *http.Request) (string, error) {
	ip := ""
	userAgent := ""
	if r != nil {
		ip = clientIP(r)
		userAgent = r.UserAgent()
	}
	_, token, err := s.store.CreateSession(user, ip, userAgent, expires)
	return token, err
}

func (s *Server) csrfTokenForRequest(r *http.Request) (string, bool) {
	token, ok := s.sessionToken(r)
	if !ok {
		return "", false
	}
	if _, ok := s.verifySession(token); !ok {
		return "", false
	}
	return s.csrfToken(token), true
}

func (s *Server) csrfToken(sessionValue string) string {
	mac := hmac.New(sha256.New, []byte(s.sessionSecret))
	mac.Write([]byte("csrf:" + sessionValue))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) requireCSRF(w http.ResponseWriter, r *http.Request) bool {
	if !csrfRequired(r.Method) {
		return true
	}
	expected, ok := s.csrfTokenForRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, errors.New("authentication required"))
		return false
	}
	actual := strings.TrimSpace(r.Header.Get(csrfHeaderName))
	if actual != "" && subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1 {
		return true
	}
	_ = s.store.LogAccess("csrf-blocked", r.URL.Path, clientIP(r), r.UserAgent())
	writeError(w, http.StatusForbidden, errors.New("csrf token is invalid"))
	return false
}

func csrfRequired(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
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
