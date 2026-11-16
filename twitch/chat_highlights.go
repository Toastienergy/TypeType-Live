package twitch

import "context"

const queryChatHighlights = `query ChatHighlights($login: String) {
  user(login: $login, lookupType: ALL) {
    viewablePoll {
      id title status startedAt endedAt durationSeconds remainingDurationMilliseconds totalVoters
      votes { total }
      choices { id title votes { total } }
    }
  }
  channel(name: $login) {
    pinnedChatMessages(first: 1) {
      edges { node {
        id endsAt
        pinnedBy { displayName }
        pinnedMessage { id content { text } sender { login displayName } }
      } }
    }
  }
}`

// ChatHighlights is what the chat of a live shows above its messages: the running poll and the pinned message.
type ChatHighlights struct {
	Poll   *Poll          `json:"poll"`
	Pinned *PinnedMessage `json:"pinned"`
}

type Poll struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Status      string       `json:"status"`
	StartedAt   string       `json:"startedAt"`
	EndedAt     string       `json:"endedAt,omitempty"`
	Duration    int          `json:"durationSeconds"`
	RemainingMs int          `json:"remainingMs"`
	Voters      int          `json:"voters"`
	Votes       int          `json:"votes"`
	Choices     []PollChoice `json:"choices"`
}

type PollChoice struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Votes int    `json:"votes"`
}

type PinnedMessage struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
	PinnedBy    string `json:"pinnedBy,omitempty"`
	EndsAt      string `json:"endsAt,omitempty"`
}

type votesTotal struct {
	Total int `json:"total"`
}

type gqlPoll struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	StartedAt   string     `json:"startedAt"`
	EndedAt     string     `json:"endedAt"`
	Duration    int        `json:"durationSeconds"`
	RemainingMs int        `json:"remainingDurationMilliseconds"`
	Voters      int        `json:"totalVoters"`
	Votes       votesTotal `json:"votes"`
	Choices     []struct {
		ID    string     `json:"id"`
		Title string     `json:"title"`
		Votes votesTotal `json:"votes"`
	} `json:"choices"`
}

func (p gqlPoll) model() *Poll {
	poll := &Poll{
		ID: p.ID, Title: p.Title, Status: p.Status, StartedAt: p.StartedAt, EndedAt: p.EndedAt,
		Duration: p.Duration, RemainingMs: p.RemainingMs, Voters: p.Voters, Votes: p.Votes.Total,
		Choices: make([]PollChoice, 0, len(p.Choices)),
	}
	for _, choice := range p.Choices {
		poll.Choices = append(poll.Choices, PollChoice{ID: choice.ID, Title: choice.Title, Votes: choice.Votes.Total})
	}
	return poll
}

type gqlPinnedNode struct {
	ID       string `json:"id"`
	EndsAt   string `json:"endsAt"`
	PinnedBy *struct {
		DisplayName string `json:"displayName"`
	} `json:"pinnedBy"`
	Message *struct {
		Content struct {
			Text string `json:"text"`
		} `json:"content"`
		Sender *struct {
			Login       string `json:"login"`
			DisplayName string `json:"displayName"`
		} `json:"sender"`
	} `json:"pinnedMessage"`
}

func (n gqlPinnedNode) model() *PinnedMessage {
	if n.Message == nil || n.Message.Content.Text == "" {
		return nil
	}
	pinned := &PinnedMessage{ID: n.ID, Text: n.Message.Content.Text, EndsAt: n.EndsAt}
	if n.Message.Sender != nil {
		pinned.Login = n.Message.Sender.Login
		pinned.DisplayName = n.Message.Sender.DisplayName
	}
	if n.PinnedBy != nil {
		pinned.PinnedBy = n.PinnedBy.DisplayName
	}
	return pinned
}

func (c *Client) ChatHighlights(ctx context.Context, login string) (ChatHighlights, error) {
	var data struct {
		User *struct {
			Poll *gqlPoll `json:"viewablePoll"`
		} `json:"user"`
		Channel *struct {
			Pinned *struct {
				Edges []struct {
					Node gqlPinnedNode `json:"node"`
				} `json:"edges"`
			} `json:"pinnedChatMessages"`
		} `json:"channel"`
	}
	req := gqlRequest{Query: queryChatHighlights, Variables: map[string]any{"login": login}}
	if err := c.gql(ctx, "", nil, req, &data); err != nil {
		return ChatHighlights{}, err
	}
	if data.User == nil {
		return ChatHighlights{}, ErrNotFound
	}
	var out ChatHighlights
	if data.User.Poll != nil {
		out.Poll = data.User.Poll.model()
	}
	if ch := data.Channel; ch != nil && ch.Pinned != nil && len(ch.Pinned.Edges) > 0 {
		out.Pinned = ch.Pinned.Edges[0].Node.model()
	}
	return out, nil
}
