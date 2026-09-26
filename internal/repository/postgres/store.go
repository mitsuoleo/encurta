package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mitsuoleo/encurta/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) InsertLink(ctx context.Context, in domain.Link) (domain.Link, error) {
	const q = `
		INSERT INTO links (short_code, original_url, owner_id, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, short_code, original_url, owner_id, is_active, expires_at, created_at`
	var link domain.Link
	err := s.pool.QueryRow(ctx, q, in.ShortCode, in.OriginalURL, in.OwnerID, in.ExpiresAt).Scan(
		&link.ID, &link.ShortCode, &link.OriginalURL, &link.OwnerID, &link.IsActive, &link.ExpiresAt, &link.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Link{}, domain.ErrCodeCollision
		}
		return domain.Link{}, fmt.Errorf("insert link: %w", err)
	}
	return link, nil
}

func (s *Store) GetByCode(ctx context.Context, code string) (domain.Link, error) {
	const q = `
		SELECT id, short_code, original_url, owner_id, is_active, expires_at, created_at
		FROM links WHERE short_code = $1`
	var link domain.Link
	err := s.pool.QueryRow(ctx, q, code).Scan(
		&link.ID, &link.ShortCode, &link.OriginalURL, &link.OwnerID, &link.IsActive, &link.ExpiresAt, &link.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Link{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Link{}, fmt.Errorf("get link: %w", err)
	}
	return link, nil
}

// GetByCodeForCache holds a shared row lock until the cache population finishes.
// Writers take the same row's exclusive lock before invalidating the cache.
func (s *Store) GetByCodeForCache(ctx context.Context, code string, populate func(domain.Link) error) (domain.Link, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Link{}, fmt.Errorf("begin cache read: %w", err)
	}
	defer rollback(tx)
	const q = `SELECT id, short_code, original_url, owner_id, is_active, expires_at, created_at
		FROM links WHERE short_code = $1 FOR SHARE`
	var link domain.Link
	err = tx.QueryRow(ctx, q, code).Scan(
		&link.ID, &link.ShortCode, &link.OriginalURL, &link.OwnerID, &link.IsActive, &link.ExpiresAt, &link.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Link{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Link{}, fmt.Errorf("get link for cache: %w", err)
	}
	if err := populate(link); err != nil {
		return domain.Link{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Link{}, fmt.Errorf("commit cache read: %w", err)
	}
	return link, nil
}

func (s *Store) ListByOwner(ctx context.Context, ownerID int64, limit, offset int) ([]domain.Link, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM links WHERE owner_id = $1`, ownerID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count links: %w", err)
	}
	const q = `
		SELECT id, short_code, original_url, owner_id, is_active, expires_at, created_at
		FROM links WHERE owner_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := s.pool.Query(ctx, q, ownerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list links: %w", err)
	}
	defer rows.Close()
	var out []domain.Link
	for rows.Next() {
		var link domain.Link
		if err := rows.Scan(&link.ID, &link.ShortCode, &link.OriginalURL, &link.OwnerID, &link.IsActive, &link.ExpiresAt, &link.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, link)
	}
	if out == nil {
		out = []domain.Link{}
	}
	return out, total, rows.Err()
}

func (s *Store) UpdateLink(ctx context.Context, code string, ownerID int64, url *string, expiresAt *time.Time, clearExpiry bool, isActive *bool, invalidate func() error) (domain.Link, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Link{}, fmt.Errorf("begin update link: %w", err)
	}
	defer rollback(tx)
	const q = `
		UPDATE links SET
			original_url = COALESCE($3, original_url),
			expires_at = CASE
				WHEN $4 THEN NULL
				WHEN $5::timestamptz IS NOT NULL THEN $5
				ELSE expires_at
			END,
			is_active = COALESCE($6, is_active)
		WHERE short_code = $1 AND owner_id = $2
		RETURNING id, short_code, original_url, owner_id, is_active, expires_at, created_at`
	var link domain.Link
	err = tx.QueryRow(ctx, q, code, ownerID, url, clearExpiry, expiresAt, isActive).Scan(
		&link.ID, &link.ShortCode, &link.OriginalURL, &link.OwnerID, &link.IsActive, &link.ExpiresAt, &link.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Link{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Link{}, fmt.Errorf("update link: %w", err)
	}
	if err := invalidate(); err != nil {
		return domain.Link{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Link{}, fmt.Errorf("commit update link: %w", err)
	}
	return link, nil
}

func (s *Store) Deactivate(ctx context.Context, code string, ownerID int64, invalidate func() error) (domain.Link, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Link{}, fmt.Errorf("begin deactivate link: %w", err)
	}
	defer rollback(tx)
	const q = `
		UPDATE links SET is_active = FALSE
		WHERE short_code = $1 AND owner_id = $2
		RETURNING id, short_code, original_url, owner_id, is_active, expires_at, created_at`
	var link domain.Link
	err = tx.QueryRow(ctx, q, code, ownerID).Scan(
		&link.ID, &link.ShortCode, &link.OriginalURL, &link.OwnerID, &link.IsActive, &link.ExpiresAt, &link.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Link{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Link{}, fmt.Errorf("deactivate link: %w", err)
	}
	if err := invalidate(); err != nil {
		return domain.Link{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Link{}, fmt.Errorf("commit deactivate link: %w", err)
	}
	return link, nil
}

func (s *Store) InsertClick(ctx context.Context, ev domain.ClickEvent) error {
	device, browser := ev.Device, ev.Browser
	if device == "" || browser == "" {
		device, browser = domain.ClassifyUserAgent(ev.UserAgent)
	}
	const q = `
		INSERT INTO clicks (link_id, clicked_at, ip_hash, user_agent, referer, device, browser)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.pool.Exec(ctx, q, ev.LinkID, ev.ClickedAt, ev.IPHash, ev.UserAgent, ev.Referer, device, browser)
	if err != nil {
		return fmt.Errorf("insert click: %w", err)
	}
	return nil
}

func (s *Store) Analytics(ctx context.Context, linkID int64) (domain.Analytics, error) {
	var out domain.Analytics
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(DISTINCT ip_hash) FROM clicks WHERE link_id = $1`, linkID).Scan(&out.TotalClicks, &out.UniqueVisitors)
	if err != nil {
		return domain.Analytics{}, fmt.Errorf("count clicks: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT to_char(clicked_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS day, COUNT(*)
		FROM clicks WHERE link_id = $1
		GROUP BY day ORDER BY day`, linkID)
	if err != nil {
		return domain.Analytics{}, fmt.Errorf("clicks by day: %w", err)
	}
	out.ClicksByDay, err = scanNamedDays(rows)
	if err != nil {
		return domain.Analytics{}, err
	}

	rows, err = s.pool.Query(ctx, `
		SELECT COALESCE(NULLIF(referer, ''), '(direct)') AS name, COUNT(*) AS c
		FROM clicks WHERE link_id = $1
		GROUP BY name ORDER BY c DESC LIMIT 10`, linkID)
	if err != nil {
		return domain.Analytics{}, fmt.Errorf("top referrers: %w", err)
	}
	out.TopReferrers, err = scanNamed(rows)
	if err != nil {
		return domain.Analytics{}, err
	}

	rows, err = s.pool.Query(ctx, `
		SELECT COALESCE(NULLIF(device, ''), 'unknown') AS name, COUNT(*) AS c
		FROM clicks WHERE link_id = $1
		GROUP BY name ORDER BY c DESC`, linkID)
	if err != nil {
		return domain.Analytics{}, fmt.Errorf("devices: %w", err)
	}
	out.DeviceBreakdown, err = scanNamed(rows)
	if err != nil {
		return domain.Analytics{}, err
	}

	rows, err = s.pool.Query(ctx, `
		SELECT COALESCE(NULLIF(browser, ''), 'unknown') AS name, COUNT(*) AS c
		FROM clicks WHERE link_id = $1
		GROUP BY name ORDER BY c DESC`, linkID)
	if err != nil {
		return domain.Analytics{}, fmt.Errorf("browsers: %w", err)
	}
	out.BrowserBreakdown, err = scanNamed(rows)
	if err != nil {
		return domain.Analytics{}, err
	}

	if out.ClicksByDay == nil {
		out.ClicksByDay = []domain.DayCount{}
	}
	if out.TopReferrers == nil {
		out.TopReferrers = []domain.NamedCount{}
	}
	if out.DeviceBreakdown == nil {
		out.DeviceBreakdown = []domain.NamedCount{}
	}
	if out.BrowserBreakdown == nil {
		out.BrowserBreakdown = []domain.NamedCount{}
	}
	return out, nil
}

func scanNamedDays(rows pgx.Rows) ([]domain.DayCount, error) {
	defer rows.Close()
	var out []domain.DayCount
	for rows.Next() {
		var d domain.DayCount
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func scanNamed(rows pgx.Rows) ([]domain.NamedCount, error) {
	defer rows.Close()
	var out []domain.NamedCount
	for rows.Next() {
		var n domain.NamedCount
		if err := rows.Scan(&n.Name, &n.Count); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func WaitReady(ctx context.Context, pool *pgxpool.Pool) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		if err := pool.Ping(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("postgres not ready")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx) // Commit closes the transaction on the success path.
}
