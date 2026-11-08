package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/ammyy9908/clipper/internal/auth"
	"github.com/ammyy9908/clipper/internal/config"
	"github.com/ammyy9908/clipper/internal/store"
)

// authState is everything the API needs to know who is calling and how much
// they may download. It is built once from config.
type authState struct {
	verifier *auth.Verifier
	sessions *auth.Sessions
	loc      *time.Location
	trusted  []netip.Prefix
}

func newAuthState(cfg config.Config, log *slog.Logger) authState {
	st := authState{loc: time.UTC}

	if cfg.QuotaTZ != "" {
		if loc, err := time.LoadLocation(cfg.QuotaTZ); err == nil {
			st.loc = loc
		} else {
			log.Warn("unknown QUOTA_TZ, using UTC", "tz", cfg.QuotaTZ, "err", err)
		}
	}
	for _, raw := range cfg.TrustedProxies {
		if p, err := netip.ParsePrefix(raw); err == nil {
			st.trusted = append(st.trusted, p)
		} else if ip, err := netip.ParseAddr(raw); err == nil {
			st.trusted = append(st.trusted, netip.PrefixFrom(ip, ip.BitLen()))
		} else {
			log.Warn("ignoring bad TRUSTED_PROXY_CIDRS entry", "value", raw)
		}
	}

	if cfg.AnonIPDailyLimit > 0 && len(st.trusted) == 0 {
		// Without this, every visitor arriving through a web server or proxy is
		// counted as that server's address, so they share one IP allowance.
		log.Warn("per-IP limits are on but TRUSTED_PROXY_CIDRS is empty: callers are identified by the address that connects to this API. If a web app or proxy sits in front, list its address there")
	}

	if cfg.GoogleClientID == "" && cfg.SessionSecret == "" {
		return st
	}
	sessions, err := auth.NewSessions(cfg.SessionSecret, cfg.SessionTTL)
	if err != nil || cfg.GoogleClientID == "" {
		log.Warn("Google sign-in disabled: needs both GOOGLE_CLIENT_ID and a SESSION_SECRET of 32+ characters", "err", err)
		return st
	}
	st.sessions = sessions
	st.verifier = auth.NewVerifier(cfg.GoogleClientID)
	if cfg.GoogleJWKSURL != "" {
		st.verifier.JWKSURL = cfg.GoogleJWKSURL
	}
	return st
}

func (s authState) enabled() bool { return s.verifier != nil && s.sessions != nil }

func (s authState) location() *time.Location {
	if s.loc == nil {
		return time.UTC
	}
	return s.loc
}

// identity is who a request is from.
type identity struct {
	Signed  bool
	Sub     string
	Email   string
	Name    string
	Picture string

	// OwnerID is what new jobs are filed under: the Google account when signed
	// in (so history follows the person across devices), otherwise the device id.
	OwnerID  string
	DeviceID string
	IP       string
}

