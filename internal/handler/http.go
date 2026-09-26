package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/mitsuoleo/encurta/internal/domain"
	mw "github.com/mitsuoleo/encurta/internal/middleware"
	"github.com/mitsuoleo/encurta/internal/observability"
	"github.com/mitsuoleo/encurta/internal/service"
	"github.com/mitsuoleo/encurta/internal/web"
	"github.com/mitsuoleo/encurta/internal/worker"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type API struct {
	svc          *service.LinkService
	auth         *service.AuthService
	clicks       worker.Publisher
	ipSalt       string
	log          *slog.Logger
	pingDB       func() error
	pingCache    func() error
	limiter      mw.Limiter
	metricsToken string
	trusted      []*net.IPNet
}

func New(svc *service.LinkService, auth *service.AuthService, clicks worker.Publisher, ipSalt string, log *slog.Logger, pingDB, pingCache func() error, limiter mw.Limiter, metricsToken string, trusted []*net.IPNet) *API {
	return &API{
		svc:          svc,
		auth:         auth,
		clicks:       clicks,
		ipSalt:       ipSalt,
		log:          log,
		pingDB:       pingDB,
		pingCache:    pingCache,
		limiter:      limiter,
		metricsToken: metricsToken,
		trusted:      trusted,
	}
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(mw.Recoverer)
	r.Use(mw.SecurityHeaders)
	r.Use(mw.JSONLogger(a.log))

	r.Get("/", a.ui)
	r.Get("/ui/{asset}", a.uiAsset)
	r.Get("/openapi.yaml", a.openapi)
	r.Get("/health", a.health)
	r.Handle("/metrics", a.protectMetrics(promhttp.Handler()))
	r.With(mw.RateLimitAuth(a.limiter, a.log)).Post("/auth/register", a.register)
	r.With(mw.RateLimitAuth(a.limiter, a.log)).Post("/auth/login", a.login)
	r.Post("/auth/logout", a.logout)

	r.Route("/links", func(r chi.Router) {
		r.Use(mw.RequireAuth(a.auth))
		r.With(mw.RateLimitCreates(a.limiter, a.log)).Post("/", a.createLink)
		r.Get("/", a.listLinks)
		r.Get("/{code}/analytics", a.analytics)
		r.Patch("/{code}", a.updateLink)
		r.Delete("/{code}", a.deactivate)
	})
	r.With(mw.RateLimitRedirects(a.limiter, a.log)).Get("/{code}", a.redirect)
	return r
}

func (a *API) ui(w http.ResponseWriter, r *http.Request) {
	_ = r
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(web.Index)
}

func (a *API) uiAsset(w http.ResponseWriter, r *http.Request) {
	asset := chi.URLParam(r, "asset")
	var contentType string
	switch asset {
	case "app.css":
		contentType = "text/css; charset=utf-8"
	case "app.js", "logic.js", "qr.js":
		contentType = "text/javascript; charset=utf-8"
	default:
		http.NotFound(w, r)
		return
	}
	data, err := web.Assets.ReadFile(asset)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(data)
}

func (a *API) openapi(w http.ResponseWriter, r *http.Request) {
	_ = r
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(web.OpenAPI)
}

func (a *API) protectMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.metricsToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		got := r.Header.Get("Authorization")
		if got != "Bearer "+a.metricsToken {
			writeError(w, http.StatusUnauthorized, domain.ErrUnauthorized.Error())
			return
		}
		next.ServeHTTP(w, r)
	})
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

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	_ = r
	w.WriteHeader(http.StatusNoContent)
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
	limit := atoiDefault(r.URL.Query().Get("limit"), service.DefaultListLimit)
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)
	page, err := a.svc.List(r.Context(), uid, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list links")
		return
	}
	views := make([]linkView, 0, len(page.Links))
	for _, l := range page.Links {
		views = append(views, a.toView(l))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"links":  views,
		"total":  page.Total,
		"limit":  page.Limit,
		"offset": page.Offset,
	})
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
	ua, referer := domain.SanitizeClickMeta(r.UserAgent(), r.Referer())
	device, browser := domain.ClassifyUserAgent(ua)
	a.clicks.Publish(context.WithoutCancel(r.Context()), domain.ClickEvent{
		LinkID:    link.ID,
		ClickedAt: time.Now().UTC(),
		IPHash:    domain.HashIP(a.ipSalt, mw.ClientIP(r, a.trusted)),
		UserAgent: ua,
		Referer:   referer,
		Device:    device,
		Browser:   browser,
	})
	w.Header().Set("Cache-Control", "no-store, private")
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

type updateRequest struct {
	URL       *string `json:"url"`
	ExpiresAt *string `json:"expires_at"`
	IsActive  *bool   `json:"is_active"`
}

func (a *API) updateLink(w http.ResponseWriter, r *http.Request) {
	uid, _ := mw.UserID(r.Context())
	code := chi.URLParam(r, "code")
	var req updateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidPayload.Error())
		return
	}
	in := service.UpdateInput{URL: req.URL, IsActive: req.IsActive}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			in.ClearExpiry = true
		} else {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				writeError(w, http.StatusBadRequest, domain.ErrInvalidExpiry.Error())
				return
			}
			utc := t.UTC()
			in.ExpiresAt = &utc
		}
	}
	link, err := a.svc.Update(r.Context(), uid, code, in)
	if err != nil {
		writeOwnerErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.toView(link))
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

func atoiDefault(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func ready(ok bool) string {
	if ok {
		return "ok"
	}
	return "down"
}

func statusForResolve(w http.ResponseWriter, err error) {
	w.Header().Set("Cache-Control", "no-store, private")
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
	case errors.Is(err, domain.ErrCacheUnavailable):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "could not create link")
	}
}

func writeOwnerErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusNotFound, domain.ErrNotFound.Error())
	case errors.Is(err, domain.ErrInvalidURL), errors.Is(err, domain.ErrInvalidExpiry):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrCacheUnavailable):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "request failed")
	}
}

func writeAuthErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrEmailTaken):
		writeError(w, http.StatusBadRequest, "could not register")
	case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrWeakPassword), errors.Is(err, domain.ErrPasswordTooLong), errors.Is(err, domain.ErrInvalidPayload):
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
