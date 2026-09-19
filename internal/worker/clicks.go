package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/esposo/url-shortener/internal/domain"
	"github.com/esposo/url-shortener/internal/observability"
	"github.com/redis/go-redis/v9"
)

type ClickWriter interface {
	InsertClick(ctx context.Context, ev domain.ClickEvent) error
}

type StreamReader interface {
	ReadClicks(ctx context.Context, consumer string, count int64, block time.Duration) ([]redis.XMessage, error)
	AckClick(ctx context.Context, id string) error
}

func ConsumeStream(ctx context.Context, streams StreamReader, writer ClickWriter, consumer string, log *slog.Logger) {
	for {
		if ctx.Err() != nil {
			return
		}
		msgs, err := streams.ReadClicks(ctx, consumer, 32, 2*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error("read click stream", "err", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		for _, msg := range msgs {
			ev, err := EventFromStream(msg)
			if err != nil {
				log.Error("decode click stream", "err", err, "id", msg.ID)
				_ = streams.AckClick(ctx, msg.ID)
				continue
			}
			if err := writer.InsertClick(ctx, ev); err != nil {
				log.Error("persist click failed", "err", err, "link_id", ev.LinkID)
				continue
			}
			if err := streams.AckClick(ctx, msg.ID); err != nil {
				log.Error("ack click failed", "err", err, "id", msg.ID)
			}
		}
	}
}

func EventFromStream(msg redis.XMessage) (domain.ClickEvent, error) {
	linkID, err := strconv.ParseInt(fmt.Sprint(msg.Values["link_id"]), 10, 64)
	if err != nil {
		return domain.ClickEvent{}, err
	}
	clickedAt, err := time.Parse(time.RFC3339Nano, fmt.Sprint(msg.Values["clicked_at"]))
	if err != nil {
		clickedAt, err = time.Parse(time.RFC3339, fmt.Sprint(msg.Values["clicked_at"]))
		if err != nil {
			clickedAt = time.Now().UTC()
		}
	}
	return domain.ClickEvent{
		LinkID:    linkID,
		ClickedAt: clickedAt.UTC(),
		IPHash:    fmt.Sprint(msg.Values["ip_hash"]),
		UserAgent: fmt.Sprint(msg.Values["user_agent"]),
		Referer:   fmt.Sprint(msg.Values["referer"]),
	}, nil
}

type Publisher func(ctx context.Context, ev domain.ClickEvent) error

func (p Publisher) Publish(ctx context.Context, ev domain.ClickEvent) {
	if err := p(ctx, ev); err != nil {
		observability.ClicksDropped.Inc()
	}
}
