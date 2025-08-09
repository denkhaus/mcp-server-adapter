package mcpadapter

import (
	"context"
	"fmt"
	"sync"
	"time"

	// Correctly aliased client package

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tmc/langchaingo/tools"
)

// MockAdapter is a mock implementation of MCPAdapter for testing purposes.
type MockAdapter struct {
	mu sync.RWMutex

	// Mock state
	serverStatuses map[string]ServerStatus
	tools          map[string][]tools.Tool
	errors         map[string]error // Errors to return for specific operations

	// Configuration state
	config *Config

	// Behavior configuration
	startDelay    time.Duration
	configWatcher bool

	// Call tracking
	startServerCalls []string
	stopServerCalls  []string
	getToolsCalls    []string
	getAllToolsCalls int

	// MockMCPClient related
	mockClientInitializeFunc func(
		ctx context.Context,
		request mcp.InitializeRequest,
	) (*mcp.InitializeResult, error) // #nosec G101
	mockClientListToolsFunc func(
		ctx context.Context,
		request mcp.ListToolsRequest,
	) (*mcp.ListToolsResult, error) // #nosec G101
	mockClientCloseFunc   func() error // #nosec G101
	mockClientUseToolFunc func(
		ctx context.Context,
		request mcp.CallToolRequest,
	) (*mcp.CallToolResult, error) // #nosec G101
	mockClientGetResourcesFunc func(
		ctx context.Context,
		request mcp.ReadResourceRequest,
	) (*mcp.ReadResourceResult, error) // #nosec G101
	mockClientCompleteFunc func(
		ctx context.Context,
		request mcp.CompleteRequest,
	) (*mcp.CompleteResult, error) // #nosec G101
	mockClientGetPromptFunc func(
		ctx context.Context,
		request mcp.GetPromptRequest,
	) (*mcp.GetPromptResult, error) // #nosec G101
	mockClientListPromptsFunc func(
		ctx context.Context,
		request mcp.ListPromptsRequest,
	) (*mcp.ListPromptsResult, error) // #nosec G101
	mockClientListPromptsByPageFunc func(
		ctx context.Context,
		request mcp.ListPromptsRequest,
	) (*mcp.ListPromptsResult, error) // #nosec G101
	mockClientListResourceTemplatesFunc func(
		ctx context.Context,
		request mcp.ListResourceTemplatesRequest,
	) (*mcp.ListResourceTemplatesResult, error) // #nosec G101
	mockClientListResourceTemplatesByPageFunc func(
		ctx context.Context,
		request mcp.ListResourceTemplatesRequest,
	) (*mcp.ListResourceTemplatesResult, error) // #nosec G101
	mockClientListResourcesFunc func(
		ctx context.Context,
		request mcp.ListResourcesRequest,
	) (*mcp.ListResourcesResult, error) // #nosec G101
	mockClientListResourcesByPageFunc func(
		ctx context.Context,
		request mcp.ListResourcesRequest,
	) (*mcp.ListResourcesResult, error) // #nosec G101
	mockClientListToolsByPageFunc func(
		ctx context.Context,
		request mcp.ListToolsRequest,
	) (*mcp.ListToolsResult, error) // #nosec G101
	mockClientOnNotificationFunc func(handler func(notification mcp.JSONRPCNotification))
	mockClientPingFunc           func(ctx context.Context) error
	mockClientSetLevelFunc       func(ctx context.Context, request mcp.SetLevelRequest) error
	mockClientSubscribeFunc      func(ctx context.Context, request mcp.SubscribeRequest) error
	mockClientUnsubscribeFunc    func(ctx context.Context, request mcp.UnsubscribeRequest) error
}

// NewMockAdapter creates a new mock adapter for testing.
func NewMockAdapter() *MockAdapter {
	// Create default config
	defaultConfig := &Config{
		McpServers: make(map[string]*ServerConfig),
	}

	return &MockAdapter{
		serverStatuses: make(map[string]ServerStatus),
		tools:          make(map[string][]tools.Tool),
		errors:         make(map[string]error),
		config:         defaultConfig,
		startDelay:     0,
		configWatcher:  false,
	}
}

// SetServerStatus sets the status for a mock server.
func (m *MockAdapter) SetServerStatus(serverName string, status ServerStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serverStatuses[serverName] = status
}

