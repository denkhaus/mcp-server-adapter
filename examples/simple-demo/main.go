package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	mcpadapter "github.com/denkhaus/mcp-server-adapter"
)

const (
	exeExtension = ".exe"
)

func main() {
	fmt.Println("🚀 MCP Server Adapter - Simple Demo")
	fmt.Println("===================================")
	fmt.Println("This demo shows basic MCP integration without requiring LLM API keys")
	fmt.Println()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Create MCP adapter with the fetch server
	adapter, err := createMCPAdapter()
	if err != nil {
		return fmt.Errorf("create MCP adapter: %w", err)
	}
	defer func() {
		if err := adapter.Close(); err != nil {
			fmt.Printf("Failed to close adapter: %v\n", err)
		}
	}()

	ctx := context.Background()

	// Get available tools
	fmt.Println("🔧 Getting available tools...")
	tools, err := adapter.GetAllTools(ctx)
	if err != nil {
		return fmt.Errorf("get tools: %w", err)
	}

	fmt.Printf("✅ Found %d tools:\n", len(tools))
	for i, tool := range tools {
		fmt.Printf("   %d. %s\n", i+1, tool.Name())
		fmt.Printf("      Description: %s\n", truncateString(tool.Description(), 100))
	}
	fmt.Println()

	// Test the fetch_url tool
	if len(tools) > 0 {
		fmt.Println("🧪 Testing fetch_url tool...")
		tool := tools[0] // Should be fetch_url

		// Test with a simple URL
		testURL := "https://httpbin.org/json"
		testInput := fmt.Sprintf(`{"url": "%s"}`, testURL)

		fmt.Printf("   Calling %s with URL: %s\n", tool.Name(), testURL)

		result, err := tool.Call(ctx, testInput)
		if err != nil {
			fmt.Printf("   ⚠️  Tool call failed: %v\n", err)
		} else {
			fmt.Printf("   ✅ Tool result (%d chars): %s\n",
				len(result), truncateString(result, 200))
		}
	}

	fmt.Println()
	fmt.Println("🎉 Demo completed successfully!")
	fmt.Println()
	fmt.Println("💡 Next steps:")
	fmt.Println("   • Set an LLM API key (OPENAI_API_KEY, etc.)")
	fmt.Println("   • Run the full agent example: cd examples/agent && go run main.go")
	fmt.Println("   • Create your own MCP servers and tools")

	return nil
}

// createMCPAdapter creates an MCP adapter with the server configured
func createMCPAdapter() (mcpadapter.MCPAdapter, error) {
	// Build the server binary if it doesn't exist
	if err := buildServerIfNeeded(); err != nil {
		return nil, fmt.Errorf("build server: %w", err)
	}

	// Create a config for the MCP server
	config := &mcpadapter.Config{
		McpServers: map[string]*mcpadapter.ServerConfig{
			"fetch-server": {
				Command:    getServerBinaryPath(),
				Transport:  "stdio",
				Args:       []string{},
				Env:        map[string]string{},
				ToolPrefix: "magic-prefix",
			},
		},
	}

	// Create adapter with the config
	adapter, err := mcpadapter.New(
		mcpadapter.WithConfig(config),
		mcpadapter.WithLogLevel("info"),
	)
	if err != nil {
		return nil, fmt.Errorf("create adapter: %w", err)
	}

	// Start the server
	ctx := context.Background()
	if err := adapter.StartServer(ctx, "fetch-server"); err != nil {
		if err := adapter.Close(); err != nil {
			fmt.Printf("Failed to close adapter: %v\n", err)
		}
		return nil, fmt.Errorf("start server: %w", err)
	}

	// Give server time to initialize
	time.Sleep(1 * time.Second)

	return adapter, nil
}

// buildServerIfNeeded builds the MCP server binary if it doesn't exist
func buildServerIfNeeded() error {
	serverBinary := getServerBinaryPath()

	// Check if binary exists
	if _, err := os.Stat(serverBinary); os.IsNotExist(err) {
		return fmt.Errorf("MCP server binary not found at %s. "+
			"Please run 'make build-server' from the project root first", serverBinary)
	}

	return nil
}

// getServerBinaryPath returns the path to the server binary
func getServerBinaryPath() string {
	executableName := "mcp-server"
	if filepath.Ext(os.Args[0]) == exeExtension {
		executableName += exeExtension
	}

	// Try different possible locations for the binary
	possiblePaths := []string{
		filepath.Join("bin", executableName),       // When run from project root via make
		filepath.Join("../../bin", executableName), // When run from examples/simple-demo directory
		filepath.Join(".", executableName),         // When binary is in current directory
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default to the make-based path
	return filepath.Join("bin", executableName)
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
