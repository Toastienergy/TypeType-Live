package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TypeType-Video/TypeType-Live/chat"
	"github.com/TypeType-Video/TypeType-Live/twitch"
)

type ChatHub interface {
	Subscribe(channel string) (<-chan chat.Event, []chat.Event, func())
}

const heartbeat = 15 * time.Second

func (s *Server) chatEvents(w http.ResponseWriter, r *http.Request) {
	login, ok := loginParam(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "streaming_unsupported"})
		return
	}
	events, history, leave := s.chat.Subscribe(login)
	defer leave()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	send := func(ev chat.Event) bool {
		if _, err := w.Write([]byte("data: ")); err != nil {
			return false
		}
		if err := enc.Encode(ev); err != nil {
			return false
		}
		_, err := w.Write([]byte("\n"))
		return err == nil
	}
	for _, ev := range history {
		if !send(ev) {
			return
		}
	}
	flusher.Flush()
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-events:
			if !send(ev) {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

type chatRejectedBody struct {
	Error  string `json:"error"`
	Reason string `json:"reason"`
}

type sendRequest struct {
	Message string `json:"message"`
}

func (s *Server) chatSend(w http.ResponseWriter, r *http.Request) {
	login, ok := loginParam(w, r)
	if !ok {
		return
	}
	token, ok := userToken(w, r)
	if !ok {
		return
	}
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid_body"})
		return
	}
	message := strings.TrimSpace(req.Message)
	if message == "" || utf8.RuneCountInString(message) > 500 {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid_message"})
		return
	}
	ch, err := s.channels.Channel(r.Context(), login)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.twitch.SendChatMessage(r.Context(), token, ch.ID, message); err != nil {
		var rejected *twitch.ChatRejectedError
		if errors.As(err, &rejected) {
			writeJSON(w, http.StatusForbidden, chatRejectedBody{Error: "chat_rejected", Reason: rejected.Reason})
			return
		}
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) chatHighlights(w http.ResponseWriter, r *http.Request) {
	login, ok := loginParam(w, r)
	if !ok {
		return
	}
	highlights, err := s.twitch.ChatHighlights(r.Context(), login)
	respond(w, r, highlights, err)
}
