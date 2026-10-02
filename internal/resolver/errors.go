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
	// ErrTooLarge means the video is longer or bigger than this server accepts.
	ErrTooLarge = errors.New("video exceeds the size or length limit")
	// ErrBusy means every lookup slot stayed taken until the caller gave up.
	ErrBusy = errors.New("too many lookups in progress")
)

// classify turns yt-dlp stderr into something retry logic can act on. Retrying
// a stale extractor is pointless; retrying a blocked IP on a fresh proxy is
// exactly right. Treating them the same wastes worker slots.
func classify(stderr string, fallback error) error {
	s := strings.ToLower(stderr)
	msg := strings.TrimSpace(stderr)
	if len(msg) > 600 {
		msg = msg[:600]
	}
	switch {
	case strings.Contains(s, "larger than max-filesize"),
		strings.Contains(s, "does not pass filter"):
		return ErrTooLarge
	case strings.Contains(s, "not a bot"),
		strings.Contains(s, "http error 429"),
		strings.Contains(s, "rate-limit"),
		strings.Contains(s, "blocked it in your country"):
		if msg != "" {
			return fmt.Errorf("%w: %s", ErrBlocked, msg)
		}
		return ErrBlocked
	case strings.Contains(s, "private video"),
		strings.Contains(s, "video unavailable"),
		strings.Contains(s, "no video formats found"),
		strings.Contains(s, "no media found"),
		strings.Contains(s, "no video in this"),
		strings.Contains(s, "there's no video"),
		strings.Contains(s, "has been removed"),
		strings.Contains(s, "age-restricted"),
		strings.Contains(s, "login required"),
		strings.Contains(s, "empty media response"),
		strings.Contains(s, "without being logged-in"),
		strings.Contains(s, "http error 404"):
		if msg != "" {
			return fmt.Errorf("%w: %s", ErrUnavailable, msg)
		}
		return ErrUnavailable
	case strings.Contains(s, "unsupported url"):
		return ErrUnsupported
	case strings.Contains(s, "unable to extract"),
		strings.Contains(s, "failed to parse json"),
		strings.Contains(s, "nsig extraction failed"),
		strings.Contains(s, "signature extraction failed"),
		strings.Contains(s, "signature solving failed"):
		if msg != "" {
			return fmt.Errorf("%w: %s", ErrExtractorStale, msg)
		}
		return ErrExtractorStale
	}
	if msg != "" {
		return fmt.Errorf("yt-dlp: %s", msg)
	}
	return fmt.Errorf("yt-dlp: %w", fallback)
}
