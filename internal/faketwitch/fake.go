package faketwitch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Fake struct {
	Base         string
	SegmentBytes int
	Now          func() time.Time
	Expired      atomic.Bool

	TokenCalls    atomic.Int64
	MasterCalls   atomic.Int64
	PlaylistCalls atomic.Int64
	SegmentCalls  atomic.Int64
	ChannelCalls  atomic.Int64

	follows sync.Map
}

func New() *Fake {
	return &Fake{SegmentBytes: 64 << 10, Now: time.Now}
}

func (f *Fake) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /gql", f.gql)
	mux.HandleFunc("GET /api/v2/channel/hls/{file}", f.master)
	mux.HandleFunc("GET /vod/v2/{file}", f.master)
	mux.HandleFunc("GET /playlist/{kind}/{id}/{q}", f.playlist)
	mux.HandleFunc("GET /segment/{id}/{q}/{file}", f.segment)
	mux.HandleFunc("GET /cdn/{special}/{q}/{file}", f.cdn)
	return mux
}

type gqlBody struct {
	OperationName string         `json:"operationName"`
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
}

func (f *Fake) gql(w http.ResponseWriter, r *http.Request) {
	var body gqlBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	login, _ := body.Variables["login"].(string)
	switch {
	case body.OperationName == "PlaybackAccessToken":
		f.TokenCalls.Add(1)
		value, _ := json.Marshal(map[string]any{
			"expires":     f.Now().Add(20 * time.Minute).Unix(),
			"server_ads":  true,
			"player_type": body.Variables["playerType"],
		})
		key := "streamPlaybackAccessToken"
		if body.Variables["isVod"] == true {
			key = "videoPlaybackAccessToken"
		}
		writeData(w, map[string]any{key: map[string]any{"value": string(value), "signature": "sig"}})
	case body.OperationName == "FollowButton_FollowUser" || body.OperationName == "FollowButton_UnfollowUser":
		f.changeFollow(w, body)
	case strings.Contains(body.Query, "query SelfFollowing("):
		var follower any
		if at, ok := f.follows.Load("42"); ok {
			follower = map[string]any{"followedAt": at}
		}
		writeData(w, map[string]any{"user": map[string]any{"id": "42", "self": map[string]any{"follower": follower}}})
	case strings.Contains(body.Query, "query Channel("):
		f.ChannelCalls.Add(1)
		if login == "missing" {
			writeData(w, map[string]any{"user": nil})
			return
		}
		user := map[string]any{"id": "42", "login": login, "displayName": strings.ToUpper(login), "profileImageURL": "https://static-cdn.jtvnw.net/a.png"}
		if login != "offline" {
			user["stream"] = map[string]any{"id": "s1", "title": "Test stream", "viewersCount": 10, "createdAt": "2026-11-14T20:00:00Z"}
		}
		writeData(w, map[string]any{"user": user})
	case strings.Contains(body.Query, "query Video("):
		video := map[string]any{"id": body.Variables["id"], "title": "Past broadcast", "lengthSeconds": 60, "broadcastType": "ARCHIVE"}
		if body.Variables["id"] == RestrictedVOD {
			video["animatedPreviewURL"] = f.Base + "/cdn/special_" + RestrictedVOD + "/storyboards/" + RestrictedVOD + "-strip-0.jpg"
		}
		writeData(w, map[string]any{"video": video})
	default:
		writeData(w, map[string]any{})
	}
}

func (f *Fake) changeFollow(w http.ResponseWriter, body gqlBody) {
	input, _ := body.Variables["input"].(map[string]any)
	target, _ := input["targetID"].(string)
	if body.OperationName == "FollowButton_FollowUser" {
		f.follows.Store(target, f.Now().UTC().Format(time.RFC3339))
		writeData(w, map[string]any{"followUser": map[string]any{"follow": map[string]any{}, "error": nil}})
		return
	}
	f.follows.Delete(target)
	writeData(w, map[string]any{"unfollowUser": map[string]any{"follow": map[string]any{}}})
}

const RestrictedVOD = "777"

func (f *Fake) cdn(w http.ResponseWriter, r *http.Request) {
	q, file := r.PathValue("q"), r.PathValue("file")
	switch {
	case file == "index-dvr.m3u8" && (q == "chunked" || q == "720p60"):
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:10.000,\n0.ts\n#EXTINF:10.000,\n1-unmuted.ts\n#EXT-X-ENDLIST\n")
	case file == "0.ts" || file == "1-muted.ts":
		_, _ = w.Write(make([]byte, 1024))
	default:
		http.NotFound(w, r)
	}
}

