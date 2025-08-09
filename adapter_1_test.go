package mcpadapter

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestWaitForServersReady tests the WaitForServersReady method.
func TestWaitForServersReady(t *testing.T) { // #nosec G101
	mockAdapter := NewMockAdapter()
	mockClientFactory := &MockClientFactory{
		MockClient: &MockMCPClient{adapter: mockAdapter},
	}

	waitForServersReadyTests := []struct {
		name             string
		serverConfigs    map[string]*ServerConfig
		simulatedStarts  map[string]time.Duration // Server name to delay before setting StatusRunning
		expectError      bool
		expectedErrorMsg string
		expectedRunning  map[string]bool // Expected final running status for each server
	}{
		{
			name: "all servers ready",
			serverConfigs: map[string]*ServerConfig{
				"server-a": {Command: "echo"},
				"server-b": {Command: "echo"},
				"server-c": {Command: "echo", Disabled: true},
			},
			simulatedStarts: map[string]time.Duration{
				"server-a": 50 * time.Millisecond,
				"server-b": 100 * time.Millisecond,
			},
			expectError: false,
			expectedRunning: map[string]bool{
				"server-a": true,
				"server-b": true,
				"server-c": false, // Disabled
			},
		},
		{
			name: "timeout",
			serverConfigs: map[string]*ServerConfig{
				"server-a": {Command: "echo"},
				"server-b": {Command: "echo"},
			},
			simulatedStarts: map[string]time.Duration{
				"server-a": 50 * time.Millisecond,
				// server-b never starts
			},
			expectError:      true,
			expectedErrorMsg: "timeout waiting for servers to be ready",
			expectedRunning: map[string]bool{
				"server-a": true,
				"server-b": false,
			},
		},
		{
			name: "context cancellation",
			serverConfigs: map[string]*ServerConfig{
				"server-a": {Command: "echo"},
			},
			simulatedStarts:  nil, // No simulated starts, context will cancel
			expectError:      true,
			expectedErrorMsg: "context canceled",
			expectedRunning: map[string]bool{
				"server-a": false,
			},
		},
		{
			name: "disabled servers ignored",
			serverConfigs: map[string]*ServerConfig{
				"server-d": {Command: "echo", Disabled: true},
				"server-e": {Command: "echo"},
			},
			simulatedStarts: map[string]time.Duration{
				"server-e": 50 * time.Millisecond,
			},
			expectError: false,
			expectedRunning: map[string]bool{
				"server-d": false, // Disabled
				"server-e": true,
			},
		},
	}

	for _, tt := range waitForServersReadyTests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdapter.Reset()
			testConfig := &Config{
				McpServers: tt.serverConfigs,
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

			concreteAdapter := adapter.(*adapterImpl)
			// Initialize clientStatus based on whether they are expected to start
			concreteAdapter.mu.Lock()
			for serverName, config := range tt.serverConfigs {
				if _, ok := tt.simulatedStarts[serverName]; ok {
					concreteAdapter.clientStatus[serverName] = StatusStarting
				} else if config.Disabled {
					concreteAdapter.clientStatus[serverName] = StatusStopped
				} else {
					concreteAdapter.clientStatus[serverName] = StatusStopped
				}
			}
			concreteAdapter.mu.Unlock()

			ctx, cancel := context.WithCancel(context.Background())
			if tt.name == "context cancellation" {
				// Cancel immediately for this specific test case
				go func() {
					time.Sleep(50 * time.Millisecond)
					cancel()
				}()
			} else {
				defer cancel()
			}

			// Simulate servers starting asynchronously
			for serverName, delay := range tt.simulatedStarts {
				sName := serverName // Capture loop variable
				sDelay := delay
				go func() {
					time.Sleep(sDelay)
					// Update the real adapter's status, not the mock
					concreteAdapter.mu.Lock()
					concreteAdapter.clientStatus[sName] = StatusRunning
					concreteAdapter.mu.Unlock()
				}()
			}

			waitErr := adapter.WaitForServersReady(ctx, 500*time.Millisecond) // Use a reasonable timeout
			if (waitErr != nil) != tt.expectError {
				t.Errorf("WaitForServersReady() error = %v, expectError %v", waitErr, tt.expectError)
			}
			if tt.expectError && waitErr != nil && waitErr.Error() != tt.expectedErrorMsg && waitErr != context.Canceled {
				t.Errorf("Expected error message %q, got %q", tt.expectedErrorMsg, waitErr.Error())
			}

			// Verify final statuses
			for serverName, expectedRunning := range tt.expectedRunning {
				status := adapter.GetServerStatusByName(serverName)
				if expectedRunning && status != StatusRunning {
					t.Errorf("Server %s: Expected status %s, got %s", serverName, StatusRunning, status)
				} else if !expectedRunning && status == StatusRunning {
					t.Errorf("Server %s: Expected status not %s, but got %s", serverName, StatusRunning, status)
				}
			}
		})
	}
}

