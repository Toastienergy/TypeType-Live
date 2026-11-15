package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/TypeType-Video/TypeType-Live/twitch"
)

func (e *env) follow(t *testing.T, method string) twitch.FollowState {
	t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+"/v1/me/follows/somechannel", nil)
	req.Header.Set(secretHeader, testSecret)
	req.Header.Set(tokenHeader, "user-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s follow: %d %s", method, resp.StatusCode, body)
	}
	var state twitch.FollowState
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestFollowAndUnfollow(t *testing.T) {
	e := newEnv(t)
	if state := e.follow(t, http.MethodGet); state.Following || state.ChannelID != "42" {
		t.Fatalf("initial state %+v", state)
	}
	if state := e.follow(t, http.MethodPut); !state.Following || state.FollowedAt == "" {
		t.Fatalf("after follow %+v", state)
	}
	if state := e.follow(t, http.MethodDelete); state.Following {
		t.Fatalf("after unfollow %+v", state)
	}
}

func TestFollowNeedsUserToken(t *testing.T) {
	e := newEnv(t)
	if status, _ := e.get(t, "/v1/me/follows/somechannel", true); status != http.StatusUnauthorized {
		t.Fatalf("status %d", status)
	}
}

func TestChatAssetsAnswerEvenWhenProvidersFail(t *testing.T) {
	e := newEnv(t)
	status, body := e.get(t, "/v1/chat/somechannel/assets", true)
	if status != http.StatusOK || !strings.Contains(body, `"channelId":"42"`) {
		t.Fatalf("channel assets %d %s", status, body)
	}
	if status, _ := e.get(t, "/v1/chat/assets", true); status != http.StatusOK {
		t.Fatalf("global assets %d", status)
	}
}
