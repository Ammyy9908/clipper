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
}

func Load() Config {
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
		JobTimeout:     time.Duration(envInt("JOB_TIMEOUT_MIN", 20)) * time.Minute,
		WorkerConc:     envInt("WORKER_CONCURRENCY", 2),
		YtdlpBin:       env("YTDLP_BIN", "yt-dlp"),
		Proxies:        envList("PROXY_POOL"),
		CookiesFile:    env("COOKIES_FILE", ""),
	}
}

func envList(k string) []string {
	raw := os.Getenv(k)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
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
