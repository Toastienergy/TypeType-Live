package api

import (
	"net/http"
)

func (s *Server) followState(w http.ResponseWriter, r *http.Request) {
	login, ok := loginParam(w, r)
	if !ok {
		return
	}
	token, ok := userToken(w, r)
	if !ok {
		return
	}
	state, err := s.twitch.Following(r.Context(), token, login)
	respond(w, r, state, err)
}

func (s *Server) follow(w http.ResponseWriter, r *http.Request) {
	s.changeFollow(w, r, true)
}

func (s *Server) unfollow(w http.ResponseWriter, r *http.Request) {
	s.changeFollow(w, r, false)
}

func (s *Server) changeFollow(w http.ResponseWriter, r *http.Request, follow bool) {
	login, ok := loginParam(w, r)
	if !ok {
		return
	}
	token, ok := userToken(w, r)
	if !ok {
		return
	}
	ch, err := s.twitch.Channel(r.Context(), login)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if follow {
		err = s.twitch.Follow(r.Context(), token, ch.ID)
	} else {
		err = s.twitch.Unfollow(r.Context(), token, ch.ID)
	}
	if err != nil {
		writeError(w, r, err)
		return
	}
	state, err := s.twitch.Following(r.Context(), token, login)
	respond(w, r, state, err)
}
