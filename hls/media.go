package hls

import (
	"bufio"
	"bytes"
	"net/url"
	"strconv"
	"strings"
)

const prefetchTag = "#EXT-X-TWITCH-PREFETCH:"

type Segment struct {
	Tags     []string
	Duration float64
	Title    string
	URI      string
	Ad       bool
}

type Media struct {
	Header        []string
	Segments      []Segment
	Prefetch      []string
	Trailer       []string
	MediaSequence int64
	Ended         bool
	AdBreak       bool
}

func ParseMedia(data []byte, base *url.URL) (Media, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "#EXTM3U" {
		return Media{}, ErrNotPlaylist
	}
	p := mediaParser{base: base}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "":
		case strings.HasPrefix(line, "#"):
			p.tag(line)
		default:
			p.pending.URI = resolve(base, line)
			p.pending.Ad = isAdTitle(p.pending.Title)
			p.media.AdBreak = p.media.AdBreak || p.pending.Ad
			p.media.Segments = append(p.media.Segments, p.pending)
			p.pending = Segment{}
		}
	}
	if err := scanner.Err(); err != nil {
		return Media{}, err
	}
	p.media.Trailer = p.pending.Tags
	return p.media, nil
}

type mediaParser struct {
	media      Media
	pending    Segment
	inSegments bool
	base       *url.URL
}

func (p *mediaParser) tag(line string) {
	line = mapURIAttr(line, func(ref string) string { return resolve(p.base, ref) })
	switch {
	case strings.HasPrefix(line, prefetchTag):
		p.media.Prefetch = append(p.media.Prefetch, resolve(p.base, strings.TrimPrefix(line, prefetchTag)))
	case strings.HasPrefix(line, "#EXT-X-DATERANGE:"):
		keep, ad := classifyDateRange(line)
		p.media.AdBreak = p.media.AdBreak || ad
		if keep {
			p.pending.Tags = append(p.pending.Tags, line)
		}
	case strings.HasPrefix(line, "#EXTINF:"):
		p.inSegments = true
		p.pending.Duration, p.pending.Title = parseInf(line)
	case line == "#EXT-X-ENDLIST":
		p.media.Ended = true
	case strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"):
		p.media.MediaSequence, _ = strconv.ParseInt(strings.TrimPrefix(line, "#EXT-X-MEDIA-SEQUENCE:"), 10, 64)
	case p.inSegments || isSegmentTag(line):
		p.pending.Tags = append(p.pending.Tags, line)
	default:
		p.media.Header = append(p.media.Header, line)
	}
}

func (m Media) HasAds() bool {
	for _, s := range m.Segments {
		if s.Ad {
			return true
		}
	}
	return false
}

func (m Media) WithoutAds() Media {
	out := m
	out.Segments = make([]Segment, 0, len(m.Segments))
	dropped := false
	for i, s := range m.Segments {
		if s.Ad {
			if len(out.Segments) == 0 {
				out.MediaSequence = m.MediaSequence + int64(i) + 1
			} else {
				dropped = true
			}
			continue
		}
		if dropped {
			s.Tags = append([]string{"#EXT-X-DISCONTINUITY"}, s.Tags...)
			dropped = false
		}
		out.Segments = append(out.Segments, s)
	}
	if m.HasAds() {
		out.Prefetch = nil
	}
	return out
}

func (m Media) WithMutedSegments() Media {
	out := m
	out.Segments = make([]Segment, len(m.Segments))
	for i, s := range m.Segments {
		s.URI = strings.Replace(s.URI, "-unmuted.ts", "-muted.ts", 1)
		out.Segments[i] = s
	}
	return out
}

func (m Media) Write(buf *bytes.Buffer, uri func(string) string) {
	buf.WriteString("#EXTM3U\n")
	writeTags(buf, m.Header, uri)
	buf.WriteString("#EXT-X-MEDIA-SEQUENCE:")
	buf.WriteString(strconv.FormatInt(m.MediaSequence, 10))
	buf.WriteByte('\n')
	for _, s := range m.Segments {
		writeTags(buf, s.Tags, uri)
		buf.WriteString("#EXTINF:")
		buf.WriteString(strconv.FormatFloat(s.Duration, 'f', 3, 64))
		buf.WriteByte(',')
		buf.WriteString(s.Title)
		buf.WriteByte('\n')
		buf.WriteString(uri(s.URI))
		buf.WriteByte('\n')
	}
	writeTags(buf, m.Trailer, uri)
	for _, p := range m.Prefetch {
		buf.WriteString(prefetchTag)
		buf.WriteString(uri(p))
		buf.WriteByte('\n')
	}
	if m.Ended {
		buf.WriteString("#EXT-X-ENDLIST\n")
	}
}

func writeTags(buf *bytes.Buffer, tags []string, uri func(string) string) {
	for _, tag := range tags {
		buf.WriteString(mapURIAttr(tag, uri))
		buf.WriteByte('\n')
	}
}

func classifyDateRange(line string) (keep, ad bool) {
	attrs := Attributes(line)
	if IsAdDateRange(attrs) {
		return false, true
	}
	return attrs["CLASS"] != "twitch-trigger", false
}

func parseInf(line string) (float64, string) {
	value, title, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
	duration, _ := strconv.ParseFloat(value, 64)
	return duration, title
}

func isSegmentTag(line string) bool {
	switch tagName(line) {
	case "#EXT-X-PROGRAM-DATE-TIME", "#EXT-X-DISCONTINUITY", "#EXT-X-BYTERANGE", "#EXT-X-MAP", "#EXT-X-KEY":
		return true
	}
	return false
}

func isAdTitle(title string) bool {
	return title != "" && title != "live"
}

func IsAdDateRange(attrs map[string]string) bool {
	if strings.HasPrefix(attrs["ID"], "stitched-ad-") || attrs["CLASS"] == "twitch-stitched-ad" {
		return true
	}
	for key := range attrs {
		if strings.HasPrefix(key, "X-TV-TWITCH-AD-") {
			return true
		}
	}
	return false
}
