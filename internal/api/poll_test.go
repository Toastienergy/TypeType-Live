package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func (e *env) vote(t *testing.T, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+"/v1/polls/vote", strings.NewReader(body))
	req.Header.Set(secretHeader, testSecret)
	req.Header.Set(tokenHeader, "user-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out)
}

func TestPollVoteSendsTheViewerAndAFreshVoteID(t *testing.T) {
	e := newEnv(t)
	if status, body := e.vote(t, `{"pollId":"p-1","choiceId":"c-2"}`); status != http.StatusNoContent {
		t.Fatalf("vote: %d %s", status, body)
	}
	input, _ := e.fake.LastVote.Load().(map[string]any)
	if input["pollID"] != "p-1" || input["choiceID"] != "c-2" || input["userID"] != "7" {
		t.Fatalf("vote input %+v", input)
	}
	if id, _ := input["voteID"].(string); len(id) != 36 {
		t.Fatalf("vote id %q", id)
	}
}

func TestPollVoteRejectedByTwitch(t *testing.T) {
	e := newEnv(t)
	status, body := e.vote(t, `{"pollId":"p-1","choiceId":"closed"}`)
	var got voteRejectedBody
	if err := json.Unmarshal([]byte(body), &got); err != nil || status != http.StatusForbidden || got.Code != "POLL_NOT_ACTIVE" {
		t.Fatalf("rejected vote: %d %s", status, body)
	}
}

func TestPollVoteChecksTheBody(t *testing.T) {
	e := newEnv(t)
	for _, body := range []string{`{}`, `{"pollId":"p 1","choiceId":"c"}`, `not json`} {
		if status, _ := e.vote(t, body); status != http.StatusBadRequest {
			t.Fatalf("body %s: status %d", body, status)
		}
	}
}
