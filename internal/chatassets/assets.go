package chatassets

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/TypeType-Video/TypeType-Live/cache"
	"github.com/TypeType-Video/TypeType-Live/emotes"
	"github.com/TypeType-Video/TypeType-Live/twitch"
)

type Badge struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	URL2x string `json:"url2x,omitempty"`
}

type Assets struct {
	ChannelID string           `json:"channelId,omitempty"`
	Badges    map[string]Badge `json:"badges"`
	Emotes    []emotes.Emote   `json:"emotes"`
}

type Twitch interface {
	Channel(ctx context.Context, login string) (twitch.Channel, error)
	GlobalBadges(ctx context.Context) ([]twitch.Badge, error)
	ChannelBadges(ctx context.Context, login string) ([]twitch.Badge, error)
}

type Emotes interface {
	Global(ctx context.Context) ([]emotes.Emote, error)
	Channel(ctx context.Context, channelID string) ([]emotes.Emote, error)
}

type entry struct {
	assets  Assets
	expires time.Time
}

type Service struct {
	twitch     Twitch
	emotes     Emotes
	Now        func() time.Time
	GlobalTTL  time.Duration
	ChannelTTL time.Duration
	RetryTTL   time.Duration

	mu      sync.Mutex
	entries map[string]entry
	group   cache.Group[Assets]
}

const globalKey = "\x00global"

func New(tw Twitch, em Emotes) *Service {
	return &Service{
		twitch:     tw,
		emotes:     em,
		Now:        time.Now,
		GlobalTTL:  time.Hour,
		ChannelTTL: 10 * time.Minute,
		RetryTTL:   time.Minute,
		entries:    make(map[string]entry),
	}
}

func (s *Service) Global(ctx context.Context) (Assets, error) {
	return s.cached(ctx, globalKey, func(ctx context.Context) (Assets, time.Duration, error) {
		badges, badgeErr := s.twitch.GlobalBadges(ctx)
		list, emoteErr := s.emotes.Global(ctx)
		return build("", badges, list), s.ttl(s.GlobalTTL, badgeErr, emoteErr), nil
	})
}

func (s *Service) Channel(ctx context.Context, login string) (Assets, error) {
	return s.cached(ctx, login, func(ctx context.Context) (Assets, time.Duration, error) {
		ch, err := s.twitch.Channel(ctx, login)
		if err != nil {
			return Assets{}, 0, err
		}
		badges, badgeErr := s.twitch.ChannelBadges(ctx, login)
		list, emoteErr := s.emotes.Channel(ctx, ch.ID)
		return build(ch.ID, badges, list), s.ttl(s.ChannelTTL, badgeErr, emoteErr), nil
	})
}

func (s *Service) ttl(normal time.Duration, errs ...error) time.Duration {
	for _, err := range errs {
		if err != nil {
			slog.Debug("chat assets partial", "error", err)
			return s.RetryTTL
		}
	}
	return normal
}

func (s *Service) cached(ctx context.Context, key string, load func(context.Context) (Assets, time.Duration, error)) (Assets, error) {
	s.mu.Lock()
	if e, ok := s.entries[key]; ok && s.Now().Before(e.expires) {
		s.mu.Unlock()
		return e.assets, nil
	}
	s.mu.Unlock()
	return s.group.Do(ctx, key, func(ctx context.Context) (Assets, error) {
		assets, ttl, err := load(ctx)
		if err != nil {
			return Assets{}, err
		}
		s.mu.Lock()
		s.prune()
		s.entries[key] = entry{assets: assets, expires: s.Now().Add(ttl)}
		s.mu.Unlock()
		return assets, nil
	})
}

func (s *Service) prune() {
	now := s.Now()
	for key, e := range s.entries {
		if now.After(e.expires) {
			delete(s.entries, key)
		}
	}
}

func build(channelID string, badges []twitch.Badge, list []emotes.Emote) Assets {
	out := Assets{ChannelID: channelID, Badges: make(map[string]Badge, len(badges)), Emotes: make([]emotes.Emote, 0, len(list))}
	for _, b := range badges {
		out.Badges[b.SetID+"/"+b.Version] = Badge{Title: b.Title, URL: b.URL, URL2x: b.URL2x}
	}
	seen := make(map[string]bool, len(list))
	for _, e := range list {
		if !seen[e.Name] {
			seen[e.Name] = true
			out.Emotes = append(out.Emotes, e)
		}
	}
	return out
}
