package mcpadapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileWatcherFunctionality(t *testing.T) { // #nosec G101
	initialConfig := Config{
		McpServers: map[string]*ServerConfig{
			"test-server": {
				Command: "echo",
				Args:    []string{"initial"},
			},
		},
	}
	runFileWatcherTest(t, func(t *testing.T, adapter *Adapter, configPath string) {

		if !adapter.IsConfigWatcherRunning() {
			t.Error("File watcher should be running")
		}

		initialStatuses := adapter.GetAllServerStatuses()
		if len(initialStatuses) != 1 {
			t.Errorf("Expected 1 server initially, got %d", len(initialStatuses))
		}

		modifiedConfig := Config{
			McpServers: map[string]*ServerConfig{
				"test-server": {
					Command: "echo",
					Args:    []string{"modified"},
				},
				"new-server": {
					Command: "echo",
					Args:    []string{"new"},
				},
			},
		}

		if err := writeTestConfig(configPath, &modifiedConfig); err != nil {
			t.Fatalf("Failed to write modified config: %v", err)
		}

		time.Sleep(500 * time.Millisecond)

		newStatuses := adapter.GetAllServerStatuses()
		if len(newStatuses) != 2 {
			t.Errorf("Expected 2 servers after config change, got %d", len(newStatuses))
		}

		if _, exists := newStatuses["test-server"]; !exists {
			t.Error("test-server should still exist after config change")
		}
		if _, exists := newStatuses["new-server"]; !exists {
			t.Error("new-server should exist after config change")
		}
	}, initialConfig)
}

func runFileWatcherTest(
	t *testing.T,
	testFunc func(t *testing.T, adapter *Adapter, configPath string),
	initialConfig Config,
) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	if err := writeTestConfig(configPath, &initialConfig); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	adapterIface, err := New(
		WithConfigPath(configPath),
		WithFileWatcher(true),
		WithLogLevel("debug"),
	)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	adapter := adapterIface.(*Adapter) // Type assertion here
	defer func() {
		if err := adapter.Close(); err != nil {
			t.Logf("Failed to close adapter: %v", err)
		}
	}()

	testFunc(t, adapter, configPath)
}

func TestFileWatcherWithCallback(t *testing.T) { // #nosec G101
	initialConfig := Config{
		McpServers: map[string]*ServerConfig{
			"test-server": {
				Command: "echo",
				Args:    []string{"initial"},
			},
		},
	}
	runFileWatcherTest(t, func(t *testing.T, adapter *Adapter, configPath string) {
		var mu sync.Mutex
		callbackCalled := false
		var callbackConfig *Config

		// The adapter is created here within the anonymous function for TestFileWatcherWithCallback
		// because this test specifically needs to set up a callback for the file watcher,
		// which requires the adapter to be initialized with that callback.
		// The `runFileWatcherTest` helper function provides a general adapter setup,
		// but for this specific test, we need to override that with a custom setup
		// that includes the callback.
		adapterIface, err := New(
			WithConfigPath(configPath),
			WithFileWatcher(true),
			WithConfigWatchCallback(func(config *Config) error {
				mu.Lock()
				defer mu.Unlock()
				callbackCalled = true
				callbackConfig = config
				return nil
			}),
			WithLogLevel("debug"),
		)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		adapter = adapterIface.(*Adapter) // Type assertion here
		defer func() {
			if err := adapter.Close(); err != nil {
				t.Logf("Failed to close adapter: %v", err)
			}
		}()

		modifiedConfig := Config{
			McpServers: map[string]*ServerConfig{
				"test-server": {
					Command: "echo",
					Args:    []string{"modified"},
				},
			},
		}

		if err := writeTestConfig(configPath, &modifiedConfig); err != nil {
			t.Fatalf("Failed to write modified config: %v", err)
		}

		time.Sleep(500 * time.Millisecond)

		mu.Lock()
		wasCalled := callbackCalled
		config := callbackConfig
		mu.Unlock()

		if !wasCalled {
			t.Error("Config watch callback should have been called")
		}

		if config == nil {
			t.Error("Callback should have received the new config")
		} else if len(config.McpServers) != 1 {
			t.Errorf("Callback config should have 1 server, got %d", len(config.McpServers))
		}
	}, initialConfig)
}

func TestFileWatcherDisabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	initialConfig := Config{
		McpServers: map[string]*ServerConfig{
			"test-server": {
				Command: "echo",
				Args:    []string{"initial"},
			},
		},
	}

	if err := writeTestConfig(configPath, &initialConfig); err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}

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

	if adapter.IsConfigWatcherRunning() {
		t.Error("File watcher should not be running when disabled")
	}
}

func TestServerConfigChanged(t *testing.T) { // #nosec G101
	adapter := &Adapter{}

	serverConfigChangedTests := []struct {
		name     string
		old      *ServerConfig
		new      *ServerConfig
		expected bool
	}{
		{
			name: "no change",
			old: &ServerConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
			new: &ServerConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
			expected: false,
		},
		{
			name: "command changed",
			old: &ServerConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
			new: &ServerConfig{
				Command: "cat",
				Args:    []string{"test"},
			},
			expected: true,
		},
		{
			name: "args changed",
			old: &ServerConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
			new: &ServerConfig{
				Command: "echo",
				Args:    []string{"modified"},
			},
			expected: true,
		},
		{
			name: "disabled changed",
			old: &ServerConfig{
				Command:  "echo",
				Disabled: false,
			},
			new: &ServerConfig{
				Command:  "echo",
				Disabled: true,
			},
			expected: true,
		},
	}

	for _, tt := range serverConfigChangedTests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.serverConfigChanged(tt.old, tt.new)
			if result != tt.expected {
				t.Errorf("serverConfigChanged() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetServersToRestart(t *testing.T) {
	adapter := &Adapter{}

	oldConfig := &Config{
		McpServers: map[string]*ServerConfig{
			"server1": {Command: "echo", Args: []string{"old"}},
			"server2": {Command: "cat"},
		},
	}

	newConfig := &Config{
		McpServers: map[string]*ServerConfig{
			"server1": {Command: "echo", Args: []string{"new"}}, // modified
			"server3": {Command: "ls"},                          // new
		},
	}

	serversToRestart := adapter.getServersToRestart(oldConfig, newConfig)

	// Should restart: server1 (modified), server2 (removed), server3 (new)
	expectedServers := map[string]bool{
		"server1": true,
		"server2": true,
		"server3": true,
	}

	if len(serversToRestart) != len(expectedServers) {
		t.Errorf("Expected %d servers to restart, got %d", len(expectedServers), len(serversToRestart))
	}

	for serverName := range expectedServers {
		if !serversToRestart[serverName] {
			t.Errorf("Expected server %s to be marked for restart", serverName)
		}
	}
}

// Helper function to write test configuration
func writeTestConfig(path string, config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
