package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mitsuoleo/encurta/internal/domain"
	"github.com/stretchr/testify/require"
)

type stubLinks struct {
	mu            sync.RWMutex
	inserted      domain.Link
	collisions    int32
	updateEntered chan struct{}
}

func (s *stubLinks) InsertLink(_ context.Context, in domain.Link) (domain.Link, error) {
	if atomic.AddInt32(&s.collisions, -1) >= 0 {
		return domain.Link{}, domain.ErrCodeCollision
	}
	in.ID = 1
	in.IsActive = true
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inserted = in
	return s.inserted, nil
}

func (s *stubLinks) GetByCode(_ context.Context, code string) (domain.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.inserted.ShortCode != code {
		return domain.Link{}, domain.ErrNotFound
	}
	return s.inserted, nil
}

func (s *stubLinks) GetByCodeForCache(_ context.Context, code string, populate func(domain.Link) error) (domain.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.inserted.ShortCode != code {
		return domain.Link{}, domain.ErrNotFound
	}
	link := s.inserted
	if err := populate(link); err != nil {
		return domain.Link{}, err
	}
	return link, nil
}

func (s *stubLinks) ListByOwner(_ context.Context, ownerID int64, limit, offset int) ([]domain.Link, int, error) {
	_ = limit
	_ = offset
	if s.inserted.OwnedBy(ownerID) {
		return []domain.Link{s.inserted}, 1, nil
	}
	return []domain.Link{}, 0, nil
}

func (s *stubLinks) UpdateLink(_ context.Context, code string, ownerID int64, url *string, expiresAt *time.Time, clearExpiry bool, isActive *bool, invalidate func() error) (domain.Link, error) {
	if s.updateEntered != nil {
		close(s.updateEntered)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inserted.ShortCode != code || !s.inserted.OwnedBy(ownerID) {
		return domain.Link{}, domain.ErrNotFound
	}
	updated := s.inserted
	if url != nil {
		updated.OriginalURL = *url
	}
	if clearExpiry {
		updated.ExpiresAt = nil
	} else if expiresAt != nil {
		updated.ExpiresAt = expiresAt
	}
	if isActive != nil {
		updated.IsActive = *isActive
	}
	if err := invalidate(); err != nil {
		return domain.Link{}, err
	}
	s.inserted = updated
	return s.inserted, nil
}

func (s *stubLinks) Deactivate(_ context.Context, code string, ownerID int64, invalidate func() error) (domain.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inserted.ShortCode != code || !s.inserted.OwnedBy(ownerID) {
		return domain.Link{}, domain.ErrNotFound
	}
	if err := invalidate(); err != nil {
		return domain.Link{}, err
	}
	s.inserted.IsActive = false
	return s.inserted, nil
}

func (s *stubLinks) Analytics(_ context.Context, _ int64) (domain.Analytics, error) {
	return domain.Analytics{TotalClicks: 3}, nil
}

type mapCache struct {
	mu sync.Mutex
	m  map[string]domain.CachedLink
}

func newMapCache() *mapCache {
	return &mapCache{m: map[string]domain.CachedLink{}}
}

func (c *mapCache) GetLink(_ context.Context, code string) (domain.CachedLink, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[code]
	return v, ok, nil
}

func (c *mapCache) SetLink(_ context.Context, code string, link domain.Link) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[code] = domain.LinkToCached(link)
	return nil
}

func (c *mapCache) DeleteLink(_ context.Context, code string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	cache := newMapCache()
	svc := New(&stubLinks{}, cache, "http://localhost:8080")
	got, err := svc.Create(context.Background(), CreateInput{OwnerID: 1, URL: "https://example.com", Alias: "myname"})
	require.NoError(t, err)
	require.Equal(t, "myname", got.ShortCode)
	_, cached := cache.m[got.ShortCode]
	require.False(t, cached, "creation must not populate the cache after committing")
}

func TestCreateHyphenAlias(t *testing.T) {
	svc := New(&stubLinks{}, newMapCache(), "http://localhost:8080")
	got, err := svc.Create(context.Background(), CreateInput{OwnerID: 1, URL: "https://example.com", Alias: "meu-link"})
	require.NoError(t, err)
	require.Equal(t, "meu-link", got.ShortCode)
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
	require.ErrorIs(t, err, domain.ErrNotFound)
}

type failCache struct {
	mapCache
	failSet bool
	failDel bool
}

func (c *failCache) SetLink(_ context.Context, code string, link domain.Link) error {
	if c.failSet {
		return errCache
	}
	return c.mapCache.SetLink(context.Background(), code, link)
}

func (c *failCache) DeleteLink(_ context.Context, code string) error {
	if c.failDel {
		return errCache
	}
	return c.mapCache.DeleteLink(context.Background(), code)
}

var errCache = errors.New("redis down")

func TestDeactivateCacheFailure(t *testing.T) {
	owner := int64(1)
	links := &stubLinks{inserted: domain.Link{ID: 1, ShortCode: "abc123A", OriginalURL: "https://example.com", IsActive: true, OwnerID: &owner}}
	cache := &failCache{mapCache: *newMapCache(), failSet: true, failDel: true}
	cache.m["abc123A"] = domain.CachedLink{ID: 1, OriginalURL: "https://example.com", IsActive: true}
	svc := New(links, cache, "http://localhost:8080")
	err := svc.Deactivate(context.Background(), 1, "abc123A")
	require.ErrorIs(t, err, domain.ErrCacheUnavailable)
	require.True(t, links.inserted.IsActive)
	_, err = svc.Resolve(context.Background(), "abc123A")
	require.NoError(t, err)
}

