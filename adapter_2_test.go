// Package mcpadapter provides tests for the simplified MCP adapter using mark3labs/mcp-go.
package mcpadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewAdapter(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		wantErr bool
	}{
		{
			name:    "basic creation",
			options: []Option{},
			wantErr: false,
		},
		{
			name: "with log level",
			options: []Option{
				WithLogLevel("debug"),
			},
			wantErr: false,
		},
		{
			name: "with config path",
			options: []Option{
				WithConfigPath("nonexistent.json"),
			},
			wantErr: true, // Config file doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := New(tt.options...)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if adapter != nil {
				defer func() {
					if err := adapter.Close(); err != nil {
						t.Logf("Failed to close adapter: %v", err)
					}
				}()
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	config := Config{
		McpServers: map[string]*ServerConfig{
			"test-server": {
				Command: "echo",
				Args:    []string{"hello"},
				Timeout: 30 * time.Second,
			},
		},
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, configData, 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test loading the config
	loadedConfig, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if len(loadedConfig.McpServers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(loadedConfig.McpServers))
	}

	server, exists := loadedConfig.McpServers["test-server"]
	if !exists {
		t.Error("Expected test-server to exist")
	}

	if server.Transport != TransportStdio {
		t.Errorf("Expected transport 'stdio', got '%s'", server.Transport)
	}

	if server.Command != "echo" {
		t.Errorf("Expected command 'echo', got '%s'", server.Command)
	}
}

func TestAdapterWithConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	config := Config{
		McpServers: map[string]*ServerConfig{
			"echo-server": {
				Command: "echo",
				Args:    []string{"hello world"},
			},
		},
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, configData, 0600); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create adapter with config
	adapter, err := New(
		WithConfigPath(configPath),
		WithLogLevel("debug"),
	)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer func() {
		if err := adapter.Close(); err != nil {
			t.Logf("Failed to close adapter: %v", err)
		}
	}()

	// Check client status using public interface
	statuses := adapter.GetAllServerStatuses()
	if len(statuses) != 1 {
		t.Errorf("Expected 1 server status, got %d", len(statuses))
	}

	status := adapter.GetServerStatus("echo-server")
	if status != StatusStopped {
		t.Errorf("Expected status stopped, got %s", status.String())
	}
}

func TestFileWatcherIntegration(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	// Create initial configuration
	initialConfig := Config{
		McpServers: map[string]*ServerConfig{
			"test-server": {
				Command: "echo",
				Args:    []string{"initial"},
			},
		},
	}

	// Write initial config
	if err := writeConfig(configPath, &initialConfig); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Create adapter with file watcher disabled initially
	adapter, err := New(
		WithConfigPath(configPath),
		WithFileWatcher(false),
		WithLogLevel("debug"),
	)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer func() {
		if err := adapter.Close(); err != nil {
			t.Logf("Failed to close adapter: %v", err)
		}
	}()

	// Config watcher functionality is not yet implemented
	// Just test that IsConfigWatcherRunning returns false
	if adapter.IsConfigWatcherRunning() {
		t.Error("Config watcher should not be running (not implemented yet)")
	}

	if adapter.IsConfigWatcherRunning() {
		t.Error("Config watcher should not be running after stop")
	}
}

func TestCreateMCPClient(t *testing.T) { // #nosec G101
	adapter := &Adapter{}

	createClientTests := []struct {
		name    string
		config  *ServerConfig
		wantErr bool
	}{
		{
			name: "stdio transport",
			config: &ServerConfig{
				Transport: "stdio",
				Command:   "echo",
				Args:      []string{"test"},
			},
			wantErr: false,
		},
		{
			name: "missing command for stdio",
			config: &ServerConfig{
				Transport: "stdio",
			},
			wantErr: true,
		},
		{
			name: "sse transport",
			config: &ServerConfig{
				Transport: "sse",
				URL:       "https://example.com/sse",
			},
			wantErr: false,
		},
		{
			name: "missing url for sse",
			config: &ServerConfig{
				Transport: "sse",
			},
			wantErr: true,
		},
		{
			name: "http transport",
			config: &ServerConfig{
				Transport: "http",
				URL:       "https://example.com/http",
			},
			wantErr: false,
		},
		{
			name: "unsupported transport",
			config: &ServerConfig{
				Transport: "unsupported",
			},
			wantErr: true,
		},
	}

	for _, tt := range createClientTests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := adapter.createMCPClient(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("createMCPClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if client != nil {
				// Don't start the client in tests, just verify it was created
				if err := client.Close(); err != nil {
					t.Logf("Failed to close client: %v", err)
				}
			}
		})
	}
}

