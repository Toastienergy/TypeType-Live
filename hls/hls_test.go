package hls

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
)

const masterSample = `#EXTM3U
#EXT-X-SESSION-DATA:DATA-ID="USER-IP",VALUE="203.0.113.7"
#EXT-X-SESSION-DATA:DATA-ID="NODE",VALUE="edge"
#EXT-X-STREAM-INF:BANDWIDTH=7867033,RESOLUTION=1920x1080,CODECS="avc1.4D402A,mp4a.40.2",FRAME-RATE=60.000,STABLE-VARIANT-ID="1080p60",IVS-NAME="1080p60"
https://euw.playlist.ttvnw.net/v1/playlist/source.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=630000,RESOLUTION=640x360,CODECS="avc1.4D401F,mp4a.40.2",FRAME-RATE=30.000,STABLE-VARIANT-ID="360p30",IVS-NAME="360p"
360p.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=160000,CODECS="mp4a.40.2",STABLE-VARIANT-ID="audio_only",IVS-NAME="audio_only"
https://euw.playlist.ttvnw.net/v1/playlist/audio.m3u8
`

const mediaSample = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:100
#EXT-X-TWITCH-ELAPSED-SECS:10.0
#EXT-X-DATERANGE:ID="trigger-1",CLASS="twitch-trigger",START-DATE="2026-11-14T22:17:00.983Z",X-TV-TWITCH-TRIGGER-URL="https://example.test/beacon"
#EXT-X-DATERANGE:ID="source-1",CLASS="twitch-stream-source",START-DATE="2026-11-14T22:17:00.983Z"
#EXT-X-PROGRAM-DATE-TIME:2026-11-14T22:17:00.983Z
#EXTINF:2.002,live
https://a.hls.ttvnw.net/v1/segment/one.ts
#EXT-X-PROGRAM-DATE-TIME:2026-11-14T22:17:02.985Z
#EXTINF:2.002,live
two.ts
#EXT-X-TWITCH-PREFETCH:https://a.hls.ttvnw.net/v1/segment/three.ts
`

func TestAttributesHandlesQuotedCommas(t *testing.T) {
	attrs := Attributes(`#EXT-X-STREAM-INF:BANDWIDTH=1,CODECS="avc1.4D402A,mp4a.40.2",FRAME-RATE=60.000`)
	if attrs["CODECS"] != "avc1.4D402A,mp4a.40.2" || attrs["BANDWIDTH"] != "1" || attrs["FRAME-RATE"] != "60.000" {
		t.Fatalf("unexpected attributes %v", attrs)
	}
}

