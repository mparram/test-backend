package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Backend represents the HTTP server component
type Backend struct {
	config         *BackendConfig
	server         *http.Server
	logger         *Logger
	metrics        *Metrics
	metricsHandler http.Handler
}

// NewBackend creates a new HTTP backend server
func NewBackend(config *BackendConfig, logger *Logger, metrics *Metrics) *Backend {
	return &Backend{
		config:  config,
		logger:  logger,
		metrics: metrics,
	}
}

// Run starts the HTTP server
func (b *Backend) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	// Register metrics endpoint
	if b.metricsHandler != nil {
		mux.Handle("/metrics", b.metricsHandler)
		b.logger.Info("Registering Prometheus metrics endpoint: /metrics")
	}

	// Register all configured endpoints
	for _, endpoint := range b.config.Endpoints {
		b.registerEndpoint(mux, endpoint)
	}

	b.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", b.config.Port),
		Handler: b.loggingMiddleware(mux),
	}

	b.logger.Info("Starting HTTP backend server on port %d...", b.config.Port)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := b.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		b.logger.Info("Backend shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return b.server.Shutdown(shutdownCtx)
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}
}

// registerEndpoint registers a single endpoint handler
func (b *Backend) registerEndpoint(mux *http.ServeMux, endpoint BackendEndpoint) {
	handler := b.createHandler(endpoint)

	pattern := endpoint.Path
	b.logger.Info("Registering endpoint: %s %s -> Status %d",
		endpoint.Method, endpoint.Path, endpoint.StatusCode)

	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		// Check if method matches
		if r.Method != endpoint.Method {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	})
}

// createHandler creates a handler function for an endpoint
func (b *Backend) createHandler(endpoint BackendEndpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Simulate connection drop or idle based on percentages
		if endpoint.DropPercent > 0 || endpoint.IdlePercent > 0 {
			// Generate random number 0-100
			random := float64(time.Now().UnixNano()%10000) / 100.0

			if random < endpoint.DropPercent {
				// Drop connection: close without response
				b.logger.Warn("Dropping connection for %s %s (%.1f%% drop rate)", r.Method, r.URL.Path, endpoint.DropPercent)
				// Track drop metrics
				b.metrics.BackendDroppedTotal.WithLabelValues(r.URL.Path, r.Method).Inc()
				// Get underlying connection and close it
				if hj, ok := w.(http.Hijacker); ok {
					conn, _, err := hj.Hijack()
					if err == nil {
						conn.Close()
						return
					}
				}
				// Fallback: just return without writing anything
				return
			} else if random < (endpoint.DropPercent + endpoint.IdlePercent) {
				// Idle connection: keep open but don't respond
				idleDuration := endpoint.IdleDuration
				if idleDuration == 0 {
					idleDuration = 30 * time.Second
				}
				b.logger.Warn("Idling connection for %s %s for %v (%.1f%% idle rate)", 
					r.Method, r.URL.Path, idleDuration, endpoint.IdlePercent)
				// Track idle metrics
				b.metrics.BackendIdledTotal.WithLabelValues(r.URL.Path, r.Method).Inc()
				b.metrics.BackendIdleDuration.WithLabelValues(r.URL.Path, r.Method).Observe(idleDuration.Seconds())
				time.Sleep(idleDuration)
				// After idle, close without response
				if hj, ok := w.(http.Hijacker); ok {
					conn, _, err := hj.Hijack()
					if err == nil {
						conn.Close()
						return
					}
				}
				return
			}
		}

		// Normal response flow
		// Apply artificial delay if configured
		if endpoint.Delay > 0 {
			time.Sleep(endpoint.Delay)
		}

		// Set response headers
		for key, value := range endpoint.Headers {
			w.Header().Set(key, value)
		}

		// Set status code
		w.WriteHeader(endpoint.StatusCode)

		// Write response body
		if endpoint.Body != "" {
			w.Write([]byte(endpoint.Body))
		}

		duration := time.Since(start)
		
		// Track metrics
		b.metrics.BackendRequestsTotal.WithLabelValues(r.URL.Path, r.Method, fmt.Sprintf("%d", endpoint.StatusCode)).Inc()
		b.metrics.BackendRequestDuration.WithLabelValues(r.URL.Path, r.Method).Observe(duration.Seconds())
		if endpoint.Body != "" {
			b.metrics.BackendResponseSize.WithLabelValues(r.URL.Path, r.Method).Observe(float64(len(endpoint.Body)))
		}

		b.logger.Debug("Handled %s %s -> %d (took %v)",
			r.Method, r.URL.Path, endpoint.StatusCode, duration)
	}
}

// loggingMiddleware logs all incoming requests
func (b *Backend) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log request
		b.logger.Info("← %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		// Log request headers if verbose
		if b.logger.verbose {
			b.logger.Debug("  Request Headers:")
			for key, values := range r.Header {
				for _, value := range values {
					b.logger.Debug("    %s: %s", key, value)
				}
			}
		}

		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		b.logger.Info("→ %s %s -> %d (took %v)",
			r.Method, r.URL.Path, wrapped.statusCode, duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
