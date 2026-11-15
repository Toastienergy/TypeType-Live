package proxy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

var ErrSealed = errors.New("proxy: cannot open sealed value")

type Sealer struct {
	aead     cipher.AEAD
	nonceKey []byte
}

func NewSealer(secret []byte) (*Sealer, error) {
	key := sha256.Sum256(append([]byte("typetype-live-seal:"), secret...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceKey := sha256.Sum256(append([]byte("typetype-live-nonce:"), secret...))
	return &Sealer{aead: aead, nonceKey: nonceKey[:]}, nil
}

func (s *Sealer) Seal(plain string) string {
	mac := hmac.New(sha256.New, s.nonceKey)
	mac.Write([]byte(plain))
	nonce := make([]byte, s.aead.NonceSize(), s.aead.NonceSize()+len(plain)+s.aead.Overhead())
	copy(nonce, mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString(s.aead.Seal(nonce, nonce, []byte(plain), nil))
}

func (s *Sealer) Open(sealed string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(sealed)
	if err != nil || len(raw) < s.aead.NonceSize() {
		return "", ErrSealed
	}
	plain, err := s.aead.Open(nil, raw[:s.aead.NonceSize()], raw[s.aead.NonceSize():], nil)
	if err != nil {
		return "", ErrSealed
	}
	return string(plain), nil
}
