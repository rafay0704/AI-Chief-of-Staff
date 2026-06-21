// Command enqueue is a dev tool that pushes demo jobs onto the queue so you can
// watch the worker pool process them concurrently.
//
// Usage:
//
//	go run ./cmd/enqueue -n 10 -sleep 500
//	go run ./cmd/enqueue -n 3 -fail        # exercise retry + dead-letter
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/config"
	redisplatform "github.com/rafay0704/ai-chief-of-staff/backend/internal/platform/redis"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/queue"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/worker"
)

func main() {
	n := flag.Int("n", 5, "number of demo jobs to enqueue")
	sleepMs := flag.Int("sleep", 300, "simulated work per job in milliseconds")
	fail := flag.Bool("fail", false, "make jobs fail (to exercise retry/dead-letter)")
	flag.Parse()

	cfg, err := config.Load("../.env", ".env")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	rdb, err := redisplatform.New(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer func() { _ = rdb.Close() }()

	q := queue.New(rdb, "acos")
	for i := 0; i < *n; i++ {
		job, err := queue.NewJob(worker.JobTypeDemo, worker.DemoPayload{
			Message: fmt.Sprintf("hello #%d", i+1),
			SleepMs: *sleepMs,
			Fail:    *fail,
		})
		if err != nil {
			log.Fatalf("build job: %v", err)
		}
		if err := q.Enqueue(ctx, job); err != nil {
			log.Fatalf("enqueue: %v", err)
		}
	}

	fmt.Fprintf(os.Stdout, "enqueued %d demo job(s) (sleep=%dms fail=%t)\n", *n, *sleepMs, *fail)
}
