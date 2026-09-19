package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/esposo/url-shortener/internal/auth"
	"github.com/esposo/url-shortener/internal/domain"
	"github.com/esposo/url-shortener/internal/service"
	"github.com/esposo/url-shortener/internal/worker"
	"github.com/stretchr/testify/require"
)

type memStore struct {
	mu    sync.Mutex
	links map[string]domain.Link
	users map[string]domain.User
	next  int64
	nextU int64
}

func newMemStore() *memStore {
	return &memStore{links: map[string]domain.Link{}, users: map[string]domain.User{}}
}

func (m *memStore) InsertLink(_ context.Context, in domain.Link) (domain.Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.links[in.ShortCode]; ok {
		return domain.Link{}, domain.ErrCodeCollision
	}
	m.next++
	in.ID = m.next
	in.IsActive = true
	in.CreatedAt = time.Now().UTC()
	m.links[in.ShortCode] = in
	return in, nil
}

func (m *memStore) GetByCode(_ context.Context, code string) (domain.Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	link, ok := m.links[code]
	if !ok {
		return domain.Link{}, domain.ErrNotFound
	}
	return link, nil
}

func (m *memStore) ListByOwner(_ context.Context, ownerID int64) ([]domain.Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Link
	for _, l := range m.links {
		if l.OwnedBy(ownerID) {
			out = append(out, l)
		}
	}
	return out, nil
}

func (m *memStore) Deactivate(_ context.Context, code string, ownerID int64) (domain.Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	link, ok := m.links[code]
	if !ok || !link.OwnedBy(ownerID) {
		return domain.Link{}, domain.ErrNotFound
	}
	link.IsActive = false
	m.links[code] = link
	return link, nil
}

func (m *memStore) Analytics(_ context.Context, _ int64) (domain.Analytics, error) {
	return domain.Analytics{
		TotalClicks:      2,
		ClicksByDay:      []domain.DayCount{{Day: "2026-01-01", Count: 2}},
		TopReferrers:     []domain.NamedCount{{Name: "(direct)", Count: 2}},
		DeviceBreakdown:  []domain.NamedCount{{Name: "desktop", Count: 2}},
		BrowserBreakdown: []domain.NamedCount{{Name: "chrome", Count: 2}},
	}, nil
}

func (m *memStore) CreateUser(_ context.Context, email, passwordHash string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[email]; ok {
		return domain.User{}, domain.ErrEmailTaken
	}
	m.nextU++
	u := domain.User{ID: m.nextU, Email: email, PasswordHash: passwordHash, CreatedAt: time.Now().UTC()}
	m.users[email] = u
	return u, nil
}

func (m *memStore) GetUserByEmail(_ context.Context, email string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[email]
	if !ok {
		return domain.User{}, domain.ErrInvalidCredentials
	}
	return u, nil
}

func (m *memStore) seed(link domain.Link) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.links[link.ShortCode] = link
}

type memCache struct {
	mu sync.Mutex
	m  map[string]domain.CachedLink
}

func newMemCache() *memCache {
	return &memCache{m: map[string]domain.CachedLink{}}
}

func (c *memCache) GetLink(_ context.Context, code string) (domain.CachedLink, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[code]
	return v, ok, nil
}

func (c *memCache) SetLink(_ context.Context, code string, link domain.Link) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[code] = domain.LinkToCached(link)
	return nil
}

func (c *memCache) DeleteLink(_ context.Context, code string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, code)
	return nil
}

type fixedLimiter struct {
	allow bool
}

func (f fixedLimiter) AllowCreate(*http.Request) (bool, error) {
	return f.allow, nil
}

type recPub struct {
	mu sync.Mutex
	ev []domain.ClickEvent
}

func (p *recPub) publish(_ context.Context, ev domain.ClickEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ev = append(p.ev, ev)
	return nil
}

func testAPI(t *testing.T, store *memStore, limiter fixedLimiter) (*API, *recPub) {
	t.Helper()
	tokens := auth.NewTokens("test-secret", time.Hour)
	svc := service.New(store, newMemCache(), "http://localhost:8080")
	authSvc := service.NewAuth(store, tokens)
	pub := &recPub{}
	api := New(svc, authSvc, worker.Publisher(pub.publish), "testsalt", discardLogger(), func() error { return nil }, func() error { return nil }, limiter)
	return api, pub
}