func writeData(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func (f *Fake) master(w http.ResponseWriter, r *http.Request) {
	f.MasterCalls.Add(1)
	id := strings.TrimSuffix(r.PathValue("file"), ".m3u8")
	if id == RestrictedVOD {
		http.Error(w, "subscriber only", http.StatusForbidden)
		return
	}
	kind := "live"
	if strings.HasPrefix(r.URL.Path, "/vod/") {
		kind = "vod"
	}
	if id == "offline" {
		http.NotFound(w, r)
		return
	}
	var token struct {
		PlayerType string `json:"player_type"`
	}
	_ = json.Unmarshal([]byte(r.URL.Query().Get("token")), &token)
	pt := url.QueryEscape(token.PlayerType)
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	fmt.Fprintf(w, "#EXTM3U\n#EXT-X-SESSION-DATA:DATA-ID=\"USER-IP\",VALUE=\"203.0.113.9\"\n")
	for _, v := range []struct{ id, res, bw string }{{"720p60", "1280x720", "3400000"}, {"360p30", "640x360", "630000"}} {
		fmt.Fprintf(w, "#EXT-X-STREAM-INF:BANDWIDTH=%s,RESOLUTION=%s,CODECS=\"avc1.4D401F,mp4a.40.2\",STABLE-VARIANT-ID=\"%s\"\n", v.bw, v.res, v.id)
		fmt.Fprintf(w, "%s/playlist/%s/%s/%s.m3u8?pt=%s\n", f.Base, kind, id, v.id, pt)
	}
}

func (f *Fake) playlist(w http.ResponseWriter, r *http.Request) {
	f.PlaylistCalls.Add(1)
	if f.Expired.CompareAndSwap(true, false) {
		http.Error(w, "expired", http.StatusForbidden)
		return
	}
	kind, id, q := r.PathValue("kind"), r.PathValue("id"), strings.TrimSuffix(r.PathValue("q"), ".m3u8")
	backup := r.URL.Query().Get("pt") != "site"
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	if kind == "vod" {
		fmt.Fprintf(w, "#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXT-X-MEDIA-SEQUENCE:0\n")
		for i := range 6 {
			fmt.Fprintf(w, "#EXTINF:10.000,\n%d.ts\n", i)
		}
		fmt.Fprintf(w, "#EXT-X-ENDLIST\n")
		return
	}
	seq := f.Now().Unix() / 2
	fmt.Fprintf(w, "#EXTM3U\n#EXT-X-VERSION:6\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:%d\n", seq)
	fmt.Fprintf(w, "#EXT-X-MAP:URI=\"%s/segment/%s/%s/init.mp4\"\n", f.Base, id, q)
	fmt.Fprintf(w, "#EXT-X-DATERANGE:ID=\"trigger-1\",CLASS=\"twitch-trigger\",START-DATE=\"2026-11-14T22:17:00Z\",X-TV-TWITCH-TRIGGER-URL=\"https://beacon.example/\"\n")
	ads := (strings.HasPrefix(id, "ads") && !backup) || id == "adsonly"
	for i := range int64(6) {
		title := "live"
		if ads && i >= 3 {
			title = "Amazon|ad"
			if i == 3 {
				fmt.Fprintf(w, "#EXT-X-DATERANGE:ID=\"stitched-ad-1\",CLASS=\"twitch-stitched-ad\",START-DATE=\"2026-11-14T22:17:00Z\"\n")
			}
		}
		fmt.Fprintf(w, "#EXTINF:2.000,%s\n%s/segment/%s/%s/%d.ts\n", title, f.Base, id, q, seq+i)
	}
	fmt.Fprintf(w, "#EXT-X-TWITCH-PREFETCH:%s/segment/%s/%s/%d.ts\n", f.Base, id, q, seq+6)
}

func (f *Fake) segment(w http.ResponseWriter, _ *http.Request) {
	f.SegmentCalls.Add(1)
	body := make([]byte, f.SegmentBytes)
	for i := 0; i < len(body); i += 188 {
		body[i] = 0x47
	}
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
}
