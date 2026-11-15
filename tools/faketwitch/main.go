package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/TypeType-Video/TypeType-Live/internal/faketwitch"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19090", "listen address")
	base := flag.String("base", "", "public base URL of this fake, defaults to http://<addr>")
	segment := flag.Int("segment-bytes", 1<<20, "size of every fake segment")
	flag.Parse()
	fake := faketwitch.New()
	fake.SegmentBytes = *segment
	fake.Base = *base
	if fake.Base == "" {
		fake.Base = "http://" + *addr
	}
	srv := &http.Server{Addr: *addr, Handler: fake.Handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("fake twitch on %s", fake.Base)
	log.Fatal(srv.ListenAndServe())
}
