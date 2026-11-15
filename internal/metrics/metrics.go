package metrics

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/TypeType-Video/TypeType-Live/cache"
	"github.com/TypeType-Video/TypeType-Live/internal/live"
)

type Sources struct {
	Live     *live.Metrics
	Segments func() cache.Stats
	Rooms    func() int
}

func Handler(src Sources) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		m := src.Live
		counters := []struct {
			name  string
			value uint64
		}{
			{"typetype_live_master_fetches_total", m.MasterFetches.Load()},
			{"typetype_live_playlist_fetches_total", m.PlaylistFetches.Load()},
			{"typetype_live_playlist_served_total", m.PlaylistServed.Load()},
			{"typetype_live_segment_fetches_total", m.SegmentFetches.Load()},
			{"typetype_live_segment_served_total", m.SegmentServed.Load()},
			{"typetype_live_segment_bytes_total", m.SegmentBytes.Load()},
			{"typetype_live_ad_playlists_total", m.AdPlaylists.Load()},
			{"typetype_live_ad_swaps_total", m.AdSwaps.Load()},
			{"typetype_live_ad_stripped_total", m.AdStripped.Load()},
			{"typetype_live_upstream_errors_total", m.UpstreamErrors.Load()},
		}
		for _, c := range counters {
			fmt.Fprintf(w, "# TYPE %s counter\n%s %d\n", c.name, c.name, c.value)
		}
		stats := src.Segments()
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		gauges := []struct {
			name  string
			value float64
		}{
			{"typetype_live_segment_cache_bytes", float64(stats.Bytes)},
			{"typetype_live_segment_cache_entries", float64(stats.Entries)},
			{"typetype_live_segment_cache_hits", float64(stats.Hits)},
			{"typetype_live_segment_cache_misses", float64(stats.Misses)},
			{"typetype_live_chat_rooms", float64(src.Rooms())},
			{"go_goroutines", float64(runtime.NumGoroutine())},
			{"go_memstats_heap_inuse_bytes", float64(mem.HeapInuse)},
			{"go_memstats_sys_bytes", float64(mem.Sys)},
			{"go_gc_cycles_total", float64(mem.NumGC)},
		}
		for _, g := range gauges {
			fmt.Fprintf(w, "# TYPE %s gauge\n%s %g\n", g.name, g.name, g.value)
		}
	})
}
