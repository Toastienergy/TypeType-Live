package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestLiveAnswersASignedAudioLink(t *testing.T) {
	e := newEnv(t)
	code, body := e.get(t, "/v1/live/somechannel", true)
	if code != http.StatusOK {
		t.Fatalf("live: %d %s", code, body)
	}
	var lb liveBody
	if err := json.Unmarshal([]byte(body), &lb); err != nil {
		t.Fatal(err)
	}
	if lb.AudioURL == "" || lb.AudioURL == lb.HLSURL || !strings.Contains(lb.AudioURL, "a=1") {
		t.Fatalf("audio link %q", lb.AudioURL)
	}
	if code, master := e.get(t, lb.AudioURL, false); code != http.StatusOK || !strings.Contains(master, "#EXT-X-STREAM-INF") {
		t.Fatalf("audio master without an audio rendition falls back to the whole one: %d %s", code, master)
	}
	if code, _ := e.get(t, lb.HLSURL+"&a=1", false); code == http.StatusOK {
		t.Fatal("an unsigned audio flag was accepted")
	}
}
