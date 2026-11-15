package chat

import (
	"strconv"
	"strings"
	"time"
)

type Emote struct {
	ID    string `json:"id"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

type Event struct {
	Type        string    `json:"type"`
	ID          string    `json:"id,omitempty"`
	Login       string    `json:"login,omitempty"`
	DisplayName string    `json:"displayName,omitempty"`
	Color       string    `json:"color,omitempty"`
	Text        string    `json:"text,omitempty"`
	System      string    `json:"system,omitempty"`
	Emotes      []Emote   `json:"emotes,omitempty"`
	Badges      []string  `json:"badges,omitempty"`
	Time        time.Time `json:"time"`
	Action      bool      `json:"action,omitempty"`
	TargetID    string    `json:"targetId,omitempty"`
}

type line struct {
	tags    map[string]string
	prefix  string
	command string
	params  []string
}

func parseLine(raw string) line {
	var l line
	if strings.HasPrefix(raw, "@") {
		tags, rest, _ := strings.Cut(raw[1:], " ")
		l.tags = parseTags(tags)
		raw = rest
	}
	if strings.HasPrefix(raw, ":") {
		l.prefix, raw, _ = strings.Cut(raw[1:], " ")
	}
	head, trailing, hasTrailing := strings.Cut(raw, " :")
	fields := strings.Fields(head)
	if len(fields) > 0 {
		l.command = fields[0]
		l.params = fields[1:]
	}
	if hasTrailing {
		l.params = append(l.params, trailing)
	}
	return l
}

var tagEscapes = strings.NewReplacer(`\s`, " ", `\:`, ";", `\\`, `\`, `\r`, "\r", `\n`, "\n")

func parseTags(raw string) map[string]string {
	tags := make(map[string]string, 16)
	for part := range strings.SplitSeq(raw, ";") {
		key, value, _ := strings.Cut(part, "=")
		tags[key] = tagEscapes.Replace(value)
	}
	return tags
}

func (l line) event() (Event, bool) {
	switch l.command {
	case "PRIVMSG", "USERNOTICE":
		ev := Event{
			Type:        "message",
			ID:          l.tags["id"],
			Login:       l.tags["login"],
			DisplayName: l.tags["display-name"],
			Color:       l.tags["color"],
			Emotes:      parseEmotes(l.tags["emotes"]),
			Badges:      parseBadges(l.tags["badges"]),
			Time:        parseTime(l.tags["tmi-sent-ts"]),
		}
		if ev.Login == "" {
			ev.Login, _, _ = strings.Cut(l.prefix, "!")
		}
		if l.command == "USERNOTICE" {
			ev.Type = "notice"
			ev.System = l.tags["system-msg"]
		}
		if len(l.params) > 1 {
			ev.Text = l.params[len(l.params)-1]
		}
		if text, ok := strings.CutPrefix(ev.Text, "\x01ACTION "); ok {
			ev.Text, ev.Action = strings.TrimSuffix(text, "\x01"), true
		}
		return ev, true
	case "CLEARMSG":
		return Event{Type: "delete", TargetID: l.tags["target-msg-id"], Time: time.Now()}, true
	case "CLEARCHAT":
		ev := Event{Type: "clear", Time: time.Now()}
		if len(l.params) > 1 {
			ev.Login = l.params[1]
		}
		return ev, true
	}
	return Event{}, false
}

func parseEmotes(raw string) []Emote {
	if raw == "" {
		return nil
	}
	var emotes []Emote
	for group := range strings.SplitSeq(raw, "/") {
		id, ranges, ok := strings.Cut(group, ":")
		if !ok {
			continue
		}
		for r := range strings.SplitSeq(ranges, ",") {
			from, to, ok := strings.Cut(r, "-")
			if !ok {
				continue
			}
			start, err1 := strconv.Atoi(from)
			end, err2 := strconv.Atoi(to)
			if err1 == nil && err2 == nil {
				emotes = append(emotes, Emote{ID: id, Start: start, End: end})
			}
		}
	}
	return emotes
}

func parseBadges(raw string) []string {
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func parseTime(raw string) time.Time {
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Now()
	}
	return time.UnixMilli(ms)
}