func TestUpdateCacheDeleteFailureDoesNotPersist(t *testing.T) {
	owner := int64(1)
	links := &stubLinks{inserted: domain.Link{ID: 1, ShortCode: "abc123A", OriginalURL: "https://example.com", IsActive: true, OwnerID: &owner}}
	cache := &failCache{mapCache: *newMapCache(), failDel: true}
	cache.m["abc123A"] = domain.LinkToCached(links.inserted)
	svc := New(links, cache, "http://localhost:8080")
	next := "https://example.org/new"
	_, err := svc.Update(context.Background(), 1, "abc123A", UpdateInput{URL: &next})
	require.ErrorIs(t, err, domain.ErrCacheUnavailable)
	require.Equal(t, "https://example.com", links.inserted.OriginalURL)
	got, err := svc.Resolve(context.Background(), "abc123A")
	require.NoError(t, err)
	require.Equal(t, "https://example.com", got.OriginalURL)
}

func TestUpdateCacheSetFailureReturnsPersistedResultAndResolvesFromStore(t *testing.T) {
	owner := int64(1)
	links := &stubLinks{inserted: domain.Link{ID: 1, ShortCode: "abc123A", OriginalURL: "https://example.com", IsActive: true, OwnerID: &owner}}
	cache := &failCache{mapCache: *newMapCache(), failSet: true}
	cache.m["abc123A"] = domain.LinkToCached(links.inserted)
	svc := New(links, cache, "http://localhost:8080")
	next := "https://example.org/new"
	got, err := svc.Update(context.Background(), 1, "abc123A", UpdateInput{URL: &next})
	require.NoError(t, err)
	require.Equal(t, next, got.OriginalURL)
	_, hit := cache.m["abc123A"]
	require.False(t, hit)
	resolved, err := svc.Resolve(context.Background(), "abc123A")
	require.NoError(t, err)
	require.Equal(t, next, resolved.OriginalURL)
}

func TestUpdateDestination(t *testing.T) {
	owner := int64(1)
	links := &stubLinks{inserted: domain.Link{ID: 1, ShortCode: "abc123A", OriginalURL: "https://example.com", IsActive: true, OwnerID: &owner}}
	svc := New(links, newMapCache(), "http://localhost:8080")
	next := "https://example.org/new"
	got, err := svc.Update(context.Background(), 1, "abc123A", UpdateInput{URL: &next})
	require.NoError(t, err)
	require.Equal(t, next, got.OriginalURL)
}

func TestDeactivateSetFailStillInvalidates(t *testing.T) {
	owner := int64(1)
	links := &stubLinks{inserted: domain.Link{ID: 1, ShortCode: "abc123A", OriginalURL: "https://example.com", IsActive: true, OwnerID: &owner}}
	cache := &failCache{mapCache: *newMapCache(), failSet: true}
	cache.m["abc123A"] = domain.CachedLink{ID: 1, OriginalURL: "https://example.com", IsActive: true}
	svc := New(links, cache, "http://localhost:8080")
	require.NoError(t, svc.Deactivate(context.Background(), 1, "abc123A"))
	_, ok := cache.m["abc123A"]
	require.False(t, ok)
}

func TestDeactivateLeavesCacheEmpty(t *testing.T) {
	owner := int64(1)
	links := &stubLinks{inserted: domain.Link{ID: 1, ShortCode: "abc123A", OriginalURL: "https://example.com", IsActive: true, OwnerID: &owner}}
	cache := newMapCache()
	cache.m["abc123A"] = domain.CachedLink{ID: 1, OriginalURL: "https://example.com", IsActive: true}
	svc := New(links, cache, "http://localhost:8080")
	require.NoError(t, svc.Deactivate(context.Background(), 1, "abc123A"))
	_, ok := cache.m["abc123A"]
	require.False(t, ok)
}

type blockingSetCache struct {
	*mapCache
	entered chan struct{}
	release chan struct{}
}

func (c *blockingSetCache) SetLink(ctx context.Context, code string, link domain.Link) error {
	close(c.entered)
	select {
	case <-c.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return c.mapCache.SetLink(ctx, code, link)
}

func TestConcurrentMissAndUpdateAcrossServicesDoesNotRepopulateStaleCache(t *testing.T) {
	owner := int64(1)
	links := &stubLinks{
		inserted:      domain.Link{ID: 1, ShortCode: "abc123A", OriginalURL: "https://old.example", IsActive: true, OwnerID: &owner},
		updateEntered: make(chan struct{}),
	}
	shared := newMapCache()
	readerCache := &blockingSetCache{mapCache: shared, entered: make(chan struct{}), release: make(chan struct{})}
	reader := New(links, readerCache, "http://localhost")
	writer := New(links, shared, "http://localhost")

	readDone := make(chan error, 1)
	go func() {
		_, err := reader.Resolve(context.Background(), "abc123A")
		readDone <- err
	}()
	<-readerCache.entered // The old DB value was read; cache SET has not completed.

	newURL := "https://new.example"
	writeDone := make(chan error, 1)
	go func() {
		_, err := writer.Update(context.Background(), owner, "abc123A", UpdateInput{URL: &newURL})
		writeDone <- err
	}()
	<-links.updateEntered
	select {
	case err := <-writeDone:
		t.Fatalf("writer committed while cache population held the row lock: %v", err)
	default:
	}

	close(readerCache.release)
	require.NoError(t, <-readDone)
	require.NoError(t, <-writeDone)
	resolved, err := writer.Resolve(context.Background(), "abc123A")
	require.NoError(t, err)
	require.Equal(t, newURL, resolved.OriginalURL)
}
