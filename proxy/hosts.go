package proxy

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var ErrHostNotAllowed = errors.New("proxy: host not allowed")

var DefaultAllowedSuffixes = []string{
	".ttvnw.net",
	".live-video.net",
	".twitch.tv",
	".twitchcdn.net",
	".jtvnw.net",
	".cloudfront.net",
}

type Hosts struct {
	suffixes      []string
	AllowInsecure bool
}

func NewHosts(suffixes []string) *Hosts {
	return &Hosts{suffixes: suffixes}
}

func (h *Hosts) Allowed(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return nil, ErrHostNotAllowed
	}
	if u.Scheme != "https" && (!h.AllowInsecure || u.Scheme != "http") {
		return nil, ErrHostNotAllowed
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range h.suffixes {
		if strings.HasSuffix(host, suffix) {
			return u, nil
		}
	}
	return nil, ErrHostNotAllowed
}

func PublicDialer(timeout time.Duration, allowPrivate bool) *net.Dialer {
	if allowPrivate {
		return &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	}
	return &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || !isPublic(ip) {
				return ErrHostNotAllowed
			}
			return nil
		},
	}
}

func isPublic(ip net.IP) bool {
	return !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() &&
		!ip.IsUnspecified() && !ip.IsMulticast() && !ip.IsInterfaceLocalMulticast()
}
