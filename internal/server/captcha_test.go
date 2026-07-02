package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginCaptcha(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	if err := appStore.SaveSettings(map[string]string{"loginCaptcha": "enabled", "loginLimitPerMinute": "0"}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	missing := performLoginRequest(s, "198.51.100.20", `{"username":"root","password":"password123"}`)
	if missing.Code != http.StatusForbidden {
		t.Fatalf("login without captcha should be forbidden, got %d: %s", missing.Code, missing.Body.String())
	}

	wrongChallenge := requestCaptcha(t, s)
	wrongBody := fmt.Sprintf(`{"username":"root","password":"password123","captchaID":%q,"captchaAnswer":"0"}`, wrongChallenge.ID)
	wrong := performLoginRequest(s, "198.51.100.20", wrongBody)
	if wrong.Code != http.StatusForbidden {
		t.Fatalf("login with wrong captcha should be forbidden, got %d: %s", wrong.Code, wrong.Body.String())
	}

	challenge := requestCaptcha(t, s)
	answer := solveCaptchaQuestion(t, challenge.Question)
	body := fmt.Sprintf(`{"username":"root","password":"password123","captchaID":%q,"captchaAnswer":%q}`, challenge.ID, answer)
	ok := performLoginRequest(s, "198.51.100.20", body)
	if ok.Code != http.StatusOK {
		t.Fatalf("login with captcha should succeed, got %d: %s", ok.Code, ok.Body.String())
	}
}

type captchaResponse struct {
	Required bool   `json:"required"`
	ID       string `json:"id"`
	Question string `json:"question"`
}

func requestCaptcha(t *testing.T, s *Server) captchaResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/captcha", nil)
	res := httptest.NewRecorder()
	s.mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("captcha request failed: %d %s", res.Code, res.Body.String())
	}
	var body captchaResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode captcha response: %v", err)
	}
	if !body.Required || body.ID == "" || body.Question == "" {
		t.Fatalf("unexpected captcha response: %#v", body)
	}
	return body
}

func solveCaptchaQuestion(t *testing.T, question string) string {
	t.Helper()
	var left, right int
	if _, err := fmt.Sscanf(strings.TrimSpace(question), "%d + %d = ?", &left, &right); err != nil {
		t.Fatalf("parse captcha question %q: %v", question, err)
	}
	return fmt.Sprintf("%d", left+right)
}
