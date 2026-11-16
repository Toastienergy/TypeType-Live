package api

import (
	"context"
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
	page, err := withViewerToken(r.Context(), r.Header.Get(tokenHeader),
		func(ctx context.Context, token string) (twitch.Page[twitch.VideoComment], error) {
			return s.twitch.VideoComments(ctx, token, id, offset, cursor)
		})
	respond(w, r, page, err)
}

func (s *Server) videoChapters(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !twitch.ValidID(id) {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid_id"})
		return
	}
	chapters, err := withViewerToken(r.Context(), r.Header.Get(tokenHeader),
		func(ctx context.Context, token string) ([]twitch.VideoChapter, error) {
			return s.twitch.VideoChapters(ctx, token, id)
		})
	respond(w, r, chapters, err)
}

// withViewerToken reads with the viewer's linked account when there is one, and anonymously when Twitch rejects
// that token, so a revoked link never hides content anyone can see.
func withViewerToken[T any](ctx context.Context, token string, read func(context.Context, string) (T, error)) (T, error) {
	value, err := read(ctx, token)
	if token != "" && errors.Is(err, twitch.ErrBadToken) {
		return read(ctx, "")
	}
	return value, err
}
