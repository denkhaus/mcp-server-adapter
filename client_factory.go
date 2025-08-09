package mcpadapter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
)

// ClientFactoryInterface defines the interface for creating MCPClient instances.
// This allows for mocking the client creation process in tests.
type ClientFactoryInterface interface {
	CreateClient(config *ServerConfig) (mcpclient.MCPClient, error)
}

// clientFactory implements the ClientFactoryInterface for creating actual MCP clients.
type clientFactory struct{}

// NewClientFactory creates a new client factory
func NewClientFactory() *clientFactory {
	return &clientFactory{}
}

// CreateClient creates an MCP client based on the server configuration
// This method includes the insights from the alternative implementation
func (cf *clientFactory) CreateClient(config *ServerConfig) (mcpclient.MCPClient, error) {
	transport := config.Transport
	if transport == "" {
		transport = TransportStdio // Default to stdio
	}

	switch transport {
	case "stdio":
		return cf.createStdioClient(config)
	case "sse":
		return cf.createSSEClient(config)
	case "http":
		return cf.createHTTPClient(config)
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", transport)
	}
}

// createStdioClient creates a stdio client with improved handling
func (cf *clientFactory) createStdioClient(config *ServerConfig) (mcpclient.MCPClient, error) {
	if config.Command == "" {
		return nil, fmt.Errorf("command is required for stdio transport")
	}

	env := make([]string, 0, len(config.Env))
	for key, value := range config.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Check if this is a Go project that should be built as binary
	if cf.isGoProject(config) {
		return cf.createGoProjectClient(config, env)
	}

	// For other commands (Python, Node.js, npm packages), use directly
	return mcpclient.NewStdioMCPClient(config.Command, env, config.Args...)
}

// isGoProject checks if the command is trying to run a Go project
func (cf *clientFactory) isGoProject(config *ServerConfig) bool {
	// Check if command is "go" and first arg is "run"
	if config.Command == "go" && len(config.Args) > 0 && config.Args[0] == "run" {
		return true
	}
	return false
}

// createGoProjectClient handles Go projects by building them as binaries first
func (cf *clientFactory) createGoProjectClient(config *ServerConfig, env []string) (mcpclient.MCPClient, error) {
	if len(config.Args) < 2 {
		return nil, fmt.Errorf("go run requires at least one source file")
	}

	sourceFile := config.Args[1] // "go run main.go" -> main.go
	sourceDir := filepath.Dir(sourceFile)
	if sourceDir == "." {
		sourceDir = ""
	}

	executableName := cf.getExecutableName("mcp-server-temp")
	var binaryPath string
	if sourceDir != "" {
		binaryPath = filepath.Join(sourceDir, executableName)
	} else {
		binaryPath = executableName
	}

	// Build the binary
	buildArgs := []string{"build", "-o", executableName}
	buildArgs = append(buildArgs, config.Args[1:]...) // Add source files

	// #nosec G204
	cmd := exec.Command("go", buildArgs...)
	if sourceDir != "" {
		cmd.Dir = sourceDir
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to build Go project: %w\nOutput: %s", err, output)
	}

	// Clean up binary on exit (best effort)
	defer func() {
		if err := os.Remove(binaryPath); err != nil {
			// Log but don't fail - the binary might not exist or be in use
			fmt.Printf("Warning: failed to remove temporary binary %s: %v\n", binaryPath, err)
		}
	}()

	// Use the binary
	return mcpclient.NewStdioMCPClient(binaryPath, env)
}

// getExecutableName returns the executable name with proper extension for the OS
func (cf *clientFactory) getExecutableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// createSSEClient creates an SSE client
func (cf *clientFactory) createSSEClient(config *ServerConfig) (mcpclient.MCPClient, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("url is required for SSE transport")
	}

	var options []mcptransport.ClientOption
	if len(config.Headers) > 0 {
		options = append(options, mcptransport.WithHeaders(config.Headers))
	}

	return mcpclient.NewSSEMCPClient(config.URL, options...)
}

// createHTTPClient creates an HTTP client
func (cf *clientFactory) createHTTPClient(config *ServerConfig) (mcpclient.MCPClient, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("url is required for HTTP transport")
	}

	return mcpclient.NewStreamableHttpClient(config.URL)
}
