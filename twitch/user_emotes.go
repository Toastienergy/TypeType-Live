package twitch

import "context"

const availableEmotesHash = "6c45e0ecaa823cc7db3ecdd1502af2223c775bdcfb0f18a3a0ce9a0b7db8ef6c"

const (
	emotePageLimit = 350
	emoteMaxPages  = 12
)

type UserEmote struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	OwnerID string `json:"ownerId,omitempty"`
}

type gqlEmoteSets struct {
	Edges []struct {
		Cursor string `json:"cursor"`
		Node   struct {
			Emotes []*struct {
				ID    string `json:"id"`
				Token string `json:"token"`
				Type  string `json:"type"`
			} `json:"emotes"`
			Owner *struct {
				ID string `json:"id"`
			} `json:"owner"`
		} `json:"node"`
	} `json:"edges"`
	PageInfo *struct {
		HasNextPage bool `json:"hasNextPage"`
	} `json:"pageInfo"`
}

func (c *Client) AvailableEmotes(ctx context.Context, userToken, channelID string) ([]UserEmote, error) {
	out := []UserEmote{}
	seen := map[string]bool{}
	var cursor any
	for range emoteMaxPages {
		sets, err := c.emoteSetsPage(ctx, userToken, channelID, cursor)
		if err != nil {
			return nil, err
		}
		for _, edge := range sets.Edges {
			cursor = edge.Cursor
			owner := ""
			if edge.Node.Owner != nil {
				owner = edge.Node.Owner.ID
			}
			for _, e := range edge.Node.Emotes {
				if e == nil || e.ID == "" || e.Token == "" || seen[e.ID] {
					continue
				}
				seen[e.ID] = true
				out = append(out, UserEmote{ID: e.ID, Name: e.Token, Type: e.Type, OwnerID: owner})
			}
		}
		if len(sets.Edges) == 0 || sets.PageInfo == nil || !sets.PageInfo.HasNextPage {
			break
		}
	}
	return out, nil
}

func (c *Client) emoteSetsPage(ctx context.Context, userToken, channelID string, cursor any) (*gqlEmoteSets, error) {
	req := persisted("AvailableEmotesForChannelPaginated", availableEmotesHash, map[string]any{
		"cursor": cursor, "channelID": channelID, "pageLimit": emotePageLimit, "withOwner": true,
	})
	var data struct {
		Channel *struct {
			Self *struct {
				Sets *gqlEmoteSets `json:"availableEmoteSetsPaginated"`
			} `json:"self"`
		} `json:"channel"`
	}
	if err := c.gql(ctx, userToken, nil, req, &data); err != nil {
		return nil, err
	}
	if data.Channel == nil || data.Channel.Self == nil || data.Channel.Self.Sets == nil {
		return nil, ErrBadToken
	}
	return data.Channel.Self.Sets, nil
}
