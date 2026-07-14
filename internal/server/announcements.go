package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"xfile/internal/domain"
)

func (s *Server) publicAnnouncements(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.Announcements(true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) listAnnouncements(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.Announcements(false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createAnnouncement(w http.ResponseWriter, r *http.Request) {
	var input domain.AnnouncementInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.store.CreateAnnouncement(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("announcement-create", strconv.FormatInt(item.ID, 10), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updateAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var input domain.AnnouncementInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.store.UpdateAnnouncement(id, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("announcement not found"))
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = s.store.LogAccess("announcement-update", strconv.FormatInt(item.ID, 10), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.DeleteAnnouncement(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("announcement not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = s.store.LogAccess("announcement-delete", strconv.FormatInt(id, 10), clientIP(r), r.UserAgent())
	w.WriteHeader(http.StatusNoContent)
}
