package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/mitsuoleo/encurta/internal/domain"
)

type ctxKey int

const userIDKey ctxKey = 1

type TokenParser interface {
	Parse(raw string) (int64, string, error)
}

func UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}

func WithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func RequireAuth(parser TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearer(r)
			if raw == "" {
				writeJSONError(w, http.StatusUnauthorized, domain.ErrUnauthorized.Error())
				return
			}
			id, _, err := parser.Parse(raw)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, domain.ErrUnauthorized.Error())
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), id)))
		})
	}
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
