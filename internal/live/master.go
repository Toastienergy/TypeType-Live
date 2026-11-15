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

func (s *Service) Master(ctx context.Context, values url.Values) ([]byte, error) {
	kind, id, key := values.Get("t"), values.Get("c"), values.Get("k")
	st, err := s.masterState(ctx, kind, id, key, twitch.DefaultPlayer, false)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	st.master.Write(&buf, func(v hls.Variant) string {
		link := url.Values{"t": {kind}, "c": {id}, "q": {v.ID}}
		if key != "" {
			link.Set("k", key)
		}
		return "variant.m3u8?" + s.signer.Sign(link, variantLinkTTL).Encode()
	})
	return buf.Bytes(), nil
}

func (s *Service) masterState(ctx context.Context, kind, id, key string, player twitch.Player, force bool) (*masterState, error) {
	userToken, ok := s.sessions.get(key)
	if !ok {
		return nil, ErrUnknownSession
	}
	mk := masterKey(kind, id, key) + "/" + player.Platform + "/" + player.Type
	if !force {
		s.mu.Lock()
		st := s.masters[mk]
		s.mu.Unlock()
		if st != nil && s.now().Before(st.expires) {
			return st, nil
		}
	}
	return s.masterGroup.Do(ctx, mk, func(ctx context.Context) (*masterState, error) {
		st, err := s.fetchMaster(ctx, kind, id, userToken, player)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		now := s.now()
		for k, v := range s.masters {
			if now.After(v.expires) {
				delete(s.masters, k)
			}
		}
		s.masters[mk] = st
		s.mu.Unlock()
		return st, nil
	})
}

func (s *Service) fetchMaster(ctx context.Context, kind, id, userToken string, player twitch.Player) (*masterState, error) {
	s.metrics.MasterFetches.Add(1)
	var (
		token     twitch.PlaybackToken
		err       error
		masterURL string
	)
	switch kind {
	case KindLive:
		token, err = s.twitch.LiveToken(ctx, id, player, userToken)
		if err == nil {
			masterURL = s.twitch.LiveMasterURL(id, token, player.Platform)
		}
	case KindVOD:
		token, err = s.twitch.VODToken(ctx, id, player, userToken)
		if err == nil {
			masterURL = s.twitch.VODMasterURL(id, token, player.Platform)
		}
	default:
		return nil, twitch.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	body, err := s.relay.Fetch(ctx, masterURL, maxPlaylistBytes)
	if err != nil {
		return nil, upstreamError(err, kind)
	}
	base, _ := url.Parse(masterURL)
	master, err := hls.ParseMaster(body, base)
	if err != nil {
		return nil, err
	}
	if len(master.Variants) == 0 {
		return nil, twitch.ErrRestricted
	}
	return &masterState{master: master, expires: token.Expires.Add(-tokenSafety)}, nil
}

func upstreamError(err error, kind string) error {
	var status *proxy.StatusError
	if !errors.As(err, &status) {
		return err
	}
	switch status.Status {
	case http.StatusNotFound:
		if kind == KindLive {
			return twitch.ErrOffline
		}
		return twitch.ErrNotFound
	case http.StatusForbidden:
		return twitch.ErrRestricted
	}
	return err
}
