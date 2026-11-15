package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestChatAssetsAnswerEvenWhenProvidersFail(t *testing.T) {
	e := newEnv(t)
	status, body := e.get(t, "/v1/chat/somechannel/assets", true)
	if status != http.StatusOK || !strings.Contains(body, `"channelId":"42"`) {
		t.Fatalf("channel assets %d %s", status, body)
	}
	if status, _ := e.get(t, "/v1/chat/assets", true); status != http.StatusOK {
		t.Fatalf("global assets %d", status)
	}
}

func TestLiveResponsePreparesTheMasterPlaylist(t *testing.T) {
	e := newEnv(t)
	status, body := e.get(t, "/v1/live/somechannel", true)
	if status != http.StatusOK {
		t.Fatalf("live %d %s", status, body)
	}
	deadline := time.Now().Add(2 * time.Second)
	for e.fake.MasterCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if e.fake.MasterCalls.Load() != 1 {
		t.Fatalf("master not prepared: %d calls", e.fake.MasterCalls.Load())
	}
	var live struct {
		HLSURL string `json:"hlsUrl"`
	}
	_ = json.Unmarshal([]byte(body), &live)
	if status, _ := e.get(t, "/hls/"+strings.TrimPrefix(live.HLSURL, "/hls/"), false); status != http.StatusOK {
		t.Fatalf("master %d", status)
	}
	if e.fake.MasterCalls.Load() != 1 {
		t.Fatalf("prepared master was fetched again: %d calls", e.fake.MasterCalls.Load())
	}
}

func TestChannelLookupsAreSharedBriefly(t *testing.T) {
	e := newEnv(t)
	for range 3 {
		if status, _ := e.get(t, "/v1/channels/somechannel", true); status != http.StatusOK {
			t.Fatalf("status %d", status)
		}
	}
	if calls := e.fake.ChannelCalls.Load(); calls != 1 {
		t.Fatalf("channel fetched %d times", calls)
	}
}
