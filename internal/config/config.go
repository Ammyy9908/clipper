package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	RedisAddr      string
	S3Bucket       string
	S3Region       string
	S3Endpoint     string // set for MinIO / R2; empty for real S3
	MaxUploadBytes int64
	PresignTTL     time.Duration
	OutputTTL      time.Duration
	WorkDir        string
	JobTimeout     time.Duration
	WorkerConc     int
	YtdlpBin       string
	Proxies        []string // comma-separated egress pool
	CookiesFile    string

	// PlaylistMaxItems caps how many videos one playlist request can fan out
	// into. Every entry is a full download, so this is also an egress limit.
	PlaylistMaxItems int

	// Caps on what one download may cost. Zero disables a cap. Duration is
	// checked from the video's metadata before anything is fetched; the size cap
	// is also handed to yt-dlp as a backstop.
	MaxDownloadBytes   int64
	MaxDurationSeconds int
	ResolveConcurrency int // yt-dlp lookups allowed at once; the rest wait

	// Requests per minute allowed from one IP, by kind. Zero turns a limit off.
	RateResolve int // link lookups
	RateWrite   int // starting jobs, playlists and uploads
	RateAuth    int // sign-in attempts
	RateRead    int // job status, history and account reads

	// Google sign-in. Both must be set to enable it; without them the API runs
	// as before, with everyone treated as anonymous.
	GoogleClientID string // OAuth "Web application" client id
	GoogleJWKSURL  string // override for tests; empty means Google's
	SessionSecret  string // signs our session tokens, at least 32 characters
	SessionTTL     time.Duration

	// Daily download limits. Zero means unlimited.
	AnonDailyLimit   int    // per device id
	AnonIPDailyLimit int    // per IP, higher so a shared network isn't punished
	UserDailyLimit   int    // per signed-in Google account
	QuotaTZ          string // the day rolls over at midnight here; default UTC

	// Razorpay and Credits monetization
	RazorpayKeyID         string // rzp_live_... or rzp_test_...
	RazorpayKeySecret     string
	RazorpayWebhookSecret string
	DailyFreeCredits      int  // daily free credits for signed-in users (default 10)
	CreditsEnabled        bool // enable credit checking for downloads / splits

	// Proxies whose X-Forwarded-For is believed (CIDRs or IPs). Anyone else can
	// send that header, so it is ignored unless the request came from one of these.
	TrustedProxies []string

	// Website origins allowed to call the API from a browser, e.g.
	// "https://clipplay.video". Empty means no cross-origin access.
	CORSOrigins []string
}

func Load() Config {
	cookiesDefault := ""
	if _, err := os.Stat("/etc/yt-dlp/cookies.txt"); err == nil {
		cookiesDefault = "/etc/yt-dlp/cookies.txt"
	}

	return Config{
		Port:           env("PORT", "8080"),
		DatabaseURL:    env("DATABASE_URL", "postgres://clipper:clipper@localhost:5432/clipper?sslmode=disable"),
		RedisAddr:      env("REDIS_ADDR", "localhost:6379"),
		S3Bucket:       env("S3_BUCKET", "clipper-dev"),
		S3Region:       env("AWS_REGION", "ap-south-1"),
		S3Endpoint:     env("S3_ENDPOINT", ""),
		MaxUploadBytes: int64(envInt("MAX_UPLOAD_MB", 512)) * 1024 * 1024,
		PresignTTL:     time.Duration(envInt("PRESIGN_TTL_MIN", 15)) * time.Minute,
		OutputTTL:      time.Duration(envInt("OUTPUT_TTL_HOURS", 24)) * time.Hour,
		WorkDir:        env("WORK_DIR", "/tmp/clipper"),
		JobTimeout:     time.Duration(envInt("JOB_TIMEOUT_MIN", 60)) * time.Minute,
		WorkerConc:     envInt("WORKER_CONCURRENCY", 1),
		YtdlpBin:       env("YTDLP_BIN", "yt-dlp"),
		Proxies:        envList("PROXY_POOL"),
		CookiesFile:    env("COOKIES_FILE", cookiesDefault),

		PlaylistMaxItems: clamp(envInt("PLAYLIST_MAX_ITEMS", 50), 1, 200),

		MaxDownloadBytes:   int64(envInt("MAX_DOWNLOAD_MB", 4096)) * 1024 * 1024,
		MaxDurationSeconds: envInt("MAX_DURATION_MINUTES", 360) * 60,
		ResolveConcurrency: clamp(envInt("RESOLVE_CONCURRENCY", 8), 1, 64),

		RateResolve: envInt("RATE_LIMIT_RESOLVE_PER_MIN", 20),
		RateWrite:   envInt("RATE_LIMIT_WRITE_PER_MIN", 20),
		RateAuth:    envInt("RATE_LIMIT_AUTH_PER_MIN", 10),
		RateRead:    envInt("RATE_LIMIT_READ_PER_MIN", 300),

		GoogleClientID: env("GOOGLE_CLIENT_ID", ""),
		GoogleJWKSURL:  env("GOOGLE_JWKS_URL", ""),
		SessionSecret:  env("SESSION_SECRET", ""),
		SessionTTL:     time.Duration(envInt("SESSION_TTL_HOURS", 168)) * time.Hour,

		AnonDailyLimit:   envInt("ANON_DAILY_LIMIT", 3),
		AnonIPDailyLimit: envInt("ANON_IP_DAILY_LIMIT", 9),
		UserDailyLimit:   envInt("USER_DAILY_LIMIT", 20),
		QuotaTZ:          env("QUOTA_TZ", "UTC"),

		RazorpayKeyID:         env("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret:     env("RAZORPAY_KEY_SECRET", ""),
		RazorpayWebhookSecret: env("RAZORPAY_WEBHOOK_SECRET", ""),
		DailyFreeCredits:      envInt("DAILY_FREE_CREDITS", 10),
		CreditsEnabled:        envBool("CREDITS_ENABLED", true),

		TrustedProxies: envList("TRUSTED_PROXY_CIDRS"),
		CORSOrigins:    envList("CORS_ALLOWED_ORIGINS"),
	}
}

func envBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		v = strings.ToLower(strings.TrimSpace(v))
		return v == "1" || v == "true" || v == "yes" || v == "on"
	}
	return def
}

func envList(k string) []string {
	raw := os.Getenv(k)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.Trim(strings.TrimSpace(p), "\"'"); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
