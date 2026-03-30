package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oci_http_requests_total",
		Help: "Total HTTP requests by route, method, and status code.",
	}, []string{"route", "method", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "oci_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
	}, []string{"route", "method"})

	httpActiveRequests = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "oci_http_active_requests",
		Help: "Number of HTTP requests currently being served.",
	})
)

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher for SSE streaming support.
func (sr *statusRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// instrumentHandler wraps an http.HandlerFunc with Prometheus request metrics.
func instrumentHandler(route string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		httpActiveRequests.Inc()

		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		handler.ServeHTTP(rec, r)

		httpActiveRequests.Dec()
		httpRequestsTotal.WithLabelValues(route, r.Method, strconv.Itoa(rec.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
	}
}
