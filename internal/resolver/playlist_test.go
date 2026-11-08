package resolver

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func loadFixture(t *testing.T) Info {
	t.Helper()
	raw, err := os.ReadFile("testdata/playlist_flat.json")
	if err != nil {
		t.Fatal(err)
	}
	var info Info
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatalf("fixture should decode into Info: %v", err)
	}
	return info
}

func TestInfoDecodesFlatPlaylist(t *testing.T) {
	info := loadFixture(t)
	if !info.IsPlaylist() {
		t.Fatalf("type %q should be a playlist", info.Type)
	}
	if info.Title == "" || info.PlaylistCount != 116 {
		t.Errorf("playlist metadata not decoded: title=%q count=%d", info.Title, info.PlaylistCount)
	}
	if len(info.Entries) != 6 {
		t.Fatalf("expected 6 raw entries, got %d", len(info.Entries))
	}
}

func TestSingleVideoIsNotAPlaylist(t *testing.T) {
	var info Info
	if err := json.Unmarshal([]byte(`{"id":"x","title":"one video","_type":"video","formats":[]}`), &info); err != nil {
		t.Fatal(err)
	}
	if info.IsPlaylist() {
		t.Error("a video must not be treated as a playlist")
	}
	// yt-dlp omits _type for some extractors' single results.
	if err := json.Unmarshal([]byte(`{"id":"x","title":"one video"}`), &info); err != nil {
		t.Fatal(err)
	}
	if (Info{Title: "x"}).IsPlaylist() {
		t.Error("missing _type must not be treated as a playlist")
	}
}

func TestPlaylistEntriesSkipsUnavailableAndKeepsPositions(t *testing.T) {
	info := loadFixture(t)
	entries := info.PlaylistEntries()

	// 6 raw: 3 real, [Private video], [Deleted video], and an id-only entry.
	if len(entries) != 4 {
		t.Fatalf("expected 4 usable entries, got %d: %+v", len(entries), entries)
	}
	for _, e := range entries {
		if isUnavailableTitle(e.Title) {
			t.Errorf("unavailable entry leaked through: %+v", e)
		}
		if e.URL == "" {
			t.Errorf("entry without a URL leaked through: %+v", e)
		}
	}
	// Positions refer to the original listing, so skipped rows leave gaps.
	last := entries[len(entries)-1]
	if last.Title != "Id only entry" || last.Index != 6 {
		t.Errorf("last entry should keep its original position 6, got %+v", last)
	}
	// The id-only entry gets a YouTube watch URL built from its id.
	if last.URL != "https://www.youtube.com/watch?v=ccccccccccc" {
		t.Errorf("id-only entry URL = %q", last.URL)
	}
	if entries[0].Index != 1 || entries[0].Thumbnail == "" || entries[0].Duration == 0 {
		t.Errorf("first entry missing fields: %+v", entries[0])
	}
}

func TestEntryURLRejectsNonHTTP(t *testing.T) {
	e := Entry{URL: "javascript:alert(1)", ID: "abc", IEKey: "Generic"}
	if got := entryURL(e, "Generic"); got != "" {
		t.Errorf("non-http URL must be rejected, got %q", got)
	}
}

func TestPlaylistMaxDefault(t *testing.T) {
	if got := (&Resolver{}).playlistMax(); got != DefaultPlaylistMax {
		t.Errorf("default = %d, want %d", got, DefaultPlaylistMax)
	}
	if got := (&Resolver{PlaylistMax: 12}).playlistMax(); got != 12 {
		t.Errorf("configured = %d, want 12", got)
	}
}

// Talks to the real yt-dlp and the real network. Opt in with:
//
//	YTDLP_LIVE=1 go test ./internal/resolver -run Live -v
func TestResolveLivePlaylist(t *testing.T) {
	if os.Getenv("YTDLP_LIVE") == "" {
		t.Skip("YTDLP_LIVE not set")
	}
	r := New(os.Getenv("YTDLP_BIN"), nil, "")
	r.PlaylistMax = 3

	info, err := r.Resolve(context.Background(), "https://www.youtube.com/playlist?list=PLBsm_SagFMmdWnCnrNtLjA9kzfrRkto4i")
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsPlaylist() || info.Title == "" {
		t.Fatalf("expected a playlist, got type=%q title=%q", info.Type, info.Title)
	}
	entries := info.PlaylistEntries()
	if len(entries) != 3 {
		t.Fatalf("PlaylistMax=3 should cap the listing at 3 entries, got %d", len(entries))
	}
	if info.PlaylistCount <= 3 {
		t.Errorf("playlist_count should report the real size, got %d", info.PlaylistCount)
	}
	for _, e := range entries {
		if e.URL == "" || e.Title == "" {
			t.Errorf("incomplete entry: %+v", e)
		}
	}
	t.Logf("%q: %d of %d entries, first: %q %s", info.Title, len(entries), info.PlaylistCount, entries[0].Title, entries[0].URL)

	// A watch URL that carries a list= must still resolve as ONE video.
	one, err := r.Resolve(context.Background(), entries[0].URL+"&list=PLBsm_SagFMmdWnCnrNtLjA9kzfrRkto4i")
	if err != nil {
		t.Fatal(err)
	}
	if one.IsPlaylist() || len(one.Formats) == 0 {
		t.Errorf("watch?v=..&list=.. must stay a single video: type=%q formats=%d", one.Type, len(one.Formats))
	}
}
