package twitch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

const playbackAccessTokenHash = "ed230aa1e33e07eebb8928504583da78a5173989fadfb1ac94be06a04f3cdbe9"

var (
	loginPattern = regexp.MustCompile(`^[a-zA-Z0-9_]{1,25}$`)
	idPattern    = regexp.MustCompile(`^[0-9]{1,20}$`)
)

func ValidLogin(login string) bool { return loginPattern.MatchString(login) }

func ValidID(id string) bool { return idPattern.MatchString(id) }

type Player struct {
	Type     string
	Platform string
}

var DefaultPlayer = Player{Type: "site", Platform: "web"}

type PlaybackToken struct {
	Value      string
	Signature  string
	Expires    time.Time
	ServerAds  bool
	HideAds    bool
	Subscriber bool
}

type tokenValue struct {
	Expires    int64  `json:"expires"`
	ServerAds  bool   `json:"server_ads"`
	HideAds    bool   `json:"hide_ads"`
	Subscriber bool   `json:"subscriber"`
	Geoblock   string `json:"geoblock_reason"`
}

type accessToken struct {
	Value     string `json:"value"`
	Signature string `json:"signature"`
}

func (c *Client) LiveToken(ctx context.Context, login string, player Player, userToken string) (PlaybackToken, error) {
	return c.playbackToken(ctx, login, "", player, userToken)
}

func (c *Client) VODToken(ctx context.Context, id string, player Player, userToken string) (PlaybackToken, error) {
	return c.playbackToken(ctx, "", id, player, userToken)
}

func (c *Client) playbackToken(ctx context.Context, login, vod string, player Player, userToken string) (PlaybackToken, error) {
	req := persisted("PlaybackAccessToken", playbackAccessTokenHash, map[string]any{
		"isLive":     login != "",
		"login":      login,
		"isVod":      vod != "",
		"vodID":      vod,
		"playerType": player.Type,
		"platform":   player.Platform,
	})
	var data struct {
		Stream *accessToken `json:"streamPlaybackAccessToken"`
		Video  *accessToken `json:"videoPlaybackAccessToken"`
	}
	headers := map[string]string{"X-Device-Id": deviceID()}
	if err := c.gql(ctx, userToken, headers, req, &data); err != nil {
		return PlaybackToken{}, err
	}
	token := data.Stream
	if vod != "" {
		token = data.Video
	}
	if token == nil || token.Value == "" {
		return PlaybackToken{}, ErrNotFound
	}
	var value tokenValue
	if err := json.Unmarshal([]byte(token.Value), &value); err != nil {
		return PlaybackToken{}, err
	}
	if value.Geoblock != "" {
		return PlaybackToken{}, ErrRestricted
	}
	return PlaybackToken{
		Value:      token.Value,
		Signature:  token.Signature,
		Expires:    time.Unix(value.Expires, 0),
		ServerAds:  value.ServerAds,
		HideAds:    value.HideAds,
		Subscriber: value.Subscriber,
	}, nil
}

func (c *Client) LiveMasterURL(login string, token PlaybackToken, platform string) string {
	q := c.usherQuery(token, platform)
	q.Set("fast_bread", "true")
	return c.UsherURL + "/api/v2/channel/hls/" + url.PathEscape(login) + ".m3u8?" + q.Encode()
}

func (c *Client) VODMasterURL(id string, token PlaybackToken, platform string) string {
	return c.UsherURL + "/vod/v2/" + url.PathEscape(id) + ".m3u8?" + c.usherQuery(token, platform).Encode()
}

func (c *Client) usherQuery(token PlaybackToken, platform string) url.Values {
	n, _ := rand.Int(rand.Reader, big.NewInt(9_999_999))
	return url.Values{
		"allow_source":        {"true"},
		"allow_audio_only":    {"true"},
		"include_unavailable": {"true"},
		"p":                   {strconv.FormatInt(n.Int64(), 10)},
		"platform":            {platform},
		"sig":                 {token.Signature},
		"token":               {token.Value},
	}
}

func deviceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
