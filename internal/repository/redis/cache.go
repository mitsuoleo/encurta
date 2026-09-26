package rediscache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mitsuoleo/encurta/internal/domain"
	"github.com/redis/go-redis/v9"
)

const clickStreamMaxLen = 100_000

var incrExpire = redis.NewScript(`
local n = redis.call("INCR", KEYS[1])
if n == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return n
`)

type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

func New(client *redis.Client, ttl time.Duration) *Cache {
	return &Cache{client: client, ttl: ttl}
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *Cache) GetLink(ctx context.Context, code string) (domain.CachedLink, bool, error) {
	raw, err := c.client.Get(ctx, key(code)).Result()
	if errors.Is(err, redis.Nil) {
		return domain.CachedLink{}, false, nil
	}
	if err != nil {
		return domain.CachedLink{}, false, err
	}
	var cached domain.CachedLink
	if err := json.Unmarshal([]byte(raw), &cached); err != nil {
		return domain.CachedLink{}, false, err
	}
	return cached, true, nil
}

func (c *Cache) SetLink(ctx context.Context, code string, link domain.Link) error {
	b, err := json.Marshal(domain.LinkToCached(link))
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key(code), b, c.ttl).Err()
}

func (c *Cache) DeleteLink(ctx context.Context, code string) error {
	return c.client.Del(ctx, key(code)).Err()
}

func (c *Cache) Allow(ctx context.Context, id string, limit int, ttl time.Duration) (bool, error) {
	n, err := incrExpire.Run(ctx, c.client, []string{id}, int(ttl.Seconds())).Int64()
	if err != nil {
		return false, err
	}
	return n <= int64(limit), nil
}

func (c *Cache) AllowCreate(ctx context.Context, id string, limit int) (bool, error) {
	return c.Allow(ctx, fmt.Sprintf("rl:create:%s", id), limit, time.Hour)
}

const ClickStream = "clicks"
const ClickGroup = "click-workers"

func (c *Cache) PublishClick(ctx context.Context, ev domain.ClickEvent) error {
	return c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: ClickStream,
		MaxLen: clickStreamMaxLen,
		Approx: true,
		Values: map[string]any{
			"link_id":    ev.LinkID,
			"clicked_at": ev.ClickedAt.UTC().Format(time.RFC3339Nano),
			"ip_hash":    ev.IPHash,
			"user_agent": ev.UserAgent,
			"referer":    ev.Referer,
			"device":     ev.Device,
			"browser":    ev.Browser,
		},
	}).Err()
}

func (c *Cache) EnsureClickGroup(ctx context.Context) error {
	err := c.client.XGroupCreateMkStream(ctx, ClickStream, ClickGroup, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (c *Cache) ReadClicks(ctx context.Context, consumer string, count int64, block time.Duration) ([]redis.XMessage, error) {
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ClickGroup,
		Consumer: consumer,
		Streams:  []string{ClickStream, ">"},
		Count:    count,
		Block:    block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []redis.XMessage
	for _, s := range streams {
		out = append(out, s.Messages...)
	}
	return out, nil
}

func (c *Cache) AutoClaimClicks(ctx context.Context, consumer string, minIdle time.Duration, count int64) ([]redis.XMessage, error) {
	msgs, _, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   ClickStream,
		Group:    ClickGroup,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    "0-0",
		Count:    count,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

func (c *Cache) AckClick(ctx context.Context, id string) error {
	return c.client.XAck(ctx, ClickStream, ClickGroup, id).Err()
}

func key(code string) string {
	return "link:" + code
}
