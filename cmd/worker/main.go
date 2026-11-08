package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

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

	days := int32(cfg.OutputTTL.Hours() / 24)
	if days < 1 {
		days = 1
	}
	if err := files.EnsureLifecycle(ctx, days); err != nil {
		log.Warn("r2 lifecycle config failed", "err", err)
	} else {
		log.Info("r2 lifecycle configured", "expire_days", days)
	}

	// Background pruner for expired database records and cache
	go func() {
		if n, err := db.PruneExpired(context.Background(), cfg.OutputTTL); err == nil && n > 0 {
			log.Info("initial prune of expired jobs from db", "count", n)
		}
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if n, err := db.PruneExpired(context.Background(), cfg.OutputTTL); err != nil {
				log.Warn("prune expired jobs failed", "err", err)
			} else if n > 0 {
				log.Info("pruned expired jobs from db", "count", n)
			}
		}
	}()

	if err := os.MkdirAll(cfg.WorkDir, 0o755); err != nil {
		log.Error("workdir failed", "err", err)
		os.Exit(1)
	}

	res := resolver.New(cfg.YtdlpBin, cfg.Proxies, cfg.CookiesFile)
	if v, err := res.Version(ctx); err == nil {
		log.Info("yt-dlp ready", "version", v)
	} else {
		log.Error("yt-dlp missing or broken", "err", err)
		os.Exit(1)
	}

	redisOpt, err := queue.ParseRedisOpt(cfg.RedisAddr)
	if err != nil {
		log.Error("invalid redis addr", "err", err)
		os.Exit(1)
	}

	qClient := asynq.NewClient(redisOpt)
	defer qClient.Close()

	rdb, err := queue.NewRedisClient(cfg.RedisAddr)
	if err != nil {
		log.Error("redis client failed", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	h := &hybridWorker{
		cfg:     cfg,
		db:      db,
		files:   files,
		res:     res,
		qClient: qClient,
		rdb:     rdb,
		log:     log,
	}

	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeDownload, h.handleDownload)
	mux.HandleFunc(queue.TypeProcess, h.handleProcess)
	mux.HandleFunc(queue.TypeSplit, h.handleProcess) // Alias

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: cfg.WorkerConc,
			Queues: map[string]int{
				queue.QueueDownload: 2,
				queue.QueueProcess:  2,
				"default":           1,
			},
			Logger: asynqLogger{log},
		},
	)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("hybrid worker starting", "concurrency", cfg.WorkerConc)
		if err := srv.Run(mux); err != nil && !errors.Is(err, asynq.ErrServerClosed) {
			log.Error("hybrid worker failed", "err", err)
			os.Exit(1)
		}
	}()

	<-stop
	srv.Shutdown()
	log.Info("hybrid worker stopped")
}

type hybridWorker struct {
	cfg     config.Config
	db      *store.Store
	files   *storage.Store
	res     *resolver.Resolver
	qClient *asynq.Client
	rdb     *redis.Client
	log     *slog.Logger
}

