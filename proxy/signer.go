package proxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"hash"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"time"
)

var (
	ErrBadSignature = errors.New("proxy: bad signature")
	ErrExpired      = errors.New("proxy: link expired")
)

type Signer struct {
	pool sync.Pool
	now  func() time.Time
}

func NewSigner(key []byte) *Signer {
	s := &Signer{now: time.Now}
	s.pool.New = func() any { return &scratch{mac: hmac.New(sha256.New, key), buf: make([]byte, 0, 1024)} }
	return s
}

func (s *Signer) Sign(values url.Values, ttl time.Duration) url.Values {
	return s.SignUntil(values, s.Expiry(ttl))
}

// Expiry rounds to the minute so that links signed within the same minute are identical.
func (s *Signer) Expiry(ttl time.Duration) time.Time {
	return s.now().Add(ttl).Truncate(time.Minute)
}

func (s *Signer) SignUntil(values url.Values, expires time.Time) url.Values {
	signed := make(url.Values, len(values)+2)
	for k, v := range values {
		signed[k] = v
	}
	signed.Set("e", strconv.FormatInt(expires.Unix(), 10))
	var sum [sha256.Size]byte
	s.sum(signed, &sum)
	signed.Set("s", base64.RawURLEncoding.EncodeToString(sum[:]))
	return signed
}

func (s *Signer) Verify(values url.Values) error {
	var got, want [sha256.Size]byte
	raw := values.Get("s")
	if base64.RawURLEncoding.DecodedLen(len(raw)) != sha256.Size {
		return ErrBadSignature
	}
	if _, err := base64.RawURLEncoding.Decode(got[:], []byte(raw)); err != nil {
		return ErrBadSignature
	}
	s.sum(values, &want)
	if !hmac.Equal(got[:], want[:]) {
		return ErrBadSignature
	}
	expires, err := strconv.ParseInt(values.Get("e"), 10, 64)
	if err != nil {
		return ErrBadSignature
	}
	if s.now().Unix() > expires {
		return ErrExpired
	}
	return nil
}

type scratch struct {
	mac  hash.Hash
	buf  []byte
	keys []string
}

func (s *Signer) sum(values url.Values, out *[sha256.Size]byte) {
	sc := s.pool.Get().(*scratch)
	sc.keys = sc.keys[:0]
	for k := range values {
		if k != "s" {
			sc.keys = append(sc.keys, k)
		}
	}
	slices.Sort(sc.keys)
	sc.buf = sc.buf[:0]
	for _, k := range sc.keys {
		sc.buf = appendField(sc.buf, k)
		sc.buf = appendField(sc.buf, values.Get(k))
	}
	sc.mac.Reset()
	sc.mac.Write(sc.buf)
	sc.mac.Sum(out[:0])
	s.pool.Put(sc)
}

func appendField(buf []byte, field string) []byte {
	buf = strconv.AppendInt(buf, int64(len(field)), 10)
	buf = append(buf, ':')
	return append(buf, field...)
}
