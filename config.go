// Package mcpadapter provides configuration management for the simplified MCP adapter.
package mcpadapter

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// loadConfig loads configuration from a JSON file.
func loadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path cannot be empty")
	}

	// #nosec G304
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Validate and set defaults
	if err := validateAndSetDefaults(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// validateAndSetDefaults validates the configuration and sets default values.
func validateAndSetDefaults(config *Config) error {
	if config.McpServers == nil {
		return fmt.Errorf("mcpServers configuration is required")
	}

	if len(config.McpServers) == 0 {
		return fmt.Errorf("at least one server must be configured")
	}

	for name, serverConfig := range config.McpServers {
		if err := validateServerConfig(name, serverConfig); err != nil {
			return err
		}
		setServerDefaults(serverConfig)
	}

	return nil
}

// validateServerConfig validates a single server configuration.
func validateServerConfig(name string, config *ServerConfig) error {
	transport := config.Transport
	if transport == "" {
		transport = TransportStdio // Default transport
	}

	switch transport {
	case TransportStdio:
		if config.Command == "" {
			return fmt.Errorf("mcpServers.%s.command is required for stdio transport", name)
		}
	case TransportSSE, TransportHTTP:
		if config.URL == "" {
			return fmt.Errorf("mcpServers.%s.url is required for %s transport", name, transport)
		}
	default:
		return fmt.Errorf("mcpServers.%s.transport: unsupported transport type: %s", name, transport)
	}

	return nil
}

// setServerDefaults sets default values for server configuration.
func setServerDefaults(config *ServerConfig) {
	// Default to stdio transport if not specified
	if config.Transport == "" {
		config.Transport = TransportStdio
	}

	// Set default timeout if not specified
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}

	if config.Env == nil {
		config.Env = make(map[string]string)
	}

	if config.AlwaysAllow == nil {
		config.AlwaysAllow = make([]string, 0)
	}

	// Set default HTTP method for HTTP transport
	if config.Transport == "http" && config.Method == "" {
		config.Method = "POST"
	}
}
