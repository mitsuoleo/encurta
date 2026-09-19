package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/esposo/url-shortener/internal/domain"
	"github.com/esposo/url-shortener/internal/observability"
)

const maxCodeAttempts = 5

type Links interface {
	InsertLink(ctx context.Context, in domain.Link) (domain.Link, error)
	GetByCode(ctx context.Context, code string) (domain.Link, error)
	ListByOwner(ctx context.Context, ownerID int64) ([]domain.Link, error)
	Deactivate(ctx context.Context, code string, ownerID int64) (domain.Link, error)
	Analytics(ctx context.Context, linkID int64) (domain.Analytics, error)
}

type Cache interface {
	GetLink(ctx context.Context, code string) (domain.CachedLink, bool, error)
	SetLink(ctx context.Context, code string, link domain.Link) error
	DeleteLink(ctx context.Context, code string) error
}

type LinkService struct {
	links     Links
	cache     Cache
	publicURL string
	now       func() time.Time
}

func New(links Links, cache Cache, publicURL string) *LinkService {
	return &LinkService{
		links:     links,
		cache:     cache,
		publicURL: publicURL,
		now:       time.Now,
	}
}

type CreateInput struct {
	OwnerID   int64
	URL       string
	Alias     string
	ExpiresAt *time.Time
}

func (s *LinkService) Create(ctx context.Context, in CreateInput) (domain.Link, error) {
	normalized, err := domain.NormalizeAndValidateURL(in.URL)
	if err != nil {
		return domain.Link{}, err
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(s.now()) {
		return domain.Link{}, domain.ErrInvalidExpiry
	}
	owner := in.OwnerID
	base := domain.Link{
		OriginalURL: normalized,
		OwnerID:     &owner,
		ExpiresAt:   in.ExpiresAt,
		IsActive:    true,
	}
	if in.Alias != "" {
		if !domain.ValidAlias(in.Alias) {
			return domain.Link{}, domain.ErrInvalidAlias
		}
		base.ShortCode = in.Alias
		link, err := s.links.InsertLink(ctx, base)
		if err != nil {
			return domain.Link{}, err
		}
		_ = s.cache.SetLink(ctx, link.ShortCode, link)
		return link, nil
	}
	var last error
	for i := 0; i < maxCodeAttempts; i++ {
		code, err := domain.GenerateShortCode()
		if err != nil {
			return domain.Link{}, err
		}
		if domain.ReservedCode(code) {
			continue
		}
		base.ShortCode = code
		link, err := s.links.InsertLink(ctx, base)
		if err == nil {
			_ = s.cache.SetLink(ctx, link.ShortCode, link)
			return link, nil
		}
		if errors.Is(err, domain.ErrCodeCollision) {
			last = err
			continue
		}
		return domain.Link{}, err
	}
	return domain.Link{}, fmt.Errorf("%w: %v", domain.ErrCodeCollision, last)
}

func (s *LinkService) Resolve(ctx context.Context, code string) (domain.Link, error) {
	if !domain.ValidShortCode(code) || domain.ReservedCode(code) {
		return domain.Link{}, domain.ErrNotFound
	}
	cached, hit, err := s.cache.GetLink(ctx, code)
	if err == nil && hit {
		observability.CacheHits.Inc()
		link := cached.ToLink(code)
		if err := link.Redirectable(s.now()); err != nil {
			return domain.Link{}, err
		}
		return link, nil
	}
	observability.CacheMisses.Inc()
	link, err := s.links.GetByCode(ctx, code)
	if err != nil {
		return domain.Link{}, err
	}
	_ = s.cache.SetLink(ctx, code, link)
	if err := link.Redirectable(s.now()); err != nil {
		return domain.Link{}, err
	}
	return link, nil
}

func (s *LinkService) Analytics(ctx context.Context, ownerID int64, code string) (domain.Analytics, error) {
	link, err := s.requireOwner(ctx, ownerID, code)
	if err != nil {
		return domain.Analytics{}, err
	}
	return s.links.Analytics(ctx, link.ID)
}

func (s *LinkService) List(ctx context.Context, ownerID int64) ([]domain.Link, error) {
	return s.links.ListByOwner(ctx, ownerID)
}

func (s *LinkService) Deactivate(ctx context.Context, ownerID int64, code string) error {
	if _, err := s.requireOwner(ctx, ownerID, code); err != nil {
		return err
	}
	if _, err := s.links.Deactivate(ctx, code, ownerID); err != nil {
		return err
	}
	_ = s.cache.DeleteLink(ctx, code)
	return nil
}

func (s *LinkService) requireOwner(ctx context.Context, ownerID int64, code string) (domain.Link, error) {
	if !domain.ValidShortCode(code) {
		return domain.Link{}, domain.ErrNotFound
	}
	link, err := s.links.GetByCode(ctx, code)
	if err != nil {
		return domain.Link{}, err
	}
	if !link.OwnedBy(ownerID) {
		return domain.Link{}, domain.ErrForbidden
	}
	return link, nil
}

func (s *LinkService) ShortURL(code string) string {
	base := s.publicURL
	if len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	return base + "/" + code
}
