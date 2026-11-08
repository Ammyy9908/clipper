package api

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// limiter is a token bucket per caller: a burst of perMinute requests, refilled
// evenly over the minute. It lives in this process, which is right for a single
// API instance; behind several, each would count on its own.
type limiter struct {
	mu      sync.Mutex
	rate    float64 // tokens added per second
	burst   float64
	buckets map[string]*bucket
	sweptAt time.Time
	now     func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// maxBuckets bounds memory. Past it, callers we aren't yet tracking are let
// through rather than growing the map without limit.
const maxBuckets = 200_000

// newLimiter returns nil for perMinute <= 0, meaning "no limit".
func newLimiter(perMinute int) *limiter {
	if perMinute <= 0 {
		return nil
	}
	return &limiter{
		rate:    float64(perMinute) / 60,
		burst:   float64(perMinute),
		buckets: map[string]*bucket{},
		now:     time.Now,
	}
}

// allow spends one token for key. When there is none, it says how long until there is.
func (l *limiter) allow(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)

	b := l.buckets[key]
	if b == nil {
		if len(l.buckets) >= maxBuckets {
			return true, 0
		}
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) / l.rate * float64(time.Second))
}

// sweep forgets callers idle long enough to have a full bucket again, at most once a minute.
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.sweptAt) < time.Minute {
		return
	}
	l.sweptAt = now
	idle := time.Duration(l.burst/l.rate*float64(time.Second)) + time.Minute
	for k, b := range l.buckets {
		if now.Sub(b.last) > idle {
			delete(l.buckets, k)
		}
	}
}

// limits holds one limiter per kind of request, so a page polling for job
// status can't use up the allowance for starting downloads.
type limits struct {
	resolve, write, auth, read *limiter
}

// limited wraps a handler with a per-IP limit. A nil limiter passes everything.
func (a *API) limited(l *limiter, next http.HandlerFunc) http.HandlerFunc {
	if l == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		key := a.clientIP(r)
		if key == "" {
			key = "unknown"
		}
		if ok, retry := l.allow(key); !ok {
			secs := int(math.Ceil(retry.Seconds()))
			if secs < 1 {
				secs = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":       "Too many requests. Wait a moment and try again.",
				"code":        "rate_limited",
				"retry_after": secs,
			})
			return
		}
		next(w, r)
	}
}
