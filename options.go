package mcpadapter

import "log"

// Option represents a configuration option for the adapter.
type Option func(*adapterImpl) error

// WithConfigPath sets the configuration file path.
func WithConfigPath(path string) Option {
	return func(a *adapterImpl) error {
		a.configPath = path
		return nil
	}
}

// WithLogLevel sets the logging level.
func WithLogLevel(level string) Option {
	return func(a *adapterImpl) error {
		a.logLevel = level
		return nil
	}
}

// WithLogger sets a custom logger.
func WithLogger(logger *log.Logger) Option {
	return func(a *adapterImpl) error {
		a.logger = logger
		return nil
	}
}

// WithFileWatcher enables or disables automatic configuration file watching.
func WithFileWatcher(enabled bool) Option {
	return func(a *adapterImpl) error {
		a.fileWatcherEnabled = enabled
		return nil
	}
}

// WithConfig sets the configuration directly instead of loading from file.
func WithConfig(config *Config) Option {
	return func(a *adapterImpl) error {
		a.config = config
		return nil
	}
}

// WithConfigWatchCallback sets a custom callback for configuration changes.
func WithConfigWatchCallback(callback func(*Config) error) Option {
	return func(a *adapterImpl) error {
		a.configWatchCallback = callback
		return nil
	}
}

// WithClientFactory injects a custom ClientFactory (e.g., for testing).
func WithClientFactory(factory ClientFactoryInterface) Option {
	return func(a *adapterImpl) error {
		a.clientFactory = factory
		return nil
	}
}
