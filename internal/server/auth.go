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
	"strconv"
	"strings"
	"time"
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