// SetServerTools sets the tools that a mock server should return.
func (m *MockAdapter) SetServerTools(serverName string, serverTools []tools.Tool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools[serverName] = serverTools
}

// SetError sets an error to be returned for a specific operation.
// Operations: "start", "stop", "tools", "alltools"
func (m *MockAdapter) SetError(operation string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[operation] = err
}

// SetStartDelay sets a delay for server start operations.
func (m *MockAdapter) SetStartDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startDelay = delay
}

// SetConfigWatcher sets whether the config watcher should appear to be running.
func (m *MockAdapter) SetConfigWatcher(running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configWatcher = running
}

// SetMockConfig sets the complete mock configuration.
func (m *MockAdapter) SetMockConfig(config *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// AddMockServer adds a mock server configuration.
func (m *MockAdapter) AddMockServer(name string, serverConfig *ServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config == nil {
		m.config = &Config{
			McpServers: make(map[string]*ServerConfig),
		}
	}

	m.config.McpServers[name] = serverConfig
	m.serverStatuses[name] = StatusStopped
}

// GetStartServerCalls returns the list of servers that StartServer was called for.
func (m *MockAdapter) GetStartServerCalls() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	calls := make([]string, len(m.startServerCalls))
	copy(calls, m.startServerCalls)
	return calls
}

// GetStopServerCalls returns the list of servers that StopServer was called for.
func (m *MockAdapter) GetStopServerCalls() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	calls := make([]string, len(m.stopServerCalls))
	copy(calls, m.stopServerCalls)
	return calls
}

// GetGetToolsCalls returns the list of servers that GetToolsByServerName was called for.
func (m *MockAdapter) GetGetToolsCalls() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	calls := make([]string, len(m.getToolsCalls))
	copy(calls, m.getToolsCalls)
	return calls
}

// GetGetAllToolsCalls returns the number of times GetAllLangChainTools was called.
func (m *MockAdapter) GetGetAllToolsCalls() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getAllToolsCalls
}

// Reset clears all mock state and call tracking.
func (m *MockAdapter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mockClientInitializeFunc = nil
	m.mockClientListToolsFunc = nil
	m.mockClientCloseFunc = nil
	m.mockClientUseToolFunc = nil
	m.mockClientGetResourcesFunc = nil
	m.mockClientPingFunc = nil
	m.mockClientSetLevelFunc = nil
	m.mockClientSubscribeFunc = nil
	m.mockClientUnsubscribeFunc = nil

	m.serverStatuses = make(map[string]ServerStatus)
	m.tools = make(map[string][]tools.Tool)
	m.errors = make(map[string]error)
	m.startDelay = 0
	m.configWatcher = false

	m.startServerCalls = nil
	m.stopServerCalls = nil
	m.getToolsCalls = nil
	m.getAllToolsCalls = 0
}

// MCPAdapter interface implementation

// StartServer simulates starting a server.
func (m *MockAdapter) StartServer(ctx context.Context, serverName string) error {
	m.mu.Lock()
	m.startServerCalls = append(m.startServerCalls, serverName)

	if err, exists := m.errors["start"]; exists {
		m.mu.Unlock()
		return err
	}

	delay := m.startDelay
	m.mu.Unlock()

	// Simulate start delay
	if delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Set status to running
	m.serverStatuses[serverName] = StatusRunning

	return nil
}

// StartAllServers simulates starting all configured servers.
func (m *MockAdapter) StartAllServers(ctx context.Context) error {
	m.mu.RLock()
	servers := make([]string, 0, len(m.serverStatuses))
	for serverName := range m.serverStatuses {
		servers = append(servers, serverName)
	}
	m.mu.RUnlock()

	for _, serverName := range servers {
		if err := m.StartServer(ctx, serverName); err != nil {
			return fmt.Errorf("failed to start server %s: %w", serverName, err)
		}
	}

	return nil
}

// StopServer simulates stopping a server.
func (m *MockAdapter) StopServer(serverName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopServerCalls = append(m.stopServerCalls, serverName)

	if err, exists := m.errors["stop"]; exists {
		return err
	}

	m.serverStatuses[serverName] = StatusStopped
	return nil
}

// Close simulates closing the adapter.
func (m *MockAdapter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err, exists := m.errors["close"]; exists {
		return err
	}

	// Set all servers to stopped
	for serverName := range m.serverStatuses {
		m.serverStatuses[serverName] = StatusStopped
	}

	return nil
}

