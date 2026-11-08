package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The protocol whitelist must not get in the way of ordinary local files.
func TestProbeSplitAndAudioStillWork(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "in.mp4")
	gen := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=160x120:rate=10:duration=12",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=12",
		"-c:v", "libx264", "-g", "10", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("make test video: %v: %s", err, out)
	}
	ctx := context.Background()

	ms, err := Probe(ctx, src)
	if err != nil || ms < 11000 {
		t.Fatalf("Probe = %d, %v", ms, err)
	}
	if codec, err := VideoCodec(ctx, src); err != nil || codec != "h264" {
		t.Fatalf("VideoCodec = %q, %v", codec, err)
	}
	segs, err := Split(ctx, src, filepath.Join(dir, "out"), 5, ModeCopy)
	if err != nil || len(segs) < 2 {
		t.Fatalf("Split = %d segments, %v", len(segs), err)
	}
	mp3, _, err := ExtractAudio(ctx, src, dir)
	if err != nil {
		t.Fatalf("ExtractAudio: %v", err)
	}
	if st, err := os.Stat(mp3); err != nil || st.Size() == 0 {
		t.Fatalf("mp3 missing or empty: %v", err)
	}
}

// A file that only pretends to be a video must be rejected, not followed.
func TestProbeRejectsPlaylistDisguisedAsVideo(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret.txt")
	_ = os.WriteFile(secret, []byte("do not read"), 0o600)
	evil := filepath.Join(dir, "evil.mp4")
	_ = os.WriteFile(evil, []byte("#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXTINF:10,\nfile://"+secret+"\n#EXT-X-ENDLIST\n"), 0o600)
	if _, err := Probe(context.Background(), evil); err == nil {
		t.Fatal("Probe accepted a playlist disguised as a video")
	}
}

func dims(t *testing.T, path string) (int, int) {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", path, err)
	}
	var w, h int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d,%d", &w, &h); err != nil {
		t.Fatalf("parse %q: %v", out, err)
	}
	return w, h
}

// Splitting into a chosen frame shape gives clips of that shape and of even length.
func TestReframeThenSplitGivesEvenClipsInTheChosenShape(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "wide.mp4")
	gen := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=640x360:rate=25:duration=24",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=24",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("make source: %v: %s", err, out)
	}
	ctx := context.Background()

	for aspect, want := range map[string][2]int{
		AspectVertical: {1080, 1920}, AspectPortrait: {1080, 1350},
		AspectSquare: {1080, 1080}, AspectHorizontal: {1920, 1080},
	} {
		out := filepath.Join(dir, "r-"+strings.ReplaceAll(aspect, ":", "x"))
		_ = os.MkdirAll(out, 0o755)
		re, err := Reframe(ctx, src, out, aspect, 8)
		if err != nil {
			t.Fatalf("Reframe %s: %v", aspect, err)
		}
		segs, err := Split(ctx, re, filepath.Join(out, "clips"), 8, ModeCopy)
		if err != nil || len(segs) != 3 {
			t.Fatalf("%s: Split = %d clips, %v (want 3 of 8s)", aspect, len(segs), err)
		}
		for _, s := range segs {
			if w, h := dims(t, s.Path); w != want[0] || h != want[1] {
				t.Errorf("%s: clip %d is %dx%d, want %dx%d", aspect, s.Index, w, h, want[0], want[1])
			}
			if s.DurationMS < 7000 || s.DurationMS > 9000 {
				t.Errorf("%s: clip %d is %dms, want about 8000", aspect, s.Index, s.DurationMS)
			}
		}
	}
}

func TestValidAspect(t *testing.T) {
	for _, a := range []string{"", "9:16", "4:5", "1:1", "16:9"} {
		if !ValidAspect(a) {
			t.Errorf("%q should be valid", a)
		}
	}
	for _, a := range []string{"3:2", "9:16 ", "vertical", "1:1;x"} {
		if ValidAspect(a) {
			t.Errorf("%q should be refused", a)
		}
	}
}


