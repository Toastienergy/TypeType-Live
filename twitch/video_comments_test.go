package twitch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func TestVideoCommentsKeepTheLiveChatShape(t *testing.T) {
	var vars map[string]any
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		var single struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&single)
		vars = single.Variables
		io.WriteString(w, `{"data":{"video":{"comments":{"edges":[{"cursor":"c1","node":{"id":"m1","contentOffsetSeconds":117,
			"commenter":{"login":"kaizuzu","displayName":"Kaizuzu"},
			"message":{"userColor":"#DAA520","fragments":[{"text":"héllo "},{"text":"Kappa","emote":{"emoteID":"25"}},{"text":" !"}],
			"userBadges":[{"setID":"subscriber","version":"12"}]}}}],"pageInfo":{"hasNextPage":true}}}}}`)
	})
	page, err := c.VideoComments(context.Background(), "", "2929630714", 120, "")
	if err != nil {
		t.Fatal(err)
	}
	if vars["contentOffsetSeconds"] != float64(120) || vars["videoID"] != "2929630714" {
		t.Fatalf("vars %v", vars)
	}
	want := VideoComment{
		ID: "m1", Offset: 117, Login: "kaizuzu", DisplayName: "Kaizuzu", Color: "#DAA520",
		Text: "héllo Kappa !", Emotes: []CommentEmote{{ID: "25", Start: 6, End: 10}}, Badges: []string{"subscriber/12"},
	}
	if len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0], want) || page.Cursor != "c1" || !page.HasMore {
		t.Fatalf("page %+v", page)
	}
}

func TestVideoCommentsFollowTheCursor(t *testing.T) {
	var vars map[string]any
	c := fakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		var single struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&single)
		vars = single.Variables
		io.WriteString(w, `{"data":{"video":{"comments":{"edges":[],"pageInfo":{"hasNextPage":false}}}}}`)
	})
	page, err := c.VideoComments(context.Background(), "", "1", 0, "abc")
	if err != nil || page.HasMore || len(page.Items) != 0 {
		t.Fatalf("page %+v err %v", page, err)
	}
	if vars["cursor"] != "abc" || vars["contentOffsetSeconds"] != nil {
		t.Fatalf("vars %v", vars)
	}
}
