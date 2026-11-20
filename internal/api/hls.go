package api

import (
	"net/http"
	"strings"
)

const playlistType = "application/vnd.apple.mpegurl"

// Apps that play through AVPlayer add prefetch=0 to the signed master link; it is left out of the signature.
func (s *Server) hlsMaster(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	prefetch := values.Get("prefetch") != "0"
	values.Del("prefetch")
	if err := s.live.Verify(values); err != nil {
		writeError(w, r, err)
		return
	}
	body, err := s.live.Master(r.Context(), values, prefetch)
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
	body, err := s.live.Variant(r.Context(), values, values.Get("p") != "0" && !appleMediaPlayer(r))
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

// Apps load playlists through AVPlayer as AppleCoreMedia; Safari sends its own agent and plays through hls.js.
func appleMediaPlayer(r *http.Request) bool {
	return strings.Contains(r.UserAgent(), "AppleCoreMedia")
}

func writePlaylist(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", playlistType)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(body)
}
