package twitch

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
)

const mutationVoteInPoll = `mutation VoteInPoll($input: VoteInPollInput!) {
  voteInPoll(input: $input) { error { code } }
}`

var ErrVoteRejected = errors.New("twitch: poll vote rejected")

type VoteRejectedError struct {
	Code string
}

func (e *VoteRejectedError) Error() string { return "twitch: poll vote rejected: " + e.Code }

func (e *VoteRejectedError) Unwrap() error { return ErrVoteRejected }

// VoteInPoll casts the viewer's free vote; Twitch dedupes on voteID, so each call draws a new one.
func (c *Client) VoteInPoll(ctx context.Context, userToken, userID, pollID, choiceID string) error {
	input := map[string]any{"pollID": pollID, "choiceID": choiceID, "userID": userID, "voteID": voteID()}
	var data struct {
		Vote *struct {
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"voteInPoll"`
	}
	if err := c.raw(ctx, userToken, mutationVoteInPoll, map[string]any{"input": input}, &data); err != nil {
		return err
	}
	if data.Vote == nil {
		return ErrBadToken
	}
	if data.Vote.Error != nil && data.Vote.Error.Code != "" {
		return &VoteRejectedError{Code: data.Vote.Error.Code}
	}
	return nil
}

func voteID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
