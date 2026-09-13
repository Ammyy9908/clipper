package media

import (
	"context"
	"fmt"
	"path/filepath"
)

// Aspect targets. Reframing is a re-encode, not a remux: every frame is
// decoded and re-drawn. Budget minutes of CPU per job, not seconds.
const (
	AspectOriginal = ""     // no reframe
	AspectVertical = "9:16" // Reels, Shorts, status
	AspectSquare   = "1:1"  // feed posts
)

var aspectDims = map[string][2]int{
	AspectVertical: {1080, 1920},
	AspectSquare:   {1080, 1080},
}

func ValidAspect(a string) bool {
	if a == AspectOriginal {
		return true
	}
	_, ok := aspectDims[a]
	return ok
}

// blurredPillarbox builds a filtergraph that fits the whole source frame inside
// the target canvas and fills the empty bands with a blurred, zoomed copy of
// the same frame. Nothing is cropped, so it is never wrong about what matters
// in the shot — the tradeoff a center crop can't make.
//
// The graph:
//
//	[bg] scale to COVER the canvas, crop to exact size, blur it
//	[fg] scale to FIT inside the canvas, preserving aspect
//	overlay fg centred on bg
//
// force_original_aspect_ratio=increase gives cover, =decrease gives fit.
// setsar=1 at the end because overlay can otherwise leave a non-square sample
// aspect ratio that players honour and stretch.
func blurredPillarbox(w, h int) string {
	return fmt.Sprintf(
		"split[bg][fg];"+
			"[bg]scale=%d:%d:force_original_aspect_ratio=increase,"+
			"crop=%d:%d,boxblur=luma_radius=40:luma_power=2[bgout];"+
			"[fg]scale=%d:%d:force_original_aspect_ratio=decrease[fgout];"+
			"[bgout][fgout]overlay=(W-w)/2:(H-h)/2,setsar=1",
		w, h, w, h, w, h,
	)
}

// Reframe renders src into the target aspect and returns the new path. Called
// before Split so the segment pass stays a pure copy — reframing once and
// cutting N times is far cheaper than reframing each of N clips.
func Reframe(ctx context.Context, src, outDir, aspect string) (string, error) {
	dims, ok := aspectDims[aspect]
	if !ok {
		return "", fmt.Errorf("unsupported aspect %q", aspect)
	}
	w, h := dims[0], dims[1]

	dest := filepath.Join(outDir, "reframed.mp4")
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", src,
		"-filter_complex", blurredPillarbox(w, h),
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
		"-pix_fmt", "yuv420p",
		// Audio is untouched; re-encoding it here would be pure waste.
		"-c:a", "copy",
		"-movflags", "+faststart",
		dest,
	}
	if err := run(ctx, "ffmpeg", args...); err != nil {
		return "", fmt.Errorf("reframe: %w", err)
	}
	return dest, nil
}