func register(t *testing.T, api *API, email string) string {
	t.Helper()
	body := bytes.NewBufferString(`{"email":"` + email + `","password":"password1"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", body)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var resp authResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	return resp.Token
}

func TestCreateRequiresAuth(t *testing.T) {
	api, _ := testAPI(t, newMemStore(), fixedLimiter{allow: true})
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewBufferString(`{"url":"https://example.com"}`))
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCreateLinkHappy(t *testing.T) {
	api, _ := testAPI(t, newMemStore(), fixedLimiter{allow: true})
	tok := register(t, api, "a@example.com")
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewBufferString(`{"url":"https://example.com/hello"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	var resp createResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp.ShortCode, 7)
}

func TestCreateAliasAndConflict(t *testing.T) {
	api, _ := testAPI(t, newMemStore(), fixedLimiter{allow: true})
	tok := register(t, api, "a@example.com")
	body := `{"url":"https://example.com","alias":"myalias"}`
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	req = httptest.NewRequest(http.MethodPost, "/links", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec = httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestCreateHyphenAliasRedirect(t *testing.T) {
	api, _ := testAPI(t, newMemStore(), fixedLimiter{allow: true})
	tok := register(t, api, "a@example.com")
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewBufferString(`{"url":"https://example.com","alias":"my-link"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/my-link", nil)
	rec = httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "https://example.com", rec.Header().Get("Location"))
}

func TestLogout(t *testing.T) {
	api, _ := testAPI(t, newMemStore(), fixedLimiter{allow: true})
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestCreateLinkInvalidURL(t *testing.T) {
	api, _ := testAPI(t, newMemStore(), fixedLimiter{allow: true})
	tok := register(t, api, "a@example.com")
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewBufferString(`{"url":"javascript:alert(1)"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateLinkRateLimited(t *testing.T) {
	api, _ := testAPI(t, newMemStore(), fixedLimiter{allow: false})
	tok := register(t, api, "a@example.com")
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewBufferString(`{"url":"https://example.com"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestRedirectHappyPublishesClick(t *testing.T) {
	store := newMemStore()
	owner := int64(1)
	store.seed(domain.Link{ID: 9, ShortCode: "abc123A", OriginalURL: "https://example.com/x", IsActive: true, OwnerID: &owner})
	api, pub := testAPI(t, store, fixedLimiter{allow: true})
	req := httptest.NewRequest(http.MethodGet, "/abc123A", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "https://example.com/x", rec.Header().Get("Location"))
	require.Len(t, pub.ev, 1)
	require.Equal(t, int64(9), pub.ev[0].LinkID)
}

func TestRedirectExpiredGone(t *testing.T) {
	store := newMemStore()
	past := time.Now().Add(-time.Hour)
	store.seed(domain.Link{ID: 1, ShortCode: "oldcode1", OriginalURL: "https://example.com", IsActive: true, ExpiresAt: &past})
	api, _ := testAPI(t, store, fixedLimiter{allow: true})
	req := httptest.NewRequest(http.MethodGet, "/oldcode1", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusGone, rec.Code)
}

func TestRedirectNotFound(t *testing.T) {
	api, _ := testAPI(t, newMemStore(), fixedLimiter{allow: true})
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAnalyticsOwnerOnly(t *testing.T) {
	store := newMemStore()
	api, _ := testAPI(t, store, fixedLimiter{allow: true})
	tokA := register(t, api, "a@example.com")
	tokB := register(t, api, "b@example.com")
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewBufferString(`{"url":"https://example.com","alias":"ownlink"}`))
	req.Header.Set("Authorization", "Bearer "+tokA)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/links/ownlink/analytics", nil)
	req.Header.Set("Authorization", "Bearer "+tokB)
	rec = httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/links/ownlink/analytics", nil)
	req.Header.Set("Authorization", "Bearer "+tokA)
	rec = httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestDeactivateThenGone(t *testing.T) {
	api, _ := testAPI(t, newMemStore(), fixedLimiter{allow: true})
	tok := register(t, api, "a@example.com")
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewBufferString(`{"url":"https://example.com","alias":"killme1"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	req = httptest.NewRequest(http.MethodDelete, "/links/killme1", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec = httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/killme1", nil)
	rec = httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusGone, rec.Code)
}

func TestHealth(t *testing.T) {
	api, _ := testAPI(t, newMemStore(), fixedLimiter{allow: true})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestUI(t *testing.T) {
	api, _ := testAPI(t, newMemStore(), fixedLimiter{allow: true})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "URL Shortener")
}
