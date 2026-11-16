package twitch

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func TestVideoChaptersKeepTheGameChanges(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":{"video":{"moments":{"edges":[
			{"node":{"type":"GAME_CHANGE","positionMilliseconds":0,"description":"Just Chatting","details":{"game":{"id":"509658","displayName":"Just Chatting","boxArtURL":"https://a/1.jpg"}}}},
			{"node":{"type":"OTHER","positionMilliseconds":500000,"description":"skip"}},
			{"node":{"type":"GAME_CHANGE","positionMilliseconds":10839000,"description":"","details":{"game":{"id":"33214","displayName":"Fortnite","boxArtURL":"https://a/2.jpg"}}}}]}}}}`)
	})
	chapters, err := c.VideoChapters(context.Background(), "", "2929630714")
	if err != nil {
		t.Fatal(err)
	}
	want := []VideoChapter{
		{Start: 0, Title: "Just Chatting", Game: &Game{ID: "509658", Name: "Just Chatting", BoxArt: "https://a/1.jpg"}},
		{Start: 10839, Title: "Fortnite", Game: &Game{ID: "33214", Name: "Fortnite", BoxArt: "https://a/2.jpg"}},
	}
	if !reflect.DeepEqual(chapters, want) {
		t.Fatalf("chapters %+v", chapters)
	}
}

func TestVideoChaptersOfAMissingVideo(t *testing.T) {
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":{"video":null}}`)
	})
	if _, err := c.VideoChapters(context.Background(), "", "1"); err != ErrNotFound {
		t.Fatalf("err %v", err)
	}
}
