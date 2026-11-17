package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionIsPublicAndShortensTheRevision(t *testing.T) {
	srv := httptest.NewServer(New(Options{Secret: testSecret, Build: Build{
		Version: "1.18.0", Revision: "0123456789abcdef0123", BuildTime: "2026-11-17T10:00:00Z",
	}}))
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/version")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got versionBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("version: %d %v", resp.StatusCode, err)
	}
	want := versionBody{
		Service: "live", Version: "1.18.0", Revision: "0123456789abcdef0123",
		ShortRevision: "0123456789ab", BuildTime: "2026-11-17T10:00:00Z",
	}
	if got != want {
		t.Fatalf("version %+v", got)
	}
}

func TestVersionOfALocalBuild(t *testing.T) {
	if got := (Build{}).body(); got.Version != "dev" || got.Revision != "unknown" || got.ShortRevision != "unknown" {
		t.Fatalf("local build %+v", got)
	}
}
