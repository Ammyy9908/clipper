package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const clientID = "web-client.apps.googleusercontent.com"

type keyServer struct {
	*httptest.Server
	hits atomic.Int32
	keys atomic.Pointer[[]jwk]
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func toJWK(kid string, k *rsa.PublicKey) jwk {
	return jwk{Kid: kid, Kty: "RSA",
		N: base64.RawURLEncoding.EncodeToString(k.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes())}
}

func newKeyServer(t *testing.T, keys ...jwk) *keyServer {
	t.Helper()
	ks := &keyServer{}
	ks.keys.Store(&keys)
	ks.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ks.hits.Add(1)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": *ks.keys.Load()})
	}))
	t.Cleanup(ks.Close)
	return ks
}

func seg(v any) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b)
}

// sign builds a JWT signed with RS256, letting a test bend any claim.
func sign(t *testing.T, key *rsa.PrivateKey, header, claims map[string]any) string {
	t.Helper()
	signed := seg(header) + "." + seg(claims)
	digest := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func goodClaims(mods ...func(map[string]any)) map[string]any {
	c := map[string]any{
		"iss": "https://accounts.google.com", "aud": clientID, "sub": "1234567890",
		"email": "asha@example.com", "email_verified": true, "name": "Asha Rao", "picture": "https://x/p.jpg",
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
	}
	for _, m := range mods {
		m(c)
	}
	return c
}

func goodHeader() map[string]any { return map[string]any{"alg": "RS256", "kid": "k1", "typ": "JWT"} }

func verifier(ks *keyServer) *Verifier {
	v := NewVerifier(clientID)
	v.JWKSURL = ks.URL
	return v
}

func TestVerifyAcceptsAValidToken(t *testing.T) {
	key := newKey(t)
	ks := newKeyServer(t, toJWK("k1", &key.PublicKey))
	got, err := verifier(ks).Verify(context.Background(), sign(t, key, goodHeader(), goodClaims()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Sub != "1234567890" || got.Email != "asha@example.com" || got.Name != "Asha Rao" {
		t.Errorf("claims wrong: %+v", got)
	}
}

func TestVerifyMultipleClientIDs(t *testing.T) {
	key := newKey(t)
	ks := newKeyServer(t, toJWK("k1", &key.PublicKey))

	const (
		webClient = "web-client.apps.googleusercontent.com"
		iosClient = "ios-client.apps.googleusercontent.com"
	)
	v := NewVerifier(webClient + "," + iosClient)
	v.JWKSURL = ks.URL

	// 1. Direct web token
	got, err := v.Verify(context.Background(), sign(t, key, goodHeader(), goodClaims(func(c map[string]any) {
		c["aud"] = webClient
	})))
	if err != nil {
		t.Fatalf("expected web token to verify: %v", err)
	}
	if got.Email != "asha@example.com" {
		t.Errorf("claims wrong: %+v", got)
	}

	// 2. Direct iOS token (aud = iOS client ID)
	got, err = v.Verify(context.Background(), sign(t, key, goodHeader(), goodClaims(func(c map[string]any) {
		c["aud"] = iosClient
	})))
	if err != nil {
		t.Fatalf("expected direct iOS token to verify: %v", err)
	}
	if got.Email != "asha@example.com" {
		t.Errorf("claims wrong: %+v", got)
	}

	// 3. iOS token with server client ID (aud = Web Client ID, azp = iOS Client ID)
	got, err = v.Verify(context.Background(), sign(t, key, goodHeader(), goodClaims(func(c map[string]any) {
		c["aud"] = webClient
		c["azp"] = iosClient
	})))
	if err != nil {
		t.Fatalf("expected iOS server-client token to verify: %v", err)
	}
	if got.Email != "asha@example.com" {
		t.Errorf("claims wrong: %+v", got)
	}
}

func TestVerifyRejectsForgeries(t *testing.T) {
	key, attacker := newKey(t), newKey(t)
	ks := newKeyServer(t, toJWK("k1", &key.PublicKey))

	cases := map[string]string{
		"signed by a key Google never published": sign(t, attacker, goodHeader(), goodClaims()),
		"wrong audience (a different app)":       sign(t, key, goodHeader(), goodClaims(func(c map[string]any) { c["aud"] = "other-app" })),
		"wrong issuer":                           sign(t, key, goodHeader(), goodClaims(func(c map[string]any) { c["iss"] = "https://evil.example" })),
		"expired":                                sign(t, key, goodHeader(), goodClaims(func(c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() })),
		"issued in the future":                   sign(t, key, goodHeader(), goodClaims(func(c map[string]any) { c["iat"] = time.Now().Add(time.Hour).Unix() })),
		"email not verified":                     sign(t, key, goodHeader(), goodClaims(func(c map[string]any) { c["email_verified"] = false })),
		"no subject":                             sign(t, key, goodHeader(), goodClaims(func(c map[string]any) { delete(c, "sub") })),
		"no email":                               sign(t, key, goodHeader(), goodClaims(func(c map[string]any) { delete(c, "email") })),
		"alg none":                               seg(map[string]any{"alg": "none", "kid": "k1"}) + "." + seg(goodClaims()) + ".",
		"alg HS256 (key confusion)":              sign(t, key, map[string]any{"alg": "HS256", "kid": "k1"}, goodClaims()),
		"no kid":                                 sign(t, key, map[string]any{"alg": "RS256"}, goodClaims()),
		"garbage":                                "not.a.token",
		"empty":                                  "",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := verifier(ks).Verify(context.Background(), token)
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("must be rejected as invalid, got %v", err)
			}
		})
	}
}

