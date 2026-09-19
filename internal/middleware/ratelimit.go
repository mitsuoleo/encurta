package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/esposo/url-shortener/internal/domain"
)

type Limiter interface {
	AllowCreate(r *http.Request) (bool, error)
}

func RateLimitCreates(limiter Limiter, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, err := limiter.AllowCreate(r)
			if err != nil {
				log.Error("rate limit check failed", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"` + domain.ErrRateLimited.Error() + `"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
