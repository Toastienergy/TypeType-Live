package api

import (
	"encoding/json"
	"net/http"
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
