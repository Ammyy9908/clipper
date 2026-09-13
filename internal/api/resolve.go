package api

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/ammyy9908/clipper/internal/resolver"
)

type resolveReq struct {
	URL string `json:"url"`
}

type variant struct {
	Selector string `json:"selector"`
	Label    string `json:"label"`
	Height   int    `json:"height"`
	Ext      string `json:"ext"`
	Bytes    int64  `json:"approx_bytes,omitempty"`
	Muxed    bool   `json:"muxed"`
}

type resolveResp struct {
	Title     string    `json:"title"`
	Uploader  string    `json:"uploader,omitempty"`
	Platform  string    `json:"platform"`
	Duration  float64   `json:"duration_seconds"`
	Thumbnail string    `json:"thumbnail,omitempty"`
	Variants  []variant `json:"variants"`
}

// resolve is synchronous: it's a metadata call, a couple of seconds, and the
// client can't choose a variant without it. The download itself is a job.
func (a *API) resolve(w http.ResponseWriter, r *http.Request) {
	var req resolveReq
	if err := decode(r, &req); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	if err := validateURL(req.URL); err != nil {
		badRequest(w, err.Error())
		return
	}

	info, err := a.res.Resolve(r.Context(), req.URL)
	if err != nil {
		a.resolverError(w, err, req.URL)
		return
	}

	writeJSON(w, http.StatusOK, resolveResp{
		Title:     info.Title,
		Uploader:  info.Uploader,
		Platform:  info.Extractor,
		Duration:  info.Duration,
		Thumbnail: info.Thumbnail,
		Variants:  buildVariants(info),
	})
}

// buildVariants collapses yt-dlp's format list (often 40+ entries) into the
// handful a user would actually pick between, one per resolution rung.
func buildVariants(info resolver.Info) []variant {
	best := map[int]resolver.Format{}
	for _, f := range info.Formats {
		if !f.HasVideo() || f.Height == 0 {
			continue
		}
		cur, ok := best[f.Height]
		if !ok || f.TBR > cur.TBR {
			best[f.Height] = f
		}
	}

	out := make([]variant, 0, len(best))
	for h, f := range best {
		sel := "bv*[height<=" + itoa(h) + "]+ba/b[height<=" + itoa(h) + "]/b"

		// Label by the short side. For a portrait reel (720x1280) the rung
		// users recognise is 720p, not 1280p — but the selector stays
		// height-based because that's what yt-dlp filters on.
		short := h
		if f.Width > 0 && f.Width < short {
			short = f.Width
		}

		out = append(out, variant{
			Selector: sel,
			Label:    itoa(short) + "p",
			Height:   h,
			Ext:      "mp4",
			Bytes:    f.Size(),
			Muxed:    f.HasAudio(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Height > out[j].Height })
	return out
}

func validateURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("malformed url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url must be http or https")
	}
	if u.Host == "" {
		return errors.New("url must include a host")
	}
	return nil
}

// resolverError maps extraction failures onto status codes a client can act on.
// A stale extractor is a 503 (our problem, try later); a private video is a 422
// (their problem, retrying won't help).
func (a *API) resolverError(w http.ResponseWriter, err error, url string) {
	switch {
	case errors.Is(err, resolver.ErrUnavailable):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": "this video is private, removed, or not available in this region",
			"code":  "unavailable",
		})
	case errors.Is(err, resolver.ErrUnsupported):
		badRequest(w, "this site isn't supported")
	case errors.Is(err, resolver.ErrLiveStream):
		badRequest(w, "live streams aren't supported")
	case errors.Is(err, resolver.ErrBlocked):
		a.log.Warn("egress blocked", "url", url)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "temporarily unable to fetch this video, try again shortly",
			"code":  "blocked",
		})
	case errors.Is(err, resolver.ErrExtractorStale):
		// Loud on purpose: this is the alert that means ship a yt-dlp bump.
		a.log.Error("extractor stale", "url", url, "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "this platform isn't working right now",
			"code":  "extractor_stale",
		})
	default:
		a.log.Error("resolve failed", "url", url, "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "could not read this link",
			"code":  "resolve_failed",
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
