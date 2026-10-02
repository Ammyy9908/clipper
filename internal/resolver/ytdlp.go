package resolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Resolver shells out to yt-dlp. Deliberately not a hand-written extractor:
// per-platform parsing logic rots on the platforms' schedule, and yt-dlp is
// the only thing tracking it fast enough to matter.
type Resolver struct {
	Bin     string   // path to yt-dlp
	Proxies []string // optional egress pool; datacenter IPs get blocked fast
	Cookies string   // optional cookies.txt for logged-in-only content

	// PlaylistMax caps how many entries a playlist lookup returns. Zero means
	// DefaultPlaylistMax. It bounds both the yt-dlp call and the egress a single
	// request can fan out into.
	PlaylistMax int

	// MaxFilesize, in bytes, is passed to yt-dlp so it refuses a download it
	// knows is bigger. Zero means no limit.
	MaxFilesize int64

	// MaxConcurrent bounds simultaneous Resolve calls. Each one starts a
	// yt-dlp process, so unbounded callers could exhaust the host. Zero means 8.
	MaxConcurrent int

	semOnce sync.Once
	sem     chan struct{}
}

// acquire waits for a Resolve slot, or gives up when ctx ends.
func (r *Resolver) acquire(ctx context.Context) (release func(), err error) {
	r.semOnce.Do(func() {
		n := r.MaxConcurrent
		if n <= 0 {
			n = 8
		}
		r.sem = make(chan struct{}, n)
	})
	select {
	case r.sem <- struct{}{}:
		return func() { <-r.sem }, nil
	case <-ctx.Done():
		return nil, ErrBusy
	}
}

func New(bin string, proxies []string, cookies string) *Resolver {
	if bin == "" {
		bin = "yt-dlp"
	}
	return &Resolver{Bin: bin, Proxies: proxies, Cookies: cookies}
}

type Format struct {
	ID       string  `json:"format_id"`
	Ext      string  `json:"ext"`
	Height   int     `json:"height"`
	FPS      float64 `json:"fps"`
	VCodec   string  `json:"vcodec"`
	ACodec   string  `json:"acodec"`
	Filesize int64   `json:"filesize"`
	Approx   int64   `json:"filesize_approx"`
	TBR      float64 `json:"tbr"`
	Note     string  `json:"format_note"`
	Width    int     `json:"width"`
}

func (f Format) HasVideo() bool { return f.VCodec != "" && f.VCodec != "none" }
func (f Format) HasAudio() bool { return f.ACodec != "" && f.ACodec != "none" }

func (f Format) Size() int64 {
	if f.Filesize > 0 {
		return f.Filesize
	}
	return f.Approx
}

type Info struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Extractor   string   `json:"extractor_key"`
	Duration    float64  `json:"duration"`
	Thumbnail   string   `json:"thumbnail"`
	Uploader    string   `json:"uploader"`
	Channel     string   `json:"channel"`
	Description string   `json:"description"`
	Track       string   `json:"track"`
	Artist      string   `json:"artist"`
	Album       string   `json:"album"`
	IsLive      bool     `json:"is_live"`
	Formats     []Format `json:"formats"`

	// Populated when the URL is a playlist. yt-dlp is asked for a flat listing,
	// so entries carry titles and URLs but no formats.
	Type          string      `json:"_type"`
	Entries       []Entry     `json:"entries"`
	PlaylistCount int         `json:"playlist_count"`
	Thumbnails    []Thumbnail `json:"thumbnails"`
}

type attempt struct {
	proxy         string
	cookies       string
	playerClients string
}

func (r *Resolver) candidateAttempts() []attempt {
	var attempts []attempt

	// 1. Direct attempt with cookies (tv_embedded,web_embedded,mweb)
	if r.Cookies != "" {
		attempts = append(attempts, attempt{
			proxy:         "",
			cookies:       r.Cookies,
			playerClients: "tv_embedded,web_embedded,mweb",
		})
	}

	// 2. Direct clean attempt without cookies (android,ios,web_embedded - bypasses PO token requirements)
	attempts = append(attempts, attempt{
		proxy:         "",
		cookies:       "",
		playerClients: "android,ios,web_embedded,mweb",
	})

	// 3. Proxy fallback
	if n := len(r.Proxies); n > 0 {
		perm := rand.Perm(n)
		maxFallbacks := 2
		if n < maxFallbacks {
			maxFallbacks = n
		}
		for i := 0; i < maxFallbacks; i++ {
			attempts = append(attempts, attempt{
				proxy:         r.Proxies[perm[i]],
				cookies:       r.Cookies,
				playerClients: "tv_embedded,web_embedded,mweb",
			})
		}
	}

	return attempts
}

