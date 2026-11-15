package chatassets

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TypeType-Video/TypeType-Live/emotes"
	"github.com/TypeType-Video/TypeType-Live/twitch"
)

type fakeTwitch struct{ badgeCalls atomic.Int64 }

func (f *fakeTwitch) Channel(_ context.Context, login string) (twitch.Channel, error) {
	if login == "missing" {
		return twitch.Channel{}, twitch.ErrNotFound
	}
	return twitch.Channel{User: twitch.User{ID: "42", Login: login}}, nil
}

func (f *fakeTwitch) GlobalBadges(context.Context) ([]twitch.Badge, error) {
	return []twitch.Badge{{SetID: "moderator", Version: "1", Title: "Moderator", URL: "https://static-cdn.jtvnw.net/m"}}, nil
}

func (f *fakeTwitch) ChannelBadges(context.Context, string) ([]twitch.Badge, error) {
	f.badgeCalls.Add(1)
	return []twitch.Badge{{SetID: "subscriber", Version: "3", Title: "3-Month Subscriber", URL: "https://static-cdn.jtvnw.net/s"}}, nil
}

type fakeEmotes struct{ fail bool }

func (f fakeEmotes) Global(context.Context) ([]emotes.Emote, error) {
	return []emotes.Emote{{Name: "KEKW", Provider: "ffz"}}, nil
}

func (f fakeEmotes) Channel(context.Context, string) ([]emotes.Emote, error) {
	list := []emotes.Emote{{Name: "KEKW", Provider: "7tv"}, {Name: "KEKW", Provider: "bttv"}, {Name: "peepoHey", Provider: "7tv"}}
	if f.fail {
		return list[:1], errors.New("bttv down")
	}
	return list, nil
}

func TestChannelAssetsAreCachedAndDeduped(t *testing.T) {
	tw := &fakeTwitch{}
	s := New(tw, fakeEmotes{})
	for range 3 {
		assets, err := s.Channel(context.Background(), "chan")
		if err != nil {
			t.Fatal(err)
		}
		if assets.ChannelID != "42" || len(assets.Emotes) != 2 || assets.Emotes[0].Provider != "7tv" {
			t.Fatalf("assets %+v", assets)
		}
		if assets.Badges["subscriber/3"].Title != "3-Month Subscriber" {
			t.Fatalf("badges %+v", assets.Badges)
		}
	}
	if tw.badgeCalls.Load() != 1 {
		t.Fatalf("badge calls %d", tw.badgeCalls.Load())
	}
}

func TestPartialFailureRetriesSooner(t *testing.T) {
	tw := &fakeTwitch{}
	s := New(tw, fakeEmotes{fail: true})
	now := time.Unix(1000, 0)
	s.Now = func() time.Time { return now }
	if _, err := s.Channel(context.Background(), "chan"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := s.Channel(context.Background(), "chan"); err != nil {
		t.Fatal(err)
	}
	if tw.badgeCalls.Load() != 2 {
		t.Fatalf("expected a retry after the short TTL, got %d calls", tw.badgeCalls.Load())
	}
}

func TestMissingChannelIsAnError(t *testing.T) {
	if _, err := New(&fakeTwitch{}, fakeEmotes{}).Channel(context.Background(), "missing"); !errors.Is(err, twitch.ErrNotFound) {
		t.Fatalf("err %v", err)
	}
}
