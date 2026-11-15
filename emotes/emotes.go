package emotes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type Emote struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	URL2x     string `json:"url2x,omitempty"`
	Provider  string `json:"provider"`
	Animated  bool   `json:"animated,omitempty"`
	ZeroWidth bool   `json:"zeroWidth,omitempty"`
}

type Client struct {
	HTTP       *http.Client
	SevenTVURL string
	BTTVURL    string
	FFZURL     string
}

func New(httpClient *http.Client) *Client {
	return &Client{
		HTTP:       httpClient,
		SevenTVURL: "https://7tv.io",
		BTTVURL:    "https://api.betterttv.net",
		FFZURL:     "https://api.frankerfacez.com",
	}
}

var ErrNoEmotes = errors.New("emotes: channel has no emotes on this provider")

type source func(context.Context) ([]Emote, error)

func (c *Client) Global(ctx context.Context) ([]Emote, error) {
	return collect(ctx, c.sevenTVGlobal, c.bttvGlobal, c.ffzGlobal)
}

func (c *Client) Channel(ctx context.Context, channelID string) ([]Emote, error) {
	return collect(ctx,
		func(ctx context.Context) ([]Emote, error) { return c.sevenTVChannel(ctx, channelID) },
		func(ctx context.Context) ([]Emote, error) { return c.bttvChannel(ctx, channelID) },
		func(ctx context.Context) ([]Emote, error) { return c.ffzChannel(ctx, channelID) },
	)
}

func collect(ctx context.Context, sources ...source) ([]Emote, error) {
	type result struct {
		emotes []Emote
		err    error
	}
	results := make([]chan result, len(sources))
	for i, src := range sources {
		results[i] = make(chan result, 1)
		go func() {
			list, err := src(ctx)
			results[i] <- result{list, err}
		}()
	}
	var all []Emote
	var errs []error
	for _, ch := range results {
		r := <-ch
		all = append(all, r.emotes...)
		if r.err != nil && !errors.Is(r.err, ErrNoEmotes) {
			errs = append(errs, r.err)
		}
	}
	return all, errors.Join(errs...)
}

func (c *Client) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("emotes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNoEmotes
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("emotes: %s answered %d", req.URL.Host, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
		return fmt.Errorf("emotes: decode %s: %w", req.URL.Host, err)
	}
	return nil
}
