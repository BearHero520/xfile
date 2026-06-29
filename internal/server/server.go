package server

import (
	"net/http"

	"xfile/internal/config"
	"xfile/internal/store"
)

type Server struct {
	cfg           config.Config
	store         *store.Store
	mux           *http.ServeMux
	sessionSecret string
}

func New(cfg config.Config, appStore *store.Store) *Server {
	s := &Server{cfg: cfg, store: appStore, mux: http.NewServeMux(), sessionSecret: newSessionSecret(cfg.SessionSecret)}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	return http.ListenAndServe(s.cfg.Addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("POST /api/auth/login", s.login)
	s.mux.HandleFunc("POST /api/auth/logout", s.logout)
	s.mux.HandleFunc("GET /api/auth/me", s.me)
	s.mux.HandleFunc("GET /api/dashboard", s.private(s.dashboard))
	s.mux.HandleFunc("GET /api/files", s.private(s.listFiles))
	s.mux.HandleFunc("POST /api/files/folders", s.private(s.createFolder))
	s.mux.HandleFunc("POST /api/files/upload", s.private(s.uploadFile))
	s.mux.HandleFunc("GET /api/files/download", s.private(s.downloadFile))
	s.mux.HandleFunc("DELETE /api/files", s.private(s.deleteFile))
	s.mux.HandleFunc("PATCH /api/files", s.private(s.moveFile))
	s.mux.HandleFunc("GET /api/shares", s.private(s.listShares))
	s.mux.HandleFunc("POST /api/shares", s.private(s.createShare))
	s.mux.HandleFunc("DELETE /api/shares/{id}", s.private(s.deleteShare))
	s.mux.HandleFunc("GET /api/direct-links", s.private(s.listDirectLinks))
	s.mux.HandleFunc("POST /api/direct-links", s.private(s.createDirectLink))
	s.mux.HandleFunc("PATCH /api/direct-links/{id}", s.private(s.updateDirectLink))
	s.mux.HandleFunc("DELETE /api/direct-links/{id}", s.private(s.deleteDirectLink))
	s.mux.HandleFunc("GET /api/logs", s.private(s.accessLogs))
	s.mux.HandleFunc("GET /api/settings", s.private(s.getSettings))
	s.mux.HandleFunc("PUT /api/settings", s.private(s.saveSettings))
	s.mux.HandleFunc("GET /s/{token}", s.openShare)
	s.mux.HandleFunc("GET /d/{token}", s.openDirectLink)
	s.mux.Handle("/", spaHandler(s.cfg.StaticDir))
}
