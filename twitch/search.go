package twitch

import (
	"context"
	"net/url"
)

func (c *Client) Clip(ctx context.Context, slug string) (Clip, []ClipQuality, error) {
	var data struct {
		Clip *struct {
			gqlClip
			Assets []struct {
				VideoQualities []struct {
					Quality   string  `json:"quality"`
					FrameRate float64 `json:"frameRate"`
					SourceURL string  `json:"sourceURL"`
				} `json:"videoQualities"`
			} `json:"assets"`
			Token *accessToken `json:"playbackAccessToken"`
		} `json:"clip"`
	}
	if err := c.raw(ctx, "", queryClip, map[string]any{"slug": slug}, &data); err != nil {
		return Clip{}, nil, err
	}
	if data.Clip == nil || data.Clip.Slug == "" {
		return Clip{}, nil, ErrNotFound
	}
	var qualities []ClipQuality
	for _, asset := range data.Clip.Assets {
		for _, q := range asset.VideoQualities {
			if q.SourceURL == "" {
				continue
			}
			source := q.SourceURL
			if data.Clip.Token != nil {
				source += "?" + url.Values{"sig": {data.Clip.Token.Signature}, "token": {data.Clip.Token.Value}}.Encode()
			}
			qualities = append(qualities, ClipQuality{Quality: q.Quality, FrameRate: q.FrameRate, URL: source})
		}
		if len(qualities) > 0 {
			break
		}
	}
	return data.Clip.model(nil), qualities, nil
}

func searchVars(query string, first int, after string) map[string]any {
	vars := pageVars(first, after)
	vars["query"] = query
	return vars
}

func (c *Client) SearchChannels(ctx context.Context, query string, first int, after string) (Page[Channel], error) {
	var data struct {
		Users connection[gqlChannel] `json:"searchUsers"`
	}
	if err := c.raw(ctx, "", querySearchChannels, searchVars(query, first, after), &data); err != nil {
		return Page[Channel]{}, err
	}
	return toPage(data.Users, gqlChannel.model), nil
}

func (c *Client) SearchGames(ctx context.Context, query string, first int, after string) (Page[Game], error) {
	var data struct {
		Games connection[gqlGame] `json:"searchCategories"`
	}
	if err := c.raw(ctx, "", querySearchGames, searchVars(query, first, after), &data); err != nil {
		return Page[Game]{}, err
	}
	return toPage(data.Games, func(g gqlGame) Game { return *g.model() }), nil
}

func (c *Client) SearchStreams(ctx context.Context, query string, first int, after string) (Page[Stream], error) {
	var data struct {
		Streams connection[gqlStream] `json:"searchStreams"`
	}
	if err := c.raw(ctx, "", querySearchStreams, searchVars(query, first, after), &data); err != nil {
		return Page[Stream]{}, err
	}
	return toPage(data.Streams, func(s gqlStream) Stream { return s.model(nil) }), nil
}

func (c *Client) FollowedLive(ctx context.Context, userToken string, first int, after string) (Page[Stream], error) {
	var data struct {
		User *struct {
			Live connection[gqlChannel] `json:"followedLiveUsers"`
		} `json:"user"`
	}
	if err := c.raw(ctx, userToken, queryFollowedLive, pageVars(first, after), &data); err != nil {
		return Page[Stream]{}, err
	}
	if data.User == nil {
		return Page[Stream]{}, ErrBadToken
	}
	page := toPage(data.User.Live, gqlChannel.model)
	out := Page[Stream]{Cursor: page.Cursor, HasMore: page.HasMore, Items: make([]Stream, 0, len(page.Items))}
	for _, ch := range page.Items {
		if ch.Live != nil {
			out.Items = append(out.Items, *ch.Live)
		}
	}
	return out, nil
}

// FollowedVideos lists the past broadcasts of the channels the viewer follows, newest first.
func (c *Client) FollowedVideos(ctx context.Context, userToken string, first int, after string) (Page[Video], error) {
	var data struct {
		User *struct {
			Videos connection[gqlVideo] `json:"followedVideos"`
		} `json:"user"`
	}
	if err := c.raw(ctx, userToken, queryFollowedVideos, pageVars(first, after), &data); err != nil {
		return Page[Video]{}, err
	}
	if data.User == nil {
		return Page[Video]{}, ErrBadToken
	}
	return toPage(data.User.Videos, func(v gqlVideo) Video { return v.model(nil) }), nil
}

func (c *Client) FollowedChannels(ctx context.Context, userToken string, first int, after string) (Page[User], error) {
	var data struct {
		User *struct {
			Follows connection[gqlUser] `json:"follows"`
		} `json:"user"`
	}
	if err := c.raw(ctx, userToken, queryFollowedChannels, pageVars(first, after), &data); err != nil {
		return Page[User]{}, err
	}
	if data.User == nil {
		return Page[User]{}, ErrBadToken
	}
	return toPage(data.User.Follows, gqlUser.model), nil
}

func (c *Client) CurrentUser(ctx context.Context, userToken string) (User, error) {
	var data struct {
		User *gqlUser `json:"currentUser"`
	}
	if err := c.raw(ctx, userToken, queryCurrentUser, nil, &data); err != nil {
		return User{}, err
	}
	if data.User == nil {
		return User{}, ErrBadToken
	}
	return data.User.model(), nil
}
