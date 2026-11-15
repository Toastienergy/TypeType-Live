package api

import (
	"net/http"

	"github.com/TypeType-Video/TypeType-Live/twitch"
)

const maxCommentCursor = 512

func (s *Server) videoComments(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cursor := r.URL.Query().Get("cursor")
	if !twitch.ValidID(id) || len(cursor) > maxCommentCursor {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid_request"})
		return
	}
	offset := twitch.ParseCommentOffset(r.URL.Query().Get("offset"))
	page, err := s.twitch.VideoComments(r.Context(), id, offset, cursor)
	respond(w, r, page, err)
}

func (s *Server) videoChapters(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !twitch.ValidID(id) {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid_id"})
		return
	}
	chapters, err := s.twitch.VideoChapters(r.Context(), id)
	respond(w, r, chapters, err)
}