// GetServerStatusByName returns the mock status for a server.
func (m *MockAdapter) GetServerStatusByName(serverName string) ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if status, exists := m.serverStatuses[serverName]; exists {
		return status
	}
	return StatusStopped
}

// GetAllServerStatuses returns all mock server statuses.
func (m *MockAdapter) GetAllServerStatuses() map[string]ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]ServerStatus)
	for serverName, status := range m.serverStatuses {
		statuses[serverName] = status
	}
	return statuses
}

// WaitForServersReady simulates waiting for servers to be ready.
func (m *MockAdapter) WaitForServersReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for servers to be ready")
			}

			m.mu.RLock()
			allReady := true
			for _, status := range m.serverStatuses {
				if status != StatusRunning {
					allReady = false
					break
				}
			}
			m.mu.RUnlock()

			if allReady {
				return nil
			}
		}
	}
}

// GetToolsByServerName returns mock tools for a server.
func (m *MockAdapter) GetToolsByServerName(ctx context.Context, serverName string) ([]tools.Tool, error) {
	m.mu.Lock()
	m.getToolsCalls = append(m.getToolsCalls, serverName)

	if err, exists := m.errors["tools"]; exists {
		m.mu.Unlock()
		return nil, err
	}

	serverTools, exists := m.tools[serverName]
	m.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("server %s not found or not running", serverName)
	}

	// Return a copy to avoid race conditions
	result := make([]tools.Tool, len(serverTools))
	copy(result, serverTools)
	return result, nil
}

// GetAllTools returns all mock tools from all servers.
func (m *MockAdapter) GetAllTools(ctx context.Context) ([]tools.Tool, error) {
	m.mu.Lock()
	m.getAllToolsCalls++

	if err, exists := m.errors["alltools"]; exists {
		m.mu.Unlock()
		return nil, err
	}

	var allTools []tools.Tool
	for serverName, serverTools := range m.tools {
		if status, exists := m.serverStatuses[serverName]; !exists || status != StatusRunning {
			continue
		}

		// Add tools with server prefix
		for _, tool := range serverTools {
			// Create a prefixed tool name
			prefixedTool := &MockTool{
				name:         fmt.Sprintf("%s.%s", serverName, tool.Name()),
				description:  fmt.Sprintf("[%s] %s", serverName, tool.Description()),
				originalTool: tool,
			}
			allTools = append(allTools, prefixedTool)
		}
	}
	m.mu.Unlock()

	return allTools, nil
}

// IsConfigWatcherRunning returns the mock config watcher status.
func (m *MockAdapter) IsConfigWatcherRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configWatcher
}

// GetConfig returns the current mock complete configuration.
func (m *MockAdapter) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return nil
	}

	// Return a copy to prevent external modification
	config := &Config{
		McpServers: make(map[string]*ServerConfig),
	}

	// Deep copy server configs
	for name, serverConfig := range m.config.McpServers {
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

// IsServerDisabled checks if a server is disabled in the mock configuration.
func (m *MockAdapter) IsServerDisabled(serverName string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return false, fmt.Errorf("no configuration loaded")
	}

	serverConfig, exists := m.config.McpServers[serverName]
	if !exists {
		return false, fmt.Errorf("server %s not found in configuration", serverName)
	}

	return serverConfig.Disabled, nil
}

// GetServerConfig returns the configuration for a specific mock server.
func (m *MockAdapter) GetServerConfig(serverName string) (*ServerConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	serverConfig, exists := m.config.McpServers[serverName]
	if !exists {
		return nil, fmt.Errorf("server %s not found in configuration", serverName)
	}

	return serverConfig, nil
}

