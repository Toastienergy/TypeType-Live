package config

import (
	"os"
	"path/filepath"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func TestLoadDefaultsAndValidation(t *testing.T) {
	if _, err := Load(env(nil)); err == nil {
		t.Fatal("missing secret accepted")
	}
	cfg, err := Load(env(map[string]string{"LIVE_SECRET": "0123456789abcdef0123456789abcdef", "LIVE_SEGMENT_CACHE": "16MiB"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" || cfg.SegmentBytes != 16<<20 || len(cfg.SigningKey) != 32 {
		t.Fatalf("cfg %+v", cfg)
	}
	if _, err := Load(env(map[string]string{"LIVE_SECRET": "0123456789abcdef0123456789abcdef", "LIVE_SEGMENT_CACHE": "lots"})); err == nil {
		t.Fatal("bad size accepted")
	}
}

func TestLoadReadsSecretsFromFiles(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret")
	key := filepath.Join(dir, "key")
	if err := os.WriteFile(secret, []byte("0123456789abcdef0123456789abcdef\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("fedcba9876543210fedcba9876543210"), 0o400); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(env(map[string]string{"LIVE_SECRET_FILE": secret, "LIVE_SIGNING_KEY_FILE": key}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Secret != "0123456789abcdef0123456789abcdef" || string(cfg.SigningKey) != "fedcba9876543210fedcba9876543210" {
		t.Fatalf("cfg %+v", cfg)
	}
	if _, err := Load(env(map[string]string{"LIVE_SECRET_FILE": filepath.Join(dir, "missing")})); err == nil {
		t.Fatal("missing secret file accepted")
	}
}
