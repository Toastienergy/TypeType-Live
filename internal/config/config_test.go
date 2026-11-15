package config

import "testing"

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
