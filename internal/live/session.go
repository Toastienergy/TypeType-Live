package live

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type session struct {
	token   string
	expires time.Time
}

type sessions struct {
	mu    sync.Mutex
	items map[string]session
	now   func() time.Time
}

func newSessions() *sessions {
	return &sessions{items: make(map[string]session), now: time.Now}
}

func (s *sessions) put(token string, ttl time.Duration) string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	key := base64.RawURLEncoding.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for k, v := range s.items {
		if now.After(v.expires) {
			delete(s.items, k)
		}
	}
	s.items[key] = session{token: token, expires: now.Add(ttl)}
	return key
}

func (s *sessions) get(key string) (string, bool) {
	if key == "" {
		return "", true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.items[key]
	if !ok || s.now().After(v.expires) {
		return "", false
	}
	return v.token, true
}
