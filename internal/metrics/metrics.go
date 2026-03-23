package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Security metrics group
var Security = struct {
	LoginAttempts *prometheus.CounterVec
	ActiveBlocks  prometheus.Gauge
}{
	LoginAttempts: promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_login_attempts_total",
		Help: "Total login attempts by status (allowed, blocked, failed)",
	}, []string{"status"}),

	ActiveBlocks: promauto.NewGauge(prometheus.GaugeOpts{
		Name: "auth_active_ip_blocks",
		Help: "Number of IPs currently in a hard-block state",
	}),
}

// Redis metrics group
var Redis = struct {
	ConnErrors prometheus.Counter
}{
	ConnErrors: promauto.NewCounter(prometheus.CounterOpts{
		Name: "auth_redis_errors_total",
		Help: "Total Redis connection failures in the auth layer",
	}),
}

var (
	// HTTPRequests tracks Total Number of HTTP Requests (CounterVec)
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests by method, path, and status",
	}, []string{"method", "path", "status"})

	// HTTPDuration tracks Request Latency (HistogramVec)
	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Latency of HTTP requests in seconds",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"method", "path"})
)
