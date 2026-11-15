package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	DefaultClientID = "kd1unb4b3q4t58fwlpcbzcbnm76a8fp"
	AuthClientID    = "ue6666qo983tsx6so1t0vnawi233wa"
)

var (
	ErrOffline    = errors.New("twitch: channel is offline")
	ErrNotFound   = errors.New("twitch: not found")
	ErrRestricted = errors.New("twitch: content is restricted")
	ErrIntegrity  = errors.New("twitch: integrity check required")
	ErrBadToken   = errors.New("twitch: user token rejected")
)

type GQLError struct {
	Message string
}

func (e *GQLError) Error() string {
	return "twitch: graphql: " + e.Message
}

type Client struct {
	HTTP         *http.Client
	GQLURL       string
	UsherURL     string
	IDURL        string
	ClientID     string
	AuthClientID string
}

func New(httpClient *http.Client, clientID string) *Client {
	if clientID == "" {
		clientID = DefaultClientID
	}
	return &Client{
		HTTP:         httpClient,
		GQLURL:       "https://gql.twitch.tv/gql",
		UsherURL:     "https://usher.ttvnw.net",
		IDURL:        "https://id.twitch.tv",
		ClientID:     clientID,
		AuthClientID: AuthClientID,
	}
}

type gqlRequest struct {
	OperationName string         `json:"operationName,omitempty"`
	Query         string         `json:"query,omitempty"`
	Variables     map[string]any `json:"variables,omitempty"`
	Extensions    *gqlExtensions `json:"extensions,omitempty"`
}

type gqlExtensions struct {
	PersistedQuery gqlPersisted `json:"persistedQuery"`
}

type gqlPersisted struct {
	Version    int    `json:"version"`
	SHA256Hash string `json:"sha256Hash"`
}

func persisted(operation, hash string, variables map[string]any) gqlRequest {
	return gqlRequest{
		OperationName: operation,
		Variables:     variables,
		Extensions:    &gqlExtensions{PersistedQuery: gqlPersisted{Version: 1, SHA256Hash: hash}},
	}
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Client) gql(ctx context.Context, userToken string, headers map[string]string, req gqlRequest, out any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.GQLURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Client-Id", c.ClientID)
	if userToken != "" {
		httpReq.Header.Set("Client-Id", c.AuthClientID)
		httpReq.Header.Set("Authorization", "OAuth "+userToken)
	}
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return fmt.Errorf("twitch: graphql request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("twitch: graphql read: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrBadToken
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("twitch: graphql status %d", resp.StatusCode)
	}
	return decodeGQL(raw, out)
}

func decodeGQL(raw []byte, out any) error {
	var decoded gqlResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return fmt.Errorf("twitch: graphql decode: %w", err)
	}
	if len(decoded.Errors) > 0 {
		msg := decoded.Errors[0].Message
		if strings.Contains(strings.ToLower(msg), "integrity") {
			return ErrIntegrity
		}
		if len(decoded.Data) == 0 || string(decoded.Data) == "null" {
			return &GQLError{Message: msg}
		}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(decoded.Data, out); err != nil {
		return fmt.Errorf("twitch: graphql data: %w", err)
	}
	return nil
}
