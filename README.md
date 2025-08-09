# MCP Server Adapter

## Description

![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/denkhaus/mcp-server-adapter/go.yml)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/denkhaus/mcp-server-adapter)
[![License](https://img.shields.io/github/license/denkhaus/mcp-server-adapter)](LICENSE)

The `mcp-server-adapter` is a Go package designed to simplify the integration of Model Context Protocol (MCP) servers with LangChain Go applications. It acts as an intermediary, allowing LangChain agents to seamlessly discover, use, and manage tools exposed by various MCP servers. This adapter handles server lifecycle management, configuration watching, and the conversion of MCP tools into a format consumable by LangChain.

**Note: This project is currently a work in progress. While functional, it may undergo significant changes, including breaking API changes, as it evolves.**

## Features

- **Seamless LangChain Integration**: Exposes MCP server tools as native LangChain `tools.Tool` objects.
- **MCP Server Lifecycle Management**: Start, stop, and monitor the status of multiple MCP servers.
- **Dynamic Configuration**: Supports loading server configurations from a JSON file and hot-reloading changes.
- **Server Status and Configuration Access**: Easily check if a server is disabled or retrieve its full configuration.
- **File Watcher**: Automatically detects and reacts to changes in the configuration file, restarting affected servers.
- **Multiple Transport Support**: Connects to MCP servers via standard I/O (stdio), Server-Sent Events (SSE), and HTTP transports.
- **Robust Error Handling**: Includes mechanisms for safely closing server connections and handling common errors.
- **Extensible**: Designed with interfaces for easy extension and testing (e.g., custom client factories).

## Installation

This project requires Go 1.21 or higher.

1.  **Clone the repository**:
    ```bash
    git clone https://github.com/denkhaus/mcp-server-adapter.git
    cd mcp-server-adapter
    ```
2.  **Install dependencies**:
    ```bash
    go mod tidy
    ```
3.  **Build the project**:
    ```bash
    go build ./...
    ```

## Running Examples

The examples can be built and run using the provided `Makefile`. Navigate to the project root and use `make` commands.

For instance, to build all examples:

```bash
make build-examples
```

To run a specific example, like the demo:

```bash
make run-demo
```

## Usage

The `mcp-server-adapter` is typically used within a Go application to manage connections to MCP servers and integrate their tools into an LLM agent.

1.  **Define your MCP server configuration**: Create a JSON file (e.g., `config.json`) specifying the MCP servers your application needs to connect to. See `examples/mcp-spec-config.json` for a practical example.

    ```json
    {
      "mcpServers": {
        "fetcher": {
          "command": "npx",
          "args": ["-y", "fetcher-mcp"],
          "transport": "stdio"
        },
        "tavily-mcp": {
          "command": "npx",
          "args": ["-y", "tavily-mcp@0.1.3"],
          "transport": "stdio"
        }
      }
    }
    ```

2.  **Initialize the adapter**:

    ```go
    package main

    import (
    	"context"
    	"log"
    	"time"

    	"github.com/denkhaus/mcp-server-adapter"
    )

    func main() {
    	adapter, err := mcpadapter.New(
    		mcpadapter.WithConfigPath("./config.json"),
    		mcpadapter.WithLogLevel("info"),
    		mcpadapter.WithFileWatcher(true), // Enable hot-reloading of config
    	)
    	if err != nil {
    		log.Fatalf("Failed to create MCP adapter: %v", err)
    	}
    	defer adapter.Close()

        ctx := context.Background()

    	// Start all configured MCP servers
    	if err := adapter.StartAllServers(ctx); err != nil {
    		log.Fatalf("Failed to start all MCP servers: %v", err)
    	}

    	// Wait for servers to be ready
    	if err := adapter.WaitForServersReady(ctx, 30*time.Second); err != nil {
    		log.Fatalf("Servers not ready: %v", err)
    	}

    	// Get all available tools from all running servers
    	allTools, err := adapter.GetAllTools(ctx)
    	if err != nil {
    		log.Fatalf("Failed to get LangChain tools: %v", err)
    	}

    	log.Printf("Available tools: %d", len(allTools))
    	for _, tool := range allTools {
    		log.Printf("- %s: %s", tool.Name(), tool.Description())
    	}

        // Example: Check if a server is disabled
        isDisabled, err := adapter.IsServerDisabled("fetcher")
        if err != nil {
            log.Printf("Error checking if server is disabled: %v", err)
        } else if isDisabled {
            log.Printf("Server 'fetcher' is disabled.")
        } else {
            log.Printf("Server 'fetcher' is enabled.")
        }

        // Example: Get server configuration
        serverConfig, err := adapter.GetServerConfig("tavily-mcp")
        if err != nil {
            log.Printf("Error getting server config: %v", err)
        } else {
            log.Printf("Server 'tavily-mcp' command: %s", serverConfig.Command)
        }

    	// Example: Using a tool (replace with actual tool usage)
    	// If you have a "fetcher.fetch_url" tool, you could call it like this:
    	// if len(allTools) > 0 {
    	// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    	// 	defer cancel()
    	// 	result, err := allTools[0].Call(ctx, `{"url": "https://example.com"}`)
    	// 	if err != nil {
    	// 		log.Printf("Tool call failed: %v", err)
    	// 	} else {
    	// 		log.Printf("Tool result: %s", result)
    	// 	}
    	// }
    }
    ```

## API Changes

To improve clarity and consistency, the following API functions have been renamed:

- `GetServerStatus(serverName string) ServerStatus` is now `GetServerStatusByName(serverName string) ServerStatus`.
- `GetLangChainTools(ctx context.Context, serverName string) ([]tools.Tool, error)` is now `GetToolsByServerName(ctx context.Context, serverName string) ([]tools.Tool, error)`.
- `GetAllLangChainTools(ctx context.Context) ([]tools.Tool, error)` is now `GetAllTools(ctx context.Context) ([]tools.Tool, error)`.

## Examples

The `mcp-server-adapter` includes several examples to demonstrate its capabilities and integration with LangChain applications. For detailed usage, code, and technical specifications, please refer to the [`examples/`](examples/) directory, and especially the [`examples/EXAMPLES_SUMMARY.md`](examples/EXAMPLES_SUMMARY.md) file.

Here's a summary of the key examples:

## Makefile Targets

This project uses a `Makefile` to automate various tasks, including building, testing, linting, and running examples. Below is a comprehensive list of available `make` targets:

### Build Targets

- `make build-all`: Build all binaries.
- `make build-examples`: Build all example binaries.
- `make build-demo`: Build demo example.
- `make build-agent`: Build agent example.
- `make build-server`: Build server example.
- `make build-simple-demo`: Build simple demo example.

### Test Targets

- `make test`: Run all tests.
- `make test-examples`: Run all example tests with timeout.
- `make test-demo`: Test demo example (with timeout).
- `make test-agent`: Test agent example (with timeout).
- `make test-simple-demo`: Test simple demo example (with timeout).
- `make test-servers`: Test individual MCP servers.
- `make test-config`: Validate configuration files.

### Linting and Code Quality Targets

- `make lint`: Run golangci-lint.
- `make lint-fix`: Run golangci-lint with auto-fix.
- `make fmt`: Format Go code.
- `make vet`: Run go vet.
- `make check`: Run all code quality checks.
- `make ci`: Complete CI pipeline.

### Development Targets

- `make run-demo`: Build and run demo.
- `make run-agent`: Build and run agent.
- `make run-simple-demo`: Build and run simple demo.

### Cleanup

- `make clean`: Clean build artifacts.

### Help

- `make help`: Show this help message.

---

### 1. 🖥️ **MCP Server** (`examples/server/`)

- **Purpose**: Demonstrates how to create a basic MCP server using `mark3labs/mcp-go`.
- **Features**: Implements a `fetch_url` tool for web content retrieval via stdio transport.

### 2. 🤖 **LangChain Agent** (`examples/agent/`)

- **Purpose**: Shows how to integrate MCP tools with LangChain agents.
- **Features**: Supports multi-LLM (OpenAI, Google AI, Anthropic), combines MCP tools with standard LangChain tools, and manages MCP server lifecycle automatically.

### 3. 🚀 **Simple Demo** (`examples/simple-demo/`)

- **Purpose**: A basic demonstration without requiring LLM API keys.
- **Features**: Shows MCP adapter creation and tool discovery, tests tool execution with real HTTP requests, and has no external dependencies beyond Go.

### 4. 🧪 **Test Servers** (`examples/test-servers/`)

- **Purpose**: Provides simple MCP servers implemented in different languages (Node.js, Python) for testing purposes.

### 5. ⚙️ **Configuration Files** (`examples/config.json`, `examples/enhanced-config.json`, `examples/mcp-spec-config.json`)

- **Purpose**: Illustrates various configurations for MCP servers, including `stdio` and `sse` transports, and different command arguments.

## Configuration

The `mcp-server-adapter` is configured via a JSON file. The `Config` struct (defined in `config.go` and `types.go`) outlines the expected structure.

Key configuration options include:

- `mcpServers`: A map where keys are server names and values are `ServerConfig` objects.
- `ServerConfig`:
  - `command`: (Required for `stdio` transport) The command to execute the MCP server.
  - `args`: (Optional) Arguments to pass to the command.
  - `cwd`: (Optional) The working directory for the server process.
  - `env`: (Optional) Environment variables for the server process.
  - `disabled`: (Optional, `true`/`false`) If `true`, the server will not be started.
  - `transport`: (Optional, default `stdio`) The communication transport (`stdio`, `sse`, `http`).
  - `url`: (Required for `sse`, `http` transports) The URL of the MCP server.
  - `headers`: (Optional) HTTP headers for `sse` or `http` transports.
  - `timeout`: (Optional) Timeout for server operations.
  - `Method`: (Optional, default `POST` for `http` transport) The HTTP method for `http` transport (e.g., `GET`, `POST`).
  - `tool_prefix`: (Optional, default empty string) A custom prefix for resolved tool names. When specified, the tool name format changes from `serverName.toolName` to `tool_prefix/toolname`.
    Example: `"tool_prefix": "magic-prefix"`
  - `alwaysAllow`: (Optional) A list of tool names that are always permitted, usable for permissions or access control.

## Inspiration

This project was inspired by [`langchaingo-mcp-adapter`](https://github.com/i2y/langchaingo-mcp-adapter.git).

## Contributing

Contributions are welcome! Please refer to the project's issue tracker for open tasks, or submit pull requests with improvements and bug fixes. Ensure your code adheres to the existing style and conventions.

## License

This project is licensed under the MIT License. See the [`LICENSE`](./LICENSE) file for details.