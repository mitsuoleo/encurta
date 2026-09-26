package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidURL         = errors.New("invalid url")
	ErrNotFound           = errors.New("link not found")
	ErrInactive           = errors.New("link inactive")
	ErrExpired            = errors.New("link expired")
	ErrCodeCollision      = errors.New("short code collision")
	ErrRateLimited        = errors.New("rate limited")
	ErrInvalidPayload     = errors.New("invalid payload")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidAlias       = errors.New("invalid alias")
	ErrInvalidExpiry      = errors.New("invalid expiration")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong    = errors.New("password is too long")
	ErrCacheUnavailable   = errors.New("cache unavailable")
)

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Link struct {
	ID          int64
	ShortCode   string
	OriginalURL string
	OwnerID     *int64
	IsActive    bool
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

func (l Link) OwnedBy(userID int64) bool {
	return l.OwnerID != nil && *l.OwnerID == userID
}

func (l Link) Redirectable(now time.Time) error {
	if !l.IsActive {
		return ErrInactive
	}
	if l.ExpiresAt != nil && !l.ExpiresAt.After(now) {
		return ErrExpired
	}
	return nil
}

type ClickEvent struct {
	LinkID    int64
	ClickedAt time.Time
	IPHash    string
	UserAgent string
	Referer   string
	Device    string
	Browser   string
}

type DayCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type NamedCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type Analytics struct {
	TotalClicks      int64        `json:"total_clicks"`
	UniqueVisitors   int64        `json:"unique_visitors"`
	ClicksByDay      []DayCount   `json:"clicks_by_day"`
	TopReferrers     []NamedCount `json:"top_referrers"`
	DeviceBreakdown  []NamedCount `json:"device_breakdown"`
	BrowserBreakdown []NamedCount `json:"browser_breakdown"`
}

type CachedLink struct {
	ID          int64  `json:"id"`
	OriginalURL string `json:"original_url"`
	IsActive    bool   `json:"is_active"`
	ExpiresAt   *int64 `json:"expires_at,omitempty"`
}

func (c CachedLink) ToLink(code string) Link {
	var exp *time.Time
	if c.ExpiresAt != nil {
		t := time.Unix(*c.ExpiresAt, 0).UTC()
		exp = &t
	}
	return Link{
		ID:          c.ID,
		ShortCode:   code,
		OriginalURL: c.OriginalURL,
		IsActive:    c.IsActive,
		ExpiresAt:   exp,
	}
}

func LinkToCached(l Link) CachedLink {
	var exp *int64
	if l.ExpiresAt != nil {
		v := l.ExpiresAt.Unix()
		exp = &v
	}
	return CachedLink{
		ID:          l.ID,
		OriginalURL: l.OriginalURL,
		IsActive:    l.IsActive,
		ExpiresAt:   exp,
	}
}
