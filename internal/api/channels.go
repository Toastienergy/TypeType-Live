package api

import (
	"context"
	"sync"
	"time"

	"github.com/TypeType-Video/TypeType-Live/cache"
	"github.com/TypeType-Video/TypeType-Live/twitch"
)

const (
	channelTTL     = 10 * time.Second
	channelEntries = 512
)

type channelEntry struct {
	channel twitch.Channel
	expires time.Time
}

type channelCache struct {
	twitch  *twitch.Client
	now     func() time.Time
	mu      sync.Mutex
	entries map[string]channelEntry
	group   cache.Group[twitch.Channel]
}

func newChannelCache(tw *twitch.Client) *channelCache {
	return &channelCache{twitch: tw, now: time.Now, entries: make(map[string]channelEntry)}
}

func (c *channelCache) Channel(ctx context.Context, login string) (twitch.Channel, error) {
	c.mu.Lock()
	if e, ok := c.entries[login]; ok && c.now().Before(e.expires) {
		c.mu.Unlock()
		return e.channel, nil
	}
	c.mu.Unlock()
	return c.group.Do(ctx, login, func(ctx context.Context) (twitch.Channel, error) {
		ch, err := c.twitch.Channel(ctx, login)
		if err != nil {
			return ch, err
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		now := c.now()
		if len(c.entries) >= channelEntries {
			for key, e := range c.entries {
				if now.After(e.expires) {
					delete(c.entries, key)
				}
			}
		}
		if len(c.entries) < channelEntries {
			c.entries[login] = channelEntry{channel: ch, expires: now.Add(channelTTL)}
		}
		return ch, nil
	})
}
