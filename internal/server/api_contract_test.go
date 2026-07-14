package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestV1ContractAndLegacyRoutes(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}

	if response := performJSONRequestAs(s, "root", http.MethodGet, "/api/v1/workspace/overview", ""); response.Code != http.StatusOK {
		t.Fatalf("v1 overview route returned %d: %s", response.Code, response.Body.String())
	}

	legacy := []string{
		"/api/dashboard",
		"/api/files",
		"/api/auth/me",
		"/api/storage-sources",
		"/s/legacy-token",
		"/d/legacy-token",
	}
	for _, target := range legacy {
		t.Run(target, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			s.mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("legacy route %s should be retired, got %d", target, recorder.Code)
			}
		})
	}
}
