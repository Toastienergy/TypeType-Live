package hls

import (
	"bufio"
	"bytes"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

var ErrNotPlaylist = errors.New("hls: not a playlist")

type Variant struct {
	ID         string
	Info       string
	URI        string
	Bandwidth  int64
	Resolution string
	FrameRate  float64
	Codecs     string
}

func (v Variant) AudioOnly() bool {
	return v.Resolution == "" && !strings.Contains(v.Codecs, "avc") && !strings.Contains(v.Codecs, "hev")
}

type Master struct {
	Tags     []string
	Variants []Variant
}

var droppedMasterTags = map[string]bool{
	"#EXT-X-SESSION-DATA": true,
	"#EXT-X-TWITCH-INFO":  true,
}

func ParseMaster(data []byte, base *url.URL) (Master, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "#EXTM3U" {
		return Master{}, ErrNotPlaylist
	}
	var master Master
	var pending string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "":
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF:"):
			pending = line
		case strings.HasPrefix(line, "#"):
			if !droppedMasterTags[tagName(line)] && !strings.Contains(line, `URI="`) {
				master.Tags = append(master.Tags, line)
			}
		case pending != "":
			master.Variants = append(master.Variants, NewVariant(pending, resolve(base, line)))
			pending = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return Master{}, err
	}
	return master, nil
}

func NewVariant(info, uri string) Variant {
	attrs := Attributes(info)
	bandwidth, _ := strconv.ParseInt(attrs["BANDWIDTH"], 10, 64)
	frameRate, _ := strconv.ParseFloat(attrs["FRAME-RATE"], 64)
	id := attrs["STABLE-VARIANT-ID"]
	if id == "" {
		id = attrs["IVS-NAME"]
	}
	if id == "" {
		id = attrs["VIDEO"]
	}
	if id == "" {
		id = strconv.FormatInt(bandwidth, 10)
	}
	return Variant{
		ID:         id,
		Info:       info,
		URI:        uri,
		Bandwidth:  bandwidth,
		Resolution: attrs["RESOLUTION"],
		FrameRate:  frameRate,
		Codecs:     attrs["CODECS"],
	}
}

func (m Master) Find(id string) (Variant, bool) {
	for _, v := range m.Variants {
		if v.ID == id {
			return v, true
		}
	}
	return Variant{}, false
}

func (m Master) Closest(target Variant) (Variant, bool) {
	if v, ok := m.Find(target.ID); ok {
		return v, true
	}
	var best Variant
	found := false
	for _, v := range m.Variants {
		if v.AudioOnly() != target.AudioOnly() {
			continue
		}
		if !found || abs(v.Bandwidth-target.Bandwidth) < abs(best.Bandwidth-target.Bandwidth) {
			best, found = v, true
		}
	}
	return best, found
}

func (m Master) Write(buf *bytes.Buffer, uri func(Variant) string) {
	buf.WriteString("#EXTM3U\n")
	for _, tag := range m.Tags {
		buf.WriteString(tag)
		buf.WriteByte('\n')
	}
	for _, v := range m.Variants {
		buf.WriteString(v.Info)
		buf.WriteByte('\n')
		buf.WriteString(uri(v))
		buf.WriteByte('\n')
	}
}

func resolve(base *url.URL, ref string) string {
	if base == nil {
		return ref
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(parsed).String()
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
