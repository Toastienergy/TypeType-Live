package api

import (
	"net/http"
)

const playlistType = "application/vnd.apple.mpegurl"

func (s *Server) hlsMaster(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	if err := s.live.Verify(values); err != nil {
		writeError(w, r, err)
		return
	}
	body, err := s.live.Master(r.Context(), values)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writePlaylist(w, body)
}

func (s *Server) hlsVariant(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	if err := s.live.Verify(values); err != nil {
		writeError(w, r, err)
		return
	}
	body, err := s.live.Variant(r.Context(), values)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writePlaylist(w, body)
}

func (s *Server) hlsSegment(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	if err := s.live.Verify(values); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.live.ServeSegment(w, r, values); err != nil {
		writeError(w, r, err)
	}
}

func writePlaylist(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", playlistType)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(body)
}
