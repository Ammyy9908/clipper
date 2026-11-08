// Package auth verifies Google sign-in and issues the API's own session tokens.
package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GoogleJWKSURL is where Google publishes the public keys its ID tokens are signed with.
const GoogleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

var ErrInvalidToken = errors.New("invalid Google token")

// Claims is what we keep from a verified Google ID token.
type Claims struct {
	Sub     string
	Email   string
	Name    string
	Picture string
}

// Verifier checks Google ID tokens: RS256 signature against Google's published
// keys, issuer, audience (our web & mobile client ids), and expiry.
type Verifier struct {
	ClientIDs []string
	JWKSURL   string
	Client    *http.Client
	Now       func() time.Time

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
	lastTry time.Time
}

func NewVerifier(clientIDs ...string) *Verifier {
	var list []string
	for _, raw := range clientIDs {
		for _, id := range strings.Split(raw, ",") {
			if id = strings.TrimSpace(id); id != "" {
				list = append(list, id)
			}
		}
	}
	return &Verifier{
		ClientIDs: list,
		JWKSURL:   GoogleJWKSURL,
		Client:    &http.Client{Timeout: 8 * time.Second},
		Now:       time.Now,
	}
}

type header struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

type payload struct {
	Iss           string          `json:"iss"`
	Aud           string          `json:"aud"`
	Azp           string          `json:"azp,omitempty"`
	Sub           string          `json:"sub"`
	Exp           float64         `json:"exp"`
	Iat           float64         `json:"iat"`
	Email         string          `json:"email"`
	EmailVerified json.RawMessage `json:"email_verified"`
	Name          string          `json:"name"`
	Picture       string          `json:"picture"`
}

func decodeSegment(s string, into any) error {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

// Verify returns the claims of a valid token, or an error wrapping ErrInvalidToken.
func (v *Verifier) Verify(ctx context.Context, idToken string) (Claims, error) {
	fail := func(why string) (Claims, error) { return Claims{}, fmt.Errorf("%w: %s", ErrInvalidToken, why) }

	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return fail("not a JWT")
	}
	var h header
	if err := decodeSegment(parts[0], &h); err != nil {
		return fail("bad header")
	}
	// Pin the algorithm. Accepting whatever the token claims is how "alg: none"
	// and key-confusion forgeries happen.
	if h.Alg != "RS256" || h.Kid == "" {
		return fail("unsupported algorithm")
	}
	var p payload
	if err := decodeSegment(parts[1], &p); err != nil {
		return fail("bad payload")
	}

	key, err := v.keyFor(ctx, h.Kid)
	if err != nil {
		return Claims{}, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fail("bad signature encoding")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		return fail("signature does not match")
	}

	now := v.Now()
	if p.Iss != "accounts.google.com" && p.Iss != "https://accounts.google.com" {
		return fail("wrong issuer")
	}
	if len(v.ClientIDs) > 0 {
		validAud := false
		for _, cid := range v.ClientIDs {
			if p.Aud == cid || (p.Azp != "" && p.Azp == cid) {
				validAud = true
				break
			}
		}
		if !validAud {
			return fail(fmt.Sprintf("wrong audience (aud=%s, azp=%s)", p.Aud, p.Azp))
		}
	}
	if time.Unix(int64(p.Exp), 0).Before(now) {
		return fail("expired")
	}
	if time.Unix(int64(p.Iat), 0).After(now.Add(5 * time.Minute)) {
		return fail("issued in the future")
	}
	if p.Sub == "" || p.Email == "" {
		return fail("missing subject or email")
	}
	if !truthy(p.EmailVerified) {
		return fail("email not verified")
	}
	return Claims{Sub: p.Sub, Email: p.Email, Name: p.Name, Picture: p.Picture}, nil
}

// truthy accepts both JSON true and the string "true" (Google has sent both).
func truthy(raw json.RawMessage) bool {
	s := strings.Trim(string(raw), `"`)
	return s == "true"
}

func (v *Verifier) keyFor(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.Now()
	if k, ok := v.keys[kid]; ok && now.Before(v.expires) {
		return k, nil
	}
	// Unknown kid or stale cache: refetch, but not more than once a minute, so a
	// stream of garbage tokens can't turn into a stream of requests to Google.
	if v.keys != nil && now.Sub(v.lastTry) < time.Minute {
		if k, ok := v.keys[kid]; ok {
			return k, nil
		}
		return nil, fmt.Errorf("%w: unknown signing key", ErrInvalidToken)
	}
	v.lastTry = now

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.JWKSURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Google keys: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch Google keys: status %d", resp.StatusCode)
	}
	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode Google keys: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		exp := int(new(big.Int).SetBytes(e).Int64())
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exp}
	}
	v.keys = keys
	v.expires = now.Add(cacheTTL(resp.Header.Get("Cache-Control")))

	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("%w: unknown signing key", ErrInvalidToken)
}

var maxAge = regexp.MustCompile(`max-age=(\d+)`)

func cacheTTL(cacheControl string) time.Duration {
	if m := maxAge.FindStringSubmatch(cacheControl); m != nil {
		if secs, err := strconv.Atoi(m[1]); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return time.Hour
}
