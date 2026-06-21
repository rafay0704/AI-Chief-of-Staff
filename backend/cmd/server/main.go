// Command server runs the AI Chief of Staff HTTP API.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/auth"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/config"
	apihttp "github.com/rafay0704/ai-chief-of-staff/backend/internal/http"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/http/handler"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/platform/db"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/platform/logger"
	redisplatform "github.com/rafay0704/ai-chief-of-staff/backend/internal/platform/redis"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/queue"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/service"
)

func main() {
	cfg, err := config.Load("../.env", ".env")
	log := logger.New(envOr(cfg.LogLevel, "info"), cfg.AppEnv)
	if err != nil {
		log.Error("load config", "err", err)
		os.Exit(1)
	}

	// Root context cancelled on SIGINT/SIGTERM.
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
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)
	jobQueue := queue.New(rdb, "acos")

	h := &handler.Handler{
		Auth:  service.NewAuthService(repo, tokens),
		Tasks: service.NewTaskService(repo),
		Plans: service.NewPlanService(repo, jobQueue),
		DB:    pool,
		Redis: rdb,
		Log:   log,
	}

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           apihttp.Router(h, tokens, log, cfg.IsProduction()),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Run server until the context is cancelled.
	go func() {
		log.Info("server listening", "port", cfg.HTTPPort, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
	}
	log.Info("stopped")
}

func envOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
