package emotes

import (
	"context"
	"strconv"
	"strings"
)

type sevenTVSet struct {
	Emotes []struct {
		Name  string `json:"name"`
		Flags int    `json:"flags"`
		Data  struct {
			Animated bool `json:"animated"`
			Host     struct {
				URL string `json:"url"`
			} `json:"host"`
		} `json:"data"`
	} `json:"emotes"`
}

func (s sevenTVSet) emotes() []Emote {
	out := make([]Emote, 0, len(s.Emotes))
	for _, e := range s.Emotes {
		base := e.Data.Host.URL
		if e.Name == "" || base == "" {
			continue
		}
		if strings.HasPrefix(base, "//") {
			base = "https:" + base
		}
		out = append(out, Emote{
			Name:      e.Name,
			URL:       base + "/1x.webp",
			URL2x:     base + "/2x.webp",
			Provider:  "7tv",
			Animated:  e.Data.Animated,
			ZeroWidth: e.Flags&1 == 1,
		})
	}
	return out
}

func (c *Client) sevenTVGlobal(ctx context.Context) ([]Emote, error) {
	var set sevenTVSet
	if err := c.getJSON(ctx, c.SevenTVURL+"/v3/emote-sets/global", &set); err != nil {
		return nil, err
	}
	return set.emotes(), nil
}

func (c *Client) sevenTVChannel(ctx context.Context, channelID string) ([]Emote, error) {
	var user struct {
		EmoteSet *sevenTVSet `json:"emote_set"`
	}
	if err := c.getJSON(ctx, c.SevenTVURL+"/v3/users/twitch/"+channelID, &user); err != nil {
		return nil, err
	}
	if user.EmoteSet == nil {
		return nil, nil
	}
	return user.EmoteSet.emotes(), nil
}

type bttvEmote struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	ImageType string `json:"imageType"`
	Animated  bool   `json:"animated"`
}

func bttvEmotes(list []bttvEmote) []Emote {
	out := make([]Emote, 0, len(list))
	for _, e := range list {
		if e.ID == "" || e.Code == "" {
			continue
		}
		base := "https://cdn.betterttv.net/emote/" + e.ID
		out = append(out, Emote{
			Name:     e.Code,
			URL:      base + "/1x.webp",
			URL2x:    base + "/2x.webp",
			Provider: "bttv",
			Animated: e.Animated || e.ImageType == "gif",
		})
	}
	return out
}

func (c *Client) bttvGlobal(ctx context.Context) ([]Emote, error) {
	var list []bttvEmote
	if err := c.getJSON(ctx, c.BTTVURL+"/3/cached/emotes/global", &list); err != nil {
		return nil, err
	}
	return bttvEmotes(list), nil
}

func (c *Client) bttvChannel(ctx context.Context, channelID string) ([]Emote, error) {
	var user struct {
		Channel []bttvEmote `json:"channelEmotes"`
		Shared  []bttvEmote `json:"sharedEmotes"`
	}
	if err := c.getJSON(ctx, c.BTTVURL+"/3/cached/users/twitch/"+channelID, &user); err != nil {
		return nil, err
	}
	return bttvEmotes(append(user.Channel, user.Shared...)), nil
}

type ffzURLs struct {
	One  string `json:"1"`
	Two  string `json:"2"`
	Four string `json:"4"`
}

type ffzSet struct {
	Emoticons []struct {
		Name     string   `json:"name"`
		URLs     ffzURLs  `json:"urls"`
		Animated *ffzURLs `json:"animated"`
	} `json:"emoticons"`
}

func ffzEmotes(sets map[string]ffzSet, keep func(string) bool) []Emote {
	var out []Emote
	for id, set := range sets {
		if !keep(id) {
			continue
		}
		for _, e := range set.Emoticons {
			urls, animated := e.URLs, false
			if e.Animated != nil && e.Animated.One != "" {
				urls, animated = *e.Animated, true
			}
			if e.Name == "" || urls.One == "" {
				continue
			}
			emote := Emote{Name: e.Name, URL: urls.One, URL2x: urls.Two, Provider: "ffz", Animated: animated}
			if animated {
				emote.URL, emote.URL2x = urls.One+".webp", suffixed(urls.Two, ".webp")
			}
			out = append(out, emote)
		}
	}
	return out
}

func suffixed(url, suffix string) string {
	if url == "" {
		return ""
	}
	return url + suffix
}

func (c *Client) ffzGlobal(ctx context.Context) ([]Emote, error) {
	var global struct {
		DefaultSets []int             `json:"default_sets"`
		Sets        map[string]ffzSet `json:"sets"`
	}
	if err := c.getJSON(ctx, c.FFZURL+"/v1/set/global", &global); err != nil {
		return nil, err
	}
	defaults := make(map[string]bool, len(global.DefaultSets))
	for _, id := range global.DefaultSets {
		defaults[strconv.Itoa(id)] = true
	}
	return ffzEmotes(global.Sets, func(id string) bool { return defaults[id] }), nil
}

func (c *Client) ffzChannel(ctx context.Context, channelID string) ([]Emote, error) {
	var room struct {
		Sets map[string]ffzSet `json:"sets"`
	}
	if err := c.getJSON(ctx, c.FFZURL+"/v1/room/id/"+channelID, &room); err != nil {
		return nil, err
	}
	return ffzEmotes(room.Sets, func(string) bool { return true }), nil
}
