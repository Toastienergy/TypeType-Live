package twitch

import (
	"context"
	"errors"
)

const sendChatMessageHash = "0435464292cf380ed4b3d905e4edcb73078362e82c06367a5b2181c76c822fa2"

var ErrChatRejected = errors.New("twitch: chat message rejected")

type ChatRejectedError struct {
	Reason string
}

func (e *ChatRejectedError) Error() string { return "twitch: chat message rejected: " + e.Reason }

func (e *ChatRejectedError) Unwrap() error { return ErrChatRejected }

func (c *Client) SendChatMessage(ctx context.Context, userToken, channelID, message string) error {
	req := persisted("sendChatMessage", sendChatMessageHash, map[string]any{
		"input": map[string]any{"channelID": channelID, "message": message},
	})
	var data struct {
		Send *struct {
			DropReason *string `json:"dropReason"`
		} `json:"sendChatMessage"`
	}
	if err := c.gql(ctx, userToken, nil, req, &data); err != nil {
		return err
	}
	if data.Send == nil {
		return ErrBadToken
	}
	if data.Send.DropReason != nil && *data.Send.DropReason != "" {
		return &ChatRejectedError{Reason: *data.Send.DropReason}
	}
	return nil
}
