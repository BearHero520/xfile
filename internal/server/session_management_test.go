package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"xfile/internal/domain"
)

func TestUserSessionsCanBeListedAndRevoked(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	user, err := appStore.CreateSuperAdmin("root", "password123")
	if err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	currentToken := s.signSession("root", time.Now().Add(time.Hour))
	otherToken := s.signSession("root", time.Now().Add(time.Hour))
	otherSession, err := appStore.SessionByToken(otherToken)
	if err != nil {
		t.Fatalf("resolve other session: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/"+strconv.FormatInt(user.ID, 10)+"/sessions", nil)
	listReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: currentToken})
	listRes := httptest.NewRecorder()
	s.mux.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list sessions failed: %d %s", listRes.Code, listRes.Body.String())
	}
	var sessions []domain.Session
	if err := json.NewDecoder(listRes.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %#v", sessions)
	}
	var currentCount int
	for _, session := range sessions {
		if session.Current {
			currentCount++
		}
	}
	if currentCount != 1 {
		t.Fatalf("expected exactly one current session, got %d: %#v", currentCount, sessions)
	}

	target := "/api/v1/admin/accounts/" + strconv.FormatInt(user.ID, 10) + "/sessions/" + strconv.FormatInt(otherSession.ID, 10)
	revokeReq := httptest.NewRequest(http.MethodDelete, target, nil)
	revokeReq.Header.Set(csrfHeaderName, s.csrfToken(currentToken))
	revokeReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: currentToken})
	revokeRes := httptest.NewRecorder()
	s.mux.ServeHTTP(revokeRes, revokeReq)
	if revokeRes.Code != http.StatusNoContent {
		t.Fatalf("revoke session failed: %d %s", revokeRes.Code, revokeRes.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/preferences", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: otherToken})
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session should be unauthorized, got %d: %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/preferences", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: currentToken})
	res = httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("current session should remain active, got %d: %s", res.Code, res.Body.String())
	}
}

func TestRevokingAllUserSessionsForcesLogout(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	user, err := appStore.CreateSuperAdmin("root", "password123")
	if err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	sessionToken := s.signSession("root", time.Now().Add(time.Hour))

	target := "/api/v1/admin/accounts/" + strconv.FormatInt(user.ID, 10) + "/sessions"
	req := httptest.NewRequest(http.MethodDelete, target, nil)
	req.Header.Set(csrfHeaderName, s.csrfToken(sessionToken))
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("revoke all sessions failed: %d %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/preferences", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
	res = httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("revoked current session should be unauthorized, got %d: %s", res.Code, res.Body.String())
	}
}