func TestVerifyRejectsATamperedPayload(t *testing.T) {
	key := newKey(t)
	ks := newKeyServer(t, toJWK("k1", &key.PublicKey))
	parts := strings.Split(sign(t, key, goodHeader(), goodClaims()), ".")
	// Swap in a different account but keep the original signature.
	forged := parts[0] + "." + seg(goodClaims(func(c map[string]any) { c["sub"] = "victim"; c["email"] = "victim@example.com" })) + "." + parts[2]
	if _, err := verifier(ks).Verify(context.Background(), forged); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("a payload edited after signing must fail: %v", err)
	}
}

func TestVerifyAcceptsStringEmailVerified(t *testing.T) {
	key := newKey(t)
	ks := newKeyServer(t, toJWK("k1", &key.PublicKey))
	tok := sign(t, key, goodHeader(), goodClaims(func(c map[string]any) { c["email_verified"] = "true" }))
	if _, err := verifier(ks).Verify(context.Background(), tok); err != nil {
		t.Fatalf("Google has sent email_verified as a string: %v", err)
	}
}

func TestKeysAreCachedAndRotationIsPickedUp(t *testing.T) {
	old, rotated := newKey(t), newKey(t)
	ks := newKeyServer(t, toJWK("old", &old.PublicKey))
	v := verifier(ks)
	now := time.Now()
	v.Now = func() time.Time { return now }

	hOld := map[string]any{"alg": "RS256", "kid": "old"}
	for i := 0; i < 3; i++ {
		if _, err := v.Verify(context.Background(), sign(t, old, hOld, goodClaims())); err != nil {
			t.Fatal(err)
		}
	}
	if ks.hits.Load() != 1 {
		t.Errorf("keys should be fetched once and cached, fetched %d times", ks.hits.Load())
	}

	// Google rotates: a token with a new kid arrives. Within a minute of the
	// last fetch we don't hammer Google...
	ks.keys.Store(&[]jwk{toJWK("old", &old.PublicKey), toJWK("new", &rotated.PublicKey)})
	hNew := map[string]any{"alg": "RS256", "kid": "new"}
	if _, err := v.Verify(context.Background(), sign(t, rotated, hNew, goodClaims())); err == nil {
		t.Fatal("an unknown key within a minute of the last fetch should not trigger another fetch")
	}
	// ...but after a minute we do, and the new key works.
	now = now.Add(2 * time.Minute)
	if _, err := v.Verify(context.Background(), sign(t, rotated, hNew, goodClaims())); err != nil {
		t.Fatalf("a rotated key should be picked up after the refetch window: %v", err)
	}
}

func TestVerifyFailsClosedWhenKeysAreUnreachable(t *testing.T) {
	v := NewVerifier(clientID)
	v.JWKSURL = "http://127.0.0.1:1/certs" // nothing listens here
	key := newKey(t)
	if _, err := v.Verify(context.Background(), sign(t, key, goodHeader(), goodClaims())); err == nil {
		t.Fatal("if Google's keys can't be fetched, sign-in must fail, not succeed")
	}
}

func testSessions(t *testing.T) *Sessions {
	t.Helper()
	s, err := NewSessions(strings.Repeat("s3cret-", 6), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSessionRoundTrip(t *testing.T) {
	s := testSessions(t)
	tok, exp := s.Issue(Claims{Sub: "42", Email: "a@b.c", Name: "A", Picture: "p"})
	got, err := s.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sub != "42" || got.Email != "a@b.c" || got.Name != "A" || !got.Expires.Equal(exp.Truncate(time.Second)) {
		t.Errorf("session wrong: %+v (exp %v)", got, exp)
	}
}

func TestSessionRejectsTamperingAndExpiry(t *testing.T) {
	s := testSessions(t)
	tok, _ := s.Issue(Claims{Sub: "42", Email: "a@b.c"})

	parts := strings.Split(tok, ".")
	other, _ := s.Issue(Claims{Sub: "1", Email: "admin@x.y"})
	otherPayload := strings.Split(other, ".")[1]
	forged := parts[0] + "." + otherPayload + "." + parts[2]

	otherSecret, _ := NewSessions(strings.Repeat("different", 5), time.Hour)
	late := testSessions(t)
	late.Now = func() time.Time { return time.Now().Add(2 * time.Hour) }

	for name, token := range map[string]string{
		"payload swapped, signature kept": forged,
		"signed with another secret":      mustIssue(otherSecret),
		"truncated":                       tok[:len(tok)-4],
		"empty":                           "",
		"a Google token is not a session": "eyJhbGciOiJSUzI1NiJ9.e30.abc",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := s.Parse(token); !errors.Is(err, ErrInvalidSession) {
				t.Fatalf("must be rejected, got %v", err)
			}
		})
	}
	if _, err := late.Parse(tok); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("an expired session must be rejected, got %v", err)
	}
}

func mustIssue(s *Sessions) string { t, _ := s.Issue(Claims{Sub: "42", Email: "a@b.c"}); return t }

func TestShortSecretIsRefused(t *testing.T) {
	if _, err := NewSessions("too-short", time.Hour); err == nil {
		t.Fatal("a weak secret must be refused")
	}
}
