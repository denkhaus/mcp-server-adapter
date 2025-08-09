// Package mcpadapter provides a simplified MCP Server Adapter using mark3labs/mcp-go client and langchaingo.
package mcpadapter

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tmc/langchaingo/tools"
)

const (
	// Transport types
	TransportStdio = "stdio"
	TransportSSE   = "sse"
	TransportHTTP  = "http"
)

// adapterImpl manages MCP server connections using mark3labs/mcp-go client.
type adapterImpl struct {
	config     *Config
	configPath string
	logLevel   string
	logger     *log.Logger

	// MCP clients (using mark3labs/mcp-go)
	clients      map[string]mcpclient.MCPClient
	clientStatus map[string]ServerStatus

	// File watching
	fileWatcherEnabled  bool
	configWatchCallback func(*Config) error
	fileWatcher         *fsnotify.Watcher
	watcherDone         chan bool
	watcherRunning      bool

	// Synchronization
	mu sync.RWMutex

	// Client factory for testing purposes
	clientFactory ClientFactoryInterface
}

// New creates a new MCP adapter using mark3labs/mcp-go client.
func New(options ...Option) (MCPAdapter, error) {
	adapter := &adapterImpl{
		logLevel:      "info",
		logger:        log.New(os.Stdout, "[MCP-Adapter] ", log.LstdFlags),
		clients:       make(map[string]mcpclient.MCPClient),
		clientStatus:  make(map[string]ServerStatus),
		clientFactory: NewClientFactory(), // Default factory
	}

	// Apply options
	for _, option := range options {
		if err := option(adapter); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Load configuration if path is provided
	if adapter.configPath != "" {
		config, err := loadConfig(adapter.configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
		adapter.config = config
	}

	// Initialize client status
	if adapter.config != nil {
		for serverName := range adapter.config.McpServers {
			adapter.clientStatus[serverName] = StatusStopped
		}
	}

	adapter.logf("MCP-Adapter initialized with %d servers", len(adapter.clientStatus))

	// Start file watcher if enabled
	if adapter.fileWatcherEnabled && adapter.configPath != "" {
		if err := adapter.startFileWatcher(); err != nil {
			adapter.logf("Failed to start file watcher: %v", err)
		}
	}

	return adapter, nil
}

// StartServer starts a specific MCP server using mark3labs/mcp-go client.
// This method is non-blocking and starts the server in a goroutine.
func (a *adapterImpl) StartServer(ctx context.Context, serverName string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	disabled, err := a.isServerDisabled(serverName)
	if err != nil {
		return err
	}
	if disabled {
		return fmt.Errorf("server %s is disabled", serverName)
	}

	serverConfig, err := a.getServerConfig(serverName)
	if err != nil {
		return err
	}

	// Check if already running or starting
	if status, exists := a.clientStatus[serverName]; exists && (status == StatusRunning || status == StatusStarting) {
		return nil // Already running or starting
	}

	a.clientStatus[serverName] = StatusStarting
	a.logf("Starting server: %s", serverName)

	// Start the server in a goroutine to make it non-blocking
	go func() {
		if err := a.startServerAsync(ctx, serverName, serverConfig); err != nil {
			a.mu.Lock()
			a.clientStatus[serverName] = StatusError
			a.mu.Unlock()
			a.logf("Failed to start server %s: %v", serverName, err)
		}
	}()

	return nil
}

// startServerAsync handles the actual server startup process asynchronously.
func (a *adapterImpl) startServerAsync(ctx context.Context, serverName string, serverConfig *ServerConfig) error {
	// Create MCP client using the injected client factory
	mcpClient, err := a.clientFactory.CreateClient(serverConfig)
	if err != nil {
		return fmt.Errorf("failed to create MCP client for %s: %w", serverName, err)
	}

	// Note: The client is already started when created with NewStdioMCPClient

	// Initialize the client
	initRequest := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo: mcp.Implementation{
				Name:    "mcp-server-adapter",
				Version: "2.0.0",
			},
		},
	}

	if _, err := mcpClient.Initialize(ctx, initRequest); err != nil {
		if err := mcpClient.Close(); err != nil {
			a.logf("Failed to close MCP client during cleanup: %v", err)
		}
		return fmt.Errorf("failed to initialize MCP client for %s: %w", serverName, err)
	}

	// Update status and store client
	a.mu.Lock()
	a.clients[serverName] = mcpClient
	a.clientStatus[serverName] = StatusRunning
	a.mu.Unlock()

	a.logf("Server started successfully: %s", serverName)
	return nil
}