func TestMCPTool(t *testing.T) {
	tool := &MCPTool{
		name:        "test-tool",
		description: "A test tool",
		inputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	if tool.Name() != "test-tool" {
		t.Errorf("Expected name 'test-tool', got '%s'", tool.Name())
	}

	description := tool.Description()
	if description == "" {
		t.Error("Expected non-empty description")
	}

	// Description should include schema
	if !contains(description, "Input schema") {
		t.Error("Expected description to include schema information")
	}
}

func TestResolveToolName(t *testing.T) {
	tests := []struct {
		name         string
		serverName   string
		toolName     string
		serverConfig *ServerConfig // Represents the config that would be in a.config.McpServers[serverName]
		expected     string
	}{
		{
			name:         "no tool prefix",
			serverName:   "test-server",
			toolName:     "my-tool",
			serverConfig: &ServerConfig{}, // Empty ToolPrefix
			expected:     "test-server.my-tool",
		},
		{
			name:         "with simple tool prefix",
			serverName:   "test-server",
			toolName:     "my-tool",
			serverConfig: &ServerConfig{ToolPrefix: "custom-prefix"},
			expected:     "custom-prefix/my-tool",
		},
		{
			name:         "tool prefix needs sanitization",
			serverName:   "test-server",
			toolName:     "another-tool",
			serverConfig: &ServerConfig{ToolPrefix: "bad_prefix!123"},
			expected:     "bad-prefix123/another-tool",
		},
		{
			name:         "empty server name",
			serverName:   "",
			toolName:     "my-tool",
			serverConfig: &ServerConfig{},
			expected:     ".my-tool", // Expected default behavior for empty server name
		},
		{
			name:         "empty tool name",
			serverName:   "test-server",
			toolName:     "",
			serverConfig: &ServerConfig{ToolPrefix: "custom-prefix"},
			expected:     "custom-prefix/", // Expected behavior for empty tool name
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a dummy adapter with a config containing the test server config
			adapter := &Adapter{
				config: &Config{
					McpServers: map[string]*ServerConfig{
						tt.serverName: tt.serverConfig,
					},
				},
			}

			resolved := adapter.resolveToolName(tt.serverName, tt.toolName)
			if resolved != tt.expected {
				t.Errorf("resolveToolName(%q, %q) with prefix %q: got %q, want %q",
					tt.serverName, tt.toolName, tt.serverConfig.ToolPrefix, resolved, tt.expected)
			}
		})
	}
}

// TestStartServer tests the StartServer method.
func TestStartServer(t *testing.T) { // #nosec G101
	mockAdapter := NewMockAdapter()
	mockClientFactory := &MockClientFactory{
		MockClient: &MockMCPClient{adapter: mockAdapter},
	}

	startServerTests := []struct {
		name                 string
		serverName           string
		serverConfig         *ServerConfig
		mockInitFunc         func(ctx context.Context, request mcp.InitializeRequest) (*mcp.InitializeResult, error)
		mockCloseFunc        func() error
		mockCreateClientFunc func(config *ServerConfig) (mcpclient.MCPClient, error)
		expectError          bool
		expectedStatus       ServerStatus
	}{
		{
			name:         "successful start",
			serverName:   "test-server",
			serverConfig: &ServerConfig{Command: "echo"},
			mockInitFunc: func(ctx context.Context, request mcp.InitializeRequest) (*mcp.InitializeResult, error) {
				return &mcp.InitializeResult{}, nil
			},
			mockCloseFunc:  func() error { return nil },
			expectError:    false,
			expectedStatus: StatusRunning,
		},
		{
			name:           "already running",
			serverName:     "test-server",
			serverConfig:   &ServerConfig{Command: "echo"},
			expectError:    false,
			expectedStatus: StatusRunning,
		},
		{
			name:           "disabled server",
			serverName:     "disabled-server",
			serverConfig:   &ServerConfig{Command: "echo", Disabled: true},
			expectError:    true,
			expectedStatus: StatusStopped,
		},
		{
			name:         "client creation error",
			serverName:   "error-server",
			serverConfig: &ServerConfig{Command: "invalid"},
			mockCreateClientFunc: func(config *ServerConfig) (mcpclient.MCPClient, error) {
				return nil, fmt.Errorf("mock client creation error")
			},
			expectError: false, // StartServer itself won't return an error immediately,
			// but goroutine will set status to error
			expectedStatus: StatusError,
		},
		{
			name:         "client initialization error",
			serverName:   "init-error-server",
			serverConfig: &ServerConfig{Command: "echo"},
			mockInitFunc: func(ctx context.Context, request mcp.InitializeRequest) (*mcp.InitializeResult, error) {
				return nil, fmt.Errorf("mock initialization error")
			},
			mockCloseFunc: func() error { return nil },
			expectError:   false, // StartServer itself won't return an error immediately,
			// but goroutine will set status to error
			expectedStatus: StatusError,
		},
	}

	for _, tt := range startServerTests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdapter.Reset()
			// Manually set up the config for the adapter for each test run
			testConfig := &Config{
				McpServers: map[string]*ServerConfig{},
			}
			if tt.serverConfig != nil {
				testConfig.McpServers[tt.serverName] = tt.serverConfig
			}

			adapter, err := New(
				WithClientFactory(mockClientFactory),
				WithConfig(testConfig),
			)
			if err != nil {
				t.Fatalf("Failed to create adapter: %v", err)
			}
			defer func() {
				if err := adapter.Close(); err != nil {
					t.Logf("Failed to close adapter: %v", err)
				}
			}()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Set mock functions for the current test
			mockAdapter.SetMockClientInitializeFunc(tt.mockInitFunc)
			mockAdapter.SetMockClientCloseFunc(tt.mockCloseFunc)
			mockClientFactory.CreateClientFunc = tt.mockCreateClientFunc

			// Special handling for "already running" test case
			if tt.name == "already running" {
				// Simulate the server already running before calling StartServer
				concreteAdapter := adapter.(*Adapter)
				concreteAdapter.mu.Lock()
				concreteAdapter.clientStatus[tt.serverName] = StatusRunning
				concreteAdapter.mu.Unlock()
			}

			err = adapter.StartServer(ctx, tt.serverName)
			if (err != nil) != tt.expectError {
				t.Errorf("StartServer() error = %v, expectError %v", err, tt.expectError)
			}

			// For async operations, give some time for the goroutine to run
			time.Sleep(100 * time.Millisecond)

			status := adapter.GetServerStatus(tt.serverName)
			if status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, status)
			}
		})
	}
}

