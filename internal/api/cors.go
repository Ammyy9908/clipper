package api

import (
	"net/http"
	"strings"
)

// corsAllowed reports whether a browser page on origin may call this API.
// Origins are matched exactly (scheme, host and port), as sent in the Origin header,
// with automatic handling of apex/www aliases and wildcard subdomains.
func (a *API) corsAllowed(origin string) bool {
	cleanOrigin := strings.ToLower(strings.TrimSpace(origin))
	if cleanOrigin == "" {
		return false
	}
	for _, o := range a.cfg.CORSOrigins {
		cleanO := strings.ToLower(strings.TrimSpace(o))
		if cleanO == "*" || cleanO == cleanOrigin {
			return true
		}
		// Handle wildcard subdomain e.g. "https://*.clipplay.video"
		if strings.HasPrefix(cleanO, "https://*.") {
			domain := strings.TrimPrefix(cleanO, "https://*.")
			if strings.HasPrefix(cleanOrigin, "https://") && (strings.HasSuffix(cleanOrigin, "."+domain) || cleanOrigin == "https://"+domain) {
				return true
			}
		}
		// Automatically allow www or apex if one is configured
		if cleanO == "https://clipplay.video" && cleanOrigin == "https://www.clipplay.video" {
			return true
		}
		if cleanO == "https://www.clipplay.video" && cleanOrigin == "https://clipplay.video" {
			return true
		}
	}
	return false
}

// cors lets the website, served from another origin, call the API from the
// browser. Calling the API directly (instead of through the site's own server)
// is what lets the API see each visitor's real IP, and lets job status use a
// WebSocket. No cookies are involved: sign-in is a Bearer token, so credentials
// are never allowed cross-origin.
//
// With no CORS_ALLOWED_ORIGINS configured nothing is added and behaviour is as before.
func (a *API) cors(next http.Handler) http.Handler {
	if len(a.cfg.CORSOrigins) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		allowed := origin != "" && a.corsAllowed(origin)
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Expose-Headers", "Retry-After")
		}
		// A preflight is answered here; the routes themselves only know real methods.
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			if allowed {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Device-ID, Accept, Origin")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
