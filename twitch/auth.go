package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const deviceScopes = "channel_read chat:read chat:edit user_blocks_edit user_blocks_read user_follows_edit user_read"

var (
	ErrAuthPending = errors.New("twitch: authorization pending")
	ErrSlowDown    = errors.New("twitch: slow down")
	ErrCodeExpired = errors.New("twitch: device code expired or denied")
)

type DeviceCode struct {
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	Interval        int    `json:"interval"`
	ExpiresIn       int    `json:"expiresIn"`
}

type Token struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
}

type idResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	Interval        int    `json:"interval"`
	ExpiresIn       int    `json:"expires_in"`
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token"`
	Message         string `json:"message"`
	Status          int    `json:"status"`
}

func (c *Client) StartDevice(ctx context.Context) (DeviceCode, error) {
	resp, err := c.idForm(ctx, "/oauth2/device", url.Values{"client_id": {c.AuthClientID}, "scopes": {deviceScopes}})
	if err != nil {
		return DeviceCode{}, err
	}
	if resp.DeviceCode == "" {
		return DeviceCode{}, fmt.Errorf("twitch: device code: %s", resp.Message)
	}
	return DeviceCode{
		DeviceCode:      resp.DeviceCode,
		UserCode:        resp.UserCode,
		VerificationURI: resp.VerificationURI,
		Interval:        resp.Interval,
		ExpiresIn:       resp.ExpiresIn,
	}, nil
}

func (c *Client) PollDevice(ctx context.Context, deviceCode string) (Token, error) {
	resp, err := c.idForm(ctx, "/oauth2/token", url.Values{
		"client_id":   {c.AuthClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	})
	if err != nil {
		return Token{}, err
	}
	if resp.AccessToken == "" {
		switch msg := strings.ToLower(resp.Message); {
		case strings.Contains(msg, "pending"):
			return Token{}, ErrAuthPending
		case strings.Contains(msg, "slow"):
			return Token{}, ErrSlowDown
		default:
			return Token{}, ErrCodeExpired
		}
	}
	return Token{AccessToken: resp.AccessToken, RefreshToken: resp.RefreshToken, ExpiresIn: resp.ExpiresIn}, nil
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	resp, err := c.idForm(ctx, "/oauth2/token", url.Values{
		"client_id":     {c.AuthClientID},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	})
	if err != nil {
		return Token{}, err
	}
	if resp.AccessToken == "" {
		return Token{}, ErrBadToken
	}
	return Token{AccessToken: resp.AccessToken, RefreshToken: resp.RefreshToken, ExpiresIn: resp.ExpiresIn}, nil
}

func (c *Client) Revoke(ctx context.Context, token string) error {
	_, err := c.idForm(ctx, "/oauth2/revoke", url.Values{"client_id": {c.AuthClientID}, "token": {token}})
	return err
}

func (c *Client) idForm(ctx context.Context, path string, form url.Values) (idResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.IDURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return idResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return idResponse{}, fmt.Errorf("twitch: id request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return idResponse{}, err
	}
	var out idResponse
	if len(raw) > 0 && json.Unmarshal(raw, &out) != nil && resp.StatusCode >= 300 {
		return idResponse{}, fmt.Errorf("twitch: id status %d", resp.StatusCode)
	}
	if resp.StatusCode >= 500 {
		return idResponse{}, fmt.Errorf("twitch: id status %d", resp.StatusCode)
	}
	return out, nil
}
