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
	res.MaxFilesize = cfg.MaxDownloadBytes
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

	d := &downloader{
		cfg:     cfg,
		db:      db,
		files:   files,
		res:     res,
		qClient: qClient,
		rdb:     rdb,
		log:     log,
	}

	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeDownload, d.handleDownload)
	mux.HandleFunc(queue.TypeSplit, d.handleDownload)   // Backwards compatibility for old API sending video:split
	mux.HandleFunc(queue.TypeProcess, d.handleDownload) // Backwards compatibility

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: cfg.WorkerConc,
			Queues: map[string]int{
				queue.QueueDownload: 2,
				"default":           1,
			},
			Logger: asynqLogger{log},
		},
	)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("downloader worker starting", "concurrency", cfg.WorkerConc)
		if err := srv.Run(mux); err != nil && !errors.Is(err, asynq.ErrServerClosed) {
			log.Error("downloader worker failed", "err", err)
			os.Exit(1)
		}
	}()

	<-stop
	srv.Shutdown()
	log.Info("downloader worker stopped")
}

type downloader struct {
	cfg     config.Config
	db      *store.Store
	files   *storage.Store
	res     *resolver.Resolver
	qClient *asynq.Client
	rdb     *redis.Client
	log     *slog.Logger
}

func (d *downloader) handleDownload(ctx context.Context, t *asynq.Task) error {
	payload, err := queue.ParsePayload(t)
	if err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	job, err := d.db.GetJob(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}
	log := d.log.With("job", job.ID)
	log.Info("job received from queue", "url", *job.SourceURL, "audio_only", job.AudioOnly, "mode", job.Mode)

	if job.SourceURL == nil || *job.SourceURL == "" {
		return fmt.Errorf("%w: remote job with no url", asynq.SkipRetry)
	}

	if job.Status == store.StatusFailed || job.Status == store.StatusCompleted {
		log.Info("job is already in terminal state; skipping", "status", job.Status)
		return nil
	}

	if err := d.db.SetStatus(ctx, job.ID, store.StatusDownloading, nil, nil); err != nil {
		return err
	}
	log.Info("status updated to downloading")
	_ = queue.PublishJobEvent(ctx, d.rdb, queue.JobEvent{
		ID:     job.ID,
		Status: store.StatusDownloading,
		Stage:  "downloading",
	})

	if err := d.downloadAndStore(ctx, job, log); err != nil {
		msg, code := err.Error(), errorCode(err)
		log.Error("download failed", "err", msg, "code", code)
		if serr := d.db.SetStatus(ctx, job.ID, store.StatusFailed, &msg, &code); serr != nil {
			log.Error("marking failed failed", "err", serr)
		}
		_ = queue.PublishJobEvent(ctx, d.rdb, queue.JobEvent{
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

	// Enqueue processing task into the processing queue
	task, err := queue.NewProcessTask(job.ID)
	if err != nil {
		return err
	}
	if _, err := d.qClient.EnqueueContext(ctx, task,
		asynq.Queue(queue.QueueProcess),
		asynq.MaxRetry(2),
		asynq.Timeout(d.cfg.JobTimeout),
		asynq.Retention(24*time.Hour),
	); err != nil {
		log.Error("failed to enqueue process task", "err", err)
		return err
	}

	log.Info("download completed and uploaded to R2; enqueued to processing queue", "next_queue", queue.QueueProcess)
	_ = queue.PublishJobEvent(ctx, d.rdb, queue.JobEvent{
		ID:     job.ID,
		Status: store.StatusProcessing,
		Stage:  "processing",
	})
	return nil
}

func (d *downloader) downloadAndStore(ctx context.Context, job store.Job, log *slog.Logger) error {
	dir := filepath.Join(d.cfg.WorkDir, "download_"+job.ID.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	log.Info("resolving video metadata with yt-dlp", "url", *job.SourceURL)
	info, resolveErr := d.res.Resolve(ctx, *job.SourceURL)
	// The API checks length when a link is looked up, but a job can be created
	// without that step, so the limit is enforced here too, before any bytes move.
	if resolveErr == nil && d.cfg.MaxDurationSeconds > 0 && info.Duration > float64(d.cfg.MaxDurationSeconds) {
		return resolver.ErrTooLarge
	}
	if resolveErr == nil && info.Title != "" {
		if err := d.db.SetTitle(ctx, job.ID, info.Title); err != nil {
			log.Warn("could not store title", "err", err)
		}
		log.Info("video title extracted", "title", info.Title)
		_ = queue.PublishJobEvent(ctx, d.rdb, queue.JobEvent{
			ID:     job.ID,
			Status: store.StatusDownloading,
			Stage:  "downloading",
			Title:  info.Title,
		})
	} else if resolveErr != nil {
		log.Warn("metadata resolve skipped/failed; proceeding to direct fetch", "err", resolveErr)
	}

	selector := ""
	if job.Format != nil {
		selector = *job.Format
	}
	dest := filepath.Join(dir, "source.%(ext)s")
	log.Info("fetching media stream with yt-dlp", "selector", selector, "audio_only", job.AudioOnly)
	if err := d.res.Fetch(ctx, *job.SourceURL, selector, job.AudioOnly, dest); err != nil {
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

	var size int64
	if st, err := os.Stat(localFile); err == nil {
		size = st.Size()
	}
	log.Info("media stream downloaded successfully", "file", filepath.Base(localFile), "size_bytes", size)

	sourceKey := fmt.Sprintf("sources/%s%s", job.ID, ext)
	contentType := "video/mp4"
	if job.AudioOnly {
		contentType = "audio/mpeg"
	}

	log.Info("uploading source file to R2 storage", "key", sourceKey)
	uploadedBytes, err := d.files.Upload(ctx, sourceKey, localFile, contentType)
	if err != nil {
		return fmt.Errorf("upload source to storage: %w", err)
	}
	log.Info("source uploaded to R2 storage successfully", "key", sourceKey, "bytes", uploadedBytes)

	if err := d.db.SetSourceKey(ctx, job.ID, sourceKey); err != nil {
		return fmt.Errorf("update job source key: %w", err)
	}

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
	case errors.Is(err, resolver.ErrTooLarge):
		return "too_large"
	default:
		return "internal"
	}
}

func terminal(err error) bool {
	return errors.Is(err, resolver.ErrUnavailable) ||
		errors.Is(err, resolver.ErrUnsupported) ||
		errors.Is(err, resolver.ErrLiveStream) ||
		errors.Is(err, resolver.ErrTooLarge) ||
		errors.Is(err, resolver.ErrExtractorStale)
}

type asynqLogger struct{ l *slog.Logger }

func (a asynqLogger) Debug(args ...any) { a.l.Debug(fmt.Sprint(args...)) }
func (a asynqLogger) Info(args ...any)  { a.l.Info(fmt.Sprint(args...)) }
func (a asynqLogger) Warn(args ...any)  { a.l.Warn(fmt.Sprint(args...)) }
func (a asynqLogger) Error(args ...any) { a.l.Error(fmt.Sprint(args...)) }
func (a asynqLogger) Fatal(args ...any) { a.l.Error(fmt.Sprint(args...)); os.Exit(1) }
