package twitch

import (
	"context"
	"strconv"
	"unicode/utf8"
)

const videoCommentsHash = "b70a3591ff0f4e0313d126c6a1502d79a1c02baebb288227c582044aa76adf6a"

// CommentEmote marks an emote by rune positions in Text, the same shape as the live chat (IRC emote tags).
type CommentEmote struct {
	ID    string `json:"id"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// VideoComment is one chat message of a past broadcast, in the shape of a live chat message plus its offset in the
// video, so the web replays it with the live chat renderer.
type VideoComment struct {
	ID          string         `json:"id"`
	Offset      int            `json:"offset"`
	Login       string         `json:"login,omitempty"`
	DisplayName string         `json:"displayName,omitempty"`
	Color       string         `json:"color,omitempty"`
	Text        string         `json:"text"`
	Emotes      []CommentEmote `json:"emotes,omitempty"`
	Badges      []string       `json:"badges,omitempty"`
}

type gqlVideoComment struct {
	Cursor string `json:"cursor"`
	Node   struct {
		ID                   string `json:"id"`
		ContentOffsetSeconds int    `json:"contentOffsetSeconds"`
		Commenter            *struct {
			Login       string `json:"login"`
			DisplayName string `json:"displayName"`
		} `json:"commenter"`
		Message struct {
			UserColor *string `json:"userColor"`
			Fragments []struct {
				Text  string `json:"text"`
				Emote *struct {
					EmoteID string `json:"emoteID"`
				} `json:"emote"`
			} `json:"fragments"`
			UserBadges []struct {
				SetID   string `json:"setID"`
				Version string `json:"version"`
			} `json:"userBadges"`
		} `json:"message"`
	} `json:"node"`
}

func (g gqlVideoComment) model() VideoComment {
	n := g.Node
	out := VideoComment{ID: n.ID, Offset: n.ContentOffsetSeconds}
	if n.Commenter != nil {
		out.Login, out.DisplayName = n.Commenter.Login, n.Commenter.DisplayName
	}
	if n.Message.UserColor != nil {
		out.Color = *n.Message.UserColor
	}
	position := 0
	for _, fragment := range n.Message.Fragments {
		length := utf8.RuneCountInString(fragment.Text)
		if fragment.Emote != nil && fragment.Emote.EmoteID != "" && length > 0 {
			out.Emotes = append(out.Emotes, CommentEmote{ID: fragment.Emote.EmoteID, Start: position, End: position + length - 1})
		}
		out.Text += fragment.Text
		position += length
	}
	for _, badge := range n.Message.UserBadges {
		if badge.SetID != "" {
			out.Badges = append(out.Badges, badge.SetID+"/"+badge.Version)
		}
	}
	return out
}

// VideoComments returns the chat of a past broadcast from offsetSeconds, or from the cursor of a previous page. An
// empty userToken reads it anonymously.
func (c *Client) VideoComments(ctx context.Context, userToken, videoID string, offsetSeconds int, cursor string) (Page[VideoComment], error) {
	vars := map[string]any{"videoID": videoID}
	if cursor != "" {
		vars["cursor"] = cursor
	} else {
		vars["contentOffsetSeconds"] = max(offsetSeconds, 0)
	}
	var data struct {
		Video *struct {
			Comments *struct {
				Edges    []gqlVideoComment `json:"edges"`
				PageInfo struct {
					HasNextPage bool `json:"hasNextPage"`
				} `json:"pageInfo"`
			} `json:"comments"`
		} `json:"video"`
	}
	if err := c.gql(ctx, userToken, nil, persisted("VideoCommentsByOffsetOrCursor", videoCommentsHash, vars), &data); err != nil {
		return Page[VideoComment]{}, err
	}
	if data.Video == nil {
		return Page[VideoComment]{}, ErrNotFound
	}
	page := Page[VideoComment]{Items: []VideoComment{}}
	if data.Video.Comments == nil {
		return page, nil
	}
	for _, edge := range data.Video.Comments.Edges {
		page.Items = append(page.Items, edge.model())
		page.Cursor = edge.Cursor
	}
	page.HasMore = data.Video.Comments.PageInfo.HasNextPage && page.Cursor != ""
	return page, nil
}

// ParseCommentOffset reads the offset query parameter in whole seconds.
func ParseCommentOffset(raw string) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}
	return value
}
