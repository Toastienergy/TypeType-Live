package api

import "net/http"

func (s *Server) globalAssets(w http.ResponseWriter, r *http.Request) {
	assets, err := s.assets.Global(r.Context())
	respond(w, r, assets, err)
}

func (s *Server) channelAssets(w http.ResponseWriter, r *http.Request) {
	login, ok := loginParam(w, r)
	if !ok {
		return
	}
	assets, err := s.assets.Channel(r.Context(), login)
	respond(w, r, assets, err)
}
