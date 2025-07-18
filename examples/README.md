# MCP Server Adapter Examples

This directory contains examples demonstrating how to use the MCP Server Adapter with LangChain Go.

## Examples

### 1. Server Example (`server/`)

A simple MCP server that provides a `fetch_url` tool for fetching content from URLs.

**Features:**
- Implements the MCP protocol using `mark3labs/mcp-go`
- Provides a single tool that can fetch web content
- Uses stdio transport for communication
- Cross-platform HTTP client implementation

**Usage:**
```bash
cd examples/server
go run main.go
```

### 2. Agent Example (`agent/`)

A LangChain agent that uses the MCP server as a tool provider.

**Features:**
- Integrates MCP tools with LangChain agents
- Supports multiple LLM providers (OpenAI, Google AI, Anthropic)
- Combines MCP tools with standard LangChain tools (Calculator, Wikipedia)
- Automatically builds and manages the MCP server

**Prerequisites:**
Set one of the following environment variables:
- `OPENAI_API_KEY` - for OpenAI GPT models
- `GEMINI_API_KEY` - for Google AI (Gemini) models  
- `ANTHROPIC_API_KEY` - for Anthropic Claude models

**Usage:**
```bash
cd examples/agent
export OPENAI_API_KEY="your-api-key-here"  # or other LLM provider
go run main.go
```

## How It Works

1. **Server**: The MCP server (`server/main.go`) implements a simple tool that can fetch content from URLs
2. **Agent**: The agent (`agent/main.go`) creates an MCP adapter that connects to the server and exposes its tools as LangChain tools
3. **Integration**: The agent combines MCP tools with standard tools and uses them to answer questions

## Example Flow

1. Agent receives question: "Could you provide a summary of https://raw.githubusercontent.com/..."
2. Agent identifies it needs to fetch URL content
3. Agent calls the MCP `fetch_url` tool through our adapter
4. MCP server fetches the content and returns it
5. Agent processes the content and provides a summary

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   LangChain     │    │  MCP Adapter     │    │   MCP Server    │
│   Agent         │◄──►│  (Our Library)   │◄──►│   (stdio)       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                        │                        │
        │                        │                        │
    ┌───▼────┐              ┌────▼─────┐             ┌────▼─────┐
    │ OpenAI │              │ mark3labs│             │ HTTP     │
    │ Claude │              │ mcp-go   │             │ Client   │
    │ Gemini │              │ client   │             │          │
    └────────┘              └──────────┘             └──────────┘
```

## Key Features Demonstrated

- **Multi-transport support**: Uses stdio transport for reliable communication
- **Tool integration**: Seamlessly integrates MCP tools with LangChain
- **Error handling**: Robust error handling and resource cleanup
- **Cross-platform**: Works on Windows, macOS, and Linux
- **Multiple LLM support**: Works with different LLM providers
- **Automatic server management**: Builds and manages MCP server lifecycle

## Extending the Examples

You can extend these examples by:

1. **Adding more tools** to the server (e.g., file operations, API calls)
2. **Using different transports** (SSE, HTTP) instead of stdio
3. **Adding more sophisticated agents** with memory and planning
4. **Implementing custom tool validation** and error handling
5. **Adding configuration management** for multiple servers

## Troubleshooting

### Common Issues

1. **Server hangs**: Make sure the server binary is built correctly and stdio isn't being interfered with
2. **Tool not found**: Verify the server is running and tools are properly registered
3. **LLM errors**: Check that your API key is set and valid
4. **Build errors**: Ensure you have Go 1.21+ and all dependencies are available

### Debug Mode

Enable debug logging by setting the log level:

```go
adapter, err := mcpadapter.New(
    mcpadapter.WithLogLevel("debug"),
    // ... other options
)
```

This will show detailed information about MCP communication and tool execution.