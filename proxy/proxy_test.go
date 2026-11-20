package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSignerRoundTripAndTamper(t *testing.T) {
	s := NewSigner([]byte("0123456789abcdef0123456789abcdef"))
	signed := s.Sign(url.Values{"u": {"https://a.hls.ttvnw.net/x.ts"}}, time.Minute)
	if err := s.Verify(signed); err != nil {
		t.Fatal(err)
	}
	signed.Set("u", "https://evil.example/x.ts")
	if err := s.Verify(signed); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("tampered err = %v", err)
	}
}

func TestSignerExpiry(t *testing.T) {
	s := NewSigner([]byte("k"))
	now := time.Unix(1000, 0)
	s.now = func() time.Time { return now }
	signed := s.Sign(url.Values{"c": {"x"}}, 2*time.Minute)
	now = now.Add(3 * time.Minute)
	if err := s.Verify(signed); !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v", err)
	}
}

func TestSignUntilGivesTheSameLinkForTheSameExpiry(t *testing.T) {
	s := NewSigner([]byte("k"))
	now := time.Unix(1000, 0)
	s.now = func() time.Time { return now }
	expires := s.Expiry(10 * time.Minute)
	first := s.SignUntil(url.Values{"x": {"a"}}, expires).Encode()
	now = now.Add(3 * time.Minute)
	if again := s.SignUntil(url.Values{"x": {"a"}}, expires).Encode(); again != first {
		t.Fatalf("link changed: %s then %s", first, again)
	}
	if s.Sign(url.Values{"x": {"a"}}, 10*time.Minute).Encode() == first {
		t.Fatal("Sign kept the old expiry")
	}
}

func TestHostsAllowList(t *testing.T) {
	h := NewHosts(DefaultAllowedSuffixes)
	for _, ok := range []string{"https://a.hls.ttvnw.net/s.ts", "https://x.hls.live-video.net/v1/segment/y"} {
		if _, err := h.Allowed(ok); err != nil {
			t.Errorf("%s refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"http://a.hls.ttvnw.net/s.ts", "https://ttvnw.net.evil.example/", "https://user@a.ttvnw.net/", "https://example.com/"} {
		if _, err := h.Allowed(bad); err == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}

func TestPublicDialerRefusesPrivateAddresses(t *testing.T) {
	d := PublicDialer(time.Second, false)
	for _, addr := range []string{"127.0.0.1:80", "10.0.0.1:443", "[::1]:443", "169.254.169.254:80"} {
		_, err := d.DialContext(context.Background(), "tcp", addr)
		if !errors.Is(err, ErrHostNotAllowed) {
			var opErr *net.OpError
			if !errors.As(err, &opErr) || !errors.Is(opErr.Err, ErrHostNotAllowed) {
				t.Errorf("%s: err = %v", addr, err)
			}
		}
	}
}

func testRelay(t *testing.T, handler http.HandlerFunc) (*Relay, string) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	hosts := NewHosts([]string{"127.0.0.1"})
	hosts.AllowInsecure = true
	return NewRelay(NewHTTPClient(hosts), hosts), srv.URL
}

func TestStreamForwardsRangeAndHeaders(t *testing.T) {
	relay, base := testRelay(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-3" || r.Header.Get("Cookie") != "" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 0-3/10")
		w.Header().Set("Set-Cookie", "tracker=1")
		w.WriteHeader(http.StatusPartialContent)
		io.WriteString(w, "abcd")
	})
	req := httptest.NewRequest(http.MethodGet, "/s", nil)
	req.Header.Set("Range", "bytes=0-3")
	req.Header.Set("Cookie", "session=secret")
	rec := httptest.NewRecorder()
	if err := relay.Stream(rec, req, base+"/clip.mp4"); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "abcd" || rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("code %d body %q headers %v", rec.Code, rec.Body.String(), rec.Header())
	}
}

func TestFetchLimitsBodyAndStatus(t *testing.T) {
	relay, base := testRelay(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "missing") {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, strings.Repeat("x", 100))
	})
	if _, err := relay.Fetch(context.Background(), base+"/big", 10); err == nil {
		t.Fatal("oversized body accepted")
	}
	var status *StatusError
	if _, err := relay.Fetch(context.Background(), base+"/missing", 10); !errors.As(err, &status) || status.Status != 404 {
		t.Fatalf("err = %v", err)
	}
	body, err := relay.Fetch(context.Background(), base+"/ok", 1000)
	if err != nil || len(body) != 100 {
		t.Fatalf("len %d err %v", len(body), err)
	}
}

func TestStreamRefusesOtherHosts(t *testing.T) {
	relay, _ := testRelay(t, func(http.ResponseWriter, *http.Request) {})
	rec := httptest.NewRecorder()
	err := relay.Stream(rec, httptest.NewRequest(http.MethodGet, "/s", nil), "http://example.com/x")
	if !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("err = %v", err)
	}
}

func TestSealerRoundTrip(t *testing.T) {
	s, err := NewSealer([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	sealed := s.Seal("https://a.hls.ttvnw.net/v1/segment/x.ts?dna=abc")
	if s.Seal("https://a.hls.ttvnw.net/v1/segment/x.ts?dna=abc") != sealed {
		t.Fatal("sealing must be stable so players and caches see one link per segment")
	}
	if strings.Contains(sealed, "ttvnw") {
		t.Fatal("sealed value leaks the URL")
	}
	plain, err := s.Open(sealed)
	if err != nil || plain != "https://a.hls.ttvnw.net/v1/segment/x.ts?dna=abc" {
		t.Fatalf("plain %q err %v", plain, err)
	}
	other, _ := NewSealer([]byte("other"))
	if _, err := other.Open(sealed); !errors.Is(err, ErrSealed) {
		t.Fatalf("foreign key err = %v", err)
	}
}

func TestTransportErrorsKeepTheQueryOutOfTheMessage(t *testing.T) {
	relay, base := testRelay(t, func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("no hijacker")
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	})
	_, err := relay.Fetch(context.Background(), base+"/playlist.m3u8?sig=secret-sig&token=secret-token", 10)
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "playlist") {
		t.Fatalf("error leaks the request URL: %v", err)
	}
}
