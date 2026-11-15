package main

import (
	"fmt"
	"slices"
	"sync"
	"time"
)

type series struct {
	latencies []time.Duration
	bytes     int64
	failures  int
}

type stats struct {
	mu   sync.Mutex
	data map[string]*series
}

func newStats() *stats {
	return &stats{data: map[string]*series{}}
}

func (s *stats) get(kind string) *series {
	if s.data[kind] == nil {
		s.data[kind] = &series{}
	}
	return s.data[kind]
}

func (s *stats) observe(kind string, d time.Duration, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sr := s.get(kind)
	sr.latencies = append(sr.latencies, d)
	sr.bytes += int64(n)
}

func (s *stats) fail(kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.get(kind).failures++
}

func (s *stats) report(elapsed time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("%-9s %8s %8s %9s %9s %9s %10s\n", "kind", "ok", "failed", "p50", "p95", "p99", "MB/s")
	for _, kind := range []string{"api", "master", "playlist", "segment"} {
		sr := s.data[kind]
		if sr == nil {
			continue
		}
		slices.Sort(sr.latencies)
		fmt.Printf("%-9s %8d %8d %9s %9s %9s %10.1f\n", kind, len(sr.latencies), sr.failures,
			pct(sr.latencies, 50), pct(sr.latencies, 95), pct(sr.latencies, 99),
			float64(sr.bytes)/elapsed.Seconds()/1e6)
	}
}

func pct(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[min(len(sorted)-1, len(sorted)*p/100)].Round(10 * time.Microsecond)
}
