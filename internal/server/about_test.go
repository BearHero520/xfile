package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAboutReadsGitHubRepositoryDocumentsAndChanges(t *testing.T) {
	requests := 0
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/BearHero520/xfile":
			_, _ = w.Write([]byte(`{"name":"xfile","full_name":"BearHero520/xfile","description":"私有小云盘","html_url":"https://github.com/BearHero520/xfile","default_branch":"main","updated_at":"2026-07-14T03:05:33Z","pushed_at":"2026-07-14T03:05:28Z","stargazers_count":3,"forks_count":1,"owner":{"login":"BearHero520","avatar_url":"https://avatars.example/author.png","html_url":"https://github.com/BearHero520"}}`))
		case "/repos/BearHero520/xfile/releases":
			_, _ = w.Write([]byte(`[]`))
		case "/repos/BearHero520/xfile/commits":
			_, _ = w.Write([]byte(`[{"sha":"2fb41f8d96069a4","html_url":"https://github.com/BearHero520/xfile/commit/2fb41f8","author":{"login":"BearHero520"},"commit":{"message":"Add about page\n\nRead GitHub documents.","author":{"name":"Bear Hero","date":"2026-07-14T03:05:20Z"}}}]`))
		case "/repos/BearHero520/xfile/git/trees/main":
			_, _ = w.Write([]byte(`{"tree":[{"path":"README.md","type":"blob","size":64},{"path":"docs/api-v1.md","type":"blob","size":64},{"path":"docs/更新日志.md","type":"blob","size":128},{"path":"internal/private.md","type":"blob","size":64}]}`))
		case "/raw/BearHero520/xfile/main/docs/更新日志.md":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("# XFile 更新日志\n\nLatest release notes."))
		case "/raw/BearHero520/xfile/main/README.md":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("# XFile\n\nRepository readme."))
		case "/raw/BearHero520/xfile/main/docs/api-v1.md":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("# API v1\n\nGitHub documentation."))
		default:
			http.NotFound(w, r)
		}
	}))
	defer github.Close()

	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	s.githubAPIBase = github.URL
	s.githubRawBase = github.URL + "/raw"
	s.githubClient = github.Client()

	response := performJSONRequestAs(s, "root", http.MethodGet, "/api/v1/system/about", "")
	if response.Code != http.StatusOK {
		t.Fatalf("about status = %d: %s", response.Code, response.Body.String())
	}
	var data aboutResponse
	if err := json.NewDecoder(response.Body).Decode(&data); err != nil {
		t.Fatalf("decode about response: %v", err)
	}
	if data.Repository.Author.Login != "BearHero520" {
		t.Fatalf("author = %q", data.Repository.Author.Login)
	}
	if len(data.Documents) != 3 {
		t.Fatalf("documents = %#v", data.Documents)
	}
	if data.Documents[0].Path != "docs/更新日志.md" || !strings.Contains(data.Documents[0].Content, "Latest release notes") {
		t.Fatalf("first document is not changelog: %#v", data.Documents[0])
	}
	if data.Documents[1].Path != "README.md" || data.Documents[2].Path != "docs/api-v1.md" {
		t.Fatalf("unexpected document order: %#v", data.Documents)
	}
	if len(data.Changes) != 1 || data.Changes[0].Tag != "2fb41f8" {
		t.Fatalf("changes = %#v", data.Changes)
	}

	requestCount := requests
	response = performJSONRequestAs(s, "root", http.MethodGet, "/api/v1/system/about", "")
	if response.Code != http.StatusOK {
		t.Fatalf("cached about status = %d: %s", response.Code, response.Body.String())
	}
	if requests != requestCount {
		t.Fatalf("cached request contacted GitHub again: before=%d after=%d", requestCount, requests)
	}
}

func TestAboutRequiresAuthentication(t *testing.T) {
	s, _ := newAuthzTestServer(t)
	response := httptest.NewRecorder()
	s.mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/system/about", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("about without session status = %d", response.Code)
	}
}
