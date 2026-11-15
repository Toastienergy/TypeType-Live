package live

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/TypeType-Video/TypeType-Live/hls"
	"github.com/TypeType-Video/TypeType-Live/twitch"
)

const fallbackMasterTTL = time.Hour

type fallbackQuality struct {
	id         string
	resolution string
	bandwidth  int64
	frameRate  float64
}

var fallbackQualities = []fallbackQuality{
	{"chunked", "", 8_000_000, 60},
	{"1080p60", "1920x1080", 6_000_000, 60},
	{"1080p30", "1920x1080", 4_500_000, 30},
	{"720p60", "1280x720", 3_500_000, 60},
	{"720p30", "1280x720", 2_500_000, 30},
	{"480p30", "852x480", 1_400_000, 30},
	{"360p30", "640x360", 700_000, 30},
	{"160p30", "284x160", 250_000, 30},
	{"audio_only", "", 160_000, 0},
}

func storyboardPlaylists(v twitch.Video) map[string]string {
	prefix, _, ok := strings.Cut(v.Storyboard, "/storyboards/")
	if !ok || prefix == "" {
		return nil
	}
	file := "index-dvr.m3u8"
	if strings.EqualFold(v.Type, "highlight") {
		file = "highlight-" + v.ID + ".m3u8"
	}
	out := make(map[string]string, len(fallbackQualities))
	for _, q := range fallbackQualities {
		out[q.id] = prefix + "/" + q.id + "/" + file
	}
	return out
}

func (q fallbackQuality) info() string {
	codecs := "avc1.4D401F,mp4a.40.2"
	if q.id == "audio_only" {
		codecs = "mp4a.40.2"
	}
	info := fmt.Sprintf(`#EXT-X-STREAM-INF:BANDWIDTH=%d,CODECS="%s"`, q.bandwidth, codecs)
	if q.resolution != "" {
		info += ",RESOLUTION=" + q.resolution
	}
	if q.frameRate > 0 {
		info += fmt.Sprintf(",FRAME-RATE=%.3f", q.frameRate)
	}
	return info + fmt.Sprintf(`,STABLE-VARIANT-ID="%s",IVS-NAME="%s"`, q.id, q.id)
}

func (s *Service) restrictedVODMaster(ctx context.Context, id string) (*masterState, error) {
	video, err := s.twitch.Video(ctx, id)
	if err != nil {
		return nil, err
	}
	playlists := storyboardPlaylists(video)
	if playlists == nil {
		return nil, twitch.ErrRestricted
	}
	found := make([]bool, len(fallbackQualities))
	var wg sync.WaitGroup
	for i, q := range fallbackQualities {
		wg.Go(func() { found[i] = s.relay.Exists(ctx, playlists[q.id]) })
	}
	wg.Wait()
	var master hls.Master
	for i, q := range fallbackQualities {
		if found[i] {
			master.Variants = append(master.Variants, hls.NewVariant(q.info(), playlists[q.id]))
		}
	}
	if len(master.Variants) == 0 {
		return nil, twitch.ErrRestricted
	}
	s.metrics.RestrictedVODs.Add(1)
	return &masterState{master: master, expires: s.now().Add(fallbackMasterTTL), restricted: true}, nil
}