func hasOAuth2Token() bool {
	paths := []string{
		"/etc/yt-dlp/tokens.json",
		"/etc/yt-dlp/youtube-oauth2/tokens.json",
		"/root/.cache/yt-dlp/youtube-oauth2/tokens.json",
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".cache", "yt-dlp", "youtube-oauth2", "tokens.json"))
	}
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && st.Size() > 10 {
			return true
		}
	}
	return os.Getenv("YTDLP_USE_OAUTH2") == "true"
}

// Resolve returns metadata and available formats. This is the step that breaks:
// when a platform ships new player logic it fails here, before any bytes move.
func (r *Resolver) Resolve(ctx context.Context, url string) (Info, error) {
	release, err := r.acquire(ctx)
	if err != nil {
		return Info{}, err
	}
	defer release()

	attempts := r.candidateAttempts()
	var lastErr error
	useOAuth2 := hasOAuth2Token()
	isPlaylist := LooksLikePlaylistURL(url)

	for _, att := range attempts {
		if ctx.Err() != nil {
			break
		}

		playerClients := att.playerClients
		if playerClients == "" {
			playerClients = "tv_embedded,web_embedded,mweb"
		}
		if useOAuth2 {
			playerClients = "tv_embedded,tv,mweb,web"
		}

		args := []string{
			"-J",
			"--no-warnings",
			"--no-progress",
			"--no-check-formats",
			"--remote-components", "ejs:github",
			"--js-runtimes", "node",
			"--cache-dir", "/tmp/ytdlp-cache",
			"--socket-timeout", "6",
			"--impersonate", "chrome",
			"--extractor-args", fmt.Sprintf("youtube:player_client=%s;skip=translated_subs,storyboards,comments,subtitles", playerClients),
		}
		if isPlaylist {
			args = append(args, "--flat-playlist", "--playlist-end", strconv.Itoa(r.playlistMax()))
		} else {
			args = append(args, "--no-playlist")
		}

		if useOAuth2 {
			args = append(args, "--username", "oauth2", "--password", "")
		}
		if att.proxy != "" {
			args = append(args, "--proxy", att.proxy)
		}
		if att.cookies != "" {
			args = append(args, "--cookies", att.cookies)
		} else {
			args = append(args, "--no-cookies")
		}
		args = append(args, "--", url)

		attemptCtx, attemptCancel := context.WithTimeout(ctx, 15*time.Second)
		var stderr strings.Builder
		cmd := exec.CommandContext(attemptCtx, r.Bin, args...)
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		attemptCancel()
		if err != nil {
			lastErr = classify(stderr.String(), err)
			if errors.Is(lastErr, ErrUnavailable) || errors.Is(lastErr, ErrUnsupported) || errors.Is(lastErr, ErrLiveStream) {
				return Info{}, lastErr
			}
			continue
		}

		var info Info
		if err := json.Unmarshal(out, &info); err != nil {
			return Info{}, fmt.Errorf("decode yt-dlp output: %w", err)
		}
		if info.IsLive {
			return Info{}, ErrLiveStream
		}
		return info, nil
	}
	return Info{}, lastErr
}

