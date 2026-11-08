package media

import (
	"context"
	"fmt"
	"path/filepath"
)

// Aspect targets. Reframing is a re-encode, not a remux: every frame is
// decoded and re-drawn. Budget minutes of CPU per job, not seconds.
const (
	AspectOriginal   = ""     // no reframe
	AspectVertical   = "9:16" // Reels, Shorts, status
	AspectPortrait   = "4:5"  // Instagram feed portrait
	AspectSquare     = "1:1"  // feed posts
	AspectHorizontal = "16:9" // YouTube, landscape
)

var aspectDims = map[string][2]int{
	AspectVertical:   {1080, 1920},
	AspectPortrait:   {1080, 1350},
	AspectSquare:     {1080, 1080},
	AspectHorizontal: {1920, 1080},
}

func ValidAspect(a string) bool {
	if a == AspectOriginal {
		return true
	}
	_, ok := aspectDims[a]
	return ok
}

// padFilter builds an FFmpeg video filter that scales the video to fit within
// the target canvas (w x h) while preserving the full original frame, and centers
// it on a solid black background (letterbox/pillarbox).
func padFilter(w, h int) string {
	return fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1",
		w, h, w, h,
	)
}

// Reframe renders src into the target aspect and returns the new path. Called
// before Split so the segment pass stays a pure copy — reframing once and
// cutting N times is far cheaper than reframing each of N clips.
//
// segmentSeconds is how long the clips will be, or 0 for no splitting. When set,
// a keyframe is forced on every boundary. Without that, the encoder places
// keyframes on its own schedule (every ~8s), and a copy-mode split can only cut
// on a keyframe, so clips would come out at uneven lengths.
func Reframe(ctx context.Context, src, outDir, aspect string, segmentSeconds int) (string, error) {
	dims, ok := aspectDims[aspect]
	if !ok {
		return "", fmt.Errorf("unsupported aspect %q", aspect)
	}
	w, h := dims[0], dims[1]

	dest := filepath.Join(outDir, "reframed.mp4")
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", src,
		"-vf", padFilter(w, h),
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "23",
		"-pix_fmt", "yuv420p",
		// Audio is untouched; re-encoding it here would be pure waste.
		"-c:a", "copy",
		"-movflags", "+faststart",
	}
	if segmentSeconds > 0 {
		args = append(args, "-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", segmentSeconds))
	}
	args = append(args, dest)
	if err := run(ctx, "ffmpeg", args...); err != nil {
		return "", fmt.Errorf("reframe: %w", err)
	}
	return dest, nil
}
