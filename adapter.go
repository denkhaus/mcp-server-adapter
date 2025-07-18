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

// MCPAdapter defines the interface for MCP adapter operations.
type MCPAdapter interface {
	// Server lifecycle management
	StartServer(ctx context.Context, serverName string) error
	StartAllServers(ctx context.Context) error
	StopServer(serverName string) error
	Close() error

	// Status monitoring
	GetServerStatus(serverName string) ServerStatus
	GetAllServerStatuses() map[string]ServerStatus
	WaitForServersReady(ctx context.Context, timeout time.Duration) error

	// Tool access
	GetLangChainTools(ctx context.Context, serverName string) ([]tools.Tool, error)
	GetAllLangChainTools(ctx context.Context) ([]tools.Tool, error)

	// Configuration
	IsConfigWatcherRunning() bool
	GetConfig() *Config
}

// Adapter manages MCP server connections using mark3labs/mcp-go client.
type Adapter struct {
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

// Option represents a configuration option for the adapter.
type Option func(*Adapter) error

// WithConfigPath sets the configuration file path.
func WithConfigPath(path string) Option {
	return func(a *Adapter) error {
		a.configPath = path
		return nil
	}
}

// WithLogLevel sets the logging level.
func WithLogLevel(level string) Option {
	return func(a *Adapter) error {
		a.logLevel = level
		return nil
	}
}

// WithLogger sets a custom logger.
func WithLogger(logger *log.Logger) Option {
	return func(a *Adapter) error {
		a.logger = logger
		return nil
	}
}

// WithFileWatcher enables or disables automatic configuration file watching.
func WithFileWatcher(enabled bool) Option {
	return func(a *Adapter) error {
		a.fileWatcherEnabled = enabled
		return nil
	}
}

// WithConfig sets the configuration directly instead of loading from file.
func WithConfig(config *Config) Option {
	return func(a *Adapter) error {
		a.config = config
		return nil
	}
}

// WithConfigWatchCallback sets a custom callback for configuration changes.
func WithConfigWatchCallback(callback func(*Config) error) Option {
	return func(a *Adapter) error {
		a.configWatchCallback = callback
		return nil
	}
}

// WithClientFactory injects a custom ClientFactory (e.g., for testing).
func WithClientFactory(factory ClientFactoryInterface) Option {
	return func(a *Adapter) error {
		a.clientFactory = factory
		return nil
	}
}

// New creates a new MCP adapter using mark3labs/mcp-go client.
func New(options ...Option) (MCPAdapter, error) {
	adapter := &Adapter{
		logLevel:      "info",
		logger:        log.New(os.Stdout, "[MCP-V2] ", log.LstdFlags),
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

	adapter.logf("MCP Adapter V2 initialized with %d servers", len(adapter.clientStatus))

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
func (a *Adapter) StartServer(ctx context.Context, serverName string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	serverConfig, exists := a.config.McpServers[serverName]
	if !exists {
		return fmt.Errorf("server %s not found in configuration", serverName)
	}

	if serverConfig.Disabled {
		return fmt.Errorf("server %s is disabled", serverName)
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
func (a *Adapter) startServerAsync(ctx context.Context, serverName string, serverConfig *ServerConfig) error {
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
func (a *Adapter) StartAllServers(ctx context.Context) error {
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

// GetServerStatus returns the current status of a server.
func (a *Adapter) GetServerStatus(serverName string) ServerStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if status, exists := a.clientStatus[serverName]; exists {
		return status
	}
	return StatusStopped
}

// GetAllServerStatuses returns the status of all configured servers.
func (a *Adapter) GetAllServerStatuses() map[string]ServerStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	statuses := make(map[string]ServerStatus)
	for serverName, status := range a.clientStatus {
		statuses[serverName] = status
	}
	return statuses
}

// WaitForServersReady waits for all servers to be in running state or timeout.
func (a *Adapter) WaitForServersReady(ctx context.Context, timeout time.Duration) error {
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
func (a *Adapter) StopServer(serverName string) error {
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

// GetLangChainTools returns all tools from a server as LangChain tools.
func (a *Adapter) GetLangChainTools(ctx context.Context, serverName string) ([]tools.Tool, error) {
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
			name:        mcpTool.Name,
			description: mcpTool.Description,
			inputSchema: mcpTool.InputSchema,
			client:      mcpClient,
			serverName:  serverName,
		}
		langchainTools = append(langchainTools, langchainTool)
	}

	return langchainTools, nil
}

// GetAllLangChainTools returns all tools from all running servers as LangChain tools.
func (a *Adapter) GetAllLangChainTools(ctx context.Context) ([]tools.Tool, error) {
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
			langchainTool := &MCPTool{
				name:        fmt.Sprintf("%s.%s", serverName, mcpTool.Name),
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
func (a *Adapter) createMCPClient(config *ServerConfig) (*mcpclient.Client, error) {
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

// ClientFactoryInterface defines the interface for creating MCPClient instances.
// This allows for mocking the client creation process in tests.
type ClientFactoryInterface interface {
	CreateClient(config *ServerConfig) (mcpclient.MCPClient, error)
}

// Close shuts down all MCP clients and cleans up resources.
func (a *Adapter) Close() error {
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

// closeClientSafely attempts to close a client with timeout and error handling
func (a *Adapter) closeClientSafely(client mcpclient.MCPClient, serverName string) error {
	// Create a context with timeout for the close operation
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use a channel to handle the close operation with timeout
	done := make(chan error, 1)
	go func() {
		done <- client.Close()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		a.logf("Timeout closing client %s, forcing shutdown", serverName)
		return nil // Don't return timeout as error, just log it
	}
}

// isBrokenPipeError checks if an error is a broken pipe error
func (a *Adapter) isBrokenPipeError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "broken pipe") ||
		contains(errStr, "signal: broken pipe") ||
		contains(errStr, "connection reset") ||
		contains(errStr, "EOF")
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// IsConfigWatcherRunning returns whether the config watcher is running.
func (a *Adapter) IsConfigWatcherRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.watcherRunning
}

// startFileWatcher starts monitoring the configuration file for changes.
func (a *Adapter) startFileWatcher() error {
	if a.configPath == "" {
		return fmt.Errorf("no config path specified")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	a.fileWatcher = watcher
	a.watcherDone = make(chan bool)
	a.watcherRunning = true

	// Start watching in a goroutine
	go a.watchConfigFile()

	// Add the config file to the watcher
	if err := watcher.Add(a.configPath); err != nil {
		a.stopFileWatcherUnsafe()
		return fmt.Errorf("failed to watch config file: %w", err)
	}

	a.logf("File watcher started for: %s", a.configPath)
	return nil
}

// stopFileWatcherUnsafe stops the file watcher (must be called with mutex held).
func (a *Adapter) stopFileWatcherUnsafe() {
	if a.fileWatcher != nil {
		a.watcherRunning = false
		if a.watcherDone != nil {
			close(a.watcherDone)
		}
		if err := a.fileWatcher.Close(); err != nil {
			a.logf("Error closing file watcher: %v", err)
		}
		a.fileWatcher = nil
		a.watcherDone = nil
		a.logf("File watcher stopped")
	}
}

// watchConfigFile monitors the configuration file for changes.
func (a *Adapter) watchConfigFile() {
	// Capture the done channel to avoid race conditions
	a.mu.RLock()
	done := a.watcherDone
	events := a.fileWatcher.Events
	errors := a.fileWatcher.Errors
	a.mu.RUnlock()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}

			// Handle write events (file modifications)
			if event.Op&fsnotify.Write == fsnotify.Write {
				a.logf("Configuration file changed: %s", event.Name)
				if err := a.handleConfigChange(); err != nil {
					a.logf("Failed to handle config change: %v", err)
				}
			}

		case err, ok := <-errors:
			if !ok {
				return
			}
			a.logf("File watcher error: %v", err)

		case <-done:
			return
		}
	}
}

// handleConfigChange reloads the configuration and restarts affected servers.
func (a *Adapter) handleConfigChange() error { // #nosec G101
	a.logf("Reloading configuration...")

	// Load the new configuration
	newConfig, err := loadConfig(a.configPath)
	if err != nil {
		return fmt.Errorf("failed to load new config: %w", err)
	}

	// Call custom callback if provided
	if a.configWatchCallback != nil {
		if err := a.configWatchCallback(newConfig); err != nil {
			a.logf("Config watch callback failed: %v", err)
			return err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	oldConfig := a.config
	a.config = newConfig

	// Determine which servers need to be restarted
	serversToRestart := a.getServersToRestart(oldConfig, newConfig)

	a.applyConfigChanges(oldConfig, newConfig, serversToRestart)

	a.logf("Configuration reloaded successfully")
	return nil
}

// applyConfigChanges applies the configuration changes by stopping,
// updating status, and starting servers as needed.
// This function assumes the adapter's mutex is already locked.
func (a *Adapter) applyConfigChanges(
	oldConfig, newConfig *Config,
	serversToRestart map[string]bool,
) {
	// Stop servers that are no longer in the config or have changed
	for serverName := range serversToRestart {
		if client, exists := a.clients[serverName]; exists {
			a.logf("Stopping server for restart: %s", serverName)
			if err := a.closeClientSafely(client, serverName); err != nil && !a.isBrokenPipeError(err) {
				a.logf("Error stopping server %s: %v", serverName, err)
			}
			delete(a.clients, serverName)
		}
	}

	// Update client status for all servers in new config
	for serverName := range newConfig.McpServers {
		if _, exists := a.clientStatus[serverName]; !exists {
			a.clientStatus[serverName] = StatusStopped
		}
	}

	// Remove status for servers no longer in config
	for serverName := range oldConfig.McpServers {
		if _, exists := newConfig.McpServers[serverName]; !exists {
			delete(a.clientStatus, serverName)
		}
	}

	// Start servers that need to be restarted
	ctx := context.Background()
	for serverName := range serversToRestart {
		if serverConfig, exists := newConfig.McpServers[serverName]; exists && !serverConfig.Disabled {
			a.logf("Restarting server: %s", serverName)
			a.clientStatus[serverName] = StatusStarting

			// Start server asynchronously
			go func(name string, config *ServerConfig) {
				if err := a.startServerAsync(ctx, name, config); err != nil {
					a.mu.Lock()
					a.clientStatus[name] = StatusError
					a.mu.Unlock()
					a.logf("Failed to restart server %s: %v", name, err)
				}
			}(serverName, serverConfig)
		}
	}
}

// getServersToRestart determines which servers need to be restarted based on config changes.
func (a *Adapter) getServersToRestart(oldConfig, newConfig *Config) map[string]bool {
	serversToRestart := make(map[string]bool)

	if oldConfig == nil {
		// If there was no old config, start all servers
		for serverName := range newConfig.McpServers {
			serversToRestart[serverName] = true
		}
		return serversToRestart
	}

	// Check for new servers
	for serverName := range newConfig.McpServers {
		if _, exists := oldConfig.McpServers[serverName]; !exists {
			serversToRestart[serverName] = true
		}
	}

	// Check for removed servers
	for serverName := range oldConfig.McpServers {
		if _, exists := newConfig.McpServers[serverName]; !exists {
			serversToRestart[serverName] = true
		}
	}

	// Check for modified servers
	for serverName, newServerConfig := range newConfig.McpServers {
		if oldServerConfig, exists := oldConfig.McpServers[serverName]; exists {
			if a.serverConfigChanged(oldServerConfig, newServerConfig) {
				serversToRestart[serverName] = true
			}
		}
	}

	return serversToRestart
}

// serverConfigChanged checks if a server configuration has changed.
func (a *Adapter) serverConfigChanged(old, new *ServerConfig) bool {
	// Compare key fields that would require a restart
	return old.Command != new.Command ||
		old.Transport != new.Transport ||
		old.URL != new.URL ||
		old.Disabled != new.Disabled ||
		!equalStringSlices(old.Args, new.Args) ||
		!equalStringMaps(old.Env, new.Env) ||
		!equalStringMaps(old.Headers, new.Headers)
}

// equalStringSlices compares two string slices for equality.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// equalStringMaps compares two string maps for equality.
func equalStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// GetConfig returns the current complete configuration.
func (a *Adapter) GetConfig() *Config {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil {
		return nil
	}

	// Return a copy to prevent external modification
	config := &Config{
		McpServers: make(map[string]*ServerConfig),
	}

	// Deep copy server configs
	for name, serverConfig := range a.config.McpServers {
		configCopy := *serverConfig
		// Copy maps
		if serverConfig.Env != nil {
			configCopy.Env = make(map[string]string)
			for k, v := range serverConfig.Env {
				configCopy.Env[k] = v
			}
		}
		if serverConfig.Headers != nil {
			configCopy.Headers = make(map[string]string)
			for k, v := range serverConfig.Headers {
				configCopy.Headers[k] = v
			}
		}
		if serverConfig.AlwaysAllow != nil {
			configCopy.AlwaysAllow = make([]string, len(serverConfig.AlwaysAllow))
			copy(configCopy.AlwaysAllow, serverConfig.AlwaysAllow)
		}
		config.McpServers[name] = &configCopy
	}

	return config
}

// logf logs a formatted message if logging is enabled.
func (a *Adapter) logf(format string, args ...interface{}) {
	if a.logger != nil && a.logLevel != "silent" {
		a.logger.Printf(format, args...)
	}
}
