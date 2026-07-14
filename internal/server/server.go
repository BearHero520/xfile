package server

import (
	"net/http"
	"os"
	"time"

	"golang.org/x/net/webdav"
	"xfile/internal/config"
	"xfile/internal/store"
)

type Server struct {
	cfg            config.Config
	store          *store.Store
	mux            *http.ServeMux
	sessionSecret  string
	downloads      requestRateLimiter
	logins         requestRateLimiter
	sharePasswords requestRateLimiter
	captchas       captchaStore
	davLocks       webdav.LockSystem
	githubClient   *http.Client
	githubAPIBase  string
	githubRawBase  string
	githubToken    string
	about          aboutState
}

func New(cfg config.Config, appStore *store.Store) *Server {
	s := &Server{
		cfg:           cfg,
		store:         appStore,
		mux:           http.NewServeMux(),
		sessionSecret: newSessionSecret(cfg.SessionSecret),
		davLocks:      webdav.NewMemLS(),
		githubClient:  &http.Client{Timeout: 12 * time.Second},
		githubAPIBase: "https://api.github.com",
		githubRawBase: "https://raw.githubusercontent.com",
		githubToken:   os.Getenv("GITHUB_TOKEN"),
	}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	return http.ListenAndServe(s.cfg.Addr, s.mux)
}

