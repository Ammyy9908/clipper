package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const sessionPrefix = "cp1."

var ErrInvalidSession = errors.New("invalid session")

// Session is the signed-in person a session token stands for.
type Session struct {
	Sub     string    `json:"sub"`
	Email   string    `json:"email"`
	Name    string    `json:"name,omitempty"`
	Picture string    `json:"picture,omitempty"`
	Expires time.Time `json:"-"`
}

type sessionPayload struct {
	Session
	Exp int64 `json:"exp"`
}

// Sessions issues and checks the API's own tokens. Google's ID tokens only last
// an hour, so we verify one once at sign-in and hand back a longer-lived token
// that needs no call to Google to check.
type Sessions struct {
	secret []byte
	ttl    time.Duration
	Now    func() time.Time
}

// NewSessions needs a secret of at least 32 bytes.
func NewSessions(secret string, ttl time.Duration) (*Sessions, error) {
	if len(secret) < 32 {
		return nil, errors.New("SESSION_SECRET must be at least 32 characters")
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &Sessions{secret: []byte(secret), ttl: ttl, Now: time.Now}, nil
}

func (s *Sessions) mac(signed string) []byte {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(signed))
	return h.Sum(nil)
}

// Issue returns a token for a verified Google account and when it expires.
func (s *Sessions) Issue(c Claims) (string, time.Time) {
	exp := s.Now().Add(s.ttl)
	body, _ := json.Marshal(sessionPayload{
		Session: Session{Sub: c.Sub, Email: c.Email, Name: c.Name, Picture: c.Picture},
		Exp:     exp.Unix(),
	})
	signed := sessionPrefix + base64.RawURLEncoding.EncodeToString(body)
	return signed + "." + base64.RawURLEncoding.EncodeToString(s.mac(signed)), exp
}

// Parse checks a token's signature and expiry.
func (s *Sessions) Parse(token string) (Session, error) {
	fail := func(why string) (Session, error) { return Session{}, fmt.Errorf("%w: %s", ErrInvalidSession, why) }

	if !strings.HasPrefix(token, sessionPrefix) {
		return fail("wrong format")
	}
	i := strings.LastIndex(token, ".")
	if i <= len(sessionPrefix) {
		return fail("wrong format")
	}
	signed, sig := token[:i], token[i+1:]
	want, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return fail("bad signature encoding")
	}
	if !hmac.Equal(want, s.mac(signed)) {
		return fail("signature does not match")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(signed, sessionPrefix))
	if err != nil {
		return fail("bad payload")
	}
	var p sessionPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.Sub == "" {
		return fail("bad payload")
	}
	p.Expires = time.Unix(p.Exp, 0)
	if !p.Expires.After(s.Now()) {
		return fail("expired")
	}
	return p.Session.withExpiry(p.Expires), nil
}

func (s Session) withExpiry(t time.Time) Session {
	s.Expires = t
	return s
}
