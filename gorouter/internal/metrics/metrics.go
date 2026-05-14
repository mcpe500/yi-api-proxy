package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var RequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	},
	[]string{"method", "endpoint", "status"},
)

var RequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"method", "endpoint"},
)

var ActiveRequests = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "http_active_requests",
		Help: "Number of currently active HTTP requests.",
	},
)

var ProviderRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "provider_requests_total",
		Help: "Total number of requests to upstream providers.",
	},
	[]string{"provider", "model", "status"},
)

var ProviderLatency = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "provider_latency_seconds",
		Help:    "Upstream provider request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"provider"},
)
