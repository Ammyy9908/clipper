package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/hibiken/asynq"

	"github.com/ammyy9908/clipper/internal/config"
	"github.com/ammyy9908/clipper/internal/media"
	"github.com/ammyy9908/clipper/internal/queue"
	"github.com/ammyy9908/clipper/internal/resolver"
	"github.com/ammyy9908/clipper/internal/storage"
	"github.com/ammyy9908/clipper/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()
	ctx := context.Background()

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("postgres connect failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	files, err := storage.New(ctx, cfg)
	if err != nil {
		log.Error("s3 init failed", "err", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(cfg.WorkDir, 0o755); err != nil {
		log.Error("workdir failed", "err", err)
		os.Exit(1)
	}

	res := resolver.New(cfg.YtdlpBin, cfg.Proxies, cfg.CookiesFile)
	if v, err := res.Version(ctx); err == nil {
		// Log it every boot: when downloads start failing, the first question
		// is always "how old is yt-dlp in this image".
		log.Info("yt-dlp ready", "version", v)
	} else {
		log.Error("yt-dlp missing or broken", "err", err)
		os.Exit(1)
	}

	p := &processor{cfg: cfg, db: db, files: files, res: res, log: log}

	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeSplit, p.handleSplit)

	// Concurrency is bounded by disk and CPU, not by how many jobs are waiting.
	// One 4K input plus its clips can be several GB of scratch space.
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{Concurrency: cfg.WorkerConc, Logger: asynqLogger{log}},
	)

	log.Info("worker starting", "concurrency", cfg.WorkerConc)
	if err := srv.Run(mux); err != nil {
		log.Error("worker failed", "err", err)
		os.Exit(1)
	}
}

type processor struct {
	cfg   config.Config
	db    *store.Store
	files *storage.Store
	res   *resolver.Resolver
	log   *slog.Logger
}

func (p *processor) handleSplit(ctx context.Context, t *asynq.Task) error {
	payload, err := queue.ParseSplitPayload(t)
	if err != nil {
		// Unparseable payload will never succeed; don't retry it.
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	job, err := p.db.GetJob(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}
	log := p.log.With("job", job.ID)

	if err := p.db.SetStatus(ctx, job.ID, store.StatusProcessing, nil, nil); err != nil {
		return err
	}

	if err := p.process(ctx, job, log); err != nil {
		msg, code := err.Error(), errorCode(err)
		log.Error("job failed", "err", msg, "code", code)
		if serr := p.db.SetStatus(ctx, job.ID, store.StatusFailed, &msg, &code); serr != nil {
			log.Error("marking failed failed", "err", serr)
		}
		// Some failures will never succeed on retry. Burning two more worker
		// slots on a private video helps nobody.
		if terminal(err) {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return err
	}
	log.Info("job completed")
	return nil
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, resolver.ErrUnavailable):
		return "unavailable"
	case errors.Is(err, resolver.ErrBlocked):
		return "blocked"
	case errors.Is(err, resolver.ErrExtractorStale):
		return "extractor_stale"
	case errors.Is(err, resolver.ErrUnsupported):
		return "unsupported"
	case errors.Is(err, resolver.ErrLiveStream):
		return "live_stream"
	default:
		return "internal"
	}
}

func terminal(err error) bool {
	return errors.Is(err, resolver.ErrUnavailable) ||
		errors.Is(err, resolver.ErrUnsupported) ||
		errors.Is(err, resolver.ErrLiveStream) ||
		errors.Is(err, resolver.ErrExtractorStale)
}

func (p *processor) process(ctx context.Context, job store.Job, log *slog.Logger) error {
	dir := filepath.Join(p.cfg.WorkDir, job.ID.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Scratch space is the scarcest resource on a worker; never leak it.
	defer os.RemoveAll(dir)

	src, err := p.acquire(ctx, job, dir, log)
	if err != nil {
		return err
	}

	total, err := media.Probe(ctx, src)
	if err != nil {
		return err
	}
	log.Info("source probed", "duration_ms", total)

	if job.Aspect != media.AspectOriginal {
		log.Info("reframing", "aspect", job.Aspect)
		reframed, err := media.Reframe(ctx, src, dir, job.Aspect)
		if err != nil {
			return err
		}
		src = reframed
	}

	// segment_seconds == 0 means the user wanted the file, not clips.
	var segs []media.Segment
	if job.SegmentSeconds == 0 {
		segs = []media.Segment{{Index: 0, Path: src, DurationMS: total}}
	} else {
		segs, err = media.Split(ctx, src, filepath.Join(dir, "out"), job.SegmentSeconds, job.Mode)
		if err != nil {
			return err
		}
	}

	clips := make([]store.Clip, 0, len(segs))
	for _, s := range segs {
		key := fmt.Sprintf("outputs/%s/clip_%04d.mp4", job.ID, s.Index)
		size, err := p.files.Upload(ctx, key, s.Path, "video/mp4")
		if err != nil {
			return fmt.Errorf("upload clip %d: %w", s.Index, err)
		}
		clips = append(clips, store.Clip{
			Index:      s.Index,
			ObjectKey:  key,
			DurationMS: s.DurationMS,
			SizeBytes:  size,
		})
	}

	return p.db.Complete(ctx, job.ID, total, clips)
}

// acquire puts the source video on local disk, whichever way it arrived.
func (p *processor) acquire(ctx context.Context, job store.Job, dir string, log *slog.Logger) (string, error) {
	if job.SourceType == store.SourceRemote {
		if job.SourceURL == nil {
			return "", fmt.Errorf("remote job with no url")
		}
		// Resolve first so we can store a real title and fail fast on dead
		// links before committing bandwidth to a download.
		info, err := p.res.Resolve(ctx, *job.SourceURL)
		if err != nil {
			return "", err
		}
		if info.Title != "" {
			if err := p.db.SetTitle(ctx, job.ID, info.Title); err != nil {
				log.Warn("could not store title", "err", err)
			}
		}

		selector := ""
		if job.Format != nil {
			selector = *job.Format
		}
		dest := filepath.Join(dir, "source.%(ext)s")
		if err := p.res.Fetch(ctx, *job.SourceURL, selector, dest); err != nil {
			return "", err
		}

		matches, err := filepath.Glob(filepath.Join(dir, "source.*"))
		if err != nil || len(matches) == 0 {
			return "", fmt.Errorf("yt-dlp reported success but produced no file")
		}
		log.Info("remote source fetched", "platform", info.Extractor)
		return matches[0], nil
	}

	if job.SourceKey == nil {
		return "", fmt.Errorf("upload job with no object key")
	}
	dest := filepath.Join(dir, "source"+filepath.Ext(*job.SourceKey))
	if err := p.files.Download(ctx, *job.SourceKey, dest); err != nil {
		return "", fmt.Errorf("download source: %w", err)
	}
	return dest, nil
}

type asynqLogger struct{ l *slog.Logger }

func (a asynqLogger) Debug(args ...any) { a.l.Debug(fmt.Sprint(args...)) }
func (a asynqLogger) Info(args ...any)  { a.l.Info(fmt.Sprint(args...)) }
func (a asynqLogger) Warn(args ...any)  { a.l.Warn(fmt.Sprint(args...)) }
func (a asynqLogger) Error(args ...any) { a.l.Error(fmt.Sprint(args...)) }
func (a asynqLogger) Fatal(args ...any) { a.l.Error(fmt.Sprint(args...)); os.Exit(1) }
