package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
)

// Resolver shells out to yt-dlp. Deliberately not a hand-written extractor:
// per-platform parsing logic rots on the platforms' schedule, and yt-dlp is
// the only thing tracking it fast enough to matter.
type Resolver struct {
	Bin     string   // path to yt-dlp
	Proxies []string // optional egress pool; datacenter IPs get blocked fast
	Cookies string   // optional cookies.txt for logged-in-only content
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
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Extractor string   `json:"extractor_key"`
	Duration  float64  `json:"duration"`
	Thumbnail string   `json:"thumbnail"`
	Uploader  string   `json:"uploader"`
	IsLive    bool     `json:"is_live"`
	Formats   []Format `json:"formats"`
}

// Resolve returns metadata and available formats. This is the step that breaks:
// when a platform ships new player logic it fails here, before any bytes move.
func (r *Resolver) Resolve(ctx context.Context, url string) (Info, error) {
	args := []string{
		"-J",
		"--no-playlist",
		"--no-warnings",
		"--no-progress",
		"--socket-timeout", "20",
	}
	args = append(args, r.netArgs()...)
	args = append(args, "--", url)

	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, r.Bin, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return Info{}, classify(stderr.String(), err)
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

// Fetch downloads the chosen format to dest. selector is a yt-dlp format
// expression, not a raw format id, so audio/video splits get merged for free.
func (r *Resolver) Fetch(ctx context.Context, url, selector, dest string) error {
	if selector == "" {
		selector = "bv*[height<=1080][vcodec^=avc1]+ba/b[height<=1080]/b"
	}
	args := []string{
		"-f", selector,
		"--no-playlist",
		"--no-warnings",
		"--no-progress",
		"--merge-output-format", "mp4",
		"--socket-timeout", "20",
		"--retries", "3",
		"--concurrent-fragments", "4",
		"-o", dest,
	}
	args = append(args, r.netArgs()...)
	args = append(args, "--", url)

	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, r.Bin, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return classify(stderr.String(), err)
	}
	return nil
}

func (r *Resolver) netArgs() []string {
	var args []string
	if len(r.Proxies) > 0 {
		args = append(args, "--proxy", r.Proxies[rand.Intn(len(r.Proxies))])
	}
	if r.Cookies != "" {
		args = append(args, "--cookies", r.Cookies)
	}
	return args
}

func (r *Resolver) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, r.Bin, "--version").Output()
	return strings.TrimSpace(string(out)), err
}