func TestParseMasterDropsSessionDataAndResolvesURIs(t *testing.T) {
	base, _ := url.Parse("https://usher.ttvnw.net/api/v2/channel/hls/test.m3u8")
	master, err := ParseMaster([]byte(masterSample), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(master.Variants) != 3 || len(master.Tags) != 0 {
		t.Fatalf("variants %d tags %v", len(master.Variants), master.Tags)
	}
	if master.Variants[1].URI != "https://usher.ttvnw.net/api/v2/channel/hls/360p.m3u8" {
		t.Fatalf("relative URI not resolved: %s", master.Variants[1].URI)
	}
	if !master.Variants[2].AudioOnly() || master.Variants[0].AudioOnly() {
		t.Fatal("audio only detection")
	}
	var buf bytes.Buffer
	master.Write(&buf, func(v Variant) string { return "v?q=" + v.ID })
	out := buf.String()
	if strings.Contains(out, "USER-IP") || strings.Contains(out, "ttvnw") || !strings.Contains(out, "v?q=360p30") {
		t.Fatalf("bad master output:\n%s", out)
	}
}

func TestClosestFallsBackByBandwidth(t *testing.T) {
	master, _ := ParseMaster([]byte(masterSample), nil)
	v, ok := master.Closest(Variant{ID: "720p60", Resolution: "1280x720", Codecs: "avc1", Bandwidth: 3_000_000})
	if !ok || v.ID != "360p30" {
		t.Fatalf("closest = %v %v", v.ID, ok)
	}
}

func TestParseMediaRewritesAndDropsTrigger(t *testing.T) {
	base, _ := url.Parse("https://a.hls.ttvnw.net/v1/playlist/x.m3u8")
	media, err := ParseMedia([]byte(mediaSample), base)
	if err != nil {
		t.Fatal(err)
	}
	if media.MediaSequence != 100 || len(media.Segments) != 2 || len(media.Prefetch) != 1 || media.AdBreak {
		t.Fatalf("parsed %+v", media)
	}
	if media.Segments[1].URI != "https://a.hls.ttvnw.net/v1/playlist/two.ts" {
		t.Fatalf("segment URI %s", media.Segments[1].URI)
	}
	var buf bytes.Buffer
	media.Write(&buf, func(u string) string { return "s?u=" + u[strings.LastIndexByte(u, '/')+1:] })
	out := buf.String()
	for _, want := range []string{"#EXT-X-MEDIA-SEQUENCE:100", "#EXTINF:2.002,live\ns?u=one.ts", "#EXT-X-TWITCH-PREFETCH:s?u=three.ts", "twitch-stream-source"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "beacon") || strings.Contains(out, "ttvnw") {
		t.Fatalf("leaked upstream data:\n%s", out)
	}
}

func TestWithoutAdsKeepsSequenceForHeadAds(t *testing.T) {
	raw := `#EXTM3U
#EXT-X-MEDIA-SEQUENCE:10
#EXT-X-DATERANGE:ID="stitched-ad-1",CLASS="twitch-stitched-ad",START-DATE="2026-11-14T22:17:00Z"
#EXTINF:2.000,Amazon|123
ad1.ts
#EXTINF:2.000,Amazon|123
ad2.ts
#EXTINF:2.000,live
live1.ts
#EXTINF:2.000,Amazon|124
ad3.ts
#EXTINF:2.000,live
live2.ts
`
	media, err := ParseMedia([]byte(raw), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !media.AdBreak || !media.HasAds() {
		t.Fatal("ads not detected")
	}
	clean := media.WithoutAds()
	if clean.MediaSequence != 12 || len(clean.Segments) != 2 {
		t.Fatalf("sequence %d segments %d", clean.MediaSequence, len(clean.Segments))
	}
	if clean.Segments[1].Tags[0] != "#EXT-X-DISCONTINUITY" {
		t.Fatalf("missing discontinuity: %v", clean.Segments[1].Tags)
	}
}

func TestParseRejectsNonPlaylist(t *testing.T) {
	if _, err := ParseMedia([]byte("<html>"), nil); err != ErrNotPlaylist {
		t.Fatalf("err = %v", err)
	}
	if _, err := ParseMaster([]byte(""), nil); err != ErrNotPlaylist {
		t.Fatalf("err = %v", err)
	}
}

func BenchmarkRewriteMedia(b *testing.B) {
	base, _ := url.Parse("https://a.hls.ttvnw.net/v1/playlist/x.m3u8")
	var buf bytes.Buffer
	b.ReportAllocs()
	for b.Loop() {
		media, _ := ParseMedia([]byte(mediaSample), base)
		buf.Reset()
		media.Write(&buf, func(u string) string { return u })
	}
}

func TestURIAttributesAreRewrittenAndKept(t *testing.T) {
	raw := `#EXTM3U
#EXT-X-VERSION:6
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:5
#EXT-X-MAP:URI="init-0.mp4"
#EXTINF:2.000,live
seg-5.mp4
#EXT-X-PRELOAD-HINT:TYPE=PART,URI="https://x.hls.ttvnw.net/v1/segment/next.mp4"
`
	base, _ := url.Parse("https://a.hls.ttvnw.net/v1/playlist/p.m3u8")
	media, err := ParseMedia([]byte(raw), base)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	media.Write(&buf, func(u string) string { return "proxied/" + u[strings.LastIndexByte(u, '/')+1:] })
	out := buf.String()
	for _, want := range []string{`#EXT-X-MAP:URI="proxied/init-0.mp4"`, "proxied/seg-5.mp4", `URI="proxied/next.mp4"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "ttvnw") {
		t.Fatalf("upstream host leaked:\n%s", out)
	}
}

func TestMasterDropsRenditionsWithURIs(t *testing.T) {
	raw := `#EXTM3U
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="aac",NAME="en",URI="https://x.ttvnw.net/audio.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=1,STABLE-VARIANT-ID="a"
v.m3u8
`
	master, _ := ParseMaster([]byte(raw), nil)
	var buf bytes.Buffer
	master.Write(&buf, func(Variant) string { return "x" })
	if strings.Contains(buf.String(), "ttvnw") {
		t.Fatalf("rendition URI leaked:\n%s", buf.String())
	}
}