// TestStopServer tests the StopServer method.
func TestStopServer(t *testing.T) { // #nosec G101
	mockAdapter := NewMockAdapter()
	mockClientFactory := &MockClientFactory{
		MockClient: &MockMCPClient{adapter: mockAdapter},
	}

	stopServerTests := []struct {
		name           string
		serverName     string
		serverConfig   *ServerConfig
		initialStatus  ServerStatus
		mockCloseFunc  func() error
		expectError    bool
		expectedStatus ServerStatus
	}{
		{
			name:           "successful stop",
			serverName:     "test-server",
			serverConfig:   &ServerConfig{Command: "echo"},
			initialStatus:  StatusRunning,
			mockCloseFunc:  func() error { return nil },
			expectError:    false,
			expectedStatus: StatusStopped,
		},
		{
			name:           "non-existent server",
			serverName:     "non-existent-server",
			serverConfig:   nil, // No config for this server
			initialStatus:  StatusStopped,
			expectError:    true,
			expectedStatus: StatusStopped,
		},
		{
			name:           "already stopped",
			serverName:     "stopped-server",
			serverConfig:   &ServerConfig{Command: "echo"},
			initialStatus:  StatusStopped,
			expectError:    true,
			expectedStatus: StatusStopped,
		},
		{
			name:           "client close error",
			serverName:     "error-close-server",
			serverConfig:   &ServerConfig{Command: "echo"},
			initialStatus:  StatusRunning,
			mockCloseFunc:  func() error { return fmt.Errorf("mock close error") },
			expectError:    true,
			expectedStatus: StatusError,
		},
	}

	for _, tt := range stopServerTests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdapter.Reset()
			testConfig := &Config{
				McpServers: map[string]*ServerConfig{},
			}
			if tt.serverConfig != nil {
				testConfig.McpServers[tt.serverName] = tt.serverConfig
			}

			adapter, err := New(
				WithClientFactory(mockClientFactory),
				WithConfig(testConfig),
			)
			if err != nil {
				t.Fatalf("Failed to create adapter: %v", err)
			}
			defer func() {
				if err := adapter.Close(); err != nil {
					t.Logf("Failed to close adapter: %v", err)
				}
			}()

			// Set initial status for the concrete adapter
			concreteAdapter := adapter.(*Adapter)
			concreteAdapter.mu.Lock()
			if tt.serverConfig != nil {
				concreteAdapter.clientStatus[tt.serverName] = tt.initialStatus
				// For running servers, also add a mock client to the clients map
				if tt.initialStatus == StatusRunning {
					concreteAdapter.clients[tt.serverName] = &MockMCPClient{adapter: mockAdapter}
				}
			}
			concreteAdapter.mu.Unlock()

			// Set mock functions for the current test
			mockAdapter.SetMockClientCloseFunc(tt.mockCloseFunc)

			err = adapter.StopServer(tt.serverName)
			if (err != nil) != tt.expectError {
				t.Errorf("StopServer() error = %v, expectError %v", err, tt.expectError)
			}

			status := adapter.GetServerStatus(tt.serverName)
			if status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, status)
			}
		})
	}
}

// MockClientFactory for injecting mock clients
type MockClientFactory struct {
	MockClient       mcpclient.MCPClient
	CreateClientFunc func(config *ServerConfig) (mcpclient.MCPClient, error) // Optional custom function
}

// CreateClient implements ClientFactoryInterface for the mock.
func (mcf *MockClientFactory) CreateClient(config *ServerConfig) (mcpclient.MCPClient, error) {
	if mcf.CreateClientFunc != nil {
		return mcf.CreateClientFunc(config)
	}
	if mcf.MockClient != nil {
		return mcf.MockClient, nil
	}
	return nil, fmt.Errorf("MockClientFactory not configured")
}
