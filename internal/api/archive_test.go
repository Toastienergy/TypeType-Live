package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLiveNamesThePastBroadcastItRecords(t *testing.T) {
	e := newEnv(t)
	code, body := e.get(t, "/v1/live/somechannel", true)
	if code != http.StatusOK {
		t.Fatalf("live: %d %s", code, body)
	}
	var lb liveBody
	if err := json.Unmarshal([]byte(body), &lb); err != nil {
		t.Fatal(err)
	}
	archive := lb.Channel.Live.Archive
	want := time.Date(2026, 11, 14, 20, 0, 4, 0, time.UTC)
	if archive == nil || archive.ID != "900" || !archive.StartedAt.Equal(want) {
		t.Fatalf("archive %+v", archive)
	}
}

func TestThePastBroadcastOfARunningLiveSaysItRecords(t *testing.T) {
	e := newEnv(t)
	for id, recording := range map[string]bool{"900": true, "123": false} {
		code, body := e.get(t, "/v1/videos/"+id, true)
		var vb videoBody
		if err := json.Unmarshal([]byte(body), &vb); err != nil || code != http.StatusOK {
			t.Fatalf("video %s: %d %v", id, code, err)
		}
		if vb.Video.Recording != recording {
			t.Fatalf("video %s recording %v", id, vb.Video.Recording)
		}
	}
}

func TestARecordingPlaylistKeepsGrowing(t *testing.T) {
	e := newEnv(t)
	segments := func(id string) int {
		t.Helper()
		_, body := e.get(t, "/v1/videos/"+id, true)
		var vb videoBody
		if err := json.Unmarshal([]byte(body), &vb); err != nil {
			t.Fatal(err)
		}
		_, master := e.get(t, vb.HLSURL, false)
		_, variant := e.get(t, "/hls/"+firstURI(master), false)
		return strings.Count(variant, "segment.ts?")
	}
	first := segments("900")
	time.Sleep(1100 * time.Millisecond)
	if next := segments("900"); next <= first {
		t.Fatalf("recording stayed at %d segments, then %d", first, next)
	}
	finished := segments("123")
	time.Sleep(1100 * time.Millisecond)
	calls := e.fake.PlaylistCalls.Load()
	if again := segments("123"); again != finished || e.fake.PlaylistCalls.Load() != calls {
		t.Fatal("a finished past broadcast was fetched again")
	}
}
