package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ammyy9908/clipper/internal/auth"
	"github.com/ammyy9908/clipper/internal/config"
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func authAPI(t *testing.T, mut func(*config.Config)) *API {
	t.Helper()
	cfg := config.Config{
		GoogleClientID: "web.apps.googleusercontent.com",
		SessionSecret:  strings.Repeat("k", 40),
		SessionTTL:     time.Hour,
		AnonDailyLimit: 3, AnonIPDailyLimit: 9, UserDailyLimit: 20,
		QuotaTZ: "UTC",
	}
	if mut != nil {
		mut(&cfg)
	}
	return &API{cfg: cfg, log: quietLog(), auth: newAuthState(cfg, quietLog())}
}

func req(remote string, headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	r.RemoteAddr = remote
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestClientIPIgnoresForwardedForFromStrangers(t *testing.T) {
	a := authAPI(t, func(c *config.Config) { c.TrustedProxies = []string{"10.0.0.0/8"} })

	// Anyone can send X-Forwarded-For. From an address that isn't a trusted
	// proxy it must be ignored, or callers could pick their own quota bucket.
	got := a.clientIP(req("203.0.113.9:5000", map[string]string{"X-Forwarded-For": "1.2.3.4"}))
	if got != "203.0.113.9" {
		t.Errorf("a stranger's X-Forwarded-For must be ignored, got %q", got)
	}
}

func TestClientIPTrustsForwardedForFromTheProxy(t *testing.T) {
	a := authAPI(t, func(c *config.Config) { c.TrustedProxies = []string{"10.0.0.0/8", "192.168.1.5"} })

	cases := []struct {
		name, remote, xff, want string
	}{
		{"one proxy", "10.1.2.3:80", "198.51.100.7", "198.51.100.7"},
		{"client tried to spoof, proxy appended the real one", "10.1.2.3:80", "1.1.1.1, 198.51.100.7", "198.51.100.7"},
		{"two proxies in a row", "10.1.2.3:80", "198.51.100.7, 10.9.9.9", "198.51.100.7"},
		{"a single trusted address, not a range", "192.168.1.5:80", "198.51.100.8", "198.51.100.8"},
		{"trusted proxy with no header", "10.1.2.3:80", "", "10.1.2.3"},
		{"garbage header falls back to the proxy", "10.1.2.3:80", "not-an-ip", "10.1.2.3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := map[string]string{}
			if c.xff != "" {
				h["X-Forwarded-For"] = c.xff
			}
			if got := a.clientIP(req(c.remote, h)); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestClientIPGroupsIPv6ByPrefix(t *testing.T) {
	a := authAPI(t, nil)
	x := a.clientIP(req("[2001:db8:1:2:aaaa:bbbb:cccc:dddd]:1", nil))
	y := a.clientIP(req("[2001:db8:1:2:1111:2222:3333:4444]:1", nil))
	other := a.clientIP(req("[2001:db8:1:9::1]:1", nil))
	if x != y || x == other || !strings.HasSuffix(x, "/64") {
		t.Errorf("addresses in one /64 must share a bucket and other subnets must not: %q %q %q", x, y, other)
	}
}

func TestIdentifyUsesAValidSessionOnly(t *testing.T) {
	a := authAPI(t, nil)
	token, _ := a.auth.sessions.Issue(auth.Claims{Sub: "42", Email: "asha@example.com", Name: "Asha"})

	signed := a.identify(req("203.0.113.1:1", map[string]string{"Authorization": "Bearer " + token, "X-Device-ID": "phone-1"}), "")
	if !signed.Signed || signed.OwnerID != "google:42" || signed.Email != "asha@example.com" || signed.DeviceID != "phone-1" {
		t.Errorf("a valid session means signed in, owned by the account: %+v", signed)
	}

	for name, header := range map[string]string{
		"garbage":      "Bearer nonsense",
		"wrong scheme": "Basic " + token,
		"empty":        "Bearer ",
		"other secret": "Bearer " + otherToken(),
		"tampered":     "Bearer " + token[:len(token)-3] + "AAA",
		"no header":    "",
	} {
		t.Run(name, func(t *testing.T) {
			h := map[string]string{"X-Device-ID": "phone-1"}
			if header != "" {
				h["Authorization"] = header
			}
			id := a.identify(req("203.0.113.1:1", h), "")
			if id.Signed || id.OwnerID != "phone-1" {
				t.Errorf("a bad session must degrade to anonymous on the device id: %+v", id)
			}
		})
	}
}

func otherToken() string {
	s, _ := auth.NewSessions(strings.Repeat("z", 40), time.Hour)
	tok, _ := s.Issue(auth.Claims{Sub: "42", Email: "a@b.c"})
	return tok
}

func TestChargeBucketsAndLimits(t *testing.T) {
	a := authAPI(t, nil)
	token, _ := a.auth.sessions.Issue(auth.Claims{Sub: "42", Email: "a@b.c"})

	anon := a.chargeFor(a.identify(req("203.0.113.1:1", map[string]string{"X-Device-ID": "phone-1"}), ""))
	if len(anon.Keys) != 2 || anon.Keys[0].Key != "dev:phone-1" || anon.Keys[0].Limit != 3 ||
		anon.Keys[1].Key != "ip:203.0.113.1" || anon.Keys[1].Limit != 9 {
		t.Errorf("anonymous callers are charged to device and IP: %+v", anon.Keys)
	}

	signed := a.chargeFor(a.identify(req("203.0.113.1:1", map[string]string{"Authorization": "Bearer " + token}), ""))
	if len(signed.Keys) != 1 || signed.Keys[0].Key != "user:42" || signed.Keys[0].Limit != 20 {
		t.Errorf("signed-in people have one bucket with the higher limit: %+v", signed.Keys)
	}

	// Rotating the device id doesn't help: the IP bucket is shared.
	other := a.chargeFor(a.identify(req("203.0.113.1:1", map[string]string{"X-Device-ID": "brand-new-id"}), ""))
	if other.Keys[1] != anon.Keys[1] {
		t.Errorf("a fresh device id from the same IP must land in the same IP bucket: %+v vs %+v", other.Keys[1], anon.Keys[1])
	}
}

func TestCleanIDRejectsAbuse(t *testing.T) {
	for in, want := range map[string]string{
		"  device-1 ":            "device-1",
		"":                       "",
		strings.Repeat("a", 129): "",
		"has\nnewline":           "",
		"nul\x00byte":            "",
	} {
		if got := cleanID(in); got != want {
			t.Errorf("cleanID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuotaExceededResponse(t *testing.T) {
	a := authAPI(t, nil)

	rec := httptest.NewRecorder()
	a.quotaExceeded(rec, identity{}, 1, 0)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != "daily_limit" || body["limit"].(float64) != 3 || body["signed_in"] != false || body["remaining"].(float64) != 0 {
		t.Errorf("body wrong: %v", body)
	}
	if !strings.Contains(body["error"].(string), "Sign in with Google") || !strings.Contains(body["error"].(string), "20") {
		t.Errorf("an anonymous person should be told signing in gives more: %v", body["error"])
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("a 429 should say when to retry")
	}
	if _, err := time.Parse(time.RFC3339, body["resets_at"].(string)); err != nil {
		t.Errorf("resets_at must be a timestamp: %v", err)
	}

	signed := httptest.NewRecorder()
	a.quotaExceeded(signed, identity{Signed: true}, 1, 0)
	if strings.Contains(signed.Body.String(), "Sign in") {
		t.Errorf("someone already signed in must not be told to sign in: %s", signed.Body)
	}

	// Without Google configured there's nothing to sign in to, so don't suggest it.
	off := authAPI(t, func(c *config.Config) { c.GoogleClientID = "" })
	plain := httptest.NewRecorder()
	off.quotaExceeded(plain, identity{}, 1, 0)
	if strings.Contains(plain.Body.String(), "Sign in") {
		t.Errorf("don't suggest signing in when it isn't available: %s", plain.Body)
	}

	playlist := httptest.NewRecorder()
	a.quotaExceeded(playlist, identity{}, 12, 3)
	if !strings.Contains(playlist.Body.String(), "12 downloads") || !strings.Contains(playlist.Body.String(), "3 left") {
		t.Errorf("a playlist refusal should say how many it needed and how many are left: %s", playlist.Body)
	}
}

func TestAuthGoogleEndpointGuards(t *testing.T) {
	off := authAPI(t, func(c *config.Config) { c.GoogleClientID = ""; c.SessionSecret = "" })
	if off.auth.enabled() {
		t.Fatal("sign-in must be disabled without configuration")
	}
	rec := post(t, off.authGoogle, "/v1/auth/google", `{"credential":"x"}`)
	if rec.Code != http.StatusNotImplemented || !strings.Contains(rec.Body.String(), "auth_disabled") {
		t.Errorf("want 501 auth_disabled, got %d %s", rec.Code, rec.Body)
	}

	on := authAPI(t, nil)
	if rec := post(t, on.authGoogle, "/v1/auth/google", `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("missing credential should be 400, got %d", rec.Code)
	}
	// A token that isn't from Google is refused before anything touches the database.
	on.auth.verifier.JWKSURL = "http://127.0.0.1:1/unreachable"
	rec = post(t, on.authGoogle, "/v1/auth/google", `{"credential":"not.a.token"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("a malformed token is 401, got %d %s", rec.Code, rec.Body)
	}
}

func TestPartialAuthConfigDisablesSignIn(t *testing.T) {
	for name, mut := range map[string]func(*config.Config){
		"client id but no secret": func(c *config.Config) { c.SessionSecret = "" },
		"secret but no client id": func(c *config.Config) { c.GoogleClientID = "" },
		"weak secret":             func(c *config.Config) { c.SessionSecret = "short" },
	} {
		t.Run(name, func(t *testing.T) {
			if authAPI(t, mut).auth.enabled() {
				t.Error("half-configured sign-in must stay off rather than run insecurely")
			}
		})
	}
}

func TestCleanIDRefusesGoogleOwners(t *testing.T) {
	for _, s := range []string{"google:1234567890", "GOOGLE:1", " google:9 "} {
		if got := cleanID(s); got != "" {
			t.Errorf("cleanID(%q) = %q, want it refused", s, got)
		}
	}
	if got := cleanID("6f1c2a3e-1111-4222-8333-944455556666"); got == "" {
		t.Error("a normal device id was refused")
	}
}
