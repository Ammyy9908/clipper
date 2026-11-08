package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ammyy9908/clipper/internal/config"
)

func corsHandler(t *testing.T, origins ...string) http.Handler {
	a := authAPI(t, func(c *config.Config) { c.CORSOrigins = origins })
	return a.cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
}

func TestCORSAllowsListedOriginOnly(t *testing.T) {
	h := corsHandler(t, "https://clipplay.video")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/resolve", nil)
	r.Header.Set("Origin", "https://clipplay.video")
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://clipplay.video" {
		t.Errorf("allowed origin got %q", got)
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Error("credentials must never be allowed cross-origin")
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/v1/resolve", nil)
	r.Header.Set("Origin", "https://www.clipplay.video")
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://www.clipplay.video" {
		t.Errorf("www subdomain origin got %q", got)
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/v1/resolve", nil)
	r.Header.Set("Origin", "https://evil.example")
	h.ServeHTTP(w, r)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unlisted origin was allowed: %q", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	h := corsHandler(t, "https://clipplay.video")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/v1/jobs", nil)
	r.Header.Set("Origin", "https://clipplay.video")
	r.Header.Set("Access-Control-Request-Method", "POST")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight answered %d", w.Code)
	}
	for _, want := range []string{"Authorization", "X-Device-ID", "Content-Type"} {
		if !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), want) {
			t.Errorf("preflight does not allow header %s", want)
		}
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodOptions, "/v1/jobs", nil)
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("Access-Control-Request-Method", "POST")
	h.ServeHTTP(w, r)
	if w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Access-Control-Allow-Methods") != "" {
		t.Error("preflight from an unlisted origin must get no CORS headers")
	}
}

func TestCORSOffWhenNotConfigured(t *testing.T) {
	h := corsHandler(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/config", nil)
	r.Header.Set("Origin", "https://clipplay.video")
	h.ServeHTTP(w, r)
	if w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Vary") != "" {
		t.Error("with no origins configured the API should behave as before")
	}
}
