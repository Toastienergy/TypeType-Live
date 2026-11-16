package twitch

import "context"

const videoChaptersHash = "8d2793384aac3773beab5e59bd5d6f585aedb923d292800119e03d40cd0f9b41"

// VideoChapter is one game segment of a past broadcast, as the Twitch player's chapter list shows it.
type VideoChapter struct {
	Start int    `json:"start"`
	Title string `json:"title"`
	Game  *Game  `json:"game,omitempty"`
}

// VideoChapters returns the game changes of a past broadcast in playback order. An empty userToken reads them
// anonymously.
func (c *Client) VideoChapters(ctx context.Context, userToken, videoID string) ([]VideoChapter, error) {
	var data struct {
		Video *struct {
			Moments *struct {
				Edges []struct {
					Node struct {
						Type           string `json:"type"`
						PositionMillis int    `json:"positionMilliseconds"`
						Description    string `json:"description"`
						Details        *struct {
							Game *struct {
								ID          string `json:"id"`
								DisplayName string `json:"displayName"`
								BoxArtURL   string `json:"boxArtURL"`
							} `json:"game"`
						} `json:"details"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"moments"`
		} `json:"video"`
	}
	vars := map[string]any{"videoID": videoID, "includePrivate": false}
	if err := c.gql(ctx, userToken, nil, persisted("VideoPlayer_ChapterSelectButtonVideo", videoChaptersHash, vars), &data); err != nil {
		return nil, err
	}
	if data.Video == nil {
		return nil, ErrNotFound
	}
	chapters := []VideoChapter{}
	if data.Video.Moments == nil {
		return chapters, nil
	}
	for _, edge := range data.Video.Moments.Edges {
		node := edge.Node
		if node.Type != "GAME_CHANGE" {
			continue
		}
		chapter := VideoChapter{Start: node.PositionMillis / 1000, Title: node.Description}
		if node.Details != nil && node.Details.Game != nil && node.Details.Game.ID != "" {
			game := node.Details.Game
			chapter.Game = &Game{ID: game.ID, Name: game.DisplayName, BoxArt: game.BoxArtURL}
			if chapter.Title == "" {
				chapter.Title = game.DisplayName
			}
		}
		chapters = append(chapters, chapter)
	}
	return chapters, nil
}
