// Package config loads and validates application configuration from the
// environment into a typed struct. A .env file at the repo root is loaded in
// development for convenience; in production, real environment variables win.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the fully-typed application configuration.
type Config struct {
	AppEnv   string `env:"APP_ENV" envDefault:"development"`
	HTTPPort string `env:"HTTP_PORT" envDefault:"8080"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	DatabaseURL string `env:"DATABASE_URL,required"`

	RedisAddr     string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	RedisPassword string `env:"REDIS_PASSWORD"`
	RedisDB       int    `env:"REDIS_DB" envDefault:"0"`

	JWTSecret   string        `env:"JWT_SECRET,required"`
	JWTTTL      time.Duration `env:"-"`
	JWTTTLHours int           `env:"JWT_TTL_HOURS" envDefault:"24"`

	AnthropicAPIKey string `env:"ANTHROPIC_API_KEY"`
	AnthropicModel  string `env:"ANTHROPIC_MODEL" envDefault:"claude-opus-4-8"`

	WorkerConcurrency int `env:"WORKER_CONCURRENCY" envDefault:"4"`
}

// IsProduction reports whether the app is running in production mode.
func (c Config) IsProduction() bool { return c.AppEnv == "production" }

// Load reads configuration from the environment. It best-effort loads a .env
// file from the given paths first (ignored if absent), then parses env vars.
func Load(dotenvPaths ...string) (Config, error) {
	// Best-effort: a missing .env is fine (e.g. in production / CI).
	_ = godotenv.Load(dotenvPaths...)

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	cfg.JWTTTL = time.Duration(cfg.JWTTTLHours) * time.Hour
	return cfg, nil
}
