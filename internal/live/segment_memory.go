package live

import (
	"sync"
	"time"
)

const segmentSweepEvery = time.Minute

// AVPlayer rejects a refreshed playlist when a segment it already listed comes back under another address or with
// another duration. Twitch rewrites the duration of its newest segments by a millisecond, and links carry an
// expiry, so a segment keeps its first duration, and the expiry of its first link until half of its lifetime has
// passed.
type segmentMemory struct {
	mu        sync.Mutex
	entries   map[string]*segmentEntry
	lastSweep time.Time
}

type segmentEntry struct {
	expires  time.Time
	duration float64
}

func newSegmentMemory() *segmentMemory {
	return &segmentMemory{entries: make(map[string]*segmentEntry)}
}

// remember answers the expiry and duration a segment was first listed with; a zero duration leaves it unknown.
func (m *segmentMemory) remember(key string, duration float64, ttl time.Duration, now time.Time, fresh func() time.Time) (time.Time, float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if now.Sub(m.lastSweep) >= segmentSweepEvery {
		for k, e := range m.entries {
			if !now.Before(e.expires) {
				delete(m.entries, k)
			}
		}
		m.lastSweep = now
	}
	e, ok := m.entries[key]
	if !ok {
		e = &segmentEntry{}
		m.entries[key] = e
	}
	if !ok || e.expires.Sub(now) <= ttl/2 {
		e.expires = fresh()
	}
	if e.duration == 0 {
		e.duration = duration
	}
	return e.expires, e.duration
}
