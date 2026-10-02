package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ammyy9908/clipper/internal/resolver"
)

type cachedResolve struct {
	resp      resolveResp
	expiresAt time.Time
}

var (
	resolveCache    = sync.Map{}
	resolveCacheTTL = 10 * time.Minute
)

var lastSweep atomic.Int64

// sweepResolveCache drops expired entries, at most once a minute. Entries are
// otherwise only removed when the same URL is asked for again, so a caller
// working through unique URLs would grow the cache without bound.
func sweepResolveCache() {
	now := time.Now()
	last := lastSweep.Load()
	if now.UnixNano()-last < int64(time.Minute) || !lastSweep.CompareAndSwap(last, now.UnixNano()) {
		return
	}
	resolveCache.Range(func(k, v any) bool {
		if now.After(v.(cachedResolve).expiresAt) {
			resolveCache.Delete(k)
		}
		return true
	})
}

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
	Audio    bool   `json:"audio"`
}

type resolveResp struct {
	// Kind is "video" or "playlist". Clients that predate playlists ignore it.
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Uploader  string    `json:"uploader,omitempty"`
	Platform  string    `json:"platform"`
	Duration  float64   `json:"duration_seconds"`
	Thumbnail string    `json:"thumbnail,omitempty"`
	Variants  []variant `json:"variants"`

	// Playlist is set when Kind is "playlist".
	Playlist *playlistInfo `json:"playlist,omitempty"`
}

// resolve is synchronous: it's a metadata call, a couple of seconds, and the
// client can't choose a variant without it. The download itself is a job.
func (a *API) resolve(w http.ResponseWriter, r *http.Request) {
	var req resolveReq
	if err := decode(r, &req); err != nil {
		badRequest(w, "invalid JSON body")
		return
	}
	req.URL = normalizeURL(req.URL)
	if err := validateURL(req.URL); err != nil {
		badRequest(w, err.Error())
		return
	}

	resp, err := a.resolveCached(r.Context(), req.URL)
	if err != nil {
		a.resolverError(w, err, req.URL)
		return
	}
	if max := a.cfg.MaxDurationSeconds; max > 0 && resp.Kind == "video" && resp.Duration > float64(max) {
		a.resolverError(w, resolver.ErrTooLarge, req.URL)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// resolveCached looks a URL up, single video or playlist, through the multi-tier
// resolve cache (in-memory + Redis) and coalesces concurrent lookups with singleflight.
func (a *API) resolveCached(ctx context.Context, rawURL string) (resolveResp, error) {
	// 1. Fast path: in-memory cache
	if val, ok := resolveCache.Load(rawURL); ok {
		cached := val.(cachedResolve)
		if time.Now().Before(cached.expiresAt) {
			return cached.resp, nil
		}
		resolveCache.Delete(rawURL)
	}

	// 2. Distributed path: Redis cache (if configured)
	if a.rdb != nil {
		if val, err := a.rdb.Get(ctx, "resolve:"+rawURL).Result(); err == nil && val != "" {
			var resp resolveResp
			if err := json.Unmarshal([]byte(val), &resp); err == nil {
				resolveCache.Store(rawURL, cachedResolve{resp: resp, expiresAt: time.Now().Add(resolveCacheTTL)})
				return resp, nil
			}
		}
	}

	// 3. Coalesce concurrent requests for the same URL with singleflight
	val, err, _ := a.sf.Do(rawURL, func() (any, error) {
		resolveCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()

		info, err := a.res.Resolve(resolveCtx, rawURL)
		if err != nil {
			// oEmbed can describe a single YouTube video, but for other platforms or
			// playlists it is not a valid fallback.
			if !looksLikePlaylistURL(rawURL) && isYouTubeURL(rawURL) {
				a.log.Warn("yt-dlp resolve failed; falling back to oembed metadata", "url", rawURL, "err", err)
				oCtx, oCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer oCancel()
				oInfo, oErr := resolveOEmbed(oCtx, rawURL)
				if oErr == nil {
					info = oInfo
					err = nil
				}
			}
			if err != nil {
				return resolveResp{}, err
			}
		}

		var resp resolveResp
		if info.IsPlaylist() {
			resp = buildPlaylistResp(info)
			if resp.Playlist == nil || len(resp.Playlist.Entries) == 0 {
				return resolveResp{}, errEmptyPlaylist
			}
		} else {
			resp = resolveResp{
				Kind:      "video",
				Title:     info.Title,
				Uploader:  info.Uploader,
				Platform:  info.Extractor,
				Duration:  info.Duration,
				Thumbnail: info.Thumbnail,
				Variants:  buildVariants(info),
			}
		}

		// Save in local memory cache
		resolveCache.Store(rawURL, cachedResolve{resp: resp, expiresAt: time.Now().Add(resolveCacheTTL)})
		sweepResolveCache()

		// Save in Redis for 6 hours
		if a.rdb != nil {
			if data, err := json.Marshal(resp); err == nil {
				_ = a.rdb.Set(context.Background(), "resolve:"+rawURL, data, 6*time.Hour).Err()
			}
		}

		return resp, nil
	})

	if err != nil {
		return resolveResp{}, err
	}
	return val.(resolveResp), nil
}

type oEmbedData struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	ThumbnailURL string `json:"thumbnail_url"`
	ProviderName string `json:"provider_name"`
}

func resolveOEmbed(ctx context.Context, rawURL string) (resolver.Info, error) {
	apiURL := "https://www.youtube.com/oembed?url=" + url.QueryEscape(rawURL) + "&format=json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return resolver.Info{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return resolver.Info{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resolver.Info{}, errors.New("oembed returned non-200 status")
	}

	var data oEmbedData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return resolver.Info{}, err
	}

	if data.Title == "" {
		return resolver.Info{}, errors.New("empty oembed title")
	}

	return resolver.Info{
		Title:     data.Title,
		Uploader:  data.AuthorName,
		Extractor: "youtube",
		Thumbnail: data.ThumbnailURL,
		Formats: []resolver.Format{
			{Height: 2160, VCodec: "avc1", ACodec: "mp4a", TBR: 10000, Note: "4K (2160p)"},
			{Height: 1440, VCodec: "avc1", ACodec: "mp4a", TBR: 6000, Note: "2K (1440p)"},
			{Height: 1080, VCodec: "avc1", ACodec: "mp4a", TBR: 4000, Note: "1080p"},
			{Height: 720, VCodec: "avc1", ACodec: "mp4a", TBR: 2500, Note: "720p"},
			{Height: 480, VCodec: "avc1", ACodec: "mp4a", TBR: 1200, Note: "480p"},
			{Height: 360, VCodec: "avc1", ACodec: "mp4a", TBR: 600, Note: "360p"},
			{VCodec: "none", ACodec: "mp4a", Note: "audio only"},
		},
	}, nil
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
		sel := videoSelector(h)

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

	if hasAudio(info) {
		out = append(out, variant{
			Selector: "bestaudio/best",
			Label:    "MP3",
			Ext:      "mp3",
			Audio:    true,
		})
	}
	return out
}

