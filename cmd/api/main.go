package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // QUOTA_TZ works even on images without a system timezone database

	"github.com/hibiken/asynq"

	"github.com/ammyy9908/clipper/internal/api"
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

	days := int32(cfg.OutputTTL.Hours() / 24)
	if days < 1 {
		days = 1
	}
	if err := files.EnsureLifecycle(ctx, days); err != nil {
		log.Warn("r2 lifecycle config failed", "err", err)
	} else {
		log.Info("r2 lifecycle configured", "expire_days", days)
	}

	res := resolver.New(cfg.YtdlpBin, cfg.Proxies, cfg.CookiesFile)
	res.PlaylistMax = cfg.PlaylistMaxItems
	res.MaxConcurrent = cfg.ResolveConcurrency

	redisOpt, err := queue.ParseRedisOpt(cfg.RedisAddr)
	if err != nil {
		log.Error("invalid redis addr", "err", err)
		os.Exit(1)
	}

	q := asynq.NewClient(redisOpt)
	defer q.Close()

	rdb, err := queue.NewRedisClient(cfg.RedisAddr)
	if err != nil {
		log.Error("redis client failed", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.New(cfg, db, q, rdb, files, res, log).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("api listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown failed", "err", err)
	}
	log.Info("api stopped")
}
