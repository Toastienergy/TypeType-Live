package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func (e *env) userAgentGet(t *testing.T, path, agent string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+path, nil)
	req.Header.Set("User-Agent", agent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestAVPlayerGetsNoPrefetchSegments(t *testing.T) {
	e := newEnv(t)
	variantPath, _ := e.liveVariant(t, "somechannel")
	code, variant := e.userAgentGet(t, variantPath, "AppleCoreMedia/1.0.0.23B85 (iPhone; U; CPU OS 26_1 like Mac OS X)")
	if code != http.StatusOK || strings.Count(variant, "#EXTINF:") != 6 || !strings.Contains(variant, "#EXT-X-TWITCH-PREFETCH") {
		t.Fatalf("AVPlayer variant: %d\n%s", code, variant)
	}
}

func TestPrefetchOffFollowsTheMasterLink(t *testing.T) {
	e := newEnv(t)
	code, body := e.get(t, "/v1/live/somechannel", true)
	if code != http.StatusOK {
		t.Fatalf("live: %d %s", code, body)
	}
	var lb liveBody
	if err := json.Unmarshal([]byte(body), &lb); err != nil {
		t.Fatal(err)
	}
	code, master := e.get(t, lb.HLSURL+"&prefetch=0", false)
	if code != http.StatusOK {
		t.Fatalf("master: %d %s", code, master)
	}
	code, variant := e.get(t, "/hls/"+firstURI(master), false)
	if code != http.StatusOK || strings.Count(variant, "#EXTINF:") != 6 {
		t.Fatalf("variant without prefetch: %d\n%s", code, variant)
	}
}
