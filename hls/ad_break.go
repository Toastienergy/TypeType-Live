package hls

import (
	"fmt"
	"time"
)

// AdBreakClass marks the break TypeType announces when it strips Twitch ads, for the player to show its own page.
const AdBreakClass = "com.typetype.ad-break"

// adBreakFallback covers a break Twitch gave no timing for; the next refresh extends it while ads remain.
const adBreakFallback = 30 * time.Second

// WithAdBreakNotice announces the ad break this playlist is in. Its ID and start follow from its expected end, so a
// player that reads the same range again always finds the same attributes, and a longer break gets a new range.
func (m Media) WithAdBreakNotice(now time.Time) Media {
	if !m.AdBreak {
		return m
	}
	end := m.AdEnds
	if !end.After(now) {
		end = now.Add(adBreakFallback)
	}
	end = end.UTC().Truncate(time.Second)
	start := m.AdStarts.UTC().Truncate(time.Second)
	if start.IsZero() || !start.Before(end) {
		start = end.Add(-adBreakFallback)
	}
	tag := fmt.Sprintf(`#EXT-X-DATERANGE:ID="typetype-ad-break-%d",CLASS=%q,START-DATE=%q,END-DATE=%q`,
		end.Unix(), AdBreakClass, start.Format(time.RFC3339), end.Format(time.RFC3339))
	out := m
	out.Header = append(append([]string{}, m.Header...), tag)
	return out
}