func (s *Server) routes() {
	// XFile API v1. The public contract is resource-oriented and deliberately
	// independent from implementation-specific controller naming schemes.
	s.mux.HandleFunc("GET /api/v1/system/health", s.health)
	s.mux.HandleFunc("GET /api/v1/system/about", s.private(s.aboutPage))
	s.mux.HandleFunc("GET /api/v1/public/bootstrap", s.accessControlled(s.publicSite))
	s.mux.HandleFunc("GET /api/v1/public/announcements", s.accessControlled(s.publicAnnouncements))
	s.mux.HandleFunc("GET /api/v1/public/branding/logo", s.brandLogo)
	s.mux.HandleFunc("GET /api/v1/public/branding/favicon", s.brandFavicon)
	s.mux.HandleFunc("GET /api/v1/public/drives/{key}/entries", s.accessControlled(s.publicStorageFiles))
	s.mux.HandleFunc("GET /api/v1/public/drives/{key}/entries/details", s.accessControlled(s.publicEntryDetails))
	s.mux.HandleFunc("GET /api/v1/public/drives/{key}/content", s.publicLinkControlled(s.publicStorageDownload))
	s.mux.HandleFunc("POST /api/v1/public/drives/{key}/archives", s.publicLinkControlled(s.publicStorageArchive))
	s.mux.HandleFunc("GET /api/v1/public/shares/{token}", s.publicLinkControlled(s.publicShare))
	s.mux.HandleFunc("GET /api/v1/public/shares/{token}/content", s.publicLinkControlled(s.downloadShare))

	s.mux.HandleFunc("POST /api/v1/identity/bootstrap", s.accessControlled(s.setup))
	s.mux.HandleFunc("POST /api/v1/identity/session", s.accessControlled(s.login))
	s.mux.HandleFunc("DELETE /api/v1/identity/session", s.accessControlled(s.logout))
	s.mux.HandleFunc("GET /api/v1/identity/session", s.accessControlled(s.me))
	s.mux.HandleFunc("GET /api/v1/identity/challenge", s.accessControlled(s.captcha))

	s.mux.HandleFunc("GET /api/v1/workspace/overview", s.private(s.dashboard))
	s.mux.HandleFunc("GET /api/v1/workspace/entries", s.private(s.listFiles))
	s.mux.HandleFunc("GET /api/v1/workspace/entries/details", s.private(s.entryDetails))
	s.mux.HandleFunc("GET /api/v1/workspace/search", s.private(s.searchFiles))
	s.mux.HandleFunc("POST /api/v1/workspace/folders", s.private(s.createFolder))
	s.mux.HandleFunc("POST /api/v1/workspace/documents", s.private(s.createEmptyFile))
	s.mux.HandleFunc("POST /api/v1/workspace/uploads", s.private(s.uploadFile))
	s.mux.HandleFunc("PUT /api/v1/workspace/text-content", s.private(s.saveTextFile))
	s.mux.HandleFunc("PUT /api/v1/workspace/metadata", s.private(s.saveFileMetadata))
	s.mux.HandleFunc("GET /api/v1/workspace/content", s.private(s.downloadFile))
	s.mux.HandleFunc("POST /api/v1/workspace/archives", s.private(s.downloadArchive))
	s.mux.HandleFunc("DELETE /api/v1/workspace/entries", s.private(s.deleteFile))
	s.mux.HandleFunc("PATCH /api/v1/workspace/entries", s.private(s.moveFile))
	s.mux.HandleFunc("POST /api/v1/workspace/actions/move", s.private(s.batchMoveFiles))
	s.mux.HandleFunc("POST /api/v1/workspace/actions/copy", s.private(s.batchCopyFiles))

	s.mux.HandleFunc("GET /api/v1/collaboration/shares", s.private(s.listShares))
	s.mux.HandleFunc("POST /api/v1/collaboration/shares", s.private(s.createShare))
	s.mux.HandleFunc("POST /api/v1/collaboration/shares/batch", s.private(s.batchCreateShares))
	s.mux.HandleFunc("DELETE /api/v1/collaboration/shares/expired", s.private(s.deleteExpiredShares))
	s.mux.HandleFunc("PATCH /api/v1/collaboration/shares/{id}", s.private(s.updateShareLimits))
	s.mux.HandleFunc("DELETE /api/v1/collaboration/shares/{id}", s.private(s.deleteShare))
	s.mux.HandleFunc("GET /api/v1/delivery/links", s.private(s.listDirectLinks))
	s.mux.HandleFunc("POST /api/v1/delivery/links", s.private(s.createDirectLink))
	s.mux.HandleFunc("POST /api/v1/delivery/links/batch", s.private(s.batchCreateDirectLinks))
	s.mux.HandleFunc("PATCH /api/v1/delivery/links/{id}", s.private(s.updateDirectLink))
	s.mux.HandleFunc("DELETE /api/v1/delivery/links/{id}", s.private(s.deleteDirectLink))
	s.mux.HandleFunc("GET /api/v1/workspace/favorites", s.private(s.listFavorites))
	s.mux.HandleFunc("POST /api/v1/workspace/favorites", s.private(s.createFavorite))
	s.mux.HandleFunc("DELETE /api/v1/workspace/favorites/{id}", s.private(s.deleteFavorite))
	s.mux.HandleFunc("GET /api/v1/insights/links", s.private(s.linkAnalytics))

	s.mux.HandleFunc("GET /api/v1/audit/events", s.private(s.accessLogs))
	s.mux.HandleFunc("DELETE /api/v1/audit/events", s.superAdminOnly(s.deleteAccessLogs))
	s.mux.HandleFunc("GET /api/v1/admin/accounts", s.superAdminOnly(s.listUsers))
	s.mux.HandleFunc("POST /api/v1/admin/accounts", s.superAdminOnly(s.createUser))
	s.mux.HandleFunc("PATCH /api/v1/admin/accounts/{id}", s.superAdminOnly(s.updateUser))
	s.mux.HandleFunc("DELETE /api/v1/admin/accounts/{id}", s.superAdminOnly(s.deleteUser))
	s.mux.HandleFunc("GET /api/v1/admin/accounts/{id}/sessions", s.superAdminOnly(s.listUserSessions))
	s.mux.HandleFunc("DELETE /api/v1/admin/accounts/{id}/sessions", s.superAdminOnly(s.revokeUserSessions))
	s.mux.HandleFunc("DELETE /api/v1/admin/accounts/{id}/sessions/{sessionID}", s.superAdminOnly(s.revokeUserSession))
	s.mux.HandleFunc("GET /api/v1/admin/storage-nodes", s.private(s.storageSources))
	s.mux.HandleFunc("POST /api/v1/admin/storage-nodes", s.superAdminOnly(s.createStorageSource))
	s.mux.HandleFunc("PATCH /api/v1/admin/storage-nodes/{id}", s.superAdminOnly(s.updateStorageSource))
	s.mux.HandleFunc("DELETE /api/v1/admin/storage-nodes/{id}", s.superAdminOnly(s.deleteStorageSource))
	s.mux.HandleFunc("GET /api/v1/admin/theme", s.superAdminOnly(s.getThemeSettings))
	s.mux.HandleFunc("PUT /api/v1/admin/theme", s.superAdminOnly(s.saveThemeSettings))
	s.mux.HandleFunc("GET /api/v1/admin/announcements", s.superAdminOnly(s.listAnnouncements))
	s.mux.HandleFunc("POST /api/v1/admin/announcements", s.superAdminOnly(s.createAnnouncement))
	s.mux.HandleFunc("PATCH /api/v1/admin/announcements/{id}", s.superAdminOnly(s.updateAnnouncement))
	s.mux.HandleFunc("DELETE /api/v1/admin/announcements/{id}", s.superAdminOnly(s.deleteAnnouncement))
	s.mux.HandleFunc("GET /api/v1/preferences", s.private(s.getSettings))
	s.mux.HandleFunc("PUT /api/v1/preferences", s.superAdminOnly(s.saveSettings))

	s.mux.HandleFunc("GET /share/{token}", s.sharePage)
	s.mux.HandleFunc("GET /open/{token}", s.publicLinkControlled(s.openDirectLink))
	// Explicitly retire the pre-v1 endpoints instead of letting them fall
	// through to the SPA handler.
	s.mux.Handle("/api/", http.NotFoundHandler())
	s.mux.Handle("/api", http.NotFoundHandler())
	s.mux.Handle("/s/", http.NotFoundHandler())
	s.mux.Handle("/s", http.NotFoundHandler())
	s.mux.Handle("/d/", http.NotFoundHandler())
	s.mux.Handle("/d", http.NotFoundHandler())
	s.mux.Handle("/", s.webDAVOrSPA())
}
