package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/mitsuoleo/encurta/internal/domain"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestEventFromStream(t *testing.T) {
	msg := redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"link_id":    "42",
			"clicked_at": time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339Nano),
			"ip_hash":    "abc",
			"user_agent": "Chrome",
			"referer":    "https://ref.example",
			"device":     "desktop",
			"browser":    "chrome",
		},
	}
	ev, err := EventFromStream(msg)
	require.NoError(t, err)
	require.Equal(t, int64(42), ev.LinkID)
	require.Equal(t, "abc", ev.IPHash)
	require.Equal(t, "Chrome", ev.UserAgent)
	require.Equal(t, "desktop", ev.Device)
}

type fakeStream struct {
	mu    sync.Mutex
	claim []redis.XMessage
	fresh []redis.XMessage
	acked []string
	reads int
}

func (f *fakeStream) ReadClicks(ctx context.Context, consumer string, count int64, block time.Duration) ([]redis.XMessage, error) {
	_ = consumer
	_ = count
	_ = block
	f.mu.Lock()
	f.reads++
	if f.reads > 1 {
		f.mu.Unlock()
		<-ctx.Done()
		return nil, ctx.Err()
	}
	out := f.fresh
	f.fresh = nil
	f.mu.Unlock()
	return out, nil
}

func (f *fakeStream) AutoClaimClicks(ctx context.Context, consumer string, minIdle time.Duration, count int64) ([]redis.XMessage, error) {
	_ = ctx
	_ = consumer
	_ = minIdle
	_ = count
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.claim
	f.claim = nil
	return out, nil
}

func (f *fakeStream) AckClick(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acked = append(f.acked, id)
	return nil
}

type fakeWriter struct {
	failID int64
	got    []domain.ClickEvent
}

func (w *fakeWriter) InsertClick(_ context.Context, ev domain.ClickEvent) error {
	if ev.LinkID == w.failID {
		return errors.New("db down")
	}
	w.got = append(w.got, ev)
	return nil
}

func clickMsg(id string, linkID string) redis.XMessage {
	return redis.XMessage{
		ID: id,
		Values: map[string]any{
			"link_id":    linkID,
			"clicked_at": time.Now().UTC().Format(time.RFC3339Nano),
			"ip_hash":    "h",
			"user_agent": "ua",
			"referer":    "",
			"device":     "desktop",
			"browser":    "chrome",
		},
	}
}

func TestConsumeStreamClaimsAndAcks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &fakeStream{
		claim: []redis.XMessage{clickMsg("1-0", "1")},
		fresh: []redis.XMessage{clickMsg("2-0", "2")},
	}
	w := &fakeWriter{failID: 99}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	done := make(chan struct{})
	go func() {
		ConsumeStream(ctx, stream, w, "click-worker", log)
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stream.mu.Lock()
		n := len(stream.acked)
		stream.mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	require.Equal(t, []string{"1-0", "2-0"}, stream.acked)
	require.Len(t, w.got, 2)
}

func TestConsumeStreamSkipsAckOnPersistFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &fakeStream{
		fresh: []redis.XMessage{clickMsg("9-0", "7")},
	}
	w := &fakeWriter{failID: 7}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	go ConsumeStream(ctx, stream, w, "click-worker", log)
	time.Sleep(150 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
	stream.mu.Lock()
	acked := append([]string(nil), stream.acked...)
	stream.mu.Unlock()
	require.Empty(t, acked)
	require.Empty(t, w.got)
}
