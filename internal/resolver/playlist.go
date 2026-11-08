package resolver

import (
	"net/url"
	"strings"
)

// DefaultPlaylistMax is how many entries a playlist lookup returns when the
// caller doesn't configure a limit.
const DefaultPlaylistMax = 50

func (r *Resolver) playlistMax() int {
	if r.PlaylistMax > 0 {
		return r.PlaylistMax
	}
	return DefaultPlaylistMax
}

// LooksLikePlaylistURL reports whether raw URL is formatted as a playlist link.
func LooksLikePlaylistURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "playlist?list=") ||
		strings.Contains(lower, "/playlist/") ||
		strings.Contains(lower, "/sets/") ||
		strings.Contains(lower, "/album/") ||
		strings.Contains(lower, "/albums/") ||
		strings.Contains(lower, "/collection/") {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	q := u.Query()
	if q.Get("list") != "" && q.Get("v") == "" {
		return true
	}
	return false
}

type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Entry is one row of yt-dlp's flat playlist listing.
type Entry struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	URL        string      `json:"url"`
	WebpageURL string      `json:"webpage_url"`
	IEKey      string      `json:"ie_key"`
	Duration   float64     `json:"duration"`
	Uploader   string      `json:"uploader"`
	Thumbnails []Thumbnail `json:"thumbnails"`
	Thumbnail  string      `json:"thumbnail"`
}

// PlaylistEntry is an Entry cleaned up for callers: 1-based position, a URL
// that can be handed straight to a download job, and no unusable rows.
type PlaylistEntry struct {
	Index     int
	Title     string
	URL       string
	Duration  float64
	Thumbnail string
	Uploader  string
}

// IsPlaylist reports whether yt-dlp returned a collection rather than one video.
func (i Info) IsPlaylist() bool {
	return i.Type == "playlist" || i.Type == "multi_video"
}

// PlaylistEntries returns the downloadable entries in order, numbered by their
// position in the original playlist so a caller can refer back to them.
//
// Private and deleted videos still appear in listings (as "[Private video]"
// and "[Deleted video]") but can never be fetched, so they are dropped here
// rather than becoming jobs that are certain to fail. Their positions are
// skipped, not renumbered, so indices stay stable.
func (i Info) PlaylistEntries() []PlaylistEntry {
	out := make([]PlaylistEntry, 0, len(i.Entries))
	for n, e := range i.Entries {
		if isUnavailableTitle(e.Title) {
			continue
		}
		link := entryURL(e, i.Extractor)
		if link == "" {
			continue
		}
		out = append(out, PlaylistEntry{
			Index:     n + 1,
			Title:     e.Title,
			URL:       link,
			Duration:  e.Duration,
			Thumbnail: entryThumbnail(e),
			Uploader:  e.Uploader,
		})
	}
	return out
}

func isUnavailableTitle(t string) bool {
	t = strings.ToLower(strings.TrimSpace(t))
	return t == "[private video]" || t == "[deleted video]"
}

// entryURL picks the page URL for an entry. Flat listings usually provide
// "url"; a few extractors only give an id, which is enough for YouTube.
func entryURL(e Entry, extractor string) string {
	for _, c := range []string{e.URL, e.WebpageURL} {
		if u, err := url.Parse(c); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			return c
		}
	}
	isYouTube := strings.EqualFold(e.IEKey, "Youtube") || strings.HasPrefix(strings.ToLower(extractor), "youtube")
	if e.ID != "" && isYouTube {
		return "https://www.youtube.com/watch?v=" + url.QueryEscape(e.ID)
	}
	return ""
}

// entryThumbnail prefers the widest thumbnail, which for flat YouTube listings
// is the last one.
func entryThumbnail(e Entry) string {
	if e.Thumbnail != "" {
		return e.Thumbnail
	}
	best := ""
	bestW := -1
	for _, t := range e.Thumbnails {
		if t.URL != "" && t.Width >= bestW {
			best, bestW = t.URL, t.Width
		}
	}
	return best
}
