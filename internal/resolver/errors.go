package resolver

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrExtractorStale means the platform changed and yt-dlp can't parse it.
	// Fix is a yt-dlp bump, not a retry.
	ErrExtractorStale = errors.New("extractor failed; yt-dlp likely needs updating")
	// ErrBlocked means the egress IP or session was rejected. Fix is another proxy.
	ErrBlocked = errors.New("request blocked; egress IP or session rejected")
	// ErrUnavailable means private, deleted, geo-blocked, or age-gated.
	ErrUnavailable = errors.New("content unavailable")
	ErrLiveStream  = errors.New("live streams are not supported")
	ErrUnsupported = errors.New("unsupported URL")
)

// classify turns yt-dlp stderr into something retry logic can act on. Retrying
// a stale extractor is pointless; retrying a blocked IP on a fresh proxy is
// exactly right. Treating them the same wastes worker slots.
func classify(stderr string, fallback error) error {
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "confirm you're not a bot"),
		strings.Contains(s, "confirm you are not a bot"),
		strings.Contains(s, "http error 429"),
		strings.Contains(s, "rate-limit"),
		strings.Contains(s, "blocked it in your country"):
		return ErrBlocked
	case strings.Contains(s, "private video"),
		strings.Contains(s, "video unavailable"),
		strings.Contains(s, "has been removed"),
		strings.Contains(s, "age-restricted"),
		strings.Contains(s, "login required"),
		strings.Contains(s, "http error 404"):
		return ErrUnavailable
	case strings.Contains(s, "unsupported url"):
		return ErrUnsupported
	case strings.Contains(s, "unable to extract"),
		strings.Contains(s, "failed to parse json"),
		strings.Contains(s, "nsig extraction failed"),
		strings.Contains(s, "signature extraction failed"):
		return ErrExtractorStale
	}
	msg := strings.TrimSpace(stderr)
	if len(msg) > 400 {
		msg = msg[:400]
	}
	if msg != "" {
		return fmt.Errorf("yt-dlp: %s", msg)
	}
	return fmt.Errorf("yt-dlp: %w", fallback)
}
