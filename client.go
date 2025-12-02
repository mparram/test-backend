package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"time"
)

// Client represents the HTTP client component
type Client struct {
	config  *ClientConfig
	client  *http.Client
	logger  *Logger
	metrics *Metrics
}

// NewClient creates a new HTTP client
func NewClient(config *ClientConfig, logger *Logger, metrics *Metrics) *Client {
	return &Client{
		config:  config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger:  logger,
		metrics: metrics,
	}
}

// Run starts the client and makes requests to configured endpoints
func (c *Client) Run(ctx context.Context) error {
	c.logger.Info("Starting HTTP client...")

	// Start a goroutine for each endpoint to handle rate limiting independently
	for _, endpoint := range c.config.Endpoints {
		go c.runEndpoint(ctx, endpoint)
	}

	// Wait for context cancellation
	<-ctx.Done()
	c.logger.Info("Client shutting down...")
	return ctx.Err()
}

// runEndpoint handles requests for a single endpoint with rate limiting
func (c *Client) runEndpoint(ctx context.Context, endpoint EndpointConfig) {
	// Create semaphore to limit concurrent requests (only if limit is set)
	var semaphore chan struct{}
	if c.config.MaxConcurrentRequests > 0 {
		semaphore = make(chan struct{}, c.config.MaxConcurrentRequests)
	}
	
	// Helper function to launch request with optional semaphore control
	launchRequest := func() {
		if semaphore != nil {
			// With concurrency limit
			go func() {
				semaphore <- struct{}{}        // Acquire semaphore
				defer func() { <-semaphore }() // Release semaphore
				c.makeRequest(ctx, endpoint)
			}()
		} else {
			// Unlimited concurrency
			go c.makeRequest(ctx, endpoint)
		}
	}
	
	if endpoint.RequestsPerSecond > 0 {
		// Rate-limited mode: N requests per second
		interval := time.Duration(float64(time.Second) / endpoint.RequestsPerSecond)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		if c.config.MaxConcurrentRequests > 0 {
			c.logger.Info("Endpoint [%s] configured for %.2f requests/second (max %d concurrent)", 
				endpoint.Name, endpoint.RequestsPerSecond, c.config.MaxConcurrentRequests)
		} else {
			c.logger.Info("Endpoint [%s] configured for %.2f requests/second (unlimited concurrent)", 
				endpoint.Name, endpoint.RequestsPerSecond)
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				launchRequest()
			}
		}
	} else {
		// Interval-based mode: use global interval
		ticker := time.NewTicker(c.config.Interval)
		defer ticker.Stop()

		if c.config.MaxConcurrentRequests > 0 {
			c.logger.Info("Endpoint [%s] configured with interval %v (max %d concurrent)", 
				endpoint.Name, c.config.Interval, c.config.MaxConcurrentRequests)
		} else {
			c.logger.Info("Endpoint [%s] configured with interval %v (unlimited concurrent)", 
				endpoint.Name, c.config.Interval)
		}

		// Make initial request immediately
		launchRequest()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				launchRequest()
			}
		}
	}
}

// makeRequest executes a single HTTP request with diagnostics
func (c *Client) makeRequest(ctx context.Context, endpoint EndpointConfig) {
	attempts := 0
	maxAttempts := endpoint.Retries + 1

	for attempts < maxAttempts {
		attempts++

		if err := c.executeRequest(ctx, endpoint, attempts); err != nil {
			c.logger.Error("Request failed [%s] (attempt %d/%d): %v",
				endpoint.Name, attempts, maxAttempts, err)

			// Track retry metrics
			if attempts > 1 {
				c.metrics.ClientRetries.WithLabelValues(endpoint.Name, endpoint.Method).Inc()
			}

			if attempts < maxAttempts {
				time.Sleep(time.Second * time.Duration(attempts))
				continue
			}
		} else {
			break
		}
	}
}

