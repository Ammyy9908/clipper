package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	ModeCopy    = "copy"    // keyframe-aligned, instant, cuts land within a GOP
	ModePrecise = "precise" // re-encode with forced keyframes, exact boundaries, ~50x CPU
)

type Segment struct {
	Index      int
	Path       string
	DurationMS int64
}

type probeOutput struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
	} `json:"streams"`
}

// Probe returns the container duration in milliseconds and verifies a video
// stream exists, so we reject audio files and junk uploads before burning a slot.
func Probe(ctx context.Context, path string) (int64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-protocol_whitelist", "file,pipe",
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}

	var p probeOutput
	if err := json.Unmarshal(out, &p); err != nil {
		return 0, fmt.Errorf("ffprobe decode: %w", err)
	}

	hasVideo := false
	for _, s := range p.Streams {
		if s.CodecType == "video" {
			hasVideo = true
			break
		}
	}
	if !hasVideo {
		return 0, fmt.Errorf("no video stream in input")
	}

	secs, err := strconv.ParseFloat(p.Format.Duration, 64)
	if err != nil || secs <= 0 {
		return 0, fmt.Errorf("unreadable duration")
	}
	return int64(secs * 1000), nil
}

// ProbeDuration returns a container's duration in milliseconds without
// requiring a video stream, unlike Probe — audio-only files are the point.
func ProbeDuration(ctx context.Context, path string) (int64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-protocol_whitelist", "file,pipe",
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}

	var p probeOutput
	if err := json.Unmarshal(out, &p); err != nil {
		return 0, fmt.Errorf("ffprobe decode: %w", err)
	}

	secs, err := strconv.ParseFloat(p.Format.Duration, 64)
	if err != nil || secs <= 0 {
		return 0, fmt.Errorf("unreadable duration")
	}
	return int64(secs * 1000), nil
}

// ExtractAudio pulls the audio track out of src and encodes it to mp3,
// regardless of whether src arrived as a bare audio stream or a full video —
// letting ffmpeg own the final format keeps this independent of whatever
// container yt-dlp or an upload happened to hand back.
func ExtractAudio(ctx context.Context, src, outDir string) (string, int64, error) {
	dest := filepath.Join(outDir, "audio.mp3")
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", src,
		"-vn",
		"-c:a", "libmp3lame", "-q:a", "2",
		dest,
	}
	if err := run(ctx, "ffmpeg", args...); err != nil {
		return "", 0, fmt.Errorf("extract audio: %w", err)
	}
	durationMS, err := ProbeDuration(ctx, dest)
	if err != nil {
		return "", 0, err
	}
	return dest, durationMS, nil
}

// Split cuts src into segments of segmentSeconds each, written into outDir.
func Split(ctx context.Context, src, outDir string, segmentSeconds int, mode string) ([]Segment, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	input := src
	if mode == ModePrecise {
		// Re-encode first, forcing a keyframe exactly on every boundary. The
		// segment pass below can then stay a copy and still land on the mark.
		reenc := filepath.Join(outDir, "normalized.mp4")
		args := []string{
			"-hide_banner", "-loglevel", "error", "-y",
			"-i", src,
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
			"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", segmentSeconds),
			"-c:a", "aac", "-b:a", "128k",
			"-movflags", "+faststart",
			reenc,
		}
		if err := run(ctx, "ffmpeg", args...); err != nil {
			return nil, fmt.Errorf("normalize: %w", err)
		}
		input = reenc
	}

	pattern := filepath.Join(outDir, "clip_%04d.mp4")
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", input,
		"-c", "copy",
		"-map", "0",
		"-f", "segment",
		"-segment_time", strconv.Itoa(segmentSeconds),
		"-reset_timestamps", "1",
		"-segment_format_options", "movflags=+faststart",
		pattern,
	}
	if err := run(ctx, "ffmpeg", args...); err != nil {
		return nil, fmt.Errorf("segment: %w", err)
	}

	matches, err := filepath.Glob(filepath.Join(outDir, "clip_*.mp4"))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("segmenting produced no clips")
	}
	sort.Strings(matches)

	segs := make([]Segment, 0, len(matches))
	for i, m := range matches {
		d, err := Probe(ctx, m)
		if err != nil {
			return nil, fmt.Errorf("probe clip %d: %w", i, err)
		}
		segs = append(segs, Segment{Index: i, Path: m, DurationMS: d})
	}
	return segs, nil
}

// inputProtocols is all ffmpeg may open while reading a file. Uploads and
// downloads are untrusted, and a crafted playlist or concat file could otherwise
// make ffmpeg read local files or fetch internal URLs. Everything here is
// already on local disk.
var inputProtocols = []string{"-protocol_whitelist", "file,pipe"}

func run(ctx context.Context, bin string, args ...string) error {
	if bin == "ffmpeg" {
		args = append(append([]string{}, inputProtocols...), args...)
	}
	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, "nice", append([]string{"-n", "15", bin}, args...)...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 500 {
			msg = msg[:500]
		}
		if msg != "" {
			return fmt.Errorf("%s: %v: %s", bin, err, msg)
		}
		return fmt.Errorf("%s: %w", bin, err)
	}
	return nil
}

// VideoCodec returns the codec of the first video stream, e.g. "h264", "vp9".
func VideoCodec(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-protocol_whitelist", "file,pipe",
		"-v", "error",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "v:0",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ffprobe codec: %w", err)
	}
	var p probeOutput
	if err := json.Unmarshal(out, &p); err != nil {
		return "", err
	}
	if len(p.Streams) == 0 {
		return "", fmt.Errorf("no video stream")
	}
	return p.Streams[0].CodecName, nil
}

// ToH264 re-encodes into H.264/AAC. Necessary because a format selector can
// only express a preference: when a platform serves VP9 or AV1 only, the file
// is valid but iOS Photos rejects it outright and older Android decodes it in
// software. Costs a full encode, so only call it when the probe says so.
func ToH264(ctx context.Context, src, outDir string) (string, error) {
	dest := filepath.Join(outDir, "h264.mp4")
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", src,
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "128k",
		"-movflags", "+faststart",
		dest,
	}
	if err := run(ctx, "ffmpeg", args...); err != nil {
		return "", fmt.Errorf("transcode: %w", err)
	}
	return dest, nil
}
