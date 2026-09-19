package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/esposo/url-shortener/internal/domain"
	"github.com/stretchr/testify/require"
)

type stubLinks struct {
	inserted   domain.Link
	collisions int32
}

func (s *stubLinks) InsertLink(_ context.Context, in domain.Link) (domain.Link, error) {
	if atomic.AddInt32(&s.collisions, -1) >= 0 {
		return domain.Link{}, domain.ErrCodeCollision
	}
	in.ID = 1
	in.IsActive = true
	s.inserted = in
	return s.inserted, nil
}

func (s *stubLinks) GetByCode(_ context.Context, code string) (domain.Link, error) {
	if s.inserted.ShortCode != code {
		return domain.Link{}, domain.ErrNotFound
	}
	return s.inserted, nil
}

func (s *stubLinks) ListByOwner(_ context.Context, ownerID int64) ([]domain.Link, error) {
	if s.inserted.OwnedBy(ownerID) {
		return []domain.Link{s.inserted}, nil
	}
	return []domain.Link{}, nil
}

func (s *stubLinks) Deactivate(_ context.Context, code string, ownerID int64) (domain.Link, error) {
	if s.inserted.ShortCode != code || !s.inserted.OwnedBy(ownerID) {
		return domain.Link{}, domain.ErrNotFound
	}
	s.inserted.IsActive = false
	return s.inserted, nil
}

func (s *stubLinks) Analytics(_ context.Context, _ int64) (domain.Analytics, error) {
	return domain.Analytics{TotalClicks: 3}, nil
}

type mapCache struct {
	m map[string]domain.CachedLink
}

func newMapCache() *mapCache {
	return &mapCache{m: map[string]domain.CachedLink{}}
}

func (c *mapCache) GetLink(_ context.Context, code string) (domain.CachedLink, bool, error) {
	v, ok := c.m[code]
	return v, ok, nil
}

func (c *mapCache) SetLink(_ context.Context, code string, link domain.Link) error {
	c.m[code] = domain.LinkToCached(link)
	return nil
}

func (c *mapCache) DeleteLink(_ context.Context, code string) error {
	delete(c.m, code)
	return nil
}

func TestCreateRejectsBadURL(t *testing.T) {
	svc := New(&stubLinks{}, newMapCache(), "http://localhost:8080")
	_, err := svc.Create(context.Background(), CreateInput{OwnerID: 1, URL: "javascript:alert(1)"})
	require.ErrorIs(t, err, domain.ErrInvalidURL)
}

func TestCreatePersists(t *testing.T) {
	links := &stubLinks{collisions: 1}
	svc := New(links, newMapCache(), "http://localhost:8080")
	got, err := svc.Create(context.Background(), CreateInput{OwnerID: 9, URL: "https://example.com"})
	require.NoError(t, err)
	require.Equal(t, "https://example.com", got.OriginalURL)
	require.Len(t, got.ShortCode, 7)
	require.True(t, got.OwnedBy(9))
}

func TestCreateAlias(t *testing.T) {
	svc := New(&stubLinks{}, newMapCache(), "http://localhost:8080")
	got, err := svc.Create(context.Background(), CreateInput{OwnerID: 1, URL: "https://example.com", Alias: "myname"})
	require.NoError(t, err)
	require.Equal(t, "myname", got.ShortCode)
}

func TestCreateBadAlias(t *testing.T) {
	svc := New(&stubLinks{}, newMapCache(), "http://localhost:8080")
	_, err := svc.Create(context.Background(), CreateInput{OwnerID: 1, URL: "https://example.com", Alias: "ab"})
	require.ErrorIs(t, err, domain.ErrInvalidAlias)
	_, err = svc.Create(context.Background(), CreateInput{OwnerID: 1, URL: "https://example.com", Alias: "auth"})
	require.ErrorIs(t, err, domain.ErrInvalidAlias)
}

func TestResolveCacheHitAndMiss(t *testing.T) {
	links := &stubLinks{}
	cache := newMapCache()
	svc := New(links, cache, "http://localhost:8080/")
	created, err := svc.Create(context.Background(), CreateInput{OwnerID: 1, URL: "https://example.com/x"})
	require.NoError(t, err)

	got, err := svc.Resolve(context.Background(), created.ShortCode)
	require.NoError(t, err)
	require.Equal(t, created.OriginalURL, got.OriginalURL)

	_, err = svc.Resolve(context.Background(), "zzzzzzz")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestResolveInactiveAndExpired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	cache := newMapCache()
	cache.m["deadxxx"] = domain.CachedLink{ID: 2, OriginalURL: "https://example.com", IsActive: false}
	exp := past.Unix()
	cache.m["oldxxxx"] = domain.CachedLink{ID: 3, OriginalURL: "https://example.com", IsActive: true, ExpiresAt: &exp}
	svc := New(&stubLinks{}, cache, "http://localhost:8080")

	_, err := svc.Resolve(context.Background(), "deadxxx")
	require.ErrorIs(t, err, domain.ErrInactive)
	_, err = svc.Resolve(context.Background(), "oldxxxx")
	require.ErrorIs(t, err, domain.ErrExpired)
}

func TestAnalyticsOwner(t *testing.T) {
	owner := int64(1)
	links := &stubLinks{inserted: domain.Link{ID: 1, ShortCode: "abc123A", OriginalURL: "https://example.com", IsActive: true, OwnerID: &owner}}
	svc := New(links, newMapCache(), "http://localhost:8080")
	stats, err := svc.Analytics(context.Background(), 1, "abc123A")
	require.NoError(t, err)
	require.Equal(t, int64(3), stats.TotalClicks)

	_, err = svc.Analytics(context.Background(), 2, "abc123A")
	require.ErrorIs(t, err, domain.ErrForbidden)
}

func TestDeactivateClearsCache(t *testing.T) {
	owner := int64(1)
	links := &stubLinks{inserted: domain.Link{ID: 1, ShortCode: "abc123A", OriginalURL: "https://example.com", IsActive: true, OwnerID: &owner}}
	cache := newMapCache()
	cache.m["abc123A"] = domain.CachedLink{ID: 1, OriginalURL: "https://example.com", IsActive: true}
	svc := New(links, cache, "http://localhost:8080")
	require.NoError(t, svc.Deactivate(context.Background(), 1, "abc123A"))
	_, ok := cache.m["abc123A"]
	require.False(t, ok)
}
