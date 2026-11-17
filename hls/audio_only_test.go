package hls

import (
	"net/url"
	"testing"
)

func TestAudioOnlyKeepsTheAudioRendition(t *testing.T) {
	base, _ := url.Parse("https://usher.ttvnw.net/api/v2/channel/hls/test.m3u8")
	master, err := ParseMaster([]byte(masterSample), base)
	if err != nil {
		t.Fatal(err)
	}
	audio := master.AudioOnly()
	if len(audio.Variants) != 1 || audio.Variants[0].ID != "audio_only" {
		t.Fatalf("audio variants %+v", audio.Variants)
	}
	if len(master.Variants) != 3 {
		t.Fatal("the original master changed")
	}
}

func TestAudioOnlyWithoutAudioKeepsTheWholeMaster(t *testing.T) {
	master := Master{Variants: []Variant{{ID: "720p60", Resolution: "1280x720", Codecs: "avc1.4D401F,mp4a.40.2"}}}
	if got := master.AudioOnly(); len(got.Variants) != 1 || got.Variants[0].ID != "720p60" {
		t.Fatalf("fallback %+v", got.Variants)
	}
}
