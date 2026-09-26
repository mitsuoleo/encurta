package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mitsuoleo/encurta/internal/domain"
	"github.com/mitsuoleo/encurta/internal/migrate"
	"github.com/mitsuoleo/encurta/internal/repository/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestIntegrationTestDSNGuard(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/production?sslmode=disable")
	for _, tc := range []struct {
		name     string
		dsn      string
		isolated string
		valid    bool
	}{
		{"no test dsn", "", "1", false},
		{"no opt in", "postgres://user:pass@localhost:5432/shortener_test?sslmode=disable", "", false},
		{"production database", "postgres://user:pass@localhost:5432/production?sslmode=disable", "1", false},
		{"isolated test database", "postgres://user:pass@localhost:5432/shortener_test?sslmode=disable", "1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEST_DATABASE_URL", tc.dsn)
			t.Setenv("TEST_DATABASE_ISOLATED", tc.isolated)
			_, err := integrationTestDSN()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestInsertClickAndAnalytics(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" && os.Getenv("TEST_DATABASE_ISOLATED") == "" {
		t.Skip("set TEST_DATABASE_URL and TEST_DATABASE_ISOLATED=1 for isolated integration test")
	}
	dsn, err := integrationTestDSN()
	require.NoError(t, err)
	path := os.Getenv("MIGRATIONS_PATH")
	if path == "" {
		path = "file://../../../migrations"
	}
	require.NoError(t, migrate.Up(path, dsn))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, postgres.WaitReady(ctx, pool))

	store := postgres.New(pool)
	user, err := store.CreateUser(ctx, "itest+"+time.Now().UTC().Format("150405.000")+"@example.com", "hashhashhashhash")
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := pool.Exec(cleanupCtx, "DELETE FROM users WHERE id = $1", user.ID)
		require.NoError(t, err)
	})
	owner := user.ID
	link, err := store.InsertLink(ctx, domain.Link{
		ShortCode:   "it" + time.Now().UTC().Format("150405"),
		OriginalURL: "https://example.com/itest",
		OwnerID:     &owner,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := pool.Exec(cleanupCtx, "DELETE FROM links WHERE id = $1", link.ID)
		require.NoError(t, err)
	})

	err = store.InsertClick(ctx, domain.ClickEvent{
		LinkID:    link.ID,
		ClickedAt: time.Now().UTC(),
		IPHash:    "abc",
		UserAgent: "Mozilla/5.0 Chrome/120.0",
		Referer:   "",
		Device:    "desktop",
		Browser:   "chrome",
	})
	require.NoError(t, err)

	stats, err := store.Analytics(ctx, link.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.TotalClicks)
	require.Equal(t, int64(1), stats.UniqueVisitors)
	require.NotEmpty(t, stats.DeviceBreakdown)
}

