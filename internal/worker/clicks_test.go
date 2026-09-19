package worker

import (
	"testing"
	"time"

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
		},
	}
	ev, err := EventFromStream(msg)
	require.NoError(t, err)
	require.Equal(t, int64(42), ev.LinkID)
	require.Equal(t, "abc", ev.IPHash)
	require.Equal(t, "Chrome", ev.UserAgent)
}
