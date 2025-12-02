package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Type    string         `yaml:"type"` // client, backend, or both
	Client  *ClientConfig  `yaml:"client,omitempty"`
	Backend *BackendConfig `yaml:"backend,omitempty"`
	Logging LoggingConfig  `yaml:"logging"`
}

// ClientConfig holds client-specific configuration
type ClientConfig struct {
	Endpoints              []EndpointConfig `yaml:"endpoints"`
	Timeout                time.Duration    `yaml:"timeout"`
	Interval               time.Duration    `yaml:"interval"` // Time between requests
	MaxConcurrentRequests  int              `yaml:"max_concurrent_requests,omitempty"` // Max concurrent requests per endpoint (0 = unlimited)
}

// EndpointConfig defines an HTTP endpoint to call
type EndpointConfig struct {
	Name             string            `yaml:"name"`
	URL              string            `yaml:"url"`
	Method           string            `yaml:"method"`
	Headers          map[string]string `yaml:"headers,omitempty"`
	Body             string            `yaml:"body,omitempty"`
	Retries          int               `yaml:"retries"`
	RequestsPerSecond float64          `yaml:"requests_per_second,omitempty"` // Rate limit: N requests per second
}

// BackendConfig holds backend server configuration
type BackendConfig struct {
	Port      int               `yaml:"port"`
	Endpoints []BackendEndpoint `yaml:"endpoints"`
}

// BackendEndpoint defines how the server should respond to requests
type BackendEndpoint struct {
	Path            string            `yaml:"path"`
	Method          string            `yaml:"method"`
	StatusCode      int               `yaml:"status_code"`
	Headers         map[string]string `yaml:"headers,omitempty"`
	Body            string            `yaml:"body,omitempty"`
	Delay           time.Duration     `yaml:"delay,omitempty"`      // Artificial delay
	DropPercent     float64           `yaml:"drop_percent,omitempty"`    // Percentage of connections to drop (0-100)
	IdlePercent     float64           `yaml:"idle_percent,omitempty"`    // Percentage of connections to leave idle (0-100)
	IdleDuration    time.Duration     `yaml:"idle_duration,omitempty"`   // How long to keep idle connections open
}

// LoggingConfig controls logging behavior
type LoggingConfig struct {
	Level   string `yaml:"level"`   // debug, info, warn, error
	Verbose bool   `yaml:"verbose"` // Include detailed diagnostics
}

// LoadConfig reads and parses the configuration file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// validateConfig ensures the configuration is valid
func validateConfig(config *Config) error {
	// Validate type
	if config.Type != "client" && config.Type != "backend" && config.Type != "both" {
		return fmt.Errorf("type must be 'client', 'backend', or 'both', got: %s", config.Type)
	}

	// Validate client config if needed
	if config.Type == "client" || config.Type == "both" {
		if config.Client == nil {
			return fmt.Errorf("client configuration is required when type is '%s'", config.Type)
		}
		if len(config.Client.Endpoints) == 0 {
			return fmt.Errorf("at least one client endpoint must be defined")
		}
		for i, ep := range config.Client.Endpoints {
			if ep.URL == "" {
				return fmt.Errorf("endpoint %d: URL is required", i)
			}
			if ep.Method == "" {
				config.Client.Endpoints[i].Method = "GET"
			}
		}
		// MaxConcurrentRequests defaults to 0 (unlimited) if not specified
	}

	// Validate backend config if needed
	if config.Type == "backend" || config.Type == "both" {
		if config.Backend == nil {
			return fmt.Errorf("backend configuration is required when type is '%s'", config.Type)
		}
		if config.Backend.Port == 0 {
			config.Backend.Port = 8080 // Default port
		}
		if len(config.Backend.Endpoints) == 0 {
			return fmt.Errorf("at least one backend endpoint must be defined")
		}
		for i, ep := range config.Backend.Endpoints {
			if ep.Path == "" {
				return fmt.Errorf("backend endpoint %d: path is required", i)
			}
			if ep.Method == "" {
				config.Backend.Endpoints[i].Method = "GET"
			}
			if ep.StatusCode == 0 {
				config.Backend.Endpoints[i].StatusCode = 200
			}
			// Validate percentages
			if ep.DropPercent < 0 || ep.DropPercent > 100 {
				return fmt.Errorf("backend endpoint %d: drop_percent must be between 0 and 100", i)
			}
			if ep.IdlePercent < 0 || ep.IdlePercent > 100 {
				return fmt.Errorf("backend endpoint %d: idle_percent must be between 0 and 100", i)
			}
			if ep.DropPercent + ep.IdlePercent > 100 {
				return fmt.Errorf("backend endpoint %d: drop_percent + idle_percent cannot exceed 100", i)
			}
			// Set default idle duration if idle_percent is set
			if ep.IdlePercent > 0 && ep.IdleDuration == 0 {
				config.Backend.Endpoints[i].IdleDuration = 30 * time.Second
			}
		}
	}

	// Set default logging level
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}

	return nil
}
