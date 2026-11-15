package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TypeType-Video/TypeType-Live/chat"
	"github.com/TypeType-Video/TypeType-Live/emotes"
	"github.com/TypeType-Video/TypeType-Live/internal/api"
	"github.com/TypeType-Video/TypeType-Live/internal/chatassets"
	"github.com/TypeType-Video/TypeType-Live/internal/config"
	"github.com/TypeType-Video/TypeType-Live/internal/live"
	"github.com/TypeType-Video/TypeType-Live/internal/metrics"
	"github.com/TypeType-Video/TypeType-Live/proxy"
	"github.com/TypeType-Video/TypeType-Live/twitch"
)

// version is set at build time from the release tag (-ldflags -X main.version=...).
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "health" {
		os.Exit(health())
	}
	if err := run(); err != nil {
		slog.Error("typetype-live stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel})))
	handler, err := newHandler(cfg)
	if err != nil {
		return err
	}
	return serve(cfg, handler)
}

func newHandler(cfg config.Config) (http.Handler, error) {
	suffixes := proxy.DefaultAllowedSuffixes
	insecure := false
	if cfg.TwitchBase != "" {
		u, err := url.Parse(cfg.TwitchBase)
		if err != nil {
			return nil, fmt.Errorf("LIVE_TWITCH_BASE: %w", err)
		}
		suffixes = []string{u.Hostname()}
		insecure = true
	}
	hosts := proxy.NewHosts(suffixes)
	hosts.AllowInsecure = insecure
	httpClient := proxy.NewHTTPClient(hosts)
	tw := twitch.New(httpClient, cfg.ClientID)
	if cfg.TwitchBase != "" {
		tw.GQLURL, tw.UsherURL, tw.IDURL = cfg.TwitchBase+"/gql", cfg.TwitchBase, cfg.TwitchBase
	}
	liveMetrics := &live.Metrics{}
	sealer, err := proxy.NewSealer(cfg.SigningKey)
	if err != nil {
		return nil, err
	}
	service := live.New(live.Config{
		Twitch:        tw,
		Relay:         proxy.NewRelay(httpClient, hosts),
		Signer:        proxy.NewSigner(cfg.SigningKey),
		Sealer:        sealer,
		SegmentBytes:  cfg.SegmentBytes,
		BackupPlayers: live.DefaultBackupPlayers,
		Metrics:       liveMetrics,
	})
	hub := chat.NewHub(cfg.ChatURL)
	return api.New(api.Options{
		Twitch:     tw,
		Assets:     chatassets.New(tw, emotes.New(httpClient)),
		Live:       service,
		Chat:       hub,
		Secret:     cfg.Secret,
		PublicBase: cfg.PublicBase,
		Metrics:    metrics.Handler(metrics.Sources{Live: liveMetrics, Segments: service.SegmentStats, Rooms: hub.Rooms}),
	}), nil
}

func serve(cfg config.Config, handler http.Handler) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if cfg.PprofAddr != "" {
		go servePprof(cfg.PprofAddr)
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	errs := make(chan error, 1)
	go func() { errs <- srv.ListenAndServe() }()
	slog.Info("typetype-live listening", "addr", cfg.Addr, "version", version)
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

func servePprof(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Warn("pprof stopped", "error", err)
	}
}

func health() int {
	addr := os.Getenv("LIVE_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host := "127.0.0.1"
	if h, port, err := splitAddr(addr); err == nil {
		if h != "" && h != "0.0.0.0" {
			host = h
		}
		addr = port
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+":"+addr+"/healthz", nil)
	if err != nil {
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return 1
	}
	return 0
}

func splitAddr(addr string) (string, string, error) {
	u, err := url.Parse("http://" + addr)
	if err != nil {
		return "", "", err
	}
	return u.Hostname(), u.Port(), nil
}
