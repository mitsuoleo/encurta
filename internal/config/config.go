package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr               string
	DatabaseURL            string
	RedisURL               string
	PublicBaseURL          string
	IPHashSalt             string
	MigrationsPath         string
	RateLimitCreatePerHour int
	CacheTTL               time.Duration
	JWTSecret              string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:               env("HTTP_ADDR", ":8080"),
		DatabaseURL:            env("DATABASE_URL", "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable"),
		RedisURL:               env("REDIS_URL", "redis://localhost:6379/0"),
		PublicBaseURL:          env("PUBLIC_BASE_URL", "http://localhost:8080"),
		IPHashSalt:             env("IP_HASH_SALT", "dev-only-change-me"),
		MigrationsPath:         env("MIGRATIONS_PATH", "file://migrations"),
		RateLimitCreatePerHour: envInt("RATE_LIMIT_CREATE_PER_HOUR", 100),
		CacheTTL:               envDuration("CACHE_TTL", 24*time.Hour),
		JWTSecret:              env("JWT_SECRET", "dev-jwt-change-me"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
