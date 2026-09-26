package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mitsuoleo/encurta/internal/auth"
	"github.com/mitsuoleo/encurta/internal/config"
	"github.com/mitsuoleo/encurta/internal/handler"
	mw "github.com/mitsuoleo/encurta/internal/middleware"
	"github.com/mitsuoleo/encurta/internal/migrate"
	"github.com/mitsuoleo/encurta/internal/repository/postgres"
	rediscache "github.com/mitsuoleo/encurta/internal/repository/redis"
	"github.com/mitsuoleo/encurta/internal/service"
	"github.com/mitsuoleo/encurta/internal/worker"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := migrate.Up(cfg.MigrationsPath, cfg.DatabaseURL); err != nil {
		log.Error("migrations", "err", err)
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := postgres.WaitReady(ctx, pool); err != nil {
		log.Error("postgres ready", "err", err)
		os.Exit(1)
	}

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Error("redis url", "err", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(opt)
	defer func() { _ = rdb.Close() }()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Error("redis ping", "err", err)
		os.Exit(1)
	}

	store := postgres.New(pool)
	cache := rediscache.New(rdb, cfg.CacheTTL)
	if err := cache.EnsureClickGroup(ctx); err != nil {
		log.Error("click stream group", "err", err)
		os.Exit(1)
	}

	svc := service.New(store, cache, cfg.PublicBaseURL)
	tokens := auth.NewTokens(cfg.JWTSecret, 24*time.Hour)
	authSvc := service.NewAuth(store, tokens)

	go worker.ConsumeStream(ctx, cache, store, cfg.ClickConsumer, log)

	limiter := redisLimiter{
		cache:         cache,
		createLimit:   cfg.RateLimitCreatePerHour,
		authLimit:     cfg.RateLimitAuthPerMinute,
		redirectLimit: cfg.RateLimitRedirectPerMin,
		trusted:       cfg.TrustedProxies,
	}
	api := handler.New(
		svc,
		authSvc,
		worker.Publisher(cache.PublishClick),
		cfg.IPHashSalt,
		log,
		func() error { return store.Ping(context.Background()) },
		func() error { return cache.Ping(context.Background()) },
		limiter,
		cfg.MetricsToken,
		cfg.TrustedProxies,
	)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Router(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

type redisLimiter struct {
	cache         *rediscache.Cache
	createLimit   int
	authLimit     int
	redirectLimit int
	trusted       []*net.IPNet
}

func (l redisLimiter) allow(r *http.Request, prefix string, limit int, ttl time.Duration) (bool, error) {
	key := prefix + "ip:" + mw.ClientIP(r, l.trusted)
	if uid, ok := mw.UserID(r.Context()); ok {
		key = fmt.Sprintf("%su:%d", prefix, uid)
	}
	return l.cache.Allow(r.Context(), key, limit, ttl)
}

func (l redisLimiter) AllowCreate(r *http.Request) (bool, error) {
	return l.allow(r, "rl:create:", l.createLimit, time.Hour)
}

func (l redisLimiter) AllowAuth(r *http.Request) (bool, error) {
	key := "rl:auth:ip:" + mw.ClientIP(r, l.trusted)
	return l.cache.Allow(r.Context(), key, l.authLimit, time.Minute)
}

func (l redisLimiter) AllowRedirect(r *http.Request) (bool, error) {
	key := "rl:redir:ip:" + mw.ClientIP(r, l.trusted)
	return l.cache.Allow(r.Context(), key, l.redirectLimit, time.Minute)
}
