package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	// Client metrics
	ClientRequestsTotal     *prometheus.CounterVec
	ClientRequestDuration   *prometheus.HistogramVec
	ClientRequestErrors     *prometheus.CounterVec
	ClientDNSDuration       *prometheus.HistogramVec
	ClientTCPDuration       *prometheus.HistogramVec
	ClientTLSDuration       *prometheus.HistogramVec
	ClientTTFBDuration      *prometheus.HistogramVec
	ClientRetries           *prometheus.CounterVec

	// Backend metrics
	BackendRequestsTotal    *prometheus.CounterVec
	BackendRequestDuration  *prometheus.HistogramVec
	BackendResponseSize     *prometheus.HistogramVec
	BackendDroppedTotal     *prometheus.CounterVec
	BackendIdledTotal       *prometheus.CounterVec
	BackendIdleDuration     *prometheus.HistogramVec
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	return &Metrics{
		// Client metrics
		ClientRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_client_requests_total",
				Help: "Total number of HTTP requests made by the client",
			},
			[]string{"endpoint", "method", "status_code"},
		),
		ClientRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_client_request_duration_seconds",
				Help:    "HTTP client request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"endpoint", "method"},
		),
		ClientRequestErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_client_request_errors_total",
				Help: "Total number of HTTP client request errors",
			},
			[]string{"endpoint", "method", "error_type"},
		),
		ClientDNSDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_client_dns_duration_seconds",
				Help:    "DNS lookup duration in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"endpoint"},
		),
		ClientTCPDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_client_tcp_duration_seconds",
				Help:    "TCP connection duration in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"endpoint"},
		),
		ClientTLSDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_client_tls_duration_seconds",
				Help:    "TLS handshake duration in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"endpoint"},
		),
		ClientTTFBDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_client_ttfb_duration_seconds",
				Help:    "Time to first byte duration in seconds",
				Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"endpoint"},
		),
		ClientRetries: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_client_retries_total",
				Help: "Total number of request retries",
			},
			[]string{"endpoint", "method"},
		),

		// Backend metrics
		BackendRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_backend_requests_total",
				Help: "Total number of HTTP requests received by the backend",
			},
			[]string{"path", "method", "status_code"},
		),
		BackendRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_backend_request_duration_seconds",
				Help:    "HTTP backend request processing duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"path", "method"},
		),
		BackendResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_backend_response_size_bytes",
				Help:    "HTTP backend response size in bytes",
				Buckets: []float64{10, 100, 1000, 10000, 100000, 1000000},
			},
			[]string{"path", "method"},
		),
		BackendDroppedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_backend_dropped_connections_total",
				Help: "Total number of dropped connections",
			},
			[]string{"path", "method"},
		),
		BackendIdledTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_backend_idled_connections_total",
				Help: "Total number of idled connections",
			},
			[]string{"path", "method"},
		),
		BackendIdleDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_backend_idle_duration_seconds",
				Help:    "Duration connections were kept idle in seconds",
				Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
			},
			[]string{"path", "method"},
		),
	}
}