// executeRequest performs the actual HTTP request with detailed diagnostics
func (c *Client) executeRequest(ctx context.Context, endpoint EndpointConfig, attempt int) error {
	start := time.Now()

	// Create request
	var bodyReader io.Reader
	if endpoint.Body != "" {
		bodyReader = bytes.NewBufferString(endpoint.Body)
	}

	req, err := http.NewRequestWithContext(ctx, endpoint.Method, endpoint.URL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	// Add trace for detailed diagnostics
	var dnsStart, connectStart, tlsStart time.Time
	var dnsDuration, connectDuration, tlsDuration, ttfbDuration time.Duration

	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			dnsDuration = time.Since(dnsStart)
		},
		ConnectStart: func(_, _ string) {
			connectStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			connectDuration = time.Since(connectStart)
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			tlsDuration = time.Since(tlsStart)
		},
		GotFirstResponseByte: func() {
			ttfbDuration = time.Since(start)
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	// Execute request
	c.logger.Info("→ [%s] %s %s (attempt %d)", endpoint.Name, endpoint.Method, endpoint.URL, attempt)

	resp, err := c.client.Do(req)
	if err != nil {
		// Track error metrics
		c.metrics.ClientRequestErrors.WithLabelValues(endpoint.Name, endpoint.Method, "request_failed").Inc()
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// Track error metrics
		c.metrics.ClientRequestErrors.WithLabelValues(endpoint.Name, endpoint.Method, "read_body_failed").Inc()
		return fmt.Errorf("failed to read response body: %w", err)
	}

	totalDuration := time.Since(start)

	// Track metrics
	c.metrics.ClientRequestsTotal.WithLabelValues(endpoint.Name, endpoint.Method, fmt.Sprintf("%d", resp.StatusCode)).Inc()
	c.metrics.ClientRequestDuration.WithLabelValues(endpoint.Name, endpoint.Method).Observe(totalDuration.Seconds())

	if dnsDuration > 0 {
		c.metrics.ClientDNSDuration.WithLabelValues(endpoint.Name).Observe(dnsDuration.Seconds())
	}
	if connectDuration > 0 {
		c.metrics.ClientTCPDuration.WithLabelValues(endpoint.Name).Observe(connectDuration.Seconds())
	}
	if tlsDuration > 0 {
		c.metrics.ClientTLSDuration.WithLabelValues(endpoint.Name).Observe(tlsDuration.Seconds())
	}
	if ttfbDuration > 0 {
		c.metrics.ClientTTFBDuration.WithLabelValues(endpoint.Name).Observe(ttfbDuration.Seconds())
	}

	// Log response
	c.logger.Info("← [%s] Status: %d, Size: %d bytes, Duration: %v",
		endpoint.Name, resp.StatusCode, len(body), totalDuration)

	// Log detailed diagnostics if verbose
	if c.logger.verbose {
		c.logger.Debug("  Diagnostics for [%s]:", endpoint.Name)
		if dnsDuration > 0 {
			c.logger.Debug("    DNS Lookup: %v", dnsDuration)
		}
		if connectDuration > 0 {
			c.logger.Debug("    TCP Connect: %v", connectDuration)
		}
		if tlsDuration > 0 {
			c.logger.Debug("    TLS Handshake: %v", tlsDuration)
		}
		if ttfbDuration > 0 {
			c.logger.Debug("    Time to First Byte: %v", ttfbDuration)
		}
		c.logger.Debug("    Total Time: %v", totalDuration)

		// Log response headers
		c.logger.Debug("  Response Headers:")
		for key, values := range resp.Header {
			for _, value := range values {
				c.logger.Debug("    %s: %s", key, value)
			}
		}

		// Log response body (truncated if too long)
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "... (truncated)"
		}
		if len(bodyStr) > 0 {
			c.logger.Debug("  Response Body: %s", bodyStr)
		}
	}

	return nil
}
