package api

import (
	"net/http"

	"github.com/TypeType-Video/TypeType-Live/twitch"
)

type userEmotesBody struct {
	ChannelID string             `json:"channelId"`
	Emotes    []twitch.UserEmote `json:"emotes"`
}

func (s *Server) meEmotes(w http.ResponseWriter, r *http.Request) {
	login, ok := loginParam(w, r)
	if !ok {
		return
	}
	token, ok := userToken(w, r)
	if !ok {
		return
	}
	ch, err := s.channels.Channel(r.Context(), login)
	if err != nil {
		writeError(w, r, err)
		return
	}
	emotes, err := s.twitch.AvailableEmotes(r.Context(), token, ch.ID)
	respond(w, r, userEmotesBody{ChannelID: ch.ID, Emotes: emotes}, err)
}
