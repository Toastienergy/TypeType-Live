package twitch

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestFollowingReadsTheViewerSubscription(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"data":{"user":{"id":"42","self":{"follower":{"followedAt":"2025-01-02T03:04:05Z"},
			"subscriptionBenefit":{"tier":"1000","purchasedWithPrime":true},
			"cumulative":{"months":14},"streak":{"months":6}}}}}`)
	})
	state, err := c.Following(context.Background(), "user-token", "somechannel")
	if err != nil {
		t.Fatal(err)
	}
	want := Subscription{Tier: "1000", Prime: true, Months: 14, Streak: 6}
	if !state.Following || state.Subscription == nil || *state.Subscription != want {
		t.Fatalf("state %+v sub %+v", state, state.Subscription)
	}
}

func TestFollowingWithoutSubscription(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"data":{"user":{"id":"42","self":{"follower":null,"subscriptionBenefit":null,
			"cumulative":{"months":0},"streak":null}}}}`)
	})
	state, err := c.Following(context.Background(), "user-token", "somechannel")
	if err != nil || state.Following || state.Subscription != nil {
		t.Fatalf("state %+v err %v", state, err)
	}
}