// videoSelector is the yt-dlp format expression for "best video up to h pixels
// tall", preferring H.264 + AAC so the result plays everywhere.
func videoSelector(h int) string {
	return "bv*[height<=" + itoa(h) + "][vcodec^=avc1]+ba[ext=m4a]/" +
		"bv*[height<=" + itoa(h) + "][vcodec^=avc1]+ba/" +
		"b[height<=" + itoa(h) + "][vcodec^=avc1]/" +
		"bv*[height<=" + itoa(h) + "]+ba[ext=m4a]/" +
		"bv*[height<=" + itoa(h) + "]+ba/" +
		"b[height<=" + itoa(h) + "]/b"
}

func hasAudio(info resolver.Info) bool {
	for _, f := range info.Formats {
		if f.HasAudio() {
			return true
		}
	}
	return false
}

func isYouTubeURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	return strings.Contains(host, "youtube.com") || strings.EqualFold(host, "youtu.be")
}

func isYouTubeMusic(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "music.youtube.com") {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	return host == "music.youtube.com" || strings.HasSuffix(host, ".music.youtube.com")
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if strings.EqualFold(u.Host, "music.youtube.com") {
		u.Host = "www.youtube.com"
	} else if strings.EqualFold(u.Host, "youtu.be") {
		vid := strings.TrimPrefix(u.Path, "/")
		u.Host = "www.youtube.com"
		u.Path = "/watch"
		q := u.Query()
		q.Set("v", vid)
		u.RawQuery = q.Encode()
	}
	if strings.Contains(u.Host, "youtube.com") {
		q := u.Query()
		q.Del("si")
		q.Del("feature")
		q.Del("pp")
		q.Del("app")
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// headRe pulls the height out of a video selector.
var heightRe = regexp.MustCompile(`^bv\*\[height<=(\d{1,5})\]`)

// validSelector accepts only the format selectors /v1/resolve hands out (or
// none). The selector goes straight to yt-dlp's -f, and its expression language
// can ask for everything at once ("all"), so a caller must not be able to
// invent one. Keep this in step with videoSelector and the audio variant.
func validSelector(s string) bool {
	if s == "" || s == "bestaudio/best" {
		return true
	}
	m := heightRe.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	h, err := strconv.Atoi(m[1])
	return err == nil && h > 0 && s == videoSelector(h)
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
	if u.Hostname() == "" {
		return errors.New("url must include a host")
	}
	if !publicHost(u.Hostname()) {
		return errors.New("url must point to a public website")
	}
	return nil
}

// publicHost reports whether host could be a public website. yt-dlp fetches
// whatever it is given, so without this a caller could aim the server at its
// own loopback, the cloud metadata address or the private network. It checks
// names and IP literals only; a public name that resolves to a private address
// still needs egress rules on the network.
func publicHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" ||
		strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsMulticast())
	}
	// A real name has a dot and a non-numeric top level. Numeric or hex forms
	// like "127.1", "2130706433" or "0x7f.0.0.1" are IPs that net.ParseIP
	// doesn't recognise but the resolver library does.
	dot := strings.LastIndex(host, ".")
	if dot < 0 {
		return false
	}
	last := host[dot+1:]
	if last == "" || strings.Trim(last, "0123456789") == "" {
		return false
	}
	return true
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
	case errors.Is(err, resolver.ErrTooLarge):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": "this video is too long to save here",
			"code":  "too_large",
		})
	case errors.Is(err, resolver.ErrBusy):
		w.Header().Set("Retry-After", "5")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "the server is busy, try again in a moment",
			"code":  "busy",
		})
	case errors.Is(err, resolver.ErrUnsupported):
		badRequest(w, "this site isn't supported")
	case errors.Is(err, resolver.ErrLiveStream):
		badRequest(w, "live streams aren't supported")
	case errors.Is(err, errEmptyPlaylist):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": "this playlist has no videos that can be downloaded",
			"code":  "empty_playlist",
		})
	case errors.Is(err, resolver.ErrBlocked):
		a.log.Warn("egress blocked", "url", url, "err", err)
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