// identify works out who is calling. A valid session token means signed in; an
// invalid or expired one is treated as anonymous rather than as an error, so a
// stale login degrades to the free allowance instead of breaking the page.
func (a *API) identify(r *http.Request, bodyID string) identity {
	id := identity{DeviceID: cleanID(requestUserID(r, bodyID)), IP: a.clientIP(r)}

	if a.auth.sessions != nil {
		if tok := bearerToken(r); tok != "" {
			if s, err := a.auth.sessions.Parse(tok); err == nil {
				id.Signed = true
				id.Sub, id.Email, id.Name, id.Picture = s.Sub, s.Email, s.Name, s.Picture
				id.OwnerID = "google:" + s.Sub
				return id
			}
		}
	}
	id.OwnerID = id.DeviceID
	return id
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// cleanID drops anything that isn't a plausible id, so it is safe to use as a
// quota bucket name.
func cleanID(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 128 {
		return ""
	}
	// Signed-in jobs are filed under "google:<sub>". A device id in that form
	// would let a caller list, or add to, someone else's history.
	if strings.HasPrefix(strings.ToLower(s), "google:") {
		return ""
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return s
}

// clientIP is the caller's address. X-Forwarded-For is only believed when the
// request came from a configured trusted proxy, and then the right-most entry
// that isn't itself a trusted proxy is used. Anyone can send the header, so
// trusting it blindly would let a caller pick their own quota bucket.
// IPv6 addresses are reduced to their /64, so rotating within a subnet doesn't help.
func (a *API) clientIP(r *http.Request) string {
	remote := parseIP(r.RemoteAddr)
	ip := remote
	if remote.IsValid() && a.auth.isTrusted(remote) {
		parts := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
		for i := len(parts) - 1; i >= 0; i-- {
			cand, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
			if err != nil {
				continue
			}
			if !a.auth.isTrusted(cand) {
				ip = cand
				break
			}
		}
	}
	if !ip.IsValid() {
		return ""
	}
	ip = ip.Unmap()
	if ip.Is6() {
		p, _ := ip.Prefix(64)
		return p.Addr().String() + "/64"
	}
	return ip.String()
}

func (s authState) isTrusted(ip netip.Addr) bool {
	for _, p := range s.trusted {
		if p.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

func parseIP(hostport string) netip.Addr {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	ip, _ := netip.ParseAddr(host)
	return ip
}

// quotaDay is today's date in the quota time zone, and when it ends.
func (a *API) quotaDay() (day string, resets time.Time) {
	now := time.Now().In(a.auth.location())
	y, m, d := now.Date()
	return now.Format("2006-01-02"), time.Date(y, m, d+1, 0, 0, 0, 0, a.auth.location())
}

// chargeFor is the quota buckets a request's downloads are billed to.
// Signed-in people have one bucket. Anonymous callers are billed to their
// device and their IP and must fit in both.
func (a *API) chargeFor(id identity) store.QuotaCharge {
	day, _ := a.quotaDay()
	c := store.QuotaCharge{Day: day}
	if id.Signed {
		c.Keys = append(c.Keys, store.QuotaKey{Key: "user:" + id.Sub, Limit: a.cfg.UserDailyLimit})
		return c
	}
	if id.DeviceID != "" {
		c.Keys = append(c.Keys, store.QuotaKey{Key: "dev:" + id.DeviceID, Limit: a.cfg.AnonDailyLimit})
	}
	if id.IP != "" {
		c.Keys = append(c.Keys, store.QuotaKey{Key: "ip:" + id.IP, Limit: a.cfg.AnonIPDailyLimit})
	}
	return c
}

func (a *API) limitFor(id identity) int {
	if id.Signed {
		return a.cfg.UserDailyLimit
	}
	return a.cfg.AnonDailyLimit
}

// quotaExceeded answers 429 with what the client needs to explain it. need is
// how many downloads were asked for and remaining how many the caller has left.
func (a *API) quotaExceeded(w http.ResponseWriter, id identity, need, remaining int) {
	_, resets := a.quotaDay()
	limit := a.limitFor(id)

	var msg string
	switch {
	case need > 1:
		msg = fmt.Sprintf("That's %d downloads and you have %d left today.", need, remaining)
	case !id.Signed && a.auth.enabled() && a.cfg.UserDailyLimit != a.cfg.AnonDailyLimit:
		msg = fmt.Sprintf("You've used your %d free downloads for today. Sign in with Google to get %d a day.",
			limit, a.cfg.UserDailyLimit)
	default:
		msg = fmt.Sprintf("You've reached today's limit of %d downloads. It resets tomorrow.", limit)
	}

	w.Header().Set("Retry-After", strconv.Itoa(int(time.Until(resets).Seconds())+1))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{
		"error":     msg,
		"code":      "daily_limit",
		"limit":     limit,
		"remaining": remaining,
		"signed_in": id.Signed,
		"resets_at": resets.UTC().Format(time.RFC3339),
	})
}

type userView struct {
	Email   string `json:"email"`
	Name    string `json:"name,omitempty"`
	Picture string `json:"picture,omitempty"`
}

// authGoogle exchanges a Google ID token (from Google Identity Services in the
// browser) for this API's session token.
func (a *API) authGoogle(w http.ResponseWriter, r *http.Request) {
	if !a.auth.enabled() {
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"error": "sign-in isn't set up on this server",
			"code":  "auth_disabled",
		})
		return
	}
	var req struct {
		Credential string `json:"credential"`
	}
	if err := decode(r, &req); err != nil || req.Credential == "" {
		badRequest(w, "missing credential")
		return
	}

	claims, err := a.auth.verifier.Verify(r.Context(), req.Credential)
	if errors.Is(err, auth.ErrInvalidToken) {
		a.log.Warn("google sign-in rejected", "err", err)
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Google sign-in didn't check out. Try again.",
			"code":  "invalid_token",
		})
		return
	}
	if err != nil {
		a.log.Error("google sign-in could not be verified", "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "Couldn't reach Google to check your sign-in. Try again in a moment.",
			"code":  "verify_unavailable",
		})
		return
	}

	if err := a.db.UpsertUser(r.Context(), store.User{
		ID: "google:" + claims.Sub, Email: claims.Email, Name: claims.Name, Picture: claims.Picture,
	}); err != nil {
		a.log.Error("save user failed", "err", err)
		serverError(w)
		return
	}

	token, exp := a.auth.sessions.Issue(claims)
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_at": exp.UTC().Format(time.RFC3339),
		"user":       userView{Email: claims.Email, Name: claims.Name, Picture: claims.Picture},
	})
}

// me reports who the caller is and how many downloads they have left today.
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	id := a.identify(r, "")
	charge := a.chargeFor(id)
	_, resets := a.quotaDay()

	remaining, err := a.db.Remaining(r.Context(), charge.Day, charge.Keys)
	if err != nil {
		a.log.Error("read usage failed", "err", err)
		serverError(w)
		return
	}

	limit := a.limitFor(id)
	view := map[string]any{
		"signed_in":       id.Signed,
		"auth_enabled":    a.auth.enabled(),
		"limit":           limit, // 0 means unlimited
		"signed_in_limit": a.cfg.UserDailyLimit,
		"resets_at":       resets.UTC().Format(time.RFC3339),
	}
	if id.Signed {
		view["user"] = userView{Email: id.Email, Name: id.Name, Picture: id.Picture}
	}
	if remaining >= 0 {
		// A caller with no device id is only bound by the (larger) IP bucket, but
		// what we tell them is their own limit.
		if limit > 0 && remaining > limit {
			remaining = limit
		}
		view["remaining"] = remaining
		view["used"] = limit - remaining
	}
	writeJSON(w, http.StatusOK, view)
}

// remainingFor is how many downloads the caller has left today, or 0 if that
// can't be read (it only decorates an error, so a failure isn't worth surfacing).
func (a *API) remainingFor(ctx context.Context, id identity) int {
	c := a.chargeFor(id)
	n, err := a.db.Remaining(ctx, c.Day, c.Keys)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
