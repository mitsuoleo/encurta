package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/mitsuoleo/encurta/internal/domain"
)

type Limiter interface {
	AllowCreate(r *http.Request) (bool, error)
	AllowAuth(r *http.Request) (bool, error)
	AllowRedirect(r *http.Request) (bool, error)
}

func RateLimitCreates(limiter Limiter, log *slog.Logger) func(http.Handler) http.Handler {
	return rateLimit(limiter.AllowCreate, log, false)
}

func RateLimitAuth(limiter Limiter, log *slog.Logger) func(http.Handler) http.Handler {
	return rateLimit(limiter.AllowAuth, log, false)
}

func RateLimitRedirects(limiter Limiter, log *slog.Logger) func(http.Handler) http.Handler {
	return rateLimit(limiter.AllowRedirect, log, true)
}

func rateLimit(check func(*http.Request) (bool, error), log *slog.Logger, failOpen bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, err := check(r)
			if err != nil {
				log.Error("rate limit check failed", "err", err)
				if failOpen {
					next.ServeHTTP(w, r)
					return
				}
				writeJSONError(w, http.StatusServiceUnavailable, "rate limiter unavailable")
				return
			}
			if !ok {
				writeJSONError(w, http.StatusTooManyRequests, domain.ErrRateLimited.Error())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ClientIP(r *http.Request, trusted []*net.IPNet) string {
	remote := remoteIP(r)
	if !ipInNets(remote, trusted) {
		return remote
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := parseIPLiteral(strings.TrimSpace(parts[i]))
			if ip == nil {
				continue
			}
			s := ip.String()
			if !ipInNets(s, trusted) {
				return s
			}
		}
	}
	return remote
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func parseIPLiteral(s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil {
		return nil
	}
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip
}

func ipInNets(ipStr string, nets []*net.IPNet) bool {
	if len(nets) == 0 {
		return false
	}
	ip := parseIPLiteral(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}
