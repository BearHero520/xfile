package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	githubRepositoryOwner = "BearHero520"
	githubRepositoryName  = "xfile"
	aboutCacheTTL         = 10 * time.Minute
	maxGitHubDocuments    = 16
	maxGitHubDocumentSize = 512 << 10
)

type aboutState struct {
	mu        sync.Mutex
	data      aboutResponse
	expiresAt time.Time
}

type aboutResponse struct {
	Repository aboutRepository `json:"repository"`
	Documents  []aboutDocument `json:"documents"`
	Changes    []aboutChange   `json:"changes"`
	Warnings   []string        `json:"warnings,omitempty"`
	FetchedAt  string          `json:"fetchedAt"`
	Stale      bool            `json:"stale,omitempty"`
}

type aboutRepository struct {
	Name          string      `json:"name"`
	FullName      string      `json:"fullName"`
	Description   string      `json:"description"`
	HTMLURL       string      `json:"htmlUrl"`
	DefaultBranch string      `json:"defaultBranch"`
	UpdatedAt     string      `json:"updatedAt"`
	PushedAt      string      `json:"pushedAt"`
	Stars         int         `json:"stars"`
	Forks         int         `json:"forks"`
	Author        aboutAuthor `json:"author"`
}

type aboutAuthor struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatarUrl"`
	HTMLURL   string `json:"htmlUrl"`
}

type aboutDocument struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	HTMLURL string `json:"htmlUrl"`
	Content string `json:"content"`
}

type aboutChange struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Body        string `json:"body,omitempty"`
	Tag         string `json:"tag,omitempty"`
	Author      string `json:"author,omitempty"`
	PublishedAt string `json:"publishedAt"`
	HTMLURL     string `json:"htmlUrl"`
}

type githubRepository struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	UpdatedAt     string `json:"updated_at"`
	PushedAt      string `json:"pushed_at"`
	Stars         int    `json:"stargazers_count"`
	Forks         int    `json:"forks_count"`
	Owner         struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
		HTMLURL   string `json:"html_url"`
	} `json:"owner"`
}

type githubRelease struct {
	ID          int64  `json:"id"`
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
}

