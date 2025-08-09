package mcpadapter

import (
	"context"
	"time"

	"github.com/tmc/langchaingo/tools"
)

// MCPAdapter defines the interface for MCP adapter operations.
type MCPAdapter interface {
	// Server lifecycle management
	StartServer(ctx context.Context, serverName string) error
	StartAllServers(ctx context.Context) error
	StopServer(serverName string) error
	Close() error

	// Status monitoring
	GetServerStatusByName(serverName string) ServerStatus
	GetAllServerStatuses() map[string]ServerStatus
	WaitForServersReady(ctx context.Context, timeout time.Duration) error

	// Tool access
	GetToolsByServerName(ctx context.Context, serverName string) ([]tools.Tool, error)
	GetAllTools(ctx context.Context) ([]tools.Tool, error)

	// Configuration
	IsConfigWatcherRunning() bool
	IsServerDisabled(serverName string) (bool, error)

	GetConfig() *Config
	GetServerConfig(serverName string) (*ServerConfig, error)
}
