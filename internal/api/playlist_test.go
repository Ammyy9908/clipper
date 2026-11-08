package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ammyy9908/clipper/internal/config"
	"github.com/ammyy9908/clipper/internal/resolver"
	"github.com/ammyy9908/clipper/internal/store"
)

// fakeYtdlp returns a Resolver whose "yt-dlp" is a script that prints the given
// file, so handlers can be exercised without network access.
func fakeYtdlp(t *testing.T, jsonFile string) *resolver.Resolver {
	t.Helper()
	abs, err := filepath.Abs(jsonFile)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "yt-dlp")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ncat '"+abs+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return resolver.New(script, nil, "")
}

func testAPI(t *testing.T, jsonFile string, maxItems int) *API {
	t.Helper()
	resolveCache.Range(func(k, _ any) bool { resolveCache.Delete(k); return true })
	return &API{
		cfg: config.Config{PlaylistMaxItems: maxItems},
		res: fakeYtdlp(t, jsonFile),
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

const videoJSON = `../resolver/testdata/video.json`

func post(t *testing.T, h http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestResolveReturnsPlaylist(t *testing.T) {
	a := testAPI(t, "../resolver/testdata/playlist_flat.json", 50)
	rec := post(t, a.resolve, "/v1/resolve", `{"url":"https://www.youtube.com/playlist?list=PLx"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	var resp resolveResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Kind != "playlist" || resp.Playlist == nil {
		t.Fatalf("expected a playlist response, got kind=%q", resp.Kind)
	}
	if resp.Playlist.Count != 4 || len(resp.Playlist.Entries) != 4 {
		t.Errorf("expected 4 downloadable entries, got count=%d entries=%d", resp.Playlist.Count, len(resp.Playlist.Entries))
	}
	if !resp.Playlist.Truncated || resp.Playlist.Total != 116 {
		t.Errorf("playlist is longer than the listing, want truncated with total 116: %+v", resp.Playlist)
	}
	if resp.Title == "" || resp.Thumbnail == "" {
		t.Errorf("title/thumbnail missing: %q %q", resp.Title, resp.Thumbnail)
	}

	var has1080, hasMP3 bool
	for _, v := range resp.Variants {
		has1080 = has1080 || v.Label == "1080p"
		hasMP3 = hasMP3 || (v.Label == "MP3" && v.Audio)
		if v.Selector == "" {
			t.Errorf("variant %q has no selector", v.Label)
		}
	}
	if !has1080 || !hasMP3 {
		t.Errorf("playlist variants should offer 1080p and MP3: %+v", resp.Variants)
	}

	// The wire shape older clients already decode must be intact.
	var raw map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	for _, k := range []string{"title", "platform", "duration_seconds", "variants"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("legacy field %q missing from resolve response", k)
		}
	}
}

func TestResolveSingleVideoUnchanged(t *testing.T) {
	a := testAPI(t, videoJSON, 50)
	rec := post(t, a.resolve, "/v1/resolve", `{"url":"https://www.youtube.com/watch?v=abc"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var resp resolveResp
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Kind != "video" || resp.Playlist != nil {
		t.Errorf("single video must stay a video: kind=%q playlist=%v", resp.Kind, resp.Playlist)
	}
	if len(resp.Variants) < 2 {
		t.Errorf("expected real variants, got %+v", resp.Variants)
	}
}

func TestCreatePlaylistValidation(t *testing.T) {
	cases := []struct {
		name string
		file string
		max  int
		body string
		want string
	}{
		{"single video link", videoJSON, 50, `{"url":"https://www.youtube.com/watch?v=abc"}`, "single video"},
		{"position not in playlist", "../resolver/testdata/playlist_flat.json", 50, `{"url":"https://www.youtube.com/playlist?list=PLx","items":[99]}`, "item 99"},
		{"private entry position", "../resolver/testdata/playlist_flat.json", 50, `{"url":"https://www.youtube.com/playlist?list=PLx","items":[4]}`, "item 4"},
		{"over the cap", "../resolver/testdata/playlist_flat.json", 2, `{"url":"https://www.youtube.com/playlist?list=PLx"}`, "at most 2"},
		{"bad url", videoJSON, 50, `{"url":"ftp://example.com/x"}`, "http"},
		{"bad json", videoJSON, 50, `{"url":`, "invalid JSON"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := testAPI(t, c.file, c.max)
			rec := post(t, a.createPlaylist, "/v1/playlists", c.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), c.want) {
				t.Errorf("error should mention %q, got %s", c.want, rec.Body)
			}
		})
	}
}

func TestSelectEntries(t *testing.T) {
	entries := []playlistEntry{{Index: 1}, {Index: 2}, {Index: 3}, {Index: 5}}

	all, err := selectEntries(entries, nil)
	if err != nil || len(all) != 4 {
		t.Fatalf("no items should select everything: %v %v", all, err)
	}

	// Playlist order wins over request order, and duplicates collapse.
	got, err := selectEntries(entries, []int{5, 1, 5, 1})
	if err != nil || len(got) != 2 || got[0].Index != 1 || got[1].Index != 5 {
		t.Fatalf("want [1 5], got %v err=%v", got, err)
	}

	if _, err := selectEntries(entries, []int{2, 4}); err == nil {
		t.Fatal("position 4 is a gap and must be rejected")
	}
}

func TestSummarizePlaylist(t *testing.T) {
	mk := func(statuses ...string) []store.PlaylistJob {
		out := make([]store.PlaylistJob, len(statuses))
		for i, s := range statuses {
			out[i] = store.PlaylistJob{Job: store.Job{Status: s}, Position: i + 1}
		}
		return out
	}
	tests := []struct {
		name          string
		jobs          []store.PlaylistJob
		status        string
		completed, ko int
	}{
		{"all queued", mk("queued", "queued"), "queued", 0, 0},
		{"one started", mk("queued", "downloading"), "processing", 0, 0},
		{"partly done", mk("completed", "queued"), "processing", 1, 0},
		{"all done", mk("completed", "completed"), "completed", 2, 0},
		{"some failed still completes", mk("completed", "failed", "completed"), "completed", 2, 1},
		{"all failed", mk("failed", "failed"), "failed", 0, 2},
		{"failed but rest pending", mk("failed", "processing"), "processing", 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, completed, failed := summarizePlaylist(tt.jobs)
			if status != tt.status || completed != tt.completed || failed != tt.ko {
				t.Errorf("got %s %d/%d, want %s %d/%d", status, completed, failed, tt.status, tt.completed, tt.ko)
			}
		})
	}
}

func TestLooksLikePlaylistURL(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://www.youtube.com/playlist?list=PL1":    true,
		"https://music.youtube.com/playlist?list=PL1":  true,
		"https://www.youtube.com/watch?v=abc&list=PL1": false,
		"https://soundcloud.com/artist/sets/album":     true,
		"https://soundcloud.com/artist/track":          false,
		"https://www.instagram.com/reel/abc/":          false,
		"notaurl%%":                                    false,
	} {
		if got := looksLikePlaylistURL(raw); got != want {
			t.Errorf("looksLikePlaylistURL(%q) = %v, want %v", raw, got, want)
		}
	}
}
