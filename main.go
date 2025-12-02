package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "config/config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	config, err := LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Create logger
	logger := NewLogger(config.Logging)
	logger.Info("=== HTTP/TCP Troubleshooting Tool ===")
	logger.Info("Mode: %s", config.Type)

	// Initialize Prometheus metrics
	metrics := NewMetrics()
	logger.Info("Prometheus metrics initialized")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Error channel for component errors
	errChan := make(chan error, 2)

	// Start components based on configuration type
	switch config.Type {
	case "client":
		go runClient(ctx, config, logger, metrics, errChan)

	case "backend":
		go runBackend(ctx, config, logger, metrics, errChan)

	case "both":
		go runClient(ctx, config, logger, metrics, errChan)
		go runBackend(ctx, config, logger, metrics, errChan)
	}

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		logger.Info("Received signal: %v", sig)
		cancel()
	case err := <-errChan:
		logger.Error("Component error: %v", err)
		cancel()
	}

	logger.Info("Shutting down gracefully...")
}

// runClient starts the HTTP client component
func runClient(ctx context.Context, config *Config, logger *Logger, metrics *Metrics, errChan chan<- error) {
	client := NewClient(config.Client, logger, metrics)
	if err := client.Run(ctx); err != nil && err != context.Canceled {
		errChan <- fmt.Errorf("client error: %w", err)
	}
}

// runBackend starts the HTTP backend server component
func runBackend(ctx context.Context, config *Config, logger *Logger, metrics *Metrics, errChan chan<- error) {
	backend := NewBackend(config.Backend, logger, metrics)
	
	// Add metrics endpoint to backend
	backend.metricsHandler = promhttp.Handler()
	
	if err := backend.Run(ctx); err != nil && err != context.Canceled {
		errChan <- fmt.Errorf("backend error: %w", err)
	}
}
