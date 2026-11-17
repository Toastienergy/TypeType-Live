package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/TypeType-Video/TypeType-Live/twitch"
)

type voteRequest struct {
	PollID   string `json:"pollId"`
	ChoiceID string `json:"choiceId"`
}

type voteRejectedBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func (s *Server) pollVote(w http.ResponseWriter, r *http.Request) {
	token, ok := userToken(w, r)
	if !ok {
		return
	}
	var req voteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil ||
		!validID(req.PollID) || !validID(req.ChoiceID) {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid_body"})
		return
	}
	user, err := s.twitch.CurrentUser(r.Context(), token)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.twitch.VoteInPoll(r.Context(), token, user.ID, req.PollID, req.ChoiceID); err != nil {
		var rejected *twitch.VoteRejectedError
		if errors.As(err, &rejected) {
			writeJSON(w, http.StatusForbidden, voteRejectedBody{Error: "vote_rejected", Code: rejected.Code})
			return
		}
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var pollIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func validID(id string) bool { return pollIDPattern.MatchString(id) }
