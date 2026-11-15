package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/TypeType-Video/TypeType-Live/internal/faketwitch"
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

func TestSubscriberOnlyVODFallsBackToTheStoryboardPlaylists(t *testing.T) {
	e := newEnv(t)
	status, body := e.get(t, "/v1/videos/"+faketwitch.RestrictedVOD, true)
	if status != http.StatusOK {
		t.Fatalf("video %d %s", status, body)
	}
	var video struct {
		HLSURL string `json:"hlsUrl"`
	}
	_ = json.Unmarshal([]byte(body), &video)
	status, master := e.get(t, video.HLSURL, false)
	if status != http.StatusOK || strings.Count(master, "#EXT-X-STREAM-INF") != 2 || !strings.Contains(master, `STABLE-VARIANT-ID="720p60"`) {
		t.Fatalf("fallback master %d:\n%s", status, master)
	}
	_, variant := e.get(t, "/hls/"+firstURI(master), false)
	if strings.Count(variant, "segment.ts?") != 2 {
		t.Fatalf("fallback variant:\n%s", variant)
	}
	last := variant[strings.LastIndex(variant, "segment.ts?"):]
	last = last[:strings.IndexByte(last, '\n')]
	if status, _ := e.get(t, "/hls/"+last, false); status != http.StatusOK {
		t.Fatalf("muted segment status %d", status)
	}
}
