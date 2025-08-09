package mcpadapter

import (
	"context"
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
	mcpclient "github.com/mark3labs/mcp-go/client"
)

// closeClientSafely attempts to close a client with timeout and error handling
func (a *adapterImpl) closeClientSafely(client mcpclient.MCPClient, serverName string) error {
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
func (a *adapterImpl) isBrokenPipeError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "broken pipe") ||
		contains(errStr, "signal: broken pipe") ||
		contains(errStr, "connection reset") ||
		contains(errStr, "EOF")
}

// getServerConfig returns the configuration for a specific server.
// This function is not thread-safe and should only be called from a function that has already acquired a lock.
func (a *adapterImpl) getServerConfig(serverName string) (*ServerConfig, error) {
	if a.config == nil {
		return nil, fmt.Errorf("no configuration loaded")
	}

	serverConfig, exists := a.config.McpServers[serverName]
	if !exists {
		return nil, fmt.Errorf("server %s not found in configuration", serverName)
	}

	return serverConfig, nil
}

// isServerDisabled checks if a server is disabled in the configuration.
// This function is not thread-safe and should only be called from a function that has already acquired a lock.
func (a *adapterImpl) isServerDisabled(serverName string) (bool, error) {
	serverConfig, err := a.getServerConfig(serverName)
	if err != nil {
		return false, err
	}

	return serverConfig.Disabled, nil
}

// startFileWatcher starts monitoring the configuration file for changes.
func (a *adapterImpl) startFileWatcher() error {
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
func (a *adapterImpl) stopFileWatcherUnsafe() {
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

// getServersToRestart determines which servers need to be restarted based on config changes.
func (a *adapterImpl) getServersToRestart(oldConfig, newConfig *Config) map[string]bool {
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

// logf logs a formatted message if logging is enabled.
func (a *adapterImpl) logf(format string, args ...interface{}) {
	if a.logger != nil && a.logLevel != "silent" {
		a.logger.Printf(format, args...)
	}
}
