package twitch

import "context"

type Badge struct {
	SetID   string `json:"setId"`
	Version string `json:"version"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	URL2x   string `json:"url2x,omitempty"`
}

const badgeFields = `setID version title url: imageURL(size: NORMAL) url2x: imageURL(size: DOUBLE)`

const queryGlobalBadges = `query GlobalBadges { badges { ` + badgeFields + ` } }`

const queryChannelBadges = `query ChannelBadges($login: String) {
  user(login: $login, lookupType: ALL) { broadcastBadges { ` + badgeFields + ` } }
}`

type gqlBadge struct {
	SetID   string `json:"setID"`
	Version string `json:"version"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	URL2x   string `json:"url2x"`
}

func badges(list []*gqlBadge) []Badge {
	out := make([]Badge, 0, len(list))
	for _, b := range list {
		if b == nil || b.SetID == "" {
			continue
		}
		url := image(b.URL)
		if url == "" {
			continue
		}
		out = append(out, Badge{SetID: b.SetID, Version: b.Version, Title: b.Title, URL: url, URL2x: image(b.URL2x)})
	}
	return out
}

func (c *Client) GlobalBadges(ctx context.Context) ([]Badge, error) {
	var data struct {
		Badges []*gqlBadge `json:"badges"`
	}
	if err := c.raw(ctx, "", queryGlobalBadges, nil, &data); err != nil {
		return nil, err
	}
	return badges(data.Badges), nil
}

func (c *Client) ChannelBadges(ctx context.Context, login string) ([]Badge, error) {
	var data struct {
		User *struct {
			Badges []*gqlBadge `json:"broadcastBadges"`
		} `json:"user"`
	}
	if err := c.raw(ctx, "", queryChannelBadges, map[string]any{"login": login}, &data); err != nil {
		return nil, err
	}
	if data.User == nil {
		return nil, ErrNotFound
	}
	return badges(data.User.Badges), nil
}
