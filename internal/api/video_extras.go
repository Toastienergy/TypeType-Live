package api

import (
	"errors"
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
	page, err := withViewerToken(r, func(token string) (twitch.Page[twitch.VideoComment], error) {
		return s.twitch.VideoComments(r.Context(), token, id, offset, cursor)
	})
	respond(w, r, page, err)
}

func (s *Server) videoChapters(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !twitch.ValidID(id) {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid_id"})
		return
	}
	chapters, err := withViewerToken(r, func(token string) ([]twitch.VideoChapter, error) {
		return s.twitch.VideoChapters(r.Context(), token, id)
	})
	respond(w, r, chapters, err)
}

// withViewerToken reads with the viewer's linked account when there is one, and anonymously when Twitch rejects
// that token, so a revoked link never hides content anyone can see.
func withViewerToken[T any](r *http.Request, read func(token string) (T, error)) (T, error) {
	token := r.Header.Get(tokenHeader)
	value, err := read(token)
	if token != "" && errors.Is(err, twitch.ErrBadToken) {
		return read("")
	}
	return value, err
}
