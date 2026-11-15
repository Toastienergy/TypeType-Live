package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/TypeType-Video/TypeType-Live/internal/live"
	"github.com/TypeType-Video/TypeType-Live/proxy"
	"github.com/TypeType-Video/TypeType-Live/twitch"
)

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := classify(err)
	if status == 0 {
		return
	}
	if status >= 500 {
		slog.Warn("request failed", "path", r.URL.Path, "error", err)
	}
	writeJSON(w, status, errorBody{Error: code})
}

type mapping struct {
	err    error
	status int
	code   string
}

var mappings = []mapping{
	{context.Canceled, 0, ""},
	{twitch.ErrOffline, http.StatusNotFound, "offline"},
	{twitch.ErrNotFound, http.StatusNotFound, "not_found"},
	{twitch.ErrRestricted, http.StatusForbidden, "restricted"},
	{twitch.ErrBadToken, http.StatusUnauthorized, "twitch_token_rejected"},
	{twitch.ErrChatRejected, http.StatusForbidden, "chat_rejected"},
	{twitch.ErrIntegrity, http.StatusServiceUnavailable, "integrity_required"},
	{twitch.ErrAuthPending, http.StatusAccepted, "authorization_pending"},
	{twitch.ErrSlowDown, http.StatusTooManyRequests, "slow_down"},
	{twitch.ErrCodeExpired, http.StatusGone, "code_expired"},
	{live.ErrUnknownSession, http.StatusGone, "link_expired"},
	{proxy.ErrExpired, http.StatusGone, "link_expired"},
	{proxy.ErrBadSignature, http.StatusForbidden, "forbidden"},
	{proxy.ErrHostNotAllowed, http.StatusForbidden, "forbidden"},
	{proxy.ErrSealed, http.StatusForbidden, "forbidden"},
	{live.ErrUnknownVariant, http.StatusNotFound, "unknown_variant"},
	{context.DeadlineExceeded, http.StatusGatewayTimeout, "upstream_timeout"},
}

func classify(err error) (int, string) {
	for _, m := range mappings {
		if errors.Is(err, m.err) {
			return m.status, m.code
		}
	}
	var upstream *proxy.StatusError
	if errors.As(err, &upstream) {
		return http.StatusBadGateway, "upstream_" + strconv.Itoa(upstream.Status)
	}
	var gqlErr *twitch.GQLError
	if errors.As(err, &gqlErr) {
		return http.StatusBadGateway, "upstream_graphql"
	}
	return http.StatusBadGateway, "upstream_error"
}

func pageParams(r *http.Request) (int, string) {
	first, _ := strconv.Atoi(r.URL.Query().Get("first"))
	return first, r.URL.Query().Get("after")
}
