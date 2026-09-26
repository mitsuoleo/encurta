package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RedirectDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "encurta_redirect_duration_seconds",
		Help:    "Redirect handler latency.",
		Buckets: prometheus.DefBuckets,
	})
	CacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "encurta_cache_hits_total",
		Help: "Redirect cache hits.",
	})
	CacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "encurta_cache_misses_total",
		Help: "Redirect cache misses.",
	})
	CreateErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "encurta_link_create_errors_total",
		Help: "Failed POST /links.",
	})
	ClicksDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "encurta_clicks_dropped_total",
		Help: "Click events dropped because Redis XADD failed.",
	})
)
