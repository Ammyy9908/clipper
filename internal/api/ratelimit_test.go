package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ammyy9908/clipper/internal/config"
)

func clockedLimiter(perMin int) (*limiter, *time.Time) {
	now := time.Unix(1_700_000_000, 0)
	l := newLimiter(perMin)
	l.now = func() time.Time { return now }
	return l, &now
}

func TestLimiterBurstThenBlockThenRefill(t *testing.T) {
	l, now := clockedLimiter(6) // one token every 10s
	for i := 0; i < 6; i++ {
		if ok, _ := l.allow("a"); !ok {
			t.Fatalf("request %d of the burst was refused", i+1)
		}
	}
	ok, retry := l.allow("a")
	if ok {
		t.Fatal("request past the burst was allowed")
	}
	if retry < 9*time.Second || retry > 11*time.Second {
		t.Errorf("retry after = %v, want about 10s", retry)
	}
	*now = now.Add(10 * time.Second)
	if ok, _ := l.allow("a"); !ok {
		t.Error("a token should have refilled after 10s")
	}
	if ok, _ := l.allow("a"); ok {
		t.Error("only one token should have refilled")
	}
}

func TestLimiterKeepsCallersApart(t *testing.T) {
	l, _ := clockedLimiter(2)
	l.allow("a")
	l.allow("a")
	if ok, _ := l.allow("a"); ok {
		t.Fatal("a should be out of tokens")
	}
	if ok, _ := l.allow("b"); !ok {
		t.Fatal("b must not be affected by a")
	}
}

func TestLimiterForgetsIdleCallers(t *testing.T) {
	l, now := clockedLimiter(60)
	l.allow("gone")
	*now = now.Add(time.Hour)
	l.allow("someone-else")
	if _, tracked := l.buckets["gone"]; tracked {
		t.Error("an idle caller should have been swept")
	}
}

func TestZeroDisablesLimit(t *testing.T) {
	if newLimiter(0) != nil {
		t.Fatal("0 per minute should mean no limiter")
	}
	a := authAPI(t, nil)
	h := a.limited(nil, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for i := 0; i < 50; i++ {
		w := httptest.NewRecorder()
		h(w, req("203.0.113.9:1", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("unlimited handler answered %d", w.Code)
		}
	}
}

func TestLimitedAnswers429WithRetryAfter(t *testing.T) {
	a := authAPI(t, nil)
	l, _ := clockedLimiter(2)
	h := a.limited(l, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h(w, req("203.0.113.9:1", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("request %d answered %d", i+1, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h(w, req("203.0.113.9:2", nil)) // same IP, new source port
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("third request answered %d, want 429", w.Code)
	}
	if secs, err := strconv.Atoi(w.Header().Get("Retry-After")); err != nil || secs < 1 {
		t.Errorf("Retry-After = %q", w.Header().Get("Retry-After"))
	}
	if !strings.Contains(w.Body.String(), `"rate_limited"`) {
		t.Errorf("body = %s", w.Body.String())
	}
}

// Changing X-Forwarded-For or a device id must not buy a fresh allowance.
func TestLimitCannotBeEvadedByForgedHeaders(t *testing.T) {
	a := authAPI(t, func(c *config.Config) { c.TrustedProxies = []string{"10.0.0.0/8"} })
	l, _ := clockedLimiter(3)
	h := a.limited(l, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })

	blocked := 0
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		h(w, req("203.0.113.9:1", map[string]string{
			"X-Forwarded-For": "198.51.100." + strconv.Itoa(i+1),
			"X-Device-ID":     "device-" + strconv.Itoa(i),
		}))
		if w.Code == http.StatusTooManyRequests {
			blocked++
		}
	}
	if blocked != 7 {
		t.Errorf("blocked %d of 10, want 7 (burst of 3)", blocked)
	}
}