// Fetch downloads the chosen format to dest. selector is a yt-dlp format
// expression, not a raw format id, so audio/video splits get merged for free.
// NOTE: Fetch deliberately does NOT route through proxy pools — video stream
// bytes come directly from Google's high-throughput CDN (googlevideo.com)
// to avoid exhausting free proxy bandwidth quotas (e.g. Webshare 1GB limit).
func (r *Resolver) Fetch(ctx context.Context, url, selector string, audioOnly bool, dest string) error {
	if selector == "" {
		if audioOnly {
			selector = "bestaudio/best"
		} else {
			selector = "bv*[height<=1080][vcodec^=avc1]+ba[ext=m4a]/bv*[height<=1080][vcodec^=avc1]+ba/b[height<=1080]/b"
		}
	}

	var attempts []attempt
	// 1. Direct with cookies fallback
	if r.Cookies != "" {
		attempts = append(attempts, attempt{proxy: "", cookies: r.Cookies})
	}
	// 2. Direct clean attempt
	attempts = append(attempts, attempt{proxy: "", cookies: ""})

	var lastErr error
	useOAuth2 := hasOAuth2Token()

	for _, att := range attempts {
		playerClients := "web_embedded,tv,mweb,web"
		if useOAuth2 {
			playerClients = "tv_embedded,tv,mweb,web"
		}
		args := []string{
			"-f", selector,
			"--no-playlist",
			"--no-warnings",
			"--newline",
			"--socket-timeout", "15",
			"--impersonate", "chrome",
			"--retries", "10",
			"--fragment-retries", "10",
			"--file-access-retries", "5",
			"--retry-sleep", "1",
			"--concurrent-fragments", "4",
			"--remote-components", "ejs:github",
			"--extractor-args", fmt.Sprintf("youtube:player_client=%s;skip=translated_subs,storyboards,comments", playerClients),
			"--js-runtimes", "node",
			"-o", dest,
		}
		if r.MaxFilesize > 0 {
			args = append(args, "--max-filesize", strconv.FormatInt(r.MaxFilesize, 10))
		}
		if useOAuth2 {
			args = append(args, "--username", "oauth2", "--password", "")
		}
		if !audioOnly {
			args = append(args, "--merge-output-format", "mp4")
		}
		if att.cookies != "" {
			args = append(args, "--cookies", att.cookies)
		}
		args = append(args, "--", url)

		var stderr strings.Builder
		cmd := exec.CommandContext(ctx, r.Bin, args...)
		cmd.Stderr = &stderr
		cmd.Stdout = os.Stdout

		// Monitor merge progress if merging video + audio
		var (
			monitorCtx, cancelMonitor = context.WithCancel(ctx)
			wg                        sync.WaitGroup
		)
		if !audioOnly {
			wg.Add(1)
			go func() {
				defer wg.Done()
				dir := filepath.Dir(dest)
				ticker := time.NewTicker(1500 * time.Millisecond)
				defer ticker.Stop()

				var lastLoggedPct int = -1
				for {
					select {
					case <-monitorCtx.Done():
						return
					case <-ticker.C:
						// Look for source.temp.mp4 or *.temp.*
						tempFiles, _ := filepath.Glob(filepath.Join(dir, "*.temp*"))
						if len(tempFiles) > 0 {
							tempFile := tempFiles[0]
							tempStat, err := os.Stat(tempFile)
							if err != nil || tempStat.Size() == 0 {
								continue
							}

							// Find source.f* raw stream files to calculate total target size
							rawFiles, _ := filepath.Glob(filepath.Join(dir, "source.f*"))
							var totalRaw int64
							for _, rf := range rawFiles {
								if st, err := os.Stat(rf); err == nil {
									totalRaw += st.Size()
								}
							}

							if totalRaw > 0 {
								pct := int((float64(tempStat.Size()) / float64(totalRaw)) * 100)
								if pct > 100 {
									pct = 100
								}
								if pct != lastLoggedPct && (pct%5 == 0 || pct >= 99) {
									lastLoggedPct = pct
									tag := strings.TrimPrefix(filepath.Base(dir), "download_")
									if len(tag) > 8 {
										tag = tag[:8]
									}
									fmt.Printf("[Merger: %s] Merging progress: %d%% (%.2f / %.2f GB)\n",
										tag,
										pct,
										float64(tempStat.Size())/(1024*1024*1024),
										float64(totalRaw)/(1024*1024*1024))
								}
							}
						}
					}
				}
			}()
		}

		err := cmd.Run()
		cancelMonitor()
		wg.Wait()

		if err != nil {
			lastErr = classify(stderr.String(), err)
			if errors.Is(lastErr, ErrUnavailable) || errors.Is(lastErr, ErrUnsupported) || errors.Is(lastErr, ErrLiveStream) {
				return lastErr
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (r *Resolver) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, r.Bin, "--version").Output()
	return strings.TrimSpace(string(out)), err
}
