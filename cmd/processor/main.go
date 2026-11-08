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

	redisOpt, err := queue.ParseRedisOpt(cfg.RedisAddr)
	if err != nil {
		log.Error("invalid redis addr", "err", err)
		os.Exit(1)
	}

	rdb, err := queue.NewRedisClient(cfg.RedisAddr)
	if err != nil {
		log.Error("redis client failed", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	p := &processor{cfg: cfg, db: db, files: files, rdb: rdb, log: log}

	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeProcess, p.handleProcess)
	mux.HandleFunc(queue.TypeSplit, p.handleProcess) // Alias

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: cfg.WorkerConc,
			Queues: map[string]int{
				queue.QueueProcess: 2,
				"default":          1,
			},
			Logger: asynqLogger{log},
		},
	)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("processor worker starting", "concurrency", cfg.WorkerConc)
		if err := srv.Run(mux); err != nil && !errors.Is(err, asynq.ErrServerClosed) {
			log.Error("processor worker failed", "err", err)
			os.Exit(1)
		}
	}()

	<-stop
	srv.Shutdown()
	log.Info("processor worker stopped")
}

type processor struct {
	cfg   config.Config
	db    *store.Store
	files *storage.Store
	rdb   *redis.Client
	log   *slog.Logger
}

func (p *processor) handleProcess(ctx context.Context, t *asynq.Task) error {
	payload, err := queue.ParsePayload(t)
	if err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	job, err := p.db.GetJob(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}
	log := p.log.With("job", job.ID)
	log.Info("job received from processing queue", "audio_only", job.AudioOnly, "aspect", job.Aspect, "segment_seconds", job.SegmentSeconds)

	if job.Status == store.StatusFailed || job.Status == store.StatusCompleted {
		log.Info("job is already in terminal state; skipping", "status", job.Status)
		return nil
	}

	if err := p.db.SetStatus(ctx, job.ID, store.StatusProcessing, nil, nil); err != nil {
		return err
	}
	log.Info("status updated to processing")
	_ = queue.PublishJobEvent(ctx, p.rdb, queue.JobEvent{
		ID:     job.ID,
		Status: store.StatusProcessing,
		Stage:  "processing",
	})

	if err := p.process(ctx, job, log); err != nil {
		msg := err.Error()
		code := "internal"
		log.Error("job processing failed", "err", msg)
		if serr := p.db.SetStatus(ctx, job.ID, store.StatusFailed, &msg, &code); serr != nil {
			log.Error("marking failed failed", "err", serr)
		}
		_ = queue.PublishJobEvent(ctx, p.rdb, queue.JobEvent{
			ID:        job.ID,
			Status:    store.StatusFailed,
			Stage:     "failed",
			Error:     msg,
			ErrorCode: code,
		})
		return err
	}
	log.Info("job completed successfully")
	return nil
}

func (p *processor) process(ctx context.Context, job store.Job, log *slog.Logger) error {
	dir := filepath.Join(p.cfg.WorkDir, "process_"+job.ID.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	if job.SourceKey == nil || *job.SourceKey == "" {
		return fmt.Errorf("job has no source_key in storage")
	}

	ext := filepath.Ext(*job.SourceKey)
	if ext == "" {
		ext = ".mp4"
	}
	src := filepath.Join(dir, "source"+ext)

	log.Info("downloading source from storage", "key", *job.SourceKey)
	if err := p.files.Download(ctx, *job.SourceKey, src); err != nil {
		return fmt.Errorf("download source: %w", err)
	}

	// The size was checked when the job was created, but the presigned upload
	// URL stays valid for a while and can be used again to replace the file.
	// Check what was actually downloaded. Fetched sources are held to a
	// separate cap when they are downloaded.
	if job.SourceType == store.SourceUpload && p.cfg.MaxUploadBytes > 0 {
		if st, err := os.Stat(src); err != nil || st.Size() > p.cfg.MaxUploadBytes {
			return fmt.Errorf("file exceeds maximum upload size")
		}
	}

	if job.AudioOnly {
		return p.processAudio(ctx, job, src, dir)
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

	mode := job.Mode
	if job.Aspect != media.AspectOriginal && media.ValidAspect(job.Aspect) {
		log.Info("reframing", "aspect", job.Aspect)
		reframed, err := media.Reframe(ctx, src, dir, job.Aspect, job.SegmentSeconds)
		if err != nil {
			return err
		}
		src = reframed
		// Reframing already put a keyframe on every boundary, so a copy is exact.
		mode = media.ModeCopy
	}

	var segs []media.Segment
	if job.SegmentSeconds == 0 {
		segs = []media.Segment{{Index: 0, Path: src, DurationMS: total}}
	} else {
		segs, err = media.Split(ctx, src, filepath.Join(dir, "out"), job.SegmentSeconds, mode)
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

	if err := p.db.Complete(ctx, job.ID, total, clips); err != nil {
		return err
	}
	_ = queue.PublishJobEvent(ctx, p.rdb, queue.JobEvent{
		ID:     job.ID,
		Status: store.StatusCompleted,
		Stage:  "completed",
		Clips:  clips,
	})
	return nil
}

func (p *processor) processAudio(ctx context.Context, job store.Job, src, dir string) error {
	mp3Path, durationMS, err := media.ExtractAudio(ctx, src, dir)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("outputs/%s/clip_0000.mp3", job.ID)
	size, err := p.files.Upload(ctx, key, mp3Path, "audio/mpeg")
	if err != nil {
		return fmt.Errorf("upload audio: %w", err)
	}

	clips := []store.Clip{{Index: 0, ObjectKey: key, DurationMS: durationMS, SizeBytes: size}}
	if err := p.db.Complete(ctx, job.ID, durationMS, clips); err != nil {
		return err
	}
	_ = queue.PublishJobEvent(ctx, p.rdb, queue.JobEvent{
		ID:     job.ID,
		Status: store.StatusCompleted,
		Stage:  "completed",
		Clips:  clips,
	})
	return nil
}

type asynqLogger struct{ l *slog.Logger }

func (a asynqLogger) Debug(args ...any) { a.l.Debug(fmt.Sprint(args...)) }
func (a asynqLogger) Info(args ...any)  { a.l.Info(fmt.Sprint(args...)) }
func (a asynqLogger) Warn(args ...any)  { a.l.Warn(fmt.Sprint(args...)) }
func (a asynqLogger) Error(args ...any) { a.l.Error(fmt.Sprint(args...)) }
func (a asynqLogger) Fatal(args ...any) { a.l.Error(fmt.Sprint(args...)); os.Exit(1) }
