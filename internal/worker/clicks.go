package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/mitsuoleo/encurta/internal/domain"
	"github.com/mitsuoleo/encurta/internal/observability"
	"github.com/redis/go-redis/v9"
)

const persistTimeout = 5 * time.Second
const claimIdle = 30 * time.Second

type ClickWriter interface {
	InsertClick(ctx context.Context, ev domain.ClickEvent) error
}

type StreamReader interface {
	ReadClicks(ctx context.Context, consumer string, count int64, block time.Duration) ([]redis.XMessage, error)
	AutoClaimClicks(ctx context.Context, consumer string, minIdle time.Duration, count int64) ([]redis.XMessage, error)
	AckClick(ctx context.Context, id string) error
}

func ConsumeStream(ctx context.Context, streams StreamReader, writer ClickWriter, consumer string, log *slog.Logger) {
	for {
		if ctx.Err() != nil {
			return
		}
		claimed, err := streams.AutoClaimClicks(ctx, consumer, claimIdle, 32)
		if err != nil && ctx.Err() == nil {
			log.Error("autoclaim click stream", "err", err)
		}
		processBatch(ctx, streams, writer, claimed, log)

		msgs, err := streams.ReadClicks(ctx, consumer, 32, 2*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error("read click stream", "err", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		processBatch(ctx, streams, writer, msgs, log)
	}
}

func processBatch(ctx context.Context, streams StreamReader, writer ClickWriter, msgs []redis.XMessage, log *slog.Logger) {
	for _, msg := range msgs {
		ev, err := EventFromStream(msg)
		if err != nil {
			log.Error("decode click stream", "err", err, "id", msg.ID)
			ack(ctx, streams, msg.ID, log)
			continue
		}
		pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
		err = writer.InsertClick(pctx, ev)
		cancel()
		if err != nil {
			log.Error("persist click failed", "err", err, "link_id", ev.LinkID)
			continue
		}
		ack(ctx, streams, msg.ID, log)
	}
}

func ack(ctx context.Context, streams StreamReader, id string, log *slog.Logger) {
	actx, cancel := context.WithTimeout(context.WithoutCancel(ctx), persistTimeout)
	defer cancel()
	if err := streams.AckClick(actx, id); err != nil {
		log.Error("ack click failed", "err", err, "id", id)
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
	ua := streamString(msg.Values["user_agent"])
	device := streamString(msg.Values["device"])
	browser := streamString(msg.Values["browser"])
	if device == "" || browser == "" {
		device, browser = domain.ClassifyUserAgent(ua)
	}
	return domain.ClickEvent{
		LinkID:    linkID,
		ClickedAt: clickedAt.UTC(),
		IPHash:    streamString(msg.Values["ip_hash"]),
		UserAgent: ua,
		Referer:   streamString(msg.Values["referer"]),
		Device:    device,
		Browser:   browser,
	}, nil
}

func streamString(v any) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprint(v)
	if s == "<nil>" {
		return ""
	}
	return s
}

type Publisher func(ctx context.Context, ev domain.ClickEvent) error

func (p Publisher) Publish(ctx context.Context, ev domain.ClickEvent) {
	if err := p(ctx, ev); err != nil {
		observability.ClicksDropped.Inc()
	}
}
