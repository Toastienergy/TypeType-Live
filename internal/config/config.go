package config

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Addr         string
	PprofAddr    string
	Secret       string
	SigningKey   []byte
	PublicBase   string
	ClientID     string
	SegmentBytes int64
	LogLevel     slog.Level
	TwitchBase   string
	ChatURL      string
}

func Load(getenv func(string) string) (Config, error) {
	secret, err := secretValue(getenv, "LIVE_SECRET")
	if err != nil {
		return Config{}, err
	}
	signingKey, err := secretValue(getenv, "LIVE_SIGNING_KEY")
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Addr:       valueOr(getenv("LIVE_ADDR"), ":8080"),
		PprofAddr:  getenv("LIVE_PPROF_ADDR"),
		Secret:     secret,
		PublicBase: strings.TrimRight(getenv("LIVE_PUBLIC_BASE"), "/"),
		ClientID:   getenv("TWITCH_CLIENT_ID"),
	}
	if len(cfg.Secret) < 32 {
		return Config{}, errors.New("config: LIVE_SECRET must be at least 32 characters")
	}
	if key := signingKey; key != "" {
		if len(key) < 32 {
			return Config{}, errors.New("config: LIVE_SIGNING_KEY must be at least 32 characters")
		}
		cfg.SigningKey = []byte(key)
	} else {
		cfg.SigningKey = make([]byte, 32)
		if _, err := rand.Read(cfg.SigningKey); err != nil {
			return Config{}, err
		}
	}
	size, err := parseSize(valueOr(getenv("LIVE_SEGMENT_CACHE"), "32MiB"))
	if err != nil {
		return Config{}, fmt.Errorf("config: LIVE_SEGMENT_CACHE: %w", err)
	}
	cfg.SegmentBytes = size
	if err := cfg.LogLevel.UnmarshalText([]byte(valueOr(getenv("LIVE_LOG_LEVEL"), "info"))); err != nil {
		return Config{}, fmt.Errorf("config: LIVE_LOG_LEVEL: %w", err)
	}
	cfg.TwitchBase = strings.TrimRight(getenv("LIVE_TWITCH_BASE"), "/")
	cfg.ChatURL = getenv("LIVE_CHAT_URL")
	return cfg, nil
}

func FromEnv() (Config, error) {
	return Load(os.Getenv)
}

// secretValue reads a secret from its variable, or from the file named by <name>_FILE, the way the TypeType stack
// hands its generated secrets to the containers.
func secretValue(getenv func(string) string, name string) (string, error) {
	if value := getenv(name); value != "" {
		return value, nil
	}
	path := getenv(name + "_FILE")
	if path == "" {
		return "", nil
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("config: %s_FILE: %w", name, err)
	}
	return strings.TrimSpace(string(raw)), nil
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func parseSize(raw string) (int64, error) {
	units := []struct {
		suffix string
		factor int64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}, {"B", 1}}
	for _, u := range units {
		if number, ok := strings.CutSuffix(raw, u.suffix); ok {
			n, err := strconv.ParseInt(strings.TrimSpace(number), 10, 64)
			if err != nil || n < 0 {
				return 0, fmt.Errorf("invalid size %q", raw)
			}
			return n * u.factor, nil
		}
	}
	return strconv.ParseInt(raw, 10, 64)
}