func (h *hybridWorker) handleDownload(ctx context.Context, t *asynq.Task) error {
	payload, err := queue.ParsePayload(t)
	if err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	job, err := h.db.GetJob(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}
	log := h.log.With("job", job.ID)

	if job.SourceURL == nil || *job.SourceURL == "" {
		return fmt.Errorf("%w: remote job with no url", asynq.SkipRetry)
	}

	if err := h.db.SetStatus(ctx, job.ID, store.StatusDownloading, nil, nil); err != nil {
		return err
	}
	_ = queue.PublishJobEvent(ctx, h.rdb, queue.JobEvent{
		ID:     job.ID,
		Status: store.StatusDownloading,
		Stage:  "downloading",
	})

	dir := filepath.Join(h.cfg.WorkDir, "download_"+job.ID.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	if info, err := h.res.Resolve(ctx, *job.SourceURL); err == nil && info.Title != "" {
		if err := h.db.SetTitle(ctx, job.ID, info.Title); err != nil {
			log.Warn("could not store title", "err", err)
		}
		_ = queue.PublishJobEvent(ctx, h.rdb, queue.JobEvent{
			ID:     job.ID,
			Status: store.StatusDownloading,
			Stage:  "downloading",
			Title:  info.Title,
		})
	} else if err != nil {
		log.Warn("metadata resolve skipped/failed; proceeding to direct fetch", "err", err)
	}

	selector := ""
	if job.Format != nil {
		selector = *job.Format
	}
	dest := filepath.Join(dir, "source.%(ext)s")
	if err := h.res.Fetch(ctx, *job.SourceURL, selector, job.AudioOnly, dest); err != nil {
		msg, code := err.Error(), errorCode(err)
		log.Error("download failed", "err", msg, "code", code)
		_ = h.db.SetStatus(ctx, job.ID, store.StatusFailed, &msg, &code)
		_ = queue.PublishJobEvent(ctx, h.rdb, queue.JobEvent{
			ID:        job.ID,
			Status:    store.StatusFailed,
			Stage:     "failed",
			Error:     msg,
			ErrorCode: code,
		})
		if terminal(err) {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return err
	}

	matches, err := filepath.Glob(filepath.Join(dir, "source.*"))
	if err != nil || len(matches) == 0 {
		return fmt.Errorf("yt-dlp reported success but produced no file")
	}
	localFile := matches[0]
	ext := filepath.Ext(localFile)
	if ext == "" {
		ext = ".mp4"
	}

	sourceKey := fmt.Sprintf("sources/%s%s", job.ID, ext)
	contentType := "video/mp4"
	if job.AudioOnly {
		contentType = "audio/mpeg"
	}

	log.Info("uploading source to storage", "key", sourceKey)
	if _, err := h.files.Upload(ctx, sourceKey, localFile, contentType); err != nil {
		return fmt.Errorf("upload source: %w", err)
	}

	if err := h.db.SetSourceKey(ctx, job.ID, sourceKey); err != nil {
		return fmt.Errorf("update job source key: %w", err)
	}

	task, err := queue.NewProcessTask(job.ID)
	if err != nil {
		return err
	}
	if _, err := h.qClient.EnqueueContext(ctx, task,
		asynq.MaxRetry(2),
		asynq.Timeout(h.cfg.JobTimeout),
		asynq.Retention(24*time.Hour),
	); err != nil {
		log.Error("failed to enqueue process task", "err", err)
		return err
	}

	log.Info("download completed, enqueued for processing")
	_ = queue.PublishJobEvent(ctx, h.rdb, queue.JobEvent{
		ID:     job.ID,
		Status: store.StatusProcessing,
		Stage:  "processing",
	})
	return nil
}

func (h *hybridWorker) handleProcess(ctx context.Context, t *asynq.Task) error {
	payload, err := queue.ParsePayload(t)
	if err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	job, err := h.db.GetJob(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}
	log := h.log.With("job", job.ID)

	if err := h.db.SetStatus(ctx, job.ID, store.StatusProcessing, nil, nil); err != nil {
		return err
	}
	_ = queue.PublishJobEvent(ctx, h.rdb, queue.JobEvent{
		ID:     job.ID,
		Status: store.StatusProcessing,
		Stage:  "processing",
	})

	if err := h.process(ctx, job, log); err != nil {
		msg, code := err.Error(), errorCode(err)
		log.Error("job failed", "err", msg, "code", code)
		if serr := h.db.SetStatus(ctx, job.ID, store.StatusFailed, &msg, &code); serr != nil {
			log.Error("marking failed failed", "err", serr)
		}
		_ = queue.PublishJobEvent(ctx, h.rdb, queue.JobEvent{
			ID:        job.ID,
			Status:    store.StatusFailed,
			Stage:     "failed",
			Error:     msg,
			ErrorCode: code,
		})
		if terminal(err) {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return err
	}
	log.Info("job completed")
	return nil
}

func (h *hybridWorker) process(ctx context.Context, job store.Job, log *slog.Logger) error {
	dir := filepath.Join(h.cfg.WorkDir, "process_"+job.ID.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	src, err := h.acquire(ctx, job, dir, log)
	if err != nil {
		return err
	}

	if job.AudioOnly {
		return h.processAudio(ctx, job, src, dir)
	}

	total, err := media.Probe(ctx, src)
	if err != nil {
		return err
	}
	log.Info("source probed", "duration_ms", total)

	if codec, err := media.VideoCodec(ctx, src); err == nil && codec != "h264" {
		log.Info("transcoding for device compatibility", "from", codec)
		src, err = media.ToH264(ctx, src, dir)
		if err != nil {
			return err
		}
	}

	if job.Aspect != media.AspectOriginal && media.ValidAspect(job.Aspect) {
		log.Info("reframing", "aspect", job.Aspect)
		reframed, err := media.Reframe(ctx, src, dir, job.Aspect, job.SegmentSeconds)
		if err != nil {
			return err
		}
		src = reframed
	}

	var segs []media.Segment
	if job.SegmentSeconds == 0 {
		segs = []media.Segment{{Index: 0, Path: src, DurationMS: total}}
	} else {
		mode := job.Mode
		if job.Aspect != media.AspectOriginal {
			// Reframing already put a keyframe on every boundary, so a copy is exact.
			mode = media.ModeCopy
		}
		segs, err = media.Split(ctx, src, filepath.Join(dir, "out"), job.SegmentSeconds, mode)
		if err != nil {
			return err
		}
	}

	clips := make([]store.Clip, 0, len(segs))
	for _, s := range segs {
		key := fmt.Sprintf("outputs/%s/clip_%04d.mp4", job.ID, s.Index)
		size, err := h.files.Upload(ctx, key, s.Path, "video/mp4")
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

	if err := h.db.Complete(ctx, job.ID, total, clips); err != nil {
		return err
	}
	_ = queue.PublishJobEvent(ctx, h.rdb, queue.JobEvent{
		ID:     job.ID,
		Status: store.StatusCompleted,
		Stage:  "completed",
		Clips:  clips,
	})
	return nil
}

func (h *hybridWorker) processAudio(ctx context.Context, job store.Job, src, dir string) error {
	mp3Path, durationMS, err := media.ExtractAudio(ctx, src, dir)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("outputs/%s/clip_0000.mp3", job.ID)
	size, err := h.files.Upload(ctx, key, mp3Path, "audio/mpeg")
	if err != nil {
		return fmt.Errorf("upload audio: %w", err)
	}

	clips := []store.Clip{{Index: 0, ObjectKey: key, DurationMS: durationMS, SizeBytes: size}}
	if err := h.db.Complete(ctx, job.ID, durationMS, clips); err != nil {
		return err
	}
	_ = queue.PublishJobEvent(ctx, h.rdb, queue.JobEvent{
		ID:     job.ID,
		Status: store.StatusCompleted,
		Stage:  "completed",
		Clips:  clips,
	})
	return nil
}

func (h *hybridWorker) acquire(ctx context.Context, job store.Job, dir string, log *slog.Logger) (string, error) {
	if job.SourceKey != nil && *job.SourceKey != "" {
		dest := filepath.Join(dir, "source"+filepath.Ext(*job.SourceKey))
		if err := h.files.Download(ctx, *job.SourceKey, dest); err != nil {
			return "", fmt.Errorf("download source: %w", err)
		}
		return dest, nil
	}

	// Fallback if source was remote and not downloaded in separate step
	if job.SourceType == store.SourceRemote {
		if job.SourceURL == nil {
			return "", fmt.Errorf("remote job with no url")
		}
		info, err := h.res.Resolve(ctx, *job.SourceURL)
		if err != nil {
			return "", err
		}
		if info.Title != "" {
			_ = h.db.SetTitle(ctx, job.ID, info.Title)
		}

		selector := ""
		if job.Format != nil {
			selector = *job.Format
		}
		dest := filepath.Join(dir, "source.%(ext)s")
		if err := h.res.Fetch(ctx, *job.SourceURL, selector, job.AudioOnly, dest); err != nil {
			return "", err
		}

		matches, err := filepath.Glob(filepath.Join(dir, "source.*"))
		if err != nil || len(matches) == 0 {
			return "", fmt.Errorf("yt-dlp reported success but produced no file")
		}
		return matches[0], nil
	}

	return "", fmt.Errorf("job has neither source_key nor source_url")
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

type asynqLogger struct{ l *slog.Logger }

func (a asynqLogger) Debug(args ...any) { a.l.Debug(fmt.Sprint(args...)) }
func (a asynqLogger) Info(args ...any)  { a.l.Info(fmt.Sprint(args...)) }
func (a asynqLogger) Warn(args ...any)  { a.l.Warn(fmt.Sprint(args...)) }
func (a asynqLogger) Error(args ...any) { a.l.Error(fmt.Sprint(args...)) }
func (a asynqLogger) Fatal(args ...any) { a.l.Error(fmt.Sprint(args...)); os.Exit(1) }
