package hls

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
)

func TestWithPrefetchSegmentsListsThemAsSegments(t *testing.T) {
	base, _ := url.Parse("https://a.hls.ttvnw.net/v1/segment/")
	media, err := ParseMedia([]byte(mediaSample), base)
	if err != nil {
		t.Fatal(err)
	}
	out := media.WithPrefetchSegments()
	if len(out.Segments) != 3 || len(out.Prefetch) != 0 || len(media.Segments) != 2 {
		t.Fatalf("segments %d prefetch %d source %d", len(out.Segments), len(out.Prefetch), len(media.Segments))
	}
	var buf bytes.Buffer
	out.Write(&buf, func(u string) string { return "s?u=" + u[strings.LastIndex(u, "/")+1:] })
	text := buf.String()
	if !strings.Contains(text, "#EXTINF:2.002,live\ns?u=three.ts") || strings.Contains(text, prefetchTag) {
		t.Fatalf("unexpected playlist:\n%s", text)
	}
}

func TestWithPrefetchSegmentsLeavesEndedAndEmptyPlaylists(t *testing.T) {
	media, _ := ParseMedia([]byte(mediaSample), nil)
	media.Ended = true
	if got := media.WithPrefetchSegments(); len(got.Segments) != 2 || len(got.Prefetch) != 1 {
		t.Fatalf("ended playlist changed: %+v", got)
	}
	empty := Media{Prefetch: []string{"three.ts"}}
	if got := empty.WithPrefetchSegments(); len(got.Segments) != 0 {
		t.Fatalf("empty playlist gained segments: %+v", got)
	}
}
