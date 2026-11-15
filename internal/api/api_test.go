package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/TypeType-Video/TypeType-Live/chat"
	"github.com/TypeType-Video/TypeType-Live/emotes"
	"github.com/TypeType-Video/TypeType-Live/internal/chatassets"
	"github.com/TypeType-Video/TypeType-Live/internal/faketwitch"
	"github.com/TypeType-Video/TypeType-Live/internal/live"
	"github.com/TypeType-Video/TypeType-Live/proxy"
	"github.com/TypeType-Video/TypeType-Live/twitch"
)

const testSecret = "0123456789abcdef0123456789abcdef"

type env struct {
	fake    *faketwitch.Fake
	srv     *httptest.Server
	metrics *live.Metrics
}

func newEnv(t *testing.T) *env {
	t.Helper()
	fake := faketwitch.New()
	upstream := httptest.NewServer(fake.Handler())
	t.Cleanup(upstream.Close)
	fake.Base = upstream.URL
	hosts := proxy.NewHosts([]string{"127.0.0.1"})
	hosts.AllowInsecure = true
	client := proxy.NewHTTPClient(hosts)
	tw := twitch.New(client, "")
	tw.GQLURL, tw.UsherURL, tw.IDURL = upstream.URL+"/gql", upstream.URL, upstream.URL
	metrics := &live.Metrics{}
	sealer, err := proxy.NewSealer([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	service := live.New(live.Config{
		Twitch:        tw,
		Relay:         proxy.NewRelay(client, hosts),
		Signer:        proxy.NewSigner([]byte(testSecret)),
		Sealer:        sealer,
		SegmentBytes:  8 << 20,
		BackupPlayers: live.DefaultBackupPlayers,
		Metrics:       metrics,
	})
	em := emotes.New(client)
	em.SevenTVURL, em.BTTVURL, em.FFZURL = upstream.URL, upstream.URL, upstream.URL
	srv := httptest.NewServer(New(Options{
		Twitch: tw, Live: service, Chat: chat.NewHub("ws://127.0.0.1:1"), Secret: testSecret,
		Assets: chatassets.New(tw, em),
	}))
	t.Cleanup(srv.Close)
	return &env{fake: fake, srv: srv, metrics: metrics}
}

func (e *env) get(t *testing.T, path string, secret bool) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+path, nil)
	if secret {
		req.Header.Set(secretHeader, testSecret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func firstURI(playlist string) string {
	for line := range strings.SplitSeq(playlist, "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

func (e *env) liveVariant(t *testing.T, login string) (string, string) {
	t.Helper()
	code, body := e.get(t, "/v1/live/"+login, true)
	if code != http.StatusOK {
		t.Fatalf("live %s: %d %s", login, code, body)
	}
	var lb liveBody
	if err := json.Unmarshal([]byte(body), &lb); err != nil {
		t.Fatal(err)
	}
	code, master := e.get(t, lb.HLSURL, false)
	if code != http.StatusOK {
		t.Fatalf("master: %d %s", code, master)
	}
	if strings.Contains(master, "USER-IP") || strings.Contains(master, "127.0.0.1") {
		t.Fatalf("master leaks upstream data:\n%s", master)
	}
	variantPath := "/hls/" + firstURI(master)
	code, variant := e.get(t, variantPath, false)
	if code != http.StatusOK {
		t.Fatalf("variant: %d %s", code, variant)
	}
	return variantPath, variant
}

func TestLivePlaybackEndToEnd(t *testing.T) {
	e := newEnv(t)
	_, variant := e.liveVariant(t, "somechannel")
	if strings.Contains(variant, "beacon") || strings.Contains(variant, "127.0.0.1") {
		t.Fatalf("variant leaks upstream data:\n%s", variant)
	}
	if !strings.Contains(variant, "#EXT-X-TWITCH-PREFETCH:segment.ts?") {
		t.Fatalf("prefetch not proxied:\n%s", variant)
	}
	segment := "/hls/" + firstURI(variant)
	for range 3 {
		code, body := e.get(t, segment, false)
		if code != http.StatusOK || len(body) != e.fake.SegmentBytes || body[0] != 0x47 {
			t.Fatalf("segment: %d len %d", code, len(body))
		}
	}
	if got := e.fake.SegmentCalls.Load(); got != 1 {
		t.Fatalf("segment fetched %d times upstream, want 1", got)
	}
}

func TestPrivateRoutesNeedSecret(t *testing.T) {
	e := newEnv(t)
	if code, _ := e.get(t, "/v1/live/somechannel", false); code != http.StatusUnauthorized {
		t.Fatalf("code = %d", code)
	}
	if code, _ := e.get(t, "/hls/master.m3u8?t=live&c=x&e=9999999999&s=forged", false); code != http.StatusForbidden {
		t.Fatalf("forged link code = %d", code)
	}
	if code, _ := e.get(t, "/v1/live/bad%20login", true); code != http.StatusBadRequest {
		t.Fatalf("bad login code = %d", code)
	}
}

func TestOfflineAndMissing(t *testing.T) {
	e := newEnv(t)
	if code, body := e.get(t, "/v1/live/offline", true); code != http.StatusNotFound || !strings.Contains(body, "offline") {
		t.Fatalf("offline: %d %s", code, body)
	}
	if code, body := e.get(t, "/v1/channels/missing", true); code != http.StatusNotFound || !strings.Contains(body, "not_found") {
		t.Fatalf("missing: %d %s", code, body)
	}
}

func TestAdsAreSwappedForABackupStream(t *testing.T) {
	e := newEnv(t)
	_, variant := e.liveVariant(t, "adschannel")
	if strings.Contains(variant, "Amazon") || strings.Contains(variant, "stitched-ad") {
		t.Fatalf("ads left in playlist:\n%s", variant)
	}
	if e.metrics.AdSwaps.Load() != 1 || e.metrics.AdStripped.Load() != 0 {
		t.Fatalf("swaps %d stripped %d", e.metrics.AdSwaps.Load(), e.metrics.AdStripped.Load())
	}
}

func TestAdsAreStrippedWhenEveryStreamHasThem(t *testing.T) {
	e := newEnv(t)
	_, variant := e.liveVariant(t, "adsonly")
	if strings.Contains(variant, "Amazon") || strings.Count(variant, "#EXTINF") != 3 {
		t.Fatalf("unexpected playlist:\n%s", variant)
	}
	if e.metrics.AdStripped.Load() != 1 {
		t.Fatalf("stripped %d", e.metrics.AdStripped.Load())
	}
}

func TestConcurrentViewersShareUpstream(t *testing.T) {
	e := newEnv(t)
	variantPath, _ := e.liveVariant(t, "busy")
	before := e.fake.PlaylistCalls.Load()
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if code, _ := e.get(t, variantPath, false); code != http.StatusOK {
				t.Errorf("code %d", code)
			}
		})
	}
	wg.Wait()
	if extra := e.fake.PlaylistCalls.Load() - before; extra > 1 {
		t.Fatalf("50 viewers caused %d upstream playlist fetches", extra)
	}
	if e.fake.TokenCalls.Load() != 1 {
		t.Fatalf("token calls %d", e.fake.TokenCalls.Load())
	}
}

func TestExpiredVariantRefreshesMaster(t *testing.T) {
	e := newEnv(t)
	e.fake.Expired.Store(true)
	e.liveVariant(t, "longstream")
	if e.fake.MasterCalls.Load() != 2 {
		t.Fatalf("master calls %d, want a refresh", e.fake.MasterCalls.Load())
	}
}

func TestVODPlayback(t *testing.T) {
	e := newEnv(t)
	code, body := e.get(t, "/v1/videos/123", true)
	if code != http.StatusOK {
		t.Fatalf("video: %d %s", code, body)
	}
	var vb videoBody
	_ = json.Unmarshal([]byte(body), &vb)
	_, master := e.get(t, vb.HLSURL, false)
	_, variant := e.get(t, "/hls/"+firstURI(master), false)
	if !strings.Contains(variant, "#EXT-X-ENDLIST") || strings.Count(variant, "segment.ts?") != 6 {
		t.Fatalf("vod playlist:\n%s", variant)
	}
	if code, _ := e.get(t, "/v1/videos/abc", true); code != http.StatusBadRequest {
		t.Fatalf("bad id code = %d", code)
	}
}
