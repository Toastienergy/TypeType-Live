package twitch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFollowedVideosKeepEachOwner(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "types: [ARCHIVE]") || r.Header.Get("Authorization") != "OAuth viewer" {
			t.Errorf("request %s %q", body, r.Header.Get("Authorization"))
		}
		io.WriteString(w, `{"data":{"user":{"followedVideos":{"edges":[
			{"cursor":"c1","node":{"id":"v1","title":"Night run","lengthSeconds":3600,"broadcastType":"ARCHIVE",
			"createdAt":"2026-11-16T20:00:00Z","owner":{"id":"1","login":"alice","displayName":"Alice"}}},
			{"cursor":"c2","node":{"id":"v2","title":"Morning","lengthSeconds":60,"broadcastType":"ARCHIVE",
			"createdAt":"2026-11-16T08:00:00Z","owner":{"id":"2","login":"bob","displayName":"Bob"}}}],
			"pageInfo":{"hasNextPage":true}}}}}`)
	})
	page, err := c.FollowedVideos(context.Background(), "viewer", 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || !page.HasMore || page.Cursor != "c2" {
		t.Fatalf("page %+v", page)
	}
	if page.Items[0].Owner.Login != "alice" || page.Items[1].Owner.Login != "bob" {
		t.Fatalf("owners %+v %+v", page.Items[0].Owner, page.Items[1].Owner)
	}
}

func TestFollowedVideosWithoutViewer(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"data":{"user":null}}`)
	})
	if _, err := c.FollowedVideos(context.Background(), "gone", 2, ""); !errors.Is(err, ErrBadToken) {
		t.Fatalf("err %v", err)
	}
}
