package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ammyy9908/clipper/internal/resolver"
	"github.com/ammyy9908/clipper/internal/store"
)

func TestIsYouTubeMusic(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://music.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"http://music.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"music.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"https://MUSIC.YOUTUBE.COM/watch?v=dQw4w9WgXcQ", true},
		{"https://music.youtube.com/playlist?list=PL12345", true},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", false},
		{"https://youtube.com/watch?v=dQw4w9WgXcQ", false},
		{"https://youtu.be/dQw4w9WgXcQ", false},
		{"https://www.instagram.com/reel/C89abc/", false},
		{"https://www.tiktok.com/@user/video/123", false},
	}

	for _, tt := range tests {
		got := isYouTubeMusic(tt.url)
		if got != tt.expected {
			t.Errorf("isYouTubeMusic(%q) = %v, want %v", tt.url, got, tt.expected)
		}
	}
}

func TestBuildVariants(t *testing.T) {
	info := resolver.Info{
		Title:     "Sample Song / Video",
		Uploader:  "Artist",
		Extractor: "youtube",
		Duration:  210,
		Formats: []resolver.Format{
			{Height: 1080, Width: 1920, VCodec: "avc1", ACodec: "mp4a", TBR: 4000},
			{Height: 720, Width: 1280, VCodec: "avc1", ACodec: "mp4a", TBR: 2500},
			{Height: 480, Width: 854, VCodec: "avc1", ACodec: "mp4a", TBR: 1200},
			{Height: 360, Width: 640, VCodec: "avc1", ACodec: "mp4a", TBR: 600},
			{VCodec: "none", ACodec: "mp4a", TBR: 128},
		},
	}

	// Video with audio: multiple resolutions + MP3
	variants := buildVariants(info)
	if len(variants) <= 1 {
		t.Fatalf("expected multiple variants for YouTube video, got %d", len(variants))
	}
	hasMP3 := false
	has1080p := false
	for _, v := range variants {
		if v.Label == "MP3" && v.Audio {
			hasMP3 = true
		}
		if v.Label == "1080p" {
			has1080p = true
		}
	}
	if !hasMP3 || !has1080p {
		t.Errorf("variants should include both 1080p and MP3, got %+v", variants)
	}

	// Pure audio media (no video stream): returns ONLY MP3
	audioOnlyInfo := resolver.Info{
		Title:     "Pure Audio Track",
		Uploader:  "Artist",
		Extractor: "youtube",
		Duration:  180,
		Formats: []resolver.Format{
			{VCodec: "none", ACodec: "mp4a", TBR: 128},
		},
	}
	audioVariants := buildVariants(audioOnlyInfo)
	if len(audioVariants) != 1 {
		t.Fatalf("expected exactly 1 variant for audio-only track, got %d", len(audioVariants))
	}
	if audioVariants[0].Label != "MP3" || !audioVariants[0].Audio || audioVariants[0].Ext != "mp3" {
		t.Errorf("expected MP3 audio variant for audio-only track, got %+v", audioVariants[0])
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://music.youtube.com/watch?v=dQw4w9WgXcQ&si=12345",
			expected: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		},
		{
			input:    "https://youtu.be/dQw4w9WgXcQ?feature=shared",
			expected: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		},
		{
			input:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ&pp=ygU",
			expected: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		},
	}

	for _, tt := range tests {
		got := normalizeURL(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeURL(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestValidateURL(t *testing.T) {
	ok := []string{
		"https://www.youtube.com/watch?v=jNQXAC9IVRw",
		"http://youtu.be/abc",
		"https://v.redd.it/xyz",
		"https://8.8.8.8/video.mp4",
	}
	for _, u := range ok {
		if err := validateURL(u); err != nil {
			t.Errorf("validateURL(%q) = %v, want nil", u, err)
		}
	}
	bad := []string{
		"", "not a url", "ftp://example.com/a", "https:///nohost",
		"http://localhost/a", "http://localhost:8080/a", "http://foo.localhost/a",
		"http://127.0.0.1/a", "http://[::1]/a", "http://10.0.0.5/a", "http://192.168.1.1/a",
		"http://169.254.169.254/latest/meta-data", "http://0.0.0.0/a",
		"http://intranet/a", "http://printer.local/a", "http://db.internal/a",
		"http://2130706433/a", "http://127.1/a", "http://0x7f.0.0.1/a", "http://0177.0.0.1/a",
	}
	for _, u := range bad {
		if err := validateURL(u); err == nil {
			t.Errorf("validateURL(%q) = nil, want an error", u)
		}
	}
}

func TestValidSelector(t *testing.T) {
	ok := []string{"", "bestaudio/best", videoSelector(144), videoSelector(1080), videoSelector(2160)}
	for _, s := range ok {
		if !validSelector(s) {
			t.Errorf("validSelector(%q) = false, want true", s)
		}
	}
	bad := []string{
		"all", "b", "best", "bv*+ba", "bestaudio", "worst",
		"bv*[height<=1080]", // the start of a real selector, but not the whole of one
		videoSelector(1080) + "/all",
		videoSelector(0), "bv*[height<=99999999]+ba",
		"--exec rm -rf /", "b; id",
	}
	for _, s := range bad {
		if validSelector(s) {
			t.Errorf("validSelector(%q) = true, want false", s)
		}
	}
}

func TestPublicErrorHidesDetail(t *testing.T) {
	raw := "yt-dlp: ERROR: proxy http://user:secret@10.0.0.9:8080 refused /tmp/clipper/x"
	code := "internal"
	j := store.Job{Error: &raw, ErrorCode: &code}
	hideInternalError(&j)
	if strings.Contains(*j.Error, "secret") || strings.Contains(*j.Error, "/tmp") {
		t.Fatalf("job error still carries raw detail: %q", *j.Error)
	}
	if publicError("unavailable") == publicError("internal") {
		t.Error("known codes should keep their own message")
	}
}

func TestCreateJobRejectsBadAspect(t *testing.T) {
	a := authAPI(t, nil) // no database: a bad request must be refused before it is needed
	cases := map[string]string{
		"unknown ratio":  `{"url":"https://www.youtube.com/watch?v=jNQXAC9IVRw","aspect":"3:2"}`,
		"injection":      `{"url":"https://www.youtube.com/watch?v=jNQXAC9IVRw","aspect":"1:1;rm -rf /"}`,
		"audio has none": `{"url":"https://www.youtube.com/watch?v=jNQXAC9IVRw","audio_only":true,"aspect":"9:16"}`,
		"upload bad":     `{"object_key":"uploads/x.mp4","aspect":"2:1"}`,
	}
	for name, body := range cases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(body))
		a.createJob(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: answered %d, want 400 (%s)", name, w.Code, w.Body.String())
		}
	}
}

func TestConfigListsAspects(t *testing.T) {
	a := authAPI(t, nil)
	w := httptest.NewRecorder()
	a.publicConfig(w, httptest.NewRequest(http.MethodGet, "/v1/config", nil))
	for _, want := range []string{`"9:16"`, `"4:5"`, `"1:1"`, `"16:9"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("config is missing %s: %s", want, w.Body.String())
		}
	}
}

func TestUnshortenRedditURL(t *testing.T) {
	ctx := t.Context()
	input := "https://www.reddit.com/r/TeenagersButBetter/s/wtUsxniDEf"
	got := unshortenURL(ctx, input)
	t.Logf("Unshortened %s -> %s", input, got)
	if strings.Contains(got, "/s/") {
		t.Errorf("unshortenURL failed to unshorten %s, got %s", input, got)
	}
	norm := normalizeURL(got)
	t.Logf("Normalized -> %s", norm)
}
