package queue

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/ammyy9908/clipper/internal/store"
)

// JobEvent represents a real-time progress or state change event sent to clients.
type JobEvent struct {
	ID        uuid.UUID    `json:"id"`
	Status    string       `json:"status"`
	Percent   int          `json:"percent,omitempty"`
	Speed     string       `json:"speed,omitempty"`
	ETA       string       `json:"eta,omitempty"`
	Stage     string       `json:"stage,omitempty"`
	Title     string       `json:"title,omitempty"`
	Error     string       `json:"error,omitempty"`
	ErrorCode string       `json:"error_code,omitempty"`
	Clips     []store.Clip `json:"clips,omitempty"`
}

// JobChannel returns the Redis Pub/Sub topic for a specific job.
func JobChannel(jobID string) string {
	return "job:" + jobID
}

// NewRedisClient creates a go-redis client supporting plain, TLS (rediss://), and host:port strings.
func NewRedisClient(addr string) (*redis.Client, error) {
	if addr == "" {
		addr = "localhost:6379"
	}

	if strings.HasPrefix(addr, "redis://") || strings.HasPrefix(addr, "rediss://") {
		opt, err := redis.ParseURL(addr)
		if err != nil {
			return nil, err
		}
		return redis.NewClient(opt), nil
	}

	// Host:port with optional password parsing
	opt := &redis.Options{
		Addr: addr,
	}
	if strings.Contains(addr, "@") {
		u, err := url.Parse("redis://" + addr)
		if err == nil {
			opt.Addr = u.Host
			if p, ok := u.User.Password(); ok {
				opt.Password = p
			}
		}
	}
	if strings.HasSuffix(addr, ":6380") || strings.Contains(addr, "upstash.io") {
		opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return redis.NewClient(opt), nil
}

// PublishJobEvent broadcasts a job update to all subscribed WebSocket/SSE clients.
func PublishJobEvent(ctx context.Context, rdb *redis.Client, event JobEvent) error {
	if rdb == nil {
		return nil
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return rdb.Publish(ctx, JobChannel(event.ID.String()), data).Err()
}
