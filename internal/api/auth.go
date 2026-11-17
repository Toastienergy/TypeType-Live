package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TypeType-Video/TypeType-Live/twitch"
)

type pollRequest struct {
	DeviceCode   string `json:"deviceCode"`
	RefreshToken string `json:"refreshToken"`
	Token        string `json:"token"`
}

type pollBody struct {
	Status string        `json:"status"`
	Token  *twitch.Token `json:"token,omitempty"`
	User   *twitch.User  `json:"user,omitempty"`
}

func decodeBody(w http.ResponseWriter, r *http.Request) (pollRequest, bool) {
	var req pollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid_body"})
		return pollRequest{}, false
	}
	return req, true
}

func (s *Server) authDevice(w http.ResponseWriter, r *http.Request) {
	code, err := s.twitch.StartDevice(r.Context())
	respond(w, r, code, err)
}

func (s *Server) authPoll(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody(w, r)
	if !ok || req.DeviceCode == "" {
		if ok {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid_body"})
		}
		return
	}
	token, err := s.twitch.PollDevice(r.Context(), req.DeviceCode)
	switch {
	case errors.Is(err, twitch.ErrAuthPending):
		writeJSON(w, http.StatusOK, pollBody{Status: "pending"})
		return
	case errors.Is(err, twitch.ErrSlowDown):
		writeJSON(w, http.StatusOK, pollBody{Status: "slow_down"})
		return
	case err != nil:
		writeError(w, r, err)
		return
	}
	user, err := s.twitch.CurrentUser(r.Context(), token.AccessToken)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pollBody{Status: "ok", Token: &token, User: &user})
}

func (s *Server) authRefresh(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody(w, r)
	if !ok {
		return
	}
	token, err := s.twitch.Refresh(r.Context(), req.RefreshToken)
	respond(w, r, token, err)
}

func (s *Server) authRevoke(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBody(w, r)
	if !ok {
		return
	}
	if err := s.twitch.Revoke(r.Context(), req.Token); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func userToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	token := r.Header.Get(tokenHeader)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "twitch_token_required"})
		return "", false
	}
	return token, true
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	token, ok := userToken(w, r)
	if !ok {
		return
	}
	user, err := s.twitch.CurrentUser(r.Context(), token)
	respond(w, r, user, err)
}

func (s *Server) meLive(w http.ResponseWriter, r *http.Request) {
	token, ok := userToken(w, r)
	if !ok {
		return
	}
	first, after := pageParams(r)
	page, err := s.twitch.FollowedLive(r.Context(), token, first, after)
	respond(w, r, page, err)
}

func (s *Server) meVideos(w http.ResponseWriter, r *http.Request) {
	token, ok := userToken(w, r)
	if !ok {
		return
	}
	first, after := pageParams(r)
	page, err := s.twitch.FollowedVideos(r.Context(), token, first, after)
	respond(w, r, page, err)
}

func (s *Server) meFollows(w http.ResponseWriter, r *http.Request) {
	token, ok := userToken(w, r)
	if !ok {
		return
	}
	first, after := pageParams(r)
	page, err := s.twitch.FollowedChannels(r.Context(), token, first, after)
	respond(w, r, page, err)
}
