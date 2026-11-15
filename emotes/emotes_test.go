package emotes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func fixtures() http.Handler {
	mux := http.NewServeMux()
	reply := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }
	}
	mux.HandleFunc("/v3/users/twitch/42", reply(`{"emote_set":{"emotes":[
		{"name":"catJAM","flags":0,"data":{"animated":true,"host":{"url":"//cdn.7tv.app/emote/1"}}},
		{"name":"RainTime","flags":1,"data":{"host":{"url":"//cdn.7tv.app/emote/2"}}}]}}`))
	mux.HandleFunc("/3/cached/users/twitch/42", reply(`{"channelEmotes":[{"id":"b1","code":"pepeD","imageType":"gif"}],
		"sharedEmotes":[{"id":"b2","code":"monkaS","imageType":"png"}],"bots":["x"]}`))
	mux.HandleFunc("/v1/room/id/42", reply(`{"sets":{"7":{"emoticons":[
		{"name":"OMEGALUL","urls":{"1":"https://cdn.frankerfacez.com/emote/9/1","2":"https://cdn.frankerfacez.com/emote/9/2"}},
		{"name":"Wiggle","urls":{"1":"https://cdn.frankerfacez.com/emote/8/1"},"animated":{"1":"https://cdn.frankerfacez.com/emote/8/animated/1"}}]}}}`))
	mux.HandleFunc("/v1/set/global", reply(`{"default_sets":[3],"sets":{"3":{"emoticons":[{"name":"ZreknarF","urls":{"1":"https://cdn.frankerfacez.com/emote/1/1"}}]},
		"4":{"emoticons":[{"name":"Hidden","urls":{"1":"https://cdn.frankerfacez.com/emote/2/1"}}]}}}`))
	mux.HandleFunc("/3/cached/emotes/global", reply(`[{"id":"g1","code":"FeelsGoodMan","imageType":"png"}]`))
	mux.HandleFunc("/v3/emote-sets/global", http.NotFound)
	return mux
}

func newTestClient(t *testing.T) *Client {
	srv := httptest.NewServer(fixtures())
	t.Cleanup(srv.Close)
	c := New(srv.Client())
	c.SevenTVURL, c.BTTVURL, c.FFZURL = srv.URL, srv.URL, srv.URL
	return c
}

func byName(list []Emote) map[string]Emote {
	m := make(map[string]Emote, len(list))
	for _, e := range list {
		m[e.Name] = e
	}
	return m
}

func TestChannelEmotesFromEveryProvider(t *testing.T) {
	list, err := newTestClient(t).Channel(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	got := byName(list)
	if len(got) != 6 {
		t.Fatalf("got %d emotes: %v", len(got), got)
	}
	if e := got["catJAM"]; e.URL != "https://cdn.7tv.app/emote/1/1x.webp" || !e.Animated || e.Provider != "7tv" {
		t.Fatalf("7tv %+v", e)
	}
	if !got["RainTime"].ZeroWidth {
		t.Fatal("7tv zero width flag lost")
	}
	if e := got["pepeD"]; e.URL != "https://cdn.betterttv.net/emote/b1/1x.webp" || !e.Animated {
		t.Fatalf("bttv %+v", e)
	}
	if e := got["Wiggle"]; e.URL != "https://cdn.frankerfacez.com/emote/8/animated/1.webp" || !e.Animated {
		t.Fatalf("ffz animated %+v", e)
	}
}

func TestGlobalSkipsMissingProvidersAndNonDefaultSets(t *testing.T) {
	list, err := newTestClient(t).Global(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := byName(list)
	if _, ok := got["Hidden"]; ok || len(got) != 2 {
		t.Fatalf("global %v", got)
	}
}
