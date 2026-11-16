package live

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/TypeType-Video/TypeType-Live/hls"
	"github.com/TypeType-Video/TypeType-Live/proxy"
	"github.com/TypeType-Video/TypeType-Live/twitch"
)

func (s *Service) Variant(ctx context.Context, values url.Values) ([]byte, error) {
	kind, id, key, q := values.Get("t"), values.Get("c"), values.Get("k"), values.Get("q")
	ttl := livePlaylistTTL
	if kind == KindVOD {
		ttl = vodPlaylistTTL
	}
	ck := "pl/" + kind + "/" + id + "/" + key + "/" + q
	s.metrics.PlaylistServed.Add(1)
	if body, ok := s.playlists.Get(ck); ok {
		return body, nil
	}
	return s.playlistGroup.Do(ctx, ck, func(ctx context.Context) ([]byte, error) {
		body, err := s.renderVariant(ctx, kind, id, key, q)
		if err != nil {
			return nil, err
		}
		s.playlists.Set(ck, body, ttl)
		return body, nil
	})
}

func (s *Service) renderVariant(ctx context.Context, kind, id, key, q string) ([]byte, error) {
	media, target, err := s.fetchVariant(ctx, kind, id, key, q, twitch.DefaultPlayer)
	if err != nil {
		return nil, err
	}
	if kind == KindLive && media.HasAds() {
		s.metrics.AdPlaylists.Add(1)
		if clean, ok := s.adFreeVariant(ctx, kind, id, key, target); ok {
			s.metrics.AdSwaps.Add(1)
			media = clean
		} else {
			s.metrics.AdStripped.Add(1)
			media = media.WithoutAds()
		}
	}
	// Both latency modes of the web player read the prefetch segments: the steady one simply stays further behind.
	if kind == KindLive {
		media = media.WithPrefetchSegments()
	}
	ttl, cacheable := vodSegmentTTL, ""
	if kind == KindLive {
		ttl, cacheable = liveSegmentTTL, "1"
	}
	var buf bytes.Buffer
	media.Write(&buf, func(segment string) string {
		link := url.Values{"x": {s.sealer.Seal(segment)}}
		if cacheable != "" {
			link.Set("m", cacheable)
		}
		return "segment.ts?" + s.signer.Sign(link, ttl).Encode()
	})
	return buf.Bytes(), nil
}

func (s *Service) fetchVariant(ctx context.Context, kind, id, key, q string, player twitch.Player) (hls.Media, hls.Variant, error) {
	for attempt := range 2 {
		st, err := s.masterState(ctx, kind, id, key, player, attempt > 0)
		if err != nil {
			return hls.Media{}, hls.Variant{}, err
		}
		variant, ok := st.master.Find(q)
		if !ok {
			return hls.Media{}, hls.Variant{}, ErrUnknownVariant
		}
		media, err := s.fetchMedia(ctx, variant.URI)
		if err == nil {
			if st.restricted {
				media = media.WithMutedSegments()
			}
			return media, variant, nil
		}
		if !retryable(err) {
			return hls.Media{}, hls.Variant{}, err
		}
	}
	return hls.Media{}, hls.Variant{}, twitch.ErrRestricted
}

func (s *Service) fetchMedia(ctx context.Context, uri string) (hls.Media, error) {
	s.metrics.PlaylistFetches.Add(1)
	body, err := s.relay.Fetch(ctx, uri, maxPlaylistBytes)
	if err != nil {
		s.metrics.UpstreamErrors.Add(1)
		return hls.Media{}, err
	}
	base, _ := url.Parse(uri)
	return hls.ParseMedia(body, base)
}

func (s *Service) adFreeVariant(ctx context.Context, kind, id, key string, target hls.Variant) (hls.Media, bool) {
	for _, player := range s.backupPlayers {
		st, err := s.masterState(ctx, kind, id, key, player, false)
		if err != nil {
			continue
		}
		variant, ok := st.master.Closest(target)
		if !ok {
			continue
		}
		media, err := s.fetchMedia(ctx, variant.URI)
		if err == nil && !media.HasAds() && len(media.Segments) > 0 {
			return media, true
		}
	}
	return hls.Media{}, false
}

func retryable(err error) bool {
	var status *proxy.StatusError
	if errors.As(err, &status) {
		switch status.Status {
		case http.StatusForbidden, http.StatusNotFound, http.StatusGone:
			return true
		}
	}
	return false
}
