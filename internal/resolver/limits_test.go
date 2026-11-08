package resolver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClassifyTooLarge(t *testing.T) {
	for _, s := range []string{
		"[download] Me at the zoo does not pass filter (duration<=?5), skipping ..",
		"File is larger than max-filesize (2000 bytes > 1000 bytes). Aborting.",
	} {
		if err := classify(s, errors.New("x")); !errors.Is(err, ErrTooLarge) {
			t.Errorf("classify(%q) = %v, want ErrTooLarge", s, err)
		}
	}
}

// Every lookup starts a yt-dlp process, so callers beyond the limit wait, and
// give up rather than pile on.
func TestResolveConcurrencyIsBounded(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-ytdlp")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nsleep 1\necho '{\"title\":\"x\",\"formats\":[]}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := New(bin, nil, "")
	r.MaxConcurrent = 1

	started := make(chan struct{})
	go func() {
		close(started)
		_, _ = r.Resolve(context.Background(), "https://example.com/a")
	}()
	<-started
	time.Sleep(200 * time.Millisecond) // let the first call take the slot

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := r.Resolve(ctx, "https://example.com/b"); !errors.Is(err, ErrBusy) {
		t.Fatalf("second Resolve = %v, want ErrBusy", err)
	}
}
