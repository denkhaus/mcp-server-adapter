package mcpadapter

import (
	"context"
	"fmt"

	"github.com/fsnotify/fsnotify"
)

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