func TestCachePopulationAndUpdateSerializeOnPostgresRow(t *testing.T) {
	store, link := setupCacheTestLink(t)
	ctx := context.Background()
	entered := make(chan struct{})
	release := make(chan struct{})
	readDone := make(chan error, 1)
	go func() {
		_, err := store.GetByCodeForCache(ctx, link.ShortCode, func(got domain.Link) error {
			if got.OriginalURL != link.OriginalURL {
				return fmt.Errorf("unexpected initial URL: %s", got.OriginalURL)
			}
			close(entered)
			<-release
			return nil
		})
		readDone <- err
	}()
	<-entered

	newURL := "https://example.com/updated"
	blockedCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	invalidated := false
	_, err := store.UpdateLink(blockedCtx, link.ShortCode, *link.OwnerID, &newURL, nil, false, nil, func() error {
		invalidated = true
		return nil
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, invalidated, "a writer must wait for cache population before invalidation")
	close(release)
	require.NoError(t, <-readDone)

	_, err = store.UpdateLink(ctx, link.ShortCode, *link.OwnerID, &newURL, nil, false, nil, func() error {
		invalidated = true
		return nil
	})
	require.NoError(t, err)
	require.True(t, invalidated)
	got, err := store.GetByCode(ctx, link.ShortCode)
	require.NoError(t, err)
	require.Equal(t, newURL, got.OriginalURL)
}

func TestCacheInvalidationFailureRollsBackUpdateAndDeactivate(t *testing.T) {
	store, link := setupCacheTestLink(t)
	ctx := context.Background()
	cacheErr := errors.New("cache unavailable")
	newURL := "https://example.com/updated"
	_, err := store.UpdateLink(ctx, link.ShortCode, *link.OwnerID, &newURL, nil, false, nil, func() error { return cacheErr })
	require.ErrorIs(t, err, cacheErr)
	got, err := store.GetByCode(ctx, link.ShortCode)
	require.NoError(t, err)
	require.Equal(t, link.OriginalURL, got.OriginalURL)

	_, err = store.Deactivate(ctx, link.ShortCode, *link.OwnerID, func() error { return cacheErr })
	require.ErrorIs(t, err, cacheErr)
	got, err = store.GetByCode(ctx, link.ShortCode)
	require.NoError(t, err)
	require.True(t, got.IsActive)
}

func TestConcurrentWritersInvalidateInCommitOrder(t *testing.T) {
	store, link := setupCacheTestLink(t)
	ctx := context.Background()
	firstURL := "https://example.com/first"
	secondURL := "https://example.com/second"
	firstInvalidating := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		_, err := store.UpdateLink(ctx, link.ShortCode, *link.OwnerID, &firstURL, nil, false, nil, func() error {
			close(firstInvalidating)
			<-releaseFirst
			return nil
		})
		firstDone <- err
	}()
	<-firstInvalidating

	blockedCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	secondInvalidated := false
	_, err := store.UpdateLink(blockedCtx, link.ShortCode, *link.OwnerID, &secondURL, nil, false, nil, func() error {
		secondInvalidated = true
		return nil
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, secondInvalidated)
	close(releaseFirst)
	require.NoError(t, <-firstDone)

	_, err = store.UpdateLink(ctx, link.ShortCode, *link.OwnerID, &secondURL, nil, false, nil, func() error {
		secondInvalidated = true
		return nil
	})
	require.NoError(t, err)
	require.True(t, secondInvalidated)
	got, err := store.GetByCode(ctx, link.ShortCode)
	require.NoError(t, err)
	require.Equal(t, secondURL, got.OriginalURL)
}

func setupCacheTestLink(t *testing.T) (*postgres.Store, domain.Link) {
	t.Helper()
	if os.Getenv("TEST_DATABASE_URL") == "" && os.Getenv("TEST_DATABASE_ISOLATED") == "" {
		t.Skip("set TEST_DATABASE_URL and TEST_DATABASE_ISOLATED=1 for isolated integration test")
	}
	dsn, err := integrationTestDSN()
	require.NoError(t, err)
	path := os.Getenv("MIGRATIONS_PATH")
	if path == "" {
		path = "file://../../../migrations"
	}
	require.NoError(t, migrate.Up(path, dsn))
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, postgres.WaitReady(ctx, pool))
	store := postgres.New(pool)
	user, err := store.CreateUser(ctx, fmt.Sprintf("cache-itest-%d@example.com", time.Now().UnixNano()), "hashhashhashhash")
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := pool.Exec(cleanupCtx, "DELETE FROM users WHERE id = $1", user.ID)
		require.NoError(t, err)
	})
	owner := user.ID
	link, err := store.InsertLink(ctx, domain.Link{
		ShortCode:   fmt.Sprintf("it%x", time.Now().UnixNano()),
		OriginalURL: "https://example.com/original",
		OwnerID:     &owner,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := pool.Exec(cleanupCtx, "DELETE FROM links WHERE id = $1", link.ID)
		require.NoError(t, err)
	})
	return store, link
}

func integrationTestDSN() (string, error) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if os.Getenv("TEST_DATABASE_ISOLATED") != "1" || dsn == "" {
		return "", fmt.Errorf("integration test requires TEST_DATABASE_URL and TEST_DATABASE_ISOLATED=1")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return "", fmt.Errorf("invalid TEST_DATABASE_URL: %w", err)
	}
	if !strings.HasSuffix(strings.ToLower(config.ConnConfig.Database), "_test") {
		return "", fmt.Errorf("TEST_DATABASE_URL database name must end in _test")
	}
	return dsn, nil
}