// SetMockClientInitializeFunc sets the mock function for MockMCPClient.Initialize.
func (m *MockAdapter) SetMockClientInitializeFunc(f func(
	ctx context.Context,
	request mcp.InitializeRequest,
) (*mcp.InitializeResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientInitializeFunc = f
}

// SetMockClientListToolsFunc sets the mock function for MockMCPClient.ListTools.
func (m *MockAdapter) SetMockClientListToolsFunc(f func(
	ctx context.Context,
	request mcp.ListToolsRequest,
) (*mcp.ListToolsResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientListToolsFunc = f
}

// SetMockClientCloseFunc sets the mock function for MockMCPClient.Close.
func (m *MockAdapter) SetMockClientCloseFunc(f func() error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientCloseFunc = f
}

// SetMockClientUseToolFunc sets the mock function for MockMCPClient.UseTool.
func (m *MockAdapter) SetMockClientUseToolFunc(f func(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientUseToolFunc = f
}

// SetMockClientGetResourcesFunc sets the mock function for MockMCPClient.GetResources.
func (m *MockAdapter) SetMockClientGetResourcesFunc(f func(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientGetResourcesFunc = f
}

// SetMockClientCompleteFunc sets the mock function for MockMCPClient.Complete.
func (m *MockAdapter) SetMockClientCompleteFunc(f func(
	ctx context.Context,
	request mcp.CompleteRequest,
) (*mcp.CompleteResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientCompleteFunc = f
}

// SetMockClientGetPromptFunc sets the mock function for MockMCPClient.GetPrompt.
func (m *MockAdapter) SetMockClientGetPromptFunc(f func(
	ctx context.Context,
	request mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientGetPromptFunc = f
}

// SetMockClientListPromptsFunc sets the mock function for MockMCPClient.ListPrompts.
func (m *MockAdapter) SetMockClientListPromptsFunc(f func(
	ctx context.Context,
	request mcp.ListPromptsRequest,
) (*mcp.ListPromptsResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientListPromptsFunc = f
}

// SetMockClientListPromptsByPageFunc sets the mock function for MockMCPClient.ListPromptsByPage.
func (m *MockAdapter) SetMockClientListPromptsByPageFunc(f func(
	ctx context.Context,
	request mcp.ListPromptsRequest,
) (*mcp.ListPromptsResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientListPromptsByPageFunc = f
}

// SetMockClientListResourceTemplatesFunc sets the mock function for MockMCPClient.ListResourceTemplates.
func (m *MockAdapter) SetMockClientListResourceTemplatesFunc(f func(
	ctx context.Context,
	request mcp.ListResourceTemplatesRequest,
) (*mcp.ListResourceTemplatesResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientListResourceTemplatesFunc = f
}

// SetMockClientListResourceTemplatesByPageFunc sets the mock function for MockMCPClient.ListResourceTemplatesByPage.
func (m *MockAdapter) SetMockClientListResourceTemplatesByPageFunc(f func(
	ctx context.Context,
	request mcp.ListResourceTemplatesRequest,
) (*mcp.ListResourceTemplatesResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientListResourceTemplatesByPageFunc = f
}

// SetMockClientListResourcesFunc sets the mock function for MockMCPClient.ListResources.
func (m *MockAdapter) SetMockClientListResourcesFunc(f func(
	ctx context.Context,
	request mcp.ListResourcesRequest,
) (*mcp.ListResourcesResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientListResourcesFunc = f
}

// SetMockClientListResourcesByPageFunc sets the mock function for MockMCPClient.ListResourcesByPage.
func (m *MockAdapter) SetMockClientListResourcesByPageFunc(f func(
	ctx context.Context,
	request mcp.ListResourcesRequest,
) (*mcp.ListResourcesResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientListResourcesByPageFunc = f
}

// SetMockClientListToolsByPageFunc sets the mock function for MockMCPClient.ListToolsByPage.
func (m *MockAdapter) SetMockClientListToolsByPageFunc(f func(
	ctx context.Context,
	request mcp.ListToolsRequest,
) (*mcp.ListToolsResult, error)) { // #nosec G101
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientListToolsByPageFunc = f
}

// SetMockClientOnNotificationFunc sets the mock function for MockMCPClient.OnNotification.
func (m *MockAdapter) SetMockClientOnNotificationFunc(f func(handler func(notification mcp.JSONRPCNotification))) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockClientOnNotificationFunc = f
}

// MockMCPClient is a mock implementation of client.MCPClient for testing.
type MockMCPClient struct {
	adapter *MockAdapter // Reference to the parent MockAdapter to access its mock functions
}

// Initialize implements client.MCPClient.
func (mc *MockMCPClient) Initialize(ctx context.Context, request mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	if mc.adapter != nil && mc.adapter.mockClientInitializeFunc != nil {
		return mc.adapter.mockClientInitializeFunc(ctx, request)
	}
	return &mcp.InitializeResult{}, nil
}

// Ping implements client.MCPClient.
func (mc *MockMCPClient) Ping(ctx context.Context) error {
	if mc.adapter != nil && mc.adapter.mockClientPingFunc != nil {
		return mc.adapter.mockClientPingFunc(ctx)
	}
	return nil
}

// ListTools implements client.MCPClient.
func (mc *MockMCPClient) ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	if mc.adapter != nil && mc.adapter.mockClientListToolsFunc != nil {
		return mc.adapter.mockClientListToolsFunc(ctx, request)
	}
	return &mcp.ListToolsResult{Tools: []mcp.Tool{}}, nil
}

// ListToolsByPage implements client.MCPClient.
func (mc *MockMCPClient) ListToolsByPage(
	ctx context.Context,
	request mcp.ListToolsRequest,
) (*mcp.ListToolsResult, error) { // #nosec G101
	if mc.adapter != nil && mc.adapter.mockClientListToolsByPageFunc != nil {
		return mc.adapter.mockClientListToolsByPageFunc(ctx, request)
	}
	return &mcp.ListToolsResult{Tools: []mcp.Tool{}}, nil
}

// CallTool implements client.MCPClient.
func (mc *MockMCPClient) CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if mc.adapter != nil && mc.adapter.mockClientUseToolFunc != nil {
		return mc.adapter.mockClientUseToolFunc(ctx, request)
	}
	return nil, fmt.Errorf("CallTool not implemented in MockMCPClient for testing purposes")
}

// ReadResource implements client.MCPClient.
func (mc *MockMCPClient) ReadResource(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) { // #nosec G101
	if mc.adapter != nil && mc.adapter.mockClientGetResourcesFunc != nil {
		return mc.adapter.mockClientGetResourcesFunc(ctx, request)
	}
	return nil, fmt.Errorf("ReadResource not implemented in MockMCPClient for testing purposes")
}

// Complete implements client.MCPClient.
func (mc *MockMCPClient) Complete(ctx context.Context, request mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	if mc.adapter != nil && mc.adapter.mockClientCompleteFunc != nil {
		return mc.adapter.mockClientCompleteFunc(ctx, request)
	}
	return nil, fmt.Errorf("Complete not implemented in MockMCPClient for testing purposes")
}

// GetPrompt implements client.MCPClient.
func (mc *MockMCPClient) GetPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	if mc.adapter != nil && mc.adapter.mockClientGetPromptFunc != nil {
		return mc.adapter.mockClientGetPromptFunc(ctx, request)
	}
	return nil, fmt.Errorf("GetPrompt not implemented in MockMCPClient for testing purposes")
}

// ListPrompts implements client.MCPClient.
func (mc *MockMCPClient) ListPrompts(
	ctx context.Context,
	request mcp.ListPromptsRequest,
) (*mcp.ListPromptsResult, error) { // #nosec G101
	if mc.adapter != nil && mc.adapter.mockClientListPromptsFunc != nil {
		return mc.adapter.mockClientListPromptsFunc(ctx, request)
	}
	return &mcp.ListPromptsResult{Prompts: []mcp.Prompt{}}, nil
}

// ListPromptsByPage implements client.MCPClient.
func (mc *MockMCPClient) ListPromptsByPage(
	ctx context.Context,
	request mcp.ListPromptsRequest,
) (*mcp.ListPromptsResult, error) { // #nosec G101
	if mc.adapter != nil && mc.adapter.mockClientListPromptsByPageFunc != nil {
		return mc.adapter.mockClientListPromptsByPageFunc(ctx, request)
	}
	return &mcp.ListPromptsResult{Prompts: []mcp.Prompt{}}, nil
}

// ListResourceTemplates implements client.MCPClient.
func (mc *MockMCPClient) ListResourceTemplates(
	ctx context.Context,
	request mcp.ListResourceTemplatesRequest,
) (*mcp.ListResourceTemplatesResult, error) { // #nosec G101
	if mc.adapter != nil && mc.adapter.mockClientListResourceTemplatesFunc != nil {
		return mc.adapter.mockClientListResourceTemplatesFunc(ctx, request)
	}
	return &mcp.ListResourceTemplatesResult{ResourceTemplates: []mcp.ResourceTemplate{}}, nil
}

// ListResourceTemplatesByPage implements client.MCPClient.
func (mc *MockMCPClient) ListResourceTemplatesByPage(
	ctx context.Context,
	request mcp.ListResourceTemplatesRequest,
) (*mcp.ListResourceTemplatesResult, error) { // #nosec G101
	if mc.adapter != nil && mc.adapter.mockClientListResourceTemplatesByPageFunc != nil {
		return mc.adapter.mockClientListResourceTemplatesByPageFunc(ctx, request)
	}
	return &mcp.ListResourceTemplatesResult{ResourceTemplates: []mcp.ResourceTemplate{}}, nil
}

// ListResources implements client.MCPClient.
func (mc *MockMCPClient) ListResources(
	ctx context.Context,
	request mcp.ListResourcesRequest,
) (*mcp.ListResourcesResult, error) { // #nosec G101
	if mc.adapter != nil && mc.adapter.mockClientListResourcesFunc != nil {
		return mc.adapter.mockClientListResourcesFunc(ctx, request)
	}
	return &mcp.ListResourcesResult{Resources: []mcp.Resource{}}, nil
}

// ListResourcesByPage implements client.MCPClient.
func (mc *MockMCPClient) ListResourcesByPage(
	ctx context.Context,
	request mcp.ListResourcesRequest,
) (*mcp.ListResourcesResult, error) { // #nosec G101
	if mc.adapter != nil && mc.adapter.mockClientListResourcesByPageFunc != nil {
		return mc.adapter.mockClientListResourcesByPageFunc(ctx, request)
	}
	return &mcp.ListResourcesResult{Resources: []mcp.Resource{}}, nil
}

// Close implements client.MCPClient.
func (mc *MockMCPClient) Close() error {
	if mc.adapter != nil && mc.adapter.mockClientCloseFunc != nil {
		return mc.adapter.mockClientCloseFunc()
	}
	return nil
}

// OnNotification implements client.MCPClient.
func (mc *MockMCPClient) OnNotification(handler func(notification mcp.JSONRPCNotification)) {
	if mc.adapter != nil && mc.adapter.mockClientOnNotificationFunc != nil {
		mc.adapter.mockClientOnNotificationFunc(handler)
	}
}

// SetLevel implements client.MCPClient.
func (mc *MockMCPClient) SetLevel(ctx context.Context, request mcp.SetLevelRequest) error {
	if mc.adapter != nil && mc.adapter.mockClientSetLevelFunc != nil {
		return mc.adapter.mockClientSetLevelFunc(ctx, request)
	}
	return nil
}

// Subscribe implements client.MCPClient.
func (mc *MockMCPClient) Subscribe(ctx context.Context, request mcp.SubscribeRequest) error {
	if mc.adapter != nil && mc.adapter.mockClientSubscribeFunc != nil {
		return mc.adapter.mockClientSubscribeFunc(ctx, request)
	}
	return nil
}

// Unsubscribe implements client.MCPClient.
func (mc *MockMCPClient) Unsubscribe(ctx context.Context, request mcp.UnsubscribeRequest) error {
	if mc.adapter != nil && mc.adapter.mockClientUnsubscribeFunc != nil {
		return mc.adapter.mockClientUnsubscribeFunc(ctx, request)
	}
	return nil
}

// MockTool is a simple mock tool implementation.
type MockTool struct {
	name         string
	description  string
	result       string
	err          error
	originalTool tools.Tool
}

// NewMockTool creates a new mock tool.
func NewMockTool(name, description string) *MockTool {
	return &MockTool{
		name:        name,
		description: description,
		result:      fmt.Sprintf("Mock result from %s", name),
	}
}

// SetResult sets the result that the mock tool should return.
func (m *MockTool) SetResult(result string) {
	m.result = result
}

// SetError sets an error that the mock tool should return.
func (m *MockTool) SetError(err error) {
	m.err = err
}

// Name returns the tool name.
func (m *MockTool) Name() string {
	return m.name
}

// Description returns the tool description.
func (m *MockTool) Description() string {
	return m.description
}

// Call simulates calling the tool.
func (m *MockTool) Call(ctx context.Context, input string) (string, error) {
	if m.err != nil {
		return "", m.err
	}

	if m.originalTool != nil {
		return m.originalTool.Call(ctx, input)
	}

	return m.result, nil
}