type githubCommit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Author  *struct {
		Login string `json:"login"`
	} `json:"author"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

type githubTree struct {
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
		Size int64  `json:"size"`
	} `json:"tree"`
}

func (s *Server) aboutPage(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	s.about.mu.Lock()
	defer s.about.mu.Unlock()

	forceRefresh := r.URL.Query().Get("refresh") == "1"
	if !forceRefresh && s.about.data.Repository.FullName != "" && now.Before(s.about.expiresAt) {
		writeJSON(w, http.StatusOK, s.about.data)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	data, err := s.fetchAbout(ctx)
	if err != nil {
		if s.about.data.Repository.FullName != "" {
			stale := s.about.data
			stale.Stale = true
			stale.Warnings = append(append([]string{}, stale.Warnings...), "GitHub 暂时无法访问，当前显示上次成功读取的内容。")
			writeJSON(w, http.StatusOK, stale)
			return
		}
		writeError(w, http.StatusBadGateway, err)
		return
	}

	s.about.data = data
	s.about.expiresAt = now.Add(aboutCacheTTL)
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) fetchAbout(ctx context.Context) (aboutResponse, error) {
	var repository githubRepository
	if err := s.githubJSON(ctx, "/repos/"+githubRepositoryOwner+"/"+githubRepositoryName, &repository); err != nil {
		return aboutResponse{}, fmt.Errorf("读取 GitHub 项目信息失败: %w", err)
	}
	if repository.FullName == "" || repository.DefaultBranch == "" {
		return aboutResponse{}, errors.New("GitHub 返回的项目信息不完整")
	}

	data := aboutResponse{
		Repository: aboutRepository{
			Name:          repository.Name,
			FullName:      repository.FullName,
			Description:   repository.Description,
			HTMLURL:       repository.HTMLURL,
			DefaultBranch: repository.DefaultBranch,
			UpdatedAt:     repository.UpdatedAt,
			PushedAt:      repository.PushedAt,
			Stars:         repository.Stars,
			Forks:         repository.Forks,
			Author: aboutAuthor{
				Login:     repository.Owner.Login,
				AvatarURL: repository.Owner.AvatarURL,
				HTMLURL:   repository.Owner.HTMLURL,
			},
		},
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}

	var releases []githubRelease
	if err := s.githubJSON(ctx, "/repos/"+repository.FullName+"/releases?per_page=8", &releases); err != nil {
		data.Warnings = append(data.Warnings, "GitHub Releases 暂时无法读取。")
	} else {
		for _, release := range releases {
			title := strings.TrimSpace(release.Name)
			if title == "" {
				title = release.TagName
			}
			data.Changes = append(data.Changes, aboutChange{
				ID:          fmt.Sprintf("release-%d", release.ID),
				Type:        "release",
				Title:       title,
				Body:        strings.TrimSpace(release.Body),
				Tag:         release.TagName,
				PublishedAt: release.PublishedAt,
				HTMLURL:     release.HTMLURL,
			})
		}
	}

	var commits []githubCommit
	if err := s.githubJSON(ctx, "/repos/"+repository.FullName+"/commits?per_page=10", &commits); err != nil {
		data.Warnings = append(data.Warnings, "最近提交记录暂时无法读取。")
	} else {
		for _, commit := range commits {
			title, body := splitCommitMessage(commit.Commit.Message)
			author := commit.Commit.Author.Name
			if commit.Author != nil && commit.Author.Login != "" {
				author = commit.Author.Login
			}
			id := commit.SHA
			if len(id) > 7 {
				id = id[:7]
			}
			data.Changes = append(data.Changes, aboutChange{
				ID:          "commit-" + commit.SHA,
				Type:        "commit",
				Title:       title,
				Body:        body,
				Tag:         id,
				Author:      author,
				PublishedAt: commit.Commit.Author.Date,
				HTMLURL:     commit.HTMLURL,
			})
		}
	}

	var tree githubTree
	treePath := "/repos/" + repository.FullName + "/git/trees/" + url.PathEscape(repository.DefaultBranch) + "?recursive=1"
	if err := s.githubJSON(ctx, treePath, &tree); err != nil {
		data.Warnings = append(data.Warnings, "GitHub 文档目录暂时无法读取。")
		return data, nil
	}

	type documentCandidate struct {
		path string
		size int64
	}
	var candidates []documentCandidate
	for _, entry := range tree.Tree {
		if entry.Type != "blob" || !isGitHubDocument(entry.Path) || entry.Size > maxGitHubDocumentSize {
			continue
		}
		candidates = append(candidates, documentCandidate{path: entry.Path, size: entry.Size})
	}
	sort.Slice(candidates, func(i, j int) bool {
		leftPriority := githubDocumentPriority(candidates[i].path)
		rightPriority := githubDocumentPriority(candidates[j].path)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		leftRoot := !strings.Contains(candidates[i].path, "/")
		rightRoot := !strings.Contains(candidates[j].path, "/")
		if leftRoot != rightRoot {
			return leftRoot
		}
		return candidates[i].path < candidates[j].path
	})
	if len(candidates) > maxGitHubDocuments {
		candidates = candidates[:maxGitHubDocuments]
	}

	for _, candidate := range candidates {
		content, err := s.githubDocument(ctx, repository.FullName, repository.DefaultBranch, candidate.path)
		if err != nil {
			data.Warnings = append(data.Warnings, "文档 "+candidate.path+" 暂时无法读取。")
			continue
		}
		data.Documents = append(data.Documents, aboutDocument{
			Name:    path.Base(candidate.path),
			Path:    candidate.path,
			Title:   markdownTitle(content, candidate.path),
			HTMLURL: repository.HTMLURL + "/blob/" + url.PathEscape(repository.DefaultBranch) + "/" + escapeURLPath(candidate.path),
			Content: content,
		})
	}

	return data, nil
}

func (s *Server) githubJSON(ctx context.Context, requestPath string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.githubAPIBase, "/")+requestPath, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "XFile-About-Page")
	if token := strings.TrimSpace(s.githubToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := s.githubClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("GitHub HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(message)))
	}
	return json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(target)
}

func (s *Server) githubDocument(ctx context.Context, fullName, branch, documentPath string) (string, error) {
	documentURL := strings.TrimRight(s.githubRawBase, "/") + "/" + escapeURLPath(fullName) + "/" + url.PathEscape(branch) + "/" + escapeURLPath(documentPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, documentURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "XFile-About-Page")
	res, err := s.githubClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("GitHub raw HTTP %d", res.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(res.Body, maxGitHubDocumentSize+1))
	if err != nil {
		return "", err
	}
	if len(content) > maxGitHubDocumentSize {
		return "", errors.New("GitHub document is too large")
	}
	return string(content), nil
}

func isGitHubDocument(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md") && (strings.HasPrefix(lower, "docs/") || lower == "readme.md" || lower == "changelog.md")
}

func githubDocumentPriority(name string) int {
	normalized := strings.ToLower(strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "./"))
	switch normalized {
	case "docs/更新日志.md":
		return 0
	case "docs/changelog.md", "changelog.md":
		return 1
	case "readme.md":
		return 2
	default:
		return 3
	}
}

func splitCommitMessage(message string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(message), "\n", 2)
	if len(parts) == 0 {
		return "未命名更新", ""
	}
	body := ""
	if len(parts) == 2 {
		body = strings.TrimSpace(parts[1])
	}
	return parts[0], body
}

func markdownTitle(content, filename string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	base := strings.TrimSuffix(path.Base(filename), path.Ext(filename))
	return strings.ReplaceAll(base, "-", " ")
}

func escapeURLPath(value string) string {
	parts := strings.Split(strings.ReplaceAll(value, "\\", "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
