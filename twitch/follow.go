package twitch

import (
	"context"
	"fmt"
)

const (
	followUserHash   = "800e7346bdf7e5278a3c1d3f21b2b56e2639928f86815677a7126b093b2fdd08"
	unfollowUserHash = "f7dae976ebf41c755ae2d758546bfd176b4eeb856656098bb40e0a672ca0d880"
)

const querySelfFollowing = `query SelfFollowing($login: String) {
  user(login: $login, lookupType: ALL) { id self {
    follower { followedAt }
    subscriptionBenefit { tier purchasedWithPrime }
    cumulative: subscriptionTenure(tenureMethod: CUMULATIVE) { months }
    streak: subscriptionTenure(tenureMethod: STREAK) { months }
  } }
}`

type FollowState struct {
	ChannelID    string        `json:"channelId"`
	Following    bool          `json:"following"`
	FollowedAt   string        `json:"followedAt,omitempty"`
	Subscription *Subscription `json:"subscription,omitempty"`
}

// Subscription is the viewer's own subscription to a channel, as the Twitch sub button reads it.
type Subscription struct {
	Tier   string `json:"tier"`
	Prime  bool   `json:"prime"`
	Months int    `json:"months"`
	Streak int    `json:"streak"`
}

type tenure struct {
	Months int `json:"months"`
}

func (c *Client) Following(ctx context.Context, userToken, login string) (FollowState, error) {
	var data struct {
		User *struct {
			ID   string `json:"id"`
			Self *struct {
				Follower *struct {
					FollowedAt string `json:"followedAt"`
				} `json:"follower"`
				Benefit *struct {
					Tier  string `json:"tier"`
					Prime bool   `json:"purchasedWithPrime"`
				} `json:"subscriptionBenefit"`
				Cumulative *tenure `json:"cumulative"`
				Streak     *tenure `json:"streak"`
			} `json:"self"`
		} `json:"user"`
	}
	req := gqlRequest{Query: querySelfFollowing, Variables: map[string]any{"login": login}}
	if err := c.gql(ctx, userToken, nil, req, &data); err != nil {
		return FollowState{}, err
	}
	if data.User == nil {
		return FollowState{}, ErrNotFound
	}
	state := FollowState{ChannelID: data.User.ID}
	self := data.User.Self
	if self == nil {
		return state, nil
	}
	if self.Follower != nil && self.Follower.FollowedAt != "" {
		state.Following = true
		state.FollowedAt = self.Follower.FollowedAt
	}
	if self.Benefit != nil {
		state.Subscription = &Subscription{Tier: self.Benefit.Tier, Prime: self.Benefit.Prime}
		if self.Cumulative != nil {
			state.Subscription.Months = self.Cumulative.Months
		}
		if self.Streak != nil {
			state.Subscription.Streak = self.Streak.Months
		}
	}
	return state, nil
}

func (c *Client) Follow(ctx context.Context, userToken, channelID string) error {
	req := persisted("FollowButton_FollowUser", followUserHash, map[string]any{
		"input": map[string]any{"disableNotifications": false, "targetID": channelID},
	})
	var data struct {
		Result *struct {
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"followUser"`
	}
	if err := c.gql(ctx, userToken, nil, req, &data); err != nil {
		return err
	}
	if data.Result == nil {
		return ErrBadToken
	}
	if data.Result.Error != nil && data.Result.Error.Code != "" {
		return &GQLError{Message: fmt.Sprintf("follow refused: %s", data.Result.Error.Code)}
	}
	return nil
}

func (c *Client) Unfollow(ctx context.Context, userToken, channelID string) error {
	req := persisted("FollowButton_UnfollowUser", unfollowUserHash, map[string]any{
		"input": map[string]any{"targetID": channelID},
	})
	var data struct {
		Result *struct {
			Follow *struct{} `json:"follow"`
		} `json:"unfollowUser"`
	}
	if err := c.gql(ctx, userToken, nil, req, &data); err != nil {
		return err
	}
	if data.Result == nil {
		return ErrBadToken
	}
	return nil
}