// StartAllServers starts all enabled servers concurrently.
func (a *adapterImpl) StartAllServers(ctx context.Context) error {
	if a.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	var errors []error
	for serverName := range a.config.McpServers {
		if err := a.StartServer(ctx, serverName); err != nil {
			errors = append(errors, fmt.Errorf("failed to start server %s: %w", serverName, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors starting servers: %v", errors)
	}
	return nil
}

// GetServerStatusByName returns the current status of a server.
func (a *adapterImpl) GetServerStatusByName(serverName string) ServerStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if status, exists := a.clientStatus[serverName]; exists {
		return status
	}
	return StatusStopped
}

// GetAllServerStatuses returns the status of all configured servers.
func (a *adapterImpl) GetAllServerStatuses() map[string]ServerStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	statuses := make(map[string]ServerStatus)
	for serverName, status := range a.clientStatus {
		statuses[serverName] = status
	}
	return statuses
}

// WaitForServersReady waits for all servers to be in running state or timeout.
func (a *adapterImpl) WaitForServersReady(ctx context.Context, timeout time.Duration) error {
	if a.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for servers to be ready")
			}

			allReady := true
			a.mu.RLock()
			for serverName, serverConfig := range a.config.McpServers {
				if serverConfig.Disabled {
					continue
				}
				status := a.clientStatus[serverName]
				if status != StatusRunning {
					allReady = false
					break
				}
			}
			a.mu.RUnlock()

			if allReady {
				return nil
			}
		}
	}
}

// StopServer stops a specific MCP server.
func (a *adapterImpl) StopServer(serverName string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	mcpClient, exists := a.clients[serverName]
	if !exists {
		return fmt.Errorf("server %s not found or not running", serverName)
	}

	a.logf("Stopping server: %s", serverName)

	if err := a.closeClientSafely(mcpClient, serverName); err != nil {
		if a.isBrokenPipeError(err) {
			a.logf("Server %s already closed (broken pipe): %v", serverName, err)
		} else {
			a.clientStatus[serverName] = StatusError
			return fmt.Errorf("failed to stop server %s: %w", serverName, err)
		}
	}

	delete(a.clients, serverName)
	a.clientStatus[serverName] = StatusStopped
	a.logf("Server stopped successfully: %s", serverName)

	return nil
}

func (a *adapterImpl) resolveToolName(serverName string, toolName string) string {
	serverConfig, err := a.getServerConfig(serverName)
	if err != nil {
		return fmt.Sprintf("%s.%s", serverName, toolName)
	}

	resolvedToolName := fmt.Sprintf("%s.%s", serverName, toolName)
	if serverConfig.ToolPrefix != "" {
		sanitizedPrefix := sanitizePrefix(serverConfig.ToolPrefix)
		resolvedToolName = fmt.Sprintf("%s/%s", sanitizedPrefix, toolName)
	}

	return resolvedToolName
}

// GetToolsByServerName returns all tools from a server as LangChain tools.
func (a *adapterImpl) GetToolsByServerName(ctx context.Context, serverName string) ([]tools.Tool, error) {
	a.mu.RLock()
	mcpClient, exists := a.clients[serverName]
	status := a.clientStatus[serverName]
	a.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server %s not found or not running", serverName)
	}

	if status != StatusRunning {
		return nil, fmt.Errorf("server %s is not running (status: %s)", serverName, status.String())
	}

	// List tools from MCP server
	listRequest := mcp.ListToolsRequest{}
	result, err := mcpClient.ListTools(ctx, listRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from server %s: %w", serverName, err)
	}

	// Convert MCP tools to LangChain tools
	langchainTools := make([]tools.Tool, 0, len(result.Tools))
	for _, mcpTool := range result.Tools {
		langchainTool := &MCPTool{
			name:        mcpTool.Name, // Use original tool name
			description: mcpTool.Description,
			inputSchema: mcpTool.InputSchema,
			client:      mcpClient,
			serverName:  serverName,
			toolName:    mcpTool.Name, // Store original tool name
		}
		langchainTools = append(langchainTools, langchainTool)
	}

	return langchainTools, nil
}

