package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/esposo/url-shortener/internal/domain"
	mw "github.com/esposo/url-shortener/internal/middleware"
	"github.com/esposo/url-shortener/internal/observability"
	"github.com/esposo/url-shortener/internal/service"
	"github.com/esposo/url-shortener/internal/web"
	"github.com/esposo/url-shortener/internal/worker"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type API struct {
	svc       *service.LinkService
	auth      *service.AuthService
	clicks    worker.Publisher
	ipSalt    string
	log       *slog.Logger
	pingDB    func() error
	pingCache func() error
	limiter   mw.Limiter
}

func New(svc *service.LinkService, auth *service.AuthService, clicks worker.Publisher, ipSalt string, log *slog.Logger, pingDB, pingCache func() error, limiter mw.Limiter) *API {
	return &API{
		svc:       svc,
		auth:      auth,
		clicks:    clicks,
		ipSalt:    ipSalt,
		log:       log,
		pingDB:    pingDB,
		pingCache: pingCache,
		limiter:   limiter,
	}
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(mw.Recoverer)
	r.Use(mw.JSONLogger(a.log))

	r.Get("/", a.ui)
	r.Get("/health", a.health)
	r.Handle("/metrics", promhttp.Handler())
	r.Post("/auth/register", a.register)
	r.Post("/auth/login", a.login)

	r.Route("/links", func(r chi.Router) {
		r.Use(mw.RequireAuth(a.auth))
		r.With(mw.RateLimitCreates(a.limiter, a.log)).Post("/", a.createLink)
		r.Get("/", a.listLinks)
		r.Get("/{code}/analytics", a.analytics)
		r.Delete("/{code}", a.deactivate)
	})
	r.Get("/{code}", a.redirect)
	return r
}

func (a *API) ui(w http.ResponseWriter, r *http.Request) {
	_ = r
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(web.Index)
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token     string `json:"access_token"`
	ExpiresIn int64  `json:"expires_in"`
	Email     string `json:"email"`
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidPayload.Error())
		return
	}
	user, tok, exp, err := a.auth.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		writeAuthErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{Token: tok, ExpiresIn: exp, Email: user.Email})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidPayload.Error())
		return
	}
	user, tok, exp, err := a.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeAuthErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: tok, ExpiresIn: exp, Email: user.Email})
}

type createRequest struct {
	URL       string  `json:"url"`
	Alias     string  `json:"alias"`
	ExpiresAt *string `json:"expires_at"`
}

type createResponse struct {
	ShortCode string `json:"short_code"`
	ShortURL  string `json:"short_url"`
}

type linkView struct {
	ShortCode   string  `json:"short_code"`
	ShortURL    string  `json:"short_url"`
	OriginalURL string  `json:"original_url"`
	IsActive    bool    `json:"is_active"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

func (a *API) createLink(w http.ResponseWriter, r *http.Request) {
	uid, _ := mw.UserID(r.Context())
	var req createRequest
	if err := decodeJSON(r, &req); err != nil {
		observability.CreateErrors.Inc()
		writeError(w, http.StatusBadRequest, domain.ErrInvalidPayload.Error())
		return
	}
	var exp *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			observability.CreateErrors.Inc()
			writeError(w, http.StatusBadRequest, domain.ErrInvalidExpiry.Error())
			return
		}
		utc := t.UTC()
		exp = &utc
	}
	link, err := a.svc.Create(r.Context(), service.CreateInput{
		OwnerID:   uid,
		URL:       req.URL,
		Alias:     req.Alias,
		ExpiresAt: exp,
	})
	if err != nil {
		observability.CreateErrors.Inc()
		writeCreateErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, createResponse{
		ShortCode: link.ShortCode,
		ShortURL:  a.svc.ShortURL(link.ShortCode),
	})
}

func (a *API) listLinks(w http.ResponseWriter, r *http.Request) {
	uid, _ := mw.UserID(r.Context())
	links, err := a.svc.List(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list links")
		return
	}
	views := make([]linkView, 0, len(links))
	for _, l := range links {
		views = append(views, a.toView(l))
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": views})
}

func (a *API) redirect(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() { observability.RedirectDuration.Observe(time.Since(start).Seconds()) }()

	code := chi.URLParam(r, "code")
	link, err := a.svc.Resolve(r.Context(), code)
	if err != nil {
		statusForResolve(w, err)
		return
	}
	a.clicks.Publish(r.Context(), domain.ClickEvent{
		LinkID:    link.ID,
		ClickedAt: time.Now().UTC(),
		IPHash:    domain.HashIP(a.ipSalt, mw.ClientIP(r)),
		UserAgent: r.UserAgent(),
		Referer:   r.Referer(),
	})
	http.Redirect(w, r, link.OriginalURL, http.StatusFound)
}

func (a *API) analytics(w http.ResponseWriter, r *http.Request) {
	uid, _ := mw.UserID(r.Context())
	code := chi.URLParam(r, "code")
	stats, err := a.svc.Analytics(r.Context(), uid, code)
	if err != nil {
		writeOwnerErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (a *API) deactivate(w http.ResponseWriter, r *http.Request) {
	uid, _ := mw.UserID(r.Context())
	code := chi.URLParam(r, "code")
	if err := a.svc.Deactivate(r.Context(), uid, code); err != nil {
		writeOwnerErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	_ = r
	dbOK := a.pingDB() == nil
	cacheOK := a.pingCache() == nil
	status := http.StatusOK
	if !dbOK || !cacheOK {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{
		"status": map[string]string{
			"app":   "ok",
			"db":    ready(dbOK),
			"cache": ready(cacheOK),
		},
	})
}

func (a *API) toView(l domain.Link) linkView {
	v := linkView{
		ShortCode:   l.ShortCode,
		ShortURL:    a.svc.ShortURL(l.ShortCode),
		OriginalURL: l.OriginalURL,
		IsActive:    l.IsActive,
		CreatedAt:   l.CreatedAt.UTC().Format(time.RFC3339),
	}
	if l.ExpiresAt != nil {
		s := l.ExpiresAt.UTC().Format(time.RFC3339)
		v.ExpiresAt = &s
	}
	return v
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}

func ready(ok bool) string {
	if ok {
		return "ok"
	}
	return "down"
}

func statusForResolve(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInactive), errors.Is(err, domain.ErrExpired):
		writeError(w, http.StatusGone, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "could not redirect")
	}
}

func writeCreateErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidURL), errors.Is(err, domain.ErrInvalidAlias), errors.Is(err, domain.ErrInvalidExpiry):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrCodeCollision):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "could not create link")
	}
}

func writeOwnerErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "request failed")
	}
}

func writeAuthErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrEmailTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrWeakPassword), errors.Is(err, domain.ErrInvalidPayload):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "auth failed")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
