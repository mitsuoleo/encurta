package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	devJWT  = "dev-jwt-change-me"
	devSalt = "dev-only-change-me"
	minJWT  = 32
)

type Config struct {
	Env                     string
	HTTPAddr                string
	DatabaseURL             string
	RedisURL                string
	PublicBaseURL           string
	IPHashSalt              string
	MigrationsPath          string
	RateLimitCreatePerHour  int
	RateLimitAuthPerMinute  int
	RateLimitRedirectPerMin int
	CacheTTL                time.Duration
	JWTSecret               string
	MetricsToken            string
	ClickConsumer           string
	TrustedProxies          []*net.IPNet
}

func Load() (Config, error) {
	cfg := Config{
		Env:                     env("ENV", "development"),
		HTTPAddr:                env("HTTP_ADDR", ":8080"),
		DatabaseURL:             env("DATABASE_URL", "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable"),
		RedisURL:                env("REDIS_URL", "redis://localhost:6379/0"),
		PublicBaseURL:           env("PUBLIC_BASE_URL", "http://localhost:8080"),
		IPHashSalt:              env("IP_HASH_SALT", devSalt),
		MigrationsPath:          env("MIGRATIONS_PATH", "file://migrations"),
		RateLimitCreatePerHour:  envInt("RATE_LIMIT_CREATE_PER_HOUR", 100),
		RateLimitAuthPerMinute:  envInt("RATE_LIMIT_AUTH_PER_MINUTE", 20),
		RateLimitRedirectPerMin: envInt("RATE_LIMIT_REDIRECT_PER_MINUTE", 300),
		CacheTTL:                envDuration("CACHE_TTL", 24*time.Hour),
		JWTSecret:               env("JWT_SECRET", devJWT),
		MetricsToken:            env("METRICS_TOKEN", ""),
		ClickConsumer:           env("CLICK_CONSUMER", ""),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.ClickConsumer == "" {
		cfg.ClickConsumer = defaultClickConsumer()
	}
	nets, err := parseCIDRs(env("TRUSTED_PROXIES", ""))
	if err != nil {
		return Config{}, err
	}
	cfg.TrustedProxies = nets
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func defaultClickConsumer() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		h = "api"
	}
	return "click-worker-" + h
}

func (c Config) Production() bool {
	return strings.EqualFold(c.Env, "production")
}

func (c Config) validate() error {
	for _, limit := range []struct {
		name  string
		value int
	}{
		{"RATE_LIMIT_CREATE_PER_HOUR", c.RateLimitCreatePerHour},
		{"RATE_LIMIT_AUTH_PER_MINUTE", c.RateLimitAuthPerMinute},
		{"RATE_LIMIT_REDIRECT_PER_MINUTE", c.RateLimitRedirectPerMin},
	} {
		if limit.value <= 0 {
			return fmt.Errorf("%s must be positive", limit.name)
		}
	}
	if !c.Production() {
		return nil
	}
	if c.JWTSecret == "" || c.JWTSecret == devJWT {
		return fmt.Errorf("JWT_SECRET must be set to a non-default value in production")
	}
	if len(c.JWTSecret) < minJWT {
		return fmt.Errorf("JWT_SECRET must be at least %d characters in production", minJWT)
	}
	if c.IPHashSalt == "" || c.IPHashSalt == devSalt {
		return fmt.Errorf("IP_HASH_SALT must be set to a non-default value in production")
	}
	if c.MetricsToken == "" {
		return fmt.Errorf("METRICS_TOKEN is required in production")
	}
	base := strings.ToLower(strings.TrimSpace(c.PublicBaseURL))
	if !strings.HasPrefix(base, "https://") {
		return fmt.Errorf("PUBLIC_BASE_URL must use https in production")
	}
	if strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1") {
		return fmt.Errorf("PUBLIC_BASE_URL must not be localhost in production")
	}
	return nil
}

func parseCIDRs(v string) ([]*net.IPNet, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	var out []*net.IPNet
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.Contains(p, "/") {
			ip := net.ParseIP(p)
			if ip == nil {
				return nil, fmt.Errorf("TRUSTED_PROXIES: invalid address %q", p)
			}
			if ip.To4() != nil {
				p = ip.String() + "/32"
			} else {
				p = ip.String() + "/128"
			}
		}
		_, n, err := net.ParseCIDR(p)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES: %w", err)
		}
		out = append(out, n)
	}
	return out, nil
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
