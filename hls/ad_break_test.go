package hls

import (
	"strings"
	"testing"
	"time"
)

const adPlaylist = `#EXTM3U
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:10
#EXT-X-DATERANGE:ID="stitched-ad-1",CLASS="twitch-stitched-ad",START-DATE="2026-11-17T22:00:00Z",DURATION=30.0
#EXTINF:2.000,Amazon|ad
a.ts
#EXT-X-DATERANGE:ID="stitched-ad-2",CLASS="twitch-stitched-ad",START-DATE="2026-11-17T22:00:30Z",DURATION=15.5
#EXTINF:2.000,Amazon|ad
b.ts
`

func TestAdBreakNoticeEndsWithTheLastAnnouncedAd(t *testing.T) {
	media, err := ParseMedia([]byte(adPlaylist), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 17, 22, 0, 10, 0, time.UTC)
	notice := strings.Join(media.WithoutAds().WithAdBreakNotice(now).Header, "\n")
	want := `#EXT-X-DATERANGE:ID="typetype-ad-break-1794952845",CLASS="com.typetype.ad-break",` +
		`START-DATE="2026-11-17T22:00:00Z",END-DATE="2026-11-17T22:00:45Z"`
	if !strings.Contains(notice, want) {
		t.Fatalf("notice %s", notice)
	}
}

func TestAdBreakWithoutTimingLastsAWhileFromNow(t *testing.T) {
	media := Media{AdBreak: true}
	now := time.Date(2026, 11, 17, 22, 0, 10, 0, time.UTC)
	notice := strings.Join(media.WithAdBreakNotice(now).Header, "\n")
	if !strings.Contains(notice, `START-DATE="2026-11-17T22:00:10Z",END-DATE="2026-11-17T22:00:40Z"`) {
		t.Fatalf("notice %s", notice)
	}
}

func TestNoNoticeOutsideAnAdBreak(t *testing.T) {
	media := Media{Header: []string{"#EXT-X-VERSION:6"}}
	if got := media.WithAdBreakNotice(time.Now()); len(got.Header) != 1 {
		t.Fatalf("header %v", got.Header)
	}
}
