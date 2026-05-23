package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorouter/gorouter/internal/metrics"
)

func MetricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip metrics endpoint itself
			if r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			metrics.ActiveRequests.Inc()
			defer metrics.ActiveRequests.Dec()

			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &statusWriter{ResponseWriter: w, statusCode: 200}
			next.ServeHTTP(wrapped, r)

			duration := time.Since(start).Seconds()
			status := strconv.Itoa(wrapped.statusCode)
			endpoint := normalizeEndpoint(r.URL.Path)
			method := r.Method

			metrics.RequestsTotal.WithLabelValues(method, endpoint, status).Inc()
			metrics.RequestDuration.WithLabelValues(method, endpoint).Observe(duration)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// normalizeEndpoint reduces path variations to avoid high cardinality labels
func normalizeEndpoint(path string) string {
	// Keep most paths as-is for now, could add templating later for dynamic routes
	return path
}