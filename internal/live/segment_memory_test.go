package live

import (
	"testing"
	"time"
)

func freshAt(now time.Time, ttl time.Duration) func() time.Time {
	return func() time.Time { return now.Add(ttl).Truncate(time.Minute) }
}

func TestASegmentKeepsItsExpiryAcrossRefreshes(t *testing.T) {
	memory := newSegmentMemory()
	start := time.Unix(1_000_000, 0)
	first, _ := memory.remember("seg", 2.002, 10*time.Minute, start, freshAt(start, 10*time.Minute))
	later := start.Add(90 * time.Second)
	if got, _ := memory.remember("seg", 2.002, 10*time.Minute, later, freshAt(later, 10*time.Minute)); !got.Equal(first) {
		t.Fatalf("expiry moved from %v to %v", first, got)
	}
	if other, _ := memory.remember("next", 2.002, 10*time.Minute, later, freshAt(later, 10*time.Minute)); other.Equal(first) {
		t.Fatal("another segment shared the first expiry")
	}
	late := first.Add(-4 * time.Minute)
	if got, _ := memory.remember("seg", 2.002, 10*time.Minute, late, freshAt(late, 10*time.Minute)); got.Equal(first) {
		t.Fatal("a link past half of its lifetime was not renewed")
	}
}

func TestASegmentKeepsItsFirstDuration(t *testing.T) {
	memory := newSegmentMemory()
	now := time.Unix(1_000_000, 0)
	if _, d := memory.remember("seg", 0, time.Minute, now, freshAt(now, time.Minute)); d != 0 {
		t.Fatalf("unknown duration became %v", d)
	}
	memory.remember("seg", 1.986, time.Minute, now, freshAt(now, time.Minute))
	if _, d := memory.remember("seg", 1.985, time.Minute, now, freshAt(now, time.Minute)); d != 1.986 {
		t.Fatalf("duration changed to %v", d)
	}
}

func TestExpiredSegmentsAreForgotten(t *testing.T) {
	memory := newSegmentMemory()
	start := time.Unix(1_000_000, 0)
	memory.remember("seg", 2, time.Minute, start, func() time.Time { return start.Add(time.Minute) })
	after := start.Add(2 * time.Minute)
	memory.remember("next", 2, time.Minute, after, func() time.Time { return after.Add(time.Minute) })
	if _, ok := memory.entries["seg"]; ok || len(memory.entries) != 1 {
		t.Fatalf("entries %v", memory.entries)
	}
}
