package hls

import "strings"

func Attributes(line string) map[string]string {
	_, list, found := strings.Cut(line, ":")
	if !found {
		return nil
	}
	attrs := make(map[string]string, 8)
	for list != "" {
		key, rest, ok := strings.Cut(list, "=")
		if !ok {
			break
		}
		var value string
		if strings.HasPrefix(rest, `"`) {
			end := strings.IndexByte(rest[1:], '"')
			if end < 0 {
				value, rest = rest[1:], ""
			} else {
				value, rest = rest[1:end+1], rest[end+2:]
			}
		} else {
			value, rest, _ = strings.Cut(rest, ",")
			attrs[strings.TrimSpace(key)] = value
			list = rest
			continue
		}
		attrs[strings.TrimSpace(key)] = value
		list = strings.TrimPrefix(rest, ",")
	}
	return attrs
}

func tagName(line string) string {
	name, _, _ := strings.Cut(line, ":")
	return name
}

func mapURIAttr(line string, fn func(string) string) string {
	i := strings.Index(line, `URI="`)
	if i < 0 {
		return line
	}
	start := i + len(`URI="`)
	end := strings.IndexByte(line[start:], '"')
	if end < 0 {
		return line
	}
	return line[:start] + fn(line[start:start+end]) + line[start+end:]
}