// GetAllTools returns all tools from all running servers as LangChain tools.
func (a *adapterImpl) GetAllTools(ctx context.Context) ([]tools.Tool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var allTools []tools.Tool
	for serverName, mcpClient := range a.clients {
		if a.clientStatus[serverName] != StatusRunning {
			continue
		}

		// List tools from this server
		listRequest := mcp.ListToolsRequest{}
		result, err := mcpClient.ListTools(ctx, listRequest)
		if err != nil {
			a.logf("Failed to list tools from server %s: %v", serverName, err)
			continue
		}

		// Convert and add tools with server prefix
		for _, mcpTool := range result.Tools {
			toolName := a.resolveToolName(serverName, mcpTool.Name)

			langchainTool := &MCPTool{
				name:        toolName,
				description: fmt.Sprintf("[%s] %s", serverName, mcpTool.Description),
				inputSchema: mcpTool.InputSchema,
				client:      mcpClient,
				serverName:  serverName,
				toolName:    mcpTool.Name, // Original tool name for calling
			}
			allTools = append(allTools, langchainTool)
		}
	}

	return allTools, nil
}

// createMCPClient creates an MCP client based on the server configuration.
func (a *adapterImpl) createMCPClient(config *ServerConfig) (*mcpclient.Client, error) {
	transport := config.Transport
	if transport == "" {
		transport = "stdio" // Default to stdio
	}

	switch transport {
	case TransportStdio:
		if config.Command == "" {
			return nil, fmt.Errorf("command is required for stdio transport")
		}

		env := make([]string, 0, len(config.Env))
		for key, value := range config.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}

		return mcpclient.NewStdioMCPClient(config.Command, env, config.Args...)

	case TransportSSE:
		if config.URL == "" {
			return nil, fmt.Errorf("url is required for SSE transport")
		}

		var options []mcptransport.ClientOption
		if len(config.Headers) > 0 {
			options = append(options, mcptransport.WithHeaders(config.Headers))
		}

		return mcpclient.NewSSEMCPClient(config.URL, options...)

	case TransportHTTP:
		if config.URL == "" {
			return nil, fmt.Errorf("url is required for HTTP transport")
		}

		// Note: mark3labs/mcp-go uses "streamable http" for HTTP transport
		return mcpclient.NewStreamableHttpClient(config.URL)

	default:
		return nil, fmt.Errorf("unsupported transport type: %s", transport)
	}
}

// Close shuts down all MCP clients and cleans up resources.
func (a *adapterImpl) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Stop file watcher first
	a.stopFileWatcherUnsafe()

	var errors []error
	for serverName, client := range a.clients {
		a.logf("Closing client: %s", serverName)
		if err := a.closeClientSafely(client, serverName); err != nil {
			// Log the error but don't fail the entire shutdown for broken pipe errors
			if a.isBrokenPipeError(err) {
				a.logf("Client %s already closed (broken pipe): %v", serverName, err)
			} else {
				errors = append(errors, fmt.Errorf("failed to close client %s: %w", serverName, err))
			}
		}
		delete(a.clients, serverName)
		a.clientStatus[serverName] = StatusStopped
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}
	return nil
}

// IsConfigWatcherRunning returns whether the config watcher is running.
func (a *adapterImpl) IsConfigWatcherRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.watcherRunning
}

// IsServerDisabled checks if a server is disabled in the configuration.
func (a *adapterImpl) IsServerDisabled(serverName string) (bool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.isServerDisabled(serverName)
}

// GetServerConfig returns the configuration for a specific server.
func (a *adapterImpl) GetServerConfig(serverName string) (*ServerConfig, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.getServerConfig(serverName)
}
