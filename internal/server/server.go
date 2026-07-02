package server

import (
	"net/http"

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
}

func New(cfg config.Config, appStore *store.Store) *Server {
	s := &Server{
		cfg:           cfg,
		store:         appStore,
		mux:           http.NewServeMux(),
		sessionSecret: newSessionSecret(cfg.SessionSecret),
		davLocks:      webdav.NewMemLS(),
	}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	return http.ListenAndServe(s.cfg.Addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("GET /api/public/site", s.accessControlled(s.publicSite))
	s.mux.HandleFunc("GET /api/public/storage/{key}/files", s.accessControlled(s.publicStorageFiles))
	s.mux.HandleFunc("GET /api/public/storage/{key}/download", s.publicLinkControlled(s.publicStorageDownload))
	s.mux.HandleFunc("GET /api/public/shares/{token}", s.publicLinkControlled(s.publicShare))
	s.mux.HandleFunc("GET /api/public/shares/{token}/download", s.publicLinkControlled(s.downloadShare))
	s.mux.HandleFunc("POST /api/auth/setup", s.accessControlled(s.setup))
	s.mux.HandleFunc("POST /api/auth/login", s.accessControlled(s.login))
	s.mux.HandleFunc("POST /api/auth/logout", s.accessControlled(s.logout))
	s.mux.HandleFunc("GET /api/auth/me", s.accessControlled(s.me))
	s.mux.HandleFunc("GET /api/auth/captcha", s.accessControlled(s.captcha))
	s.mux.HandleFunc("GET /api/dashboard", s.private(s.dashboard))
	s.mux.HandleFunc("GET /api/storage-sources", s.private(s.storageSources))
	s.mux.HandleFunc("POST /api/storage-sources", s.superAdminOnly(s.createStorageSource))
	s.mux.HandleFunc("PATCH /api/storage-sources/{id}", s.superAdminOnly(s.updateStorageSource))
	s.mux.HandleFunc("DELETE /api/storage-sources/{id}", s.superAdminOnly(s.deleteStorageSource))
	s.mux.HandleFunc("GET /api/files", s.private(s.listFiles))
	s.mux.HandleFunc("GET /api/files/search", s.private(s.searchFiles))
	s.mux.HandleFunc("POST /api/files/folders", s.private(s.createFolder))
	s.mux.HandleFunc("POST /api/files/empty", s.private(s.createEmptyFile))
	s.mux.HandleFunc("POST /api/files/upload", s.private(s.uploadFile))
	s.mux.HandleFunc("PUT /api/files/text", s.private(s.saveTextFile))
	s.mux.HandleFunc("PUT /api/files/metadata", s.private(s.saveFileMetadata))
	s.mux.HandleFunc("GET /api/files/download", s.private(s.downloadFile))
	s.mux.HandleFunc("POST /api/files/archive", s.private(s.downloadArchive))
	s.mux.HandleFunc("DELETE /api/files", s.private(s.deleteFile))
	s.mux.HandleFunc("PATCH /api/files", s.private(s.moveFile))
	s.mux.HandleFunc("PATCH /api/files/batch/move", s.private(s.batchMoveFiles))
	s.mux.HandleFunc("PATCH /api/files/batch/copy", s.private(s.batchCopyFiles))
	s.mux.HandleFunc("GET /api/shares", s.private(s.listShares))
	s.mux.HandleFunc("POST /api/shares", s.private(s.createShare))
	s.mux.HandleFunc("POST /api/shares/batch", s.private(s.batchCreateShares))
	s.mux.HandleFunc("DELETE /api/shares/expired", s.private(s.deleteExpiredShares))
	s.mux.HandleFunc("DELETE /api/shares/{id}", s.private(s.deleteShare))
	s.mux.HandleFunc("GET /api/direct-links", s.private(s.listDirectLinks))
	s.mux.HandleFunc("POST /api/direct-links", s.private(s.createDirectLink))
	s.mux.HandleFunc("POST /api/direct-links/batch", s.private(s.batchCreateDirectLinks))
	s.mux.HandleFunc("PATCH /api/direct-links/{id}", s.private(s.updateDirectLink))
	s.mux.HandleFunc("DELETE /api/direct-links/{id}", s.private(s.deleteDirectLink))
	s.mux.HandleFunc("GET /api/analytics/links", s.private(s.linkAnalytics))
	s.mux.HandleFunc("GET /api/logs", s.private(s.accessLogs))
	s.mux.HandleFunc("DELETE /api/logs", s.superAdminOnly(s.deleteAccessLogs))
	s.mux.HandleFunc("GET /api/users", s.superAdminOnly(s.listUsers))
	s.mux.HandleFunc("POST /api/users", s.superAdminOnly(s.createUser))
	s.mux.HandleFunc("PATCH /api/users/{id}", s.superAdminOnly(s.updateUser))
	s.mux.HandleFunc("DELETE /api/users/{id}", s.superAdminOnly(s.deleteUser))
	s.mux.HandleFunc("GET /api/users/{id}/sessions", s.superAdminOnly(s.listUserSessions))
	s.mux.HandleFunc("DELETE /api/users/{id}/sessions", s.superAdminOnly(s.revokeUserSessions))
	s.mux.HandleFunc("DELETE /api/users/{id}/sessions/{sessionID}", s.superAdminOnly(s.revokeUserSession))
	s.mux.HandleFunc("GET /api/settings", s.private(s.getSettings))
	s.mux.HandleFunc("PUT /api/settings", s.superAdminOnly(s.saveSettings))
	s.mux.HandleFunc("GET /s/{token}", s.sharePage)
	s.mux.HandleFunc("GET /d/{token}", s.publicLinkControlled(s.openDirectLink))
	s.mux.Handle("/", s.webDAVOrSPA())
}
