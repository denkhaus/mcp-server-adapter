// Package mcpadapter provides types for the simplified MCP adapter.
package mcpadapter

import "time"

// Config represents the complete adapter configuration.
type Config struct {
	McpServers map[string]*ServerConfig `json:"mcpServers"`
}

// ServerConfig represents configuration for a single MCP server.
type ServerConfig struct {
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Disabled    bool              `json:"disabled,omitempty"`
	AlwaysAllow []string          `json:"alwaysAllow,omitempty"`

	// Extended fields for transport abstraction
	Transport string            `json:"transport,omitempty"`
	URL       string            `json:"url,omitempty"`
	Method    string            `json:"method,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timeout   time.Duration     `json:"timeout,omitempty"`
}

// ServerStatus represents the current status of a server.
type ServerStatus int

const (
	StatusStopped ServerStatus = iota
	StatusStarting
	StatusRunning
	StatusError
)

func (s ServerStatus) String() string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusStarting:
		return "starting"
	case StatusRunning:
		return "running"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

