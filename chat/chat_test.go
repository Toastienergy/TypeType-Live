package chat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestParsePrivmsgWithTagsAndEmotes(t *testing.T) {
	raw := `@badges=subscriber/12,premium/1;color=#1E90FF;display-name=Some\sUser;emotes=25:0-4,6-10/1902:12-16;id=abc;login=someuser;tmi-sent-ts=1794694652520 :someuser!someuser@someuser.tmi.twitch.tv PRIVMSG #chan :Kappa Kappa Keepo hi`
	ev, ok := parseLine(raw).event()
	if !ok || ev.Type != "message" || ev.Login != "someuser" || ev.DisplayName != "Some User" || ev.Text != "Kappa Kappa Keepo hi" {
		t.Fatalf("event %+v", ev)
	}
	if len(ev.Emotes) != 3 || ev.Emotes[2].ID != "1902" || ev.Emotes[2].Start != 12 {
		t.Fatalf("emotes %+v", ev.Emotes)
	}
	if len(ev.Badges) != 2 || ev.Time.UnixMilli() != 1794694652520 {
		t.Fatalf("badges %v time %v", ev.Badges, ev.Time)
	}
}

func TestParseActionNoticeAndModeration(t *testing.T) {
	action, _ := parseLine("@login=a :a!a@a PRIVMSG #c :\x01ACTION waves\x01").event()
	if !action.Action || action.Text != "waves" {
		t.Fatalf("action %+v", action)
	}
	notice, _ := parseLine(`@login=b;system-msg=b\ssubscribed :tmi.twitch.tv USERNOTICE #c :hello`).event()
	if notice.Type != "notice" || notice.System != "b subscribed" || notice.Text != "hello" {
		t.Fatalf("notice %+v", notice)
	}
	del, _ := parseLine("@target-msg-id=xyz :tmi.twitch.tv CLEARMSG #c :bad").event()
	if del.Type != "delete" || del.TargetID != "xyz" {
		t.Fatalf("delete %+v", del)
	}
	if _, ok := parseLine(":tmi.twitch.tv 001 justinfan1 :Welcome").event(); ok {
		t.Fatal("numeric replies are not events")
	}
}

func TestHubSharesOneConnectionAndReplaysHistory(t *testing.T) {
	joins := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if line := string(data); strings.HasPrefix(line, "JOIN") {
				joins <- line
				_ = conn.Write(ctx, websocket.MessageText, []byte("PING :tmi.twitch.tv\r\n@login=x;id=1 :x!x@x PRIVMSG #c :first"))
			}
		}
	}))
	defer srv.Close()
	hub := NewHub("ws" + strings.TrimPrefix(srv.URL, "http"))
	events, _, leave := hub.Subscribe("c")
	select {
	case ev := <-events:
		if ev.Text != "first" {
			t.Fatalf("event %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no event")
	}
	_, history, leave2 := hub.Subscribe("c")
	if len(history) != 1 || hub.Rooms() != 1 || len(joins) != 1 {
		t.Fatalf("history %d rooms %d joins %d", len(history), hub.Rooms(), len(joins))
	}
	leave()
	leave2()
}

func TestRoomStateMergesPartialUpdates(t *testing.T) {
	r := &room{subs: map[chan Event]struct{}{}}
	full, _ := parseLine("@emote-only=0;followers-only=0;r9k=0;room-id=1;slow=0;subs-only=0 :tmi.twitch.tv ROOMSTATE #c").event()
	r.broadcast(full)
	partial, _ := parseLine("@room-id=1;slow=30 :tmi.twitch.tv ROOMSTATE #c").event()
	r.broadcast(partial)
	if r.state == nil || r.state.FollowersOnly != 0 || r.state.Slow != 30 || len(r.history) != 0 {
		t.Fatalf("state %+v history %d", r.state, len(r.history))
	}
	off, _ := parseLine("@followers-only=-1;room-id=1 :tmi.twitch.tv ROOMSTATE #c").event()
	r.broadcast(off)
	if r.state.FollowersOnly != -1 || r.state.Slow != 30 {
		t.Fatalf("state %+v", r.state)
	}
}
