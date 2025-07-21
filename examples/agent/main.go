package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	mcpadapter "github.com/denkhaus/mcp-server-adapter"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
	langchaingoTools "github.com/tmc/langchaingo/tools"
	"github.com/tmc/langchaingo/tools/wikipedia"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	// Create LLM based on available API keys
	llm, err := createLLM()
	if err != nil {
		return fmt.Errorf("create LLM: %w", err)
	}

	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	wp := wikipedia.New(userAgent)

	// Start the MCP server and create adapter
	mcpAdapter, err := createMCPAdapter()
	if err != nil {
		return fmt.Errorf("create MCP adapter: %w", err)
	}
	defer func() {
		if err := mcpAdapter.Close(); err != nil {
			log.Printf("Failed to close MCP adapter: %v", err)
		}
	}()

	// Get tools from the MCP adapter
	ctx := context.Background()
	mcpTools, err := mcpAdapter.GetAllLangChainTools(ctx)
	if err != nil {
		return fmt.Errorf("get MCP tools: %w", err)
	}

	// Combine standard tools with MCP tools
	agentTools := []langchaingoTools.Tool{
		langchaingoTools.Calculator{},
		wp,
	}
	agentTools = append(agentTools, mcpTools...)

	fmt.Printf("🔧 Available tools: %d total (%d MCP tools)\n", len(agentTools), len(mcpTools))
	for i, tool := range agentTools {
		fmt.Printf("   %d. %s\n", i+1, tool.Name())
	}

	agent := agents.NewOneShotAgent(llm,
		agentTools,
		agents.WithMaxIterations(3))
	executor := agents.NewExecutor(agent)

	question := "Could you provide a summary of " +
		"https://raw.githubusercontent.com/docker-sa/01-build-image/refs/heads/main/main.go"
	fmt.Println("🔍 Question:", question)

	fmt.Println("🚀 Starting agent...")
	answer, err := chains.Run(context.Background(), executor, question)
	if err != nil {
		return fmt.Errorf("error running chains: %w", err)
	}

	fmt.Println("🎉 Answer:", answer)

	return nil
}

// createLLM creates an LLM instance based on available API keys
func createLLM() (llms.Model, error) {
	ctx := context.Background()

	// Debug: Show available API keys
	fmt.Printf("🔍 Checking API keys...\n")
	fmt.Printf("   GEMINI_API_KEY: %s\n", maskAPIKey(os.Getenv("GEMINI_API_KEY")))
	fmt.Printf("   OPENAI_API_KEY: %s\n", maskAPIKey(os.Getenv("OPENAI_API_KEY")))
	fmt.Printf("   ANTHROPIC_API_KEY: %s\n", maskAPIKey(os.Getenv("ANTHROPIC_API_KEY")))

	// Check for Google AI
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		fmt.Println("🤖 Using Google AI (Gemini)")
		return googleai.New(ctx,
			googleai.WithDefaultModel("gemini-2.0-flash"),
			googleai.WithAPIKey(apiKey),
		)
	}

	// Check for OpenAI
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		fmt.Println("🤖 Using OpenAI")
		return openai.New(
			openai.WithToken(apiKey),
			openai.WithModel("gpt-4"),
		)
	}

	// Check for Anthropic
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		fmt.Println("🤖 Using Anthropic (Claude)")
		return anthropic.New(
			anthropic.WithToken(apiKey),
			anthropic.WithModel("claude-3-5-sonnet-20241022"),
		)
	}

	return nil, fmt.Errorf("no LLM API key found. Please set one of: " +
		"GEMINI_API_KEY, OPENAI_API_KEY, or ANTHROPIC_API_KEY")
}

// getExecutableName returns the executable name with proper extension for the OS
func getExecutableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// createMCPAdapter creates an MCP adapter with the server configured
func createMCPAdapter() (mcpadapter.MCPAdapter, error) {
	// Build the server binary if it doesn't exist
	if err := buildServerIfNeeded(); err != nil {
		return nil, fmt.Errorf("build server: %w", err)
	}

	// Create a temporary config for the MCP server
	config := &mcpadapter.Config{
		McpServers: map[string]*mcpadapter.ServerConfig{
			"fetch-server": {
				Command:   getServerBinaryPath(),
				Transport: "stdio",
				Args:      []string{},
				Env:       map[string]string{},
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
		if closeErr := adapter.Close(); closeErr != nil {
			log.Printf("Failed to close adapter: %v", closeErr)
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
	executableName := getExecutableName("mcp-server")

	// Try different possible locations for the binary
	possiblePaths := []string{
		filepath.Join("bin", executableName),       // When run from project root via make
		filepath.Join("../../bin", executableName), // When run from examples/agent directory
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

// maskAPIKey masks an API key for safe logging
func maskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) < 8 {
		return "(set but short)"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
