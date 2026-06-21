// Command migrate applies/rolls back database migrations using golang-migrate
// as a library (pgx/v5 driver), avoiding the CLI's build-tag friction.
//
// Usage:
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate down 1
//	go run ./cmd/migrate version
//	go run ./cmd/migrate force <version>
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate <up|down|version|force> [arg]")
	}

	cfg, err := config.Load("../.env", ".env")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	m, err := migrate.New("file://migrations", pgxURL(cfg.DatabaseURL))
	if err != nil {
		log.Fatalf("init migrate: %v", err)
	}
	defer func() { _, _ = m.Close() }()

	cmd := os.Args[1]
	switch cmd {
	case "up":
		run(m.Up)
	case "down":
		steps := 1
		if len(os.Args) > 2 {
			steps = atoi(os.Args[2])
		}
		run(func() error { return m.Steps(-steps) })
	case "version":
		v, dirty, verr := m.Version()
		if errors.Is(verr, migrate.ErrNilVersion) {
			fmt.Println("no migrations applied yet")
			return
		}
		if verr != nil {
			log.Fatalf("version: %v", verr)
		}
		fmt.Printf("version=%d dirty=%t\n", v, dirty)
	case "force":
		if len(os.Args) < 3 {
			log.Fatal("usage: migrate force <version>")
		}
		if err := m.Force(atoi(os.Args[2])); err != nil {
			log.Fatalf("force: %v", err)
		}
		fmt.Println("forced version", os.Args[2])
	default:
		log.Fatalf("unknown command %q", cmd)
	}
}

// run executes a migrate action and treats ErrNoChange as success.
func run(fn func() error) {
	if err := fn(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migrate: %v", err)
	}
	fmt.Println("migrate: ok")
}

// pgxURL converts a postgres:// DSN to the pgx5:// scheme that the
// golang-migrate pgx/v5 driver registers under.
func pgxURL(dsn string) string {
	for _, p := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(dsn, p) {
			return "pgx5://" + strings.TrimPrefix(dsn, p)
		}
	}
	return dsn
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		log.Fatalf("invalid number %q: %v", s, err)
	}
	return n
}
