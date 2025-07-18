// Package main demonstrates the V2 MCP Server Adapter using mark3labs/mcp-go and langchaingo.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	mcpadapter "github.com/denkhaus/mcp-server-adapter"
)

func main() { // #nosec G101
	fmt.Println("=== MCP Server Adapter V2 - Demo ===")
	fmt.Println("Using mark3labs/mcp-go client and langchaingo tools")

	adapter, err := initializeAdapter()
	if err != nil {
		log.Fatalf("Failed to initialize MCP adapter: %v", err)
	}
	defer func() {
		if err := adapter.Close(); err != nil {
			fmt.Printf("Failed to close adapter: %v\n", err)
		}
	}()

	ctx := context.Background()

	startAndMonitorServers(ctx, adapter)
	demonstrateToolUsage(ctx, adapter)
	demonstrateFileWatcher(adapter)

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("🎉 MCP Adapter Features Demonstrated:")
	fmt.Println("   • Uses mark3labs/mcp-go for robust MCP client support")
	fmt.Println("   • Implements langchaingo Tool interface for agent compatibility")
	fmt.Println("   • Supports all MCP transports (stdio, SSE, HTTP)")
	fmt.Println("   • Provides hot configuration reloading")
	fmt.Println("   • Aggregates tools from multiple servers")
	fmt.Println("   • Production-ready error handling and logging")
}

func initializeAdapter() (mcpadapter.MCPAdapter, error) {
	return mcpadapter.New(
		mcpadapter.WithConfigPath("../mcp-spec-config.json"),
		mcpadapter.WithLogLevel("info"),
		mcpadapter.WithFileWatcher(true),
	)
}

func startAndMonitorServers(ctx context.Context, adapter mcpadapter.MCPAdapter) {
	fmt.Println("\n1. Starting all MCP servers concurrently:")
	if err := adapter.StartAllServers(ctx); err != nil {
		fmt.Printf("   ⚠️  Some servers failed to start: %v\n", err)
	} else {
		fmt.Println("   ✅ All servers startup initiated")
	}

	fmt.Println("\n2. Waiting for servers to be ready...")
	if err := adapter.WaitForServersReady(ctx, 15*time.Second); err != nil {
		fmt.Printf("   ⚠️  Timeout or error waiting for servers: %v\n", err)
		fmt.Println("   📊 Detailed server statuses:")
		statuses := adapter.GetAllServerStatuses()
		for name, status := range statuses {
			fmt.Printf("      %s: %s\n", name, status.String())
		}
	} else {
		fmt.Println("   ✅ All enabled servers are ready")
	}

	fmt.Println("\n3. Server statuses:")
	statuses := adapter.GetAllServerStatuses()
	for serverName, status := range statuses {
		fmt.Printf("   %s: %s\n", serverName, status.String())
	}
}

func demonstrateToolUsage(ctx context.Context, adapter mcpadapter.MCPAdapter) {
	statuses := adapter.GetAllServerStatuses()
	var serverName string
	if statuses["youtube-transcript"].String() == "running" {
		serverName = "youtube-transcript"
	} else {
		for name, status := range statuses {
			if status.String() == "running" {
				serverName = name
				break
			}
		}
	}

	if serverName == "" {
		fmt.Println("\n4. No running servers available for tool testing")
	} else {
		fmt.Printf("\n4. Getting LangChain tools from %s:\n", serverName)
		tools, err := adapter.GetLangChainTools(ctx, serverName)
		if err != nil {
			fmt.Printf("   ❌ Failed to get tools: %v\n", err)
		} else {
			fmt.Printf("   ✅ Retrieved %d LangChain tools:\n", len(tools))
			for i, tool := range tools {
				fmt.Printf("      %d. %s\n", i+1, tool.Name())
				fmt.Printf("         Description: %s\n", truncateString(tool.Description(), 100))
			}

			if len(tools) > 0 {
				if mcpTool, ok := tools[0].(*mcpadapter.MCPTool); ok {
					testToolExecution(ctx, *mcpTool)
				} else {
					fmt.Printf("   ⚠️  Failed to cast tool to MCPTool: %T\n", tools[0])
				}
			}
		}
	}

	fmt.Println("\n6. Getting all LangChain tools from all servers:")
	allTools, err := adapter.GetAllLangChainTools(ctx)
	if err != nil {
		fmt.Printf("   ❌ Failed to get all tools: %v\n", err)
	} else {
		fmt.Printf("   ✅ Retrieved %d total tools from all servers:\n", len(allTools))
		for i, tool := range allTools {
			fmt.Printf("      %d. %s\n", i+1, tool.Name())
		}
	}
}

func testToolExecution(ctx context.Context, tool mcpadapter.MCPTool) {
	fmt.Printf("\n5. Testing tool execution:\n")
	fmt.Printf("   Testing tool: %s\n", tool.Name())

	var testInput string
	switch tool.Name() {
	case "fetch_url":
		testInput = `{"url": "https://httpbin.org/json"}`
	case "fetch_urls":
		testInput = `{"urls": ["https://httpbin.org/json"]}`
	case "get_transcript":
		testInput = `{"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"}`
	default:
		testInput = `{"text": "Hello from V2 adapter!"}`
	}

	fmt.Printf("   Input: %s\n", testInput)
	result, err := tool.Call(ctx, testInput)
	if err != nil {
		fmt.Printf("   ⚠️  Tool call failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Tool result: %s\n", truncateString(result, 300))
	}
}

func demonstrateFileWatcher(adapter mcpadapter.MCPAdapter) {
	fmt.Println("\n7. File watcher status:")
	if adapter.IsConfigWatcherRunning() {
		fmt.Println("   ✅ Configuration file watcher is running")
		fmt.Println("   💡 Try editing examples/mcp-spec-config.json to see hot reload!")
	} else {
		fmt.Println("   ⚠️  Configuration file watcher is not running")
	}
}

// Helper function to truncate strings for display
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
