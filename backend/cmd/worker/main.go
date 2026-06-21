// Command worker runs the background job processor: a bounded goroutine worker
// pool consuming jobs from the durable Redis-list queue.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/ai"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/config"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/platform/db"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/platform/logger"
	redisplatform "github.com/rafay0704/ai-chief-of-staff/backend/internal/platform/redis"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/queue"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/service"
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

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := redisplatform.New(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Error("connect redis", "err", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	repo := repository.New(pool)
	q := queue.New(rdb, "acos")
	plans := service.NewPlanService(repo, q)
	tasks := service.NewTaskService(repo)

	wp := worker.NewPool(q, cfg.WorkerConcurrency, log)
	wp.Register(worker.JobTypeDemo, worker.DemoHandler(log))

	// Register the plan handler only if Claude is configured; otherwise plan
	// jobs are marked failed rather than dead-lettered silently.
	if cfg.AnthropicAPIKey != "" {
		client, err := ai.NewClient(cfg.AnthropicAPIKey, cfg.AnthropicModel)
		if err != nil {
			log.Error("init claude client", "err", err)
			os.Exit(1)
		}
		wp.Register(service.JobTypePlan, worker.PlanHandler(plans, tasks, ai.NewAgents(client), log))
		log.Info("plan handler registered", "model", cfg.AnthropicModel)
	} else {
		wp.Register(service.JobTypePlan, worker.PlanUnavailableHandler(plans, log))
		log.Warn("ANTHROPIC_API_KEY not set; plan jobs will be marked failed")
	}

	if err := wp.Run(ctx); err != nil {
		log.Error("worker pool", "err", err)
		os.Exit(1)
	}
}
