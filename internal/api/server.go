package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/TypeType-Video/TypeType-Live/internal/live"
	"github.com/TypeType-Video/TypeType-Live/twitch"
)

const (
	secretHeader = "X-Live-Secret"
	tokenHeader  = "X-Twitch-Token"
	maxBodyBytes = 64 << 10
)

type Server struct {
	twitch     *twitch.Client
	live       *live.Service
	chat       ChatHub
	secret     []byte
	publicBase string
	metrics    http.Handler
}

type Options struct {
	Twitch     *twitch.Client
	Live       *live.Service
	Chat       ChatHub
	Secret     string
	PublicBase string
	Metrics    http.Handler
}

func New(opts Options) http.Handler {
	s := &Server{
		twitch:     opts.Twitch,
		live:       opts.Live,
		chat:       opts.Chat,
		secret:     []byte(opts.Secret),
		publicBase: opts.PublicBase,
		metrics:    opts.Metrics,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.Handle("GET /metrics", s.private(s.metrics))

	mux.HandleFunc("GET /hls/master.m3u8", s.hlsMaster)
	mux.HandleFunc("GET /hls/variant.m3u8", s.hlsVariant)
	mux.HandleFunc("GET /hls/segment.ts", s.hlsSegment)
	mux.HandleFunc("GET /hls/clip.mp4", s.hlsSegment)

	api := http.NewServeMux()
	api.HandleFunc("GET /v1/streams", s.topStreams)
	api.HandleFunc("GET /v1/games", s.topGames)
	api.HandleFunc("GET /v1/games/{slug}/streams", s.gameStreams)
	api.HandleFunc("GET /v1/channels/{login}", s.channel)
	api.HandleFunc("GET /v1/channels/{login}/videos", s.channelVideos)
	api.HandleFunc("GET /v1/channels/{login}/clips", s.channelClips)
	api.HandleFunc("GET /v1/live/{login}", s.liveStream)
	api.HandleFunc("GET /v1/videos/{id}", s.video)
	api.HandleFunc("GET /v1/clips/{slug}", s.clip)
	api.HandleFunc("GET /v1/search", s.search)
	api.HandleFunc("POST /v1/auth/device", s.authDevice)
	api.HandleFunc("POST /v1/auth/poll", s.authPoll)
	api.HandleFunc("POST /v1/auth/refresh", s.authRefresh)
	api.HandleFunc("POST /v1/auth/revoke", s.authRevoke)
	api.HandleFunc("GET /v1/me", s.me)
	api.HandleFunc("GET /v1/me/live", s.meLive)
	api.HandleFunc("GET /v1/me/follows", s.meFollows)
	api.HandleFunc("GET /v1/chat/{login}/events", s.chatEvents)
	api.HandleFunc("POST /v1/chat/{login}/messages", s.chatSend)
	mux.Handle("/v1/", s.private(api))

	return recoverer(mux)
}

func (s *Server) private(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get(secretHeader))
		if subtle.ConstantTimeCompare(got, s.secret) != 1 {
			writeJSON(w, http.StatusUnauthorized, errorBody{Error: "unauthorized"})
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				slog.Error("panic", "path", r.URL.Path, "value", v)
				writeJSON(w, http.StatusInternalServerError, errorBody{Error: "internal"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) hlsLink(link string) string {
	return s.publicBase + "/hls/" + link
}
