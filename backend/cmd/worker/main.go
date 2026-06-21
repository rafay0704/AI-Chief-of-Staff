// Command worker runs the background job processor: a bounded goroutine worker
// pool consuming jobs from the durable Redis-list queue.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/config"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/platform/logger"
	redisplatform "github.com/rafay0704/ai-chief-of-staff/backend/internal/platform/redis"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/queue"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/worker"
)

func main() {
	cfg, err := config.Load("../.env", ".env")
	log := logger.New(cfg.LogLevel, cfg.AppEnv)
	if err != nil {
		log.Error("load config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rdb, err := redisplatform.New(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Error("connect redis", "err", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	q := queue.New(rdb, "acos")
	pool := worker.NewPool(q, cfg.WorkerConcurrency, log)
	pool.Register(worker.JobTypeDemo, worker.DemoHandler(log))

	// Blocks until SIGINT/SIGTERM, then drains gracefully.
	if err := pool.Run(ctx); err != nil {
		log.Error("worker pool", "err", err)
		os.Exit(1)
	}
}