// TestClose tests the Close method.
func TestClose(t *testing.T) { // #nosec G101
	mockAdapter := NewMockAdapter()
	mockClientFactory := &MockClientFactory{
		MockClient: &MockMCPClient{adapter: mockAdapter},
	}

	closeTests := []struct {
		name             string
		serverConfigs    map[string]*ServerConfig
		initialStatuses  map[string]ServerStatus
		mockCloseFunc    func(serverName string) func() error
		expectError      bool
		expectedStatuses map[string]ServerStatus
	}{
		{
			name: "successful close",
			serverConfigs: map[string]*ServerConfig{
				"server1": {Command: "echo", Args: []string{"server1"}},
				"server2": {Command: "echo", Args: []string{"server2"}},
			},
			initialStatuses: map[string]ServerStatus{
				"server1": StatusRunning,
				"server2": StatusRunning,
			},
			mockCloseFunc: func(serverName string) func() error {
				return func() error { return nil }
			},
			expectError: false,
			expectedStatuses: map[string]ServerStatus{
				"server1": StatusStopped,
				"server2": StatusStopped,
			},
		},
		{
			name: "partial close with errors",
			serverConfigs: map[string]*ServerConfig{
				"server1": {Command: "echo", Args: []string{"server1"}},
				"server2": {Command: "echo", Args: []string{"server2"}},
			},
			initialStatuses: map[string]ServerStatus{
				"server1": StatusRunning,
				"server2": StatusRunning,
			},
			mockCloseFunc: func(serverName string) func() error {
				if serverName == "server2" {
					return func() error { return fmt.Errorf("server2 close error") }
				}
				return func() error { return nil }
			},
			expectError: true,
			expectedStatuses: map[string]ServerStatus{
				"server1": StatusStopped,
				"server2": StatusStopped,
			},
		},
	}

	for _, tt := range closeTests {
		t.Run(tt.name, func(t *testing.T) {
			mockAdapter.Reset()
			testConfig := &Config{
				McpServers: tt.serverConfigs,
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

			concreteAdapter := adapter.(*adapterImpl)
			concreteAdapter.mu.Lock()
			for serverName, status := range tt.initialStatuses {
				concreteAdapter.clientStatus[serverName] = status
				// Also add the client to the adapter's internal map, as Close iterates over it
				if status == StatusRunning {
					// Create a separate mock adapter for each client to have individual close functions
					serverMockAdapter := NewMockAdapter()
					serverMockAdapter.SetMockClientCloseFunc(tt.mockCloseFunc(serverName))
					mockClient := &MockMCPClient{adapter: serverMockAdapter}
					concreteAdapter.clients[serverName] = mockClient
				}
			}
			concreteAdapter.mu.Unlock()

			err = adapter.Close()
			if (err != nil) != tt.expectError {
				t.Errorf("Close() error = %v, expectError %v", err, tt.expectError)
			}

			for serverName, expectedStatus := range tt.expectedStatuses {
				status := adapter.GetServerStatusByName(serverName)
				if status != expectedStatus {
					t.Errorf("Expected server %s status %s, got %s", serverName, expectedStatus, status)
				}
			}
		})
	}
}
