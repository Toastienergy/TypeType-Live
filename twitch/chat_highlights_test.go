package twitch

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func TestChatHighlightsReadThePollAndThePinnedMessage(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"data":{"user":{"viewablePoll":{"id":"p1","title":"Best map?","status":"ACTIVE",
			"startedAt":"2026-11-16T20:00:00Z","endedAt":null,"durationSeconds":120,"remainingDurationMilliseconds":45000,
			"totalVoters":30,"votes":{"total":31},"choices":[{"id":"a","title":"Dust","votes":{"total":20}},
			{"id":"b","title":"Mirage","votes":{"total":11}}]}},
			"channel":{"pinnedChatMessages":{"edges":[{"node":{"id":"n1","endsAt":null,"pinnedBy":{"displayName":"Mod"},
			"pinnedMessage":{"id":"m1","content":{"text":"Giveaway at 9"},"sender":{"login":"host","displayName":"Host"}}}}]}}}}`)
	})
	got, err := c.ChatHighlights(context.Background(), "somechannel")
	if err != nil {
		t.Fatal(err)
	}
	wantPoll := &Poll{
		ID: "p1", Title: "Best map?", Status: "ACTIVE", StartedAt: "2026-11-16T20:00:00Z", Duration: 120,
		RemainingMs: 45000, Voters: 30, Votes: 31,
		Choices: []PollChoice{{ID: "a", Title: "Dust", Votes: 20}, {ID: "b", Title: "Mirage", Votes: 11}},
	}
	if !reflect.DeepEqual(got.Poll, wantPoll) {
		t.Fatalf("poll %+v", got.Poll)
	}
	wantPinned := &PinnedMessage{ID: "n1", Text: "Giveaway at 9", Login: "host", DisplayName: "Host", PinnedBy: "Mod"}
	if !reflect.DeepEqual(got.Pinned, wantPinned) {
		t.Fatalf("pinned %+v", got.Pinned)
	}
}

func TestChatHighlightsEmpty(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"data":{"user":{"viewablePoll":null},"channel":{"pinnedChatMessages":{"edges":[]}}}}`)
	})
	got, err := c.ChatHighlights(context.Background(), "somechannel")
	if err != nil || got.Poll != nil || got.Pinned != nil {
		t.Fatalf("got %+v err %v", got, err)
	}
	c = fakeClient(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"data":{"user":null,"channel":null}}`)
	})
	if _, err := c.ChatHighlights(context.Background(), "missing"); err != ErrNotFound {
		t.Fatalf("err %v", err)
	}
}
