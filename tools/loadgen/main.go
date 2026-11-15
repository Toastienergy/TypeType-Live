package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

type options struct {
	target   string
	secret   string
	channels int
	viewers  int
	duration time.Duration
	quality  int
}

func main() {
	var o options
	flag.StringVar(&o.target, "target", "http://127.0.0.1:18080", "TypeType-Live base URL")
	flag.StringVar(&o.secret, "secret", os.Getenv("LIVE_SECRET"), "service secret")
	flag.IntVar(&o.channels, "channels", 10, "number of channels")
	flag.IntVar(&o.viewers, "viewers", 100, "number of viewers spread over the channels")
	flag.DurationVar(&o.duration, "duration", 30*time.Second, "test length")
	flag.IntVar(&o.quality, "quality", 0, "variant index to watch")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, o.duration)
	defer cancel()
	stats := newStats()
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 1024}, Timeout: 10 * time.Second}
	var wg sync.WaitGroup
	start := time.Now()
	for i := range o.viewers {
		channel := fmt.Sprintf("load%03d", i%o.channels)
		wg.Go(func() { viewer(ctx, client, o, channel, stats) })
		time.Sleep(2 * time.Millisecond)
	}
	wg.Wait()
	stats.report(time.Since(start))
}

func viewer(ctx context.Context, client *http.Client, o options, channel string, s *stats) {
	var live struct {
		HLSURL string `json:"hlsUrl"`
	}
	body, err := fetch(ctx, client, o.target+"/v1/live/"+channel, o.secret, s, "api")
	if err != nil || json.Unmarshal(body, &live) != nil {
		return
	}
	master, err := fetch(ctx, client, o.target+live.HLSURL, "", s, "master")
	if err != nil {
		return
	}
	variants := uris(string(master))
	if len(variants) == 0 {
		s.fail("master")
		return
	}
	variantURL := o.target + "/hls/" + variants[min(o.quality, len(variants)-1)]
	seen := map[string]bool{}
	for ctx.Err() == nil {
		playlist, err := fetch(ctx, client, variantURL, "", s, "playlist")
		if err == nil {
			for _, seg := range uris(string(playlist)) {
				if seen[seg] {
					continue
				}
				seen[seg] = true
				if _, err := fetch(ctx, client, o.target+"/hls/"+seg, "", s, "segment"); err != nil {
					break
				}
			}
		}
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
}

func fetch(ctx context.Context, client *http.Client, url, secret string, s *stats, kind string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if secret != "" {
		req.Header.Set("X-Live-Secret", secret)
	}
	begin := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			s.fail(kind)
		}
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		if ctx.Err() == nil {
			s.fail(kind)
		}
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	s.observe(kind, time.Since(begin), len(body))
	return body, nil
}

func uris(playlist string) []string {
	var out []string
	for line := range strings.SplitSeq(playlist, "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}

func init() {
	log.SetFlags(0)
}
