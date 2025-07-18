// Package mcpadapter provides LangChain tool implementation using mark3labs/mcp-go.
package mcpadapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPTool implements the LangChain Tool interface for MCP tools using mark3labs/mcp-go.
type MCPTool struct {
	name        string
	description string
	inputSchema interface{}
	client      client.MCPClient
	serverName  string
	toolName    string // Original tool name (for prefixed tools)
}

// Name returns the name of the tool.
func (t *MCPTool) Name() string {
	return t.name
}

// Description returns the description of the tool, including JSON schema if available.
func (t *MCPTool) Description() string {
	if t.inputSchema != nil {
		// Embed the JSON schema in the description for better LLM understanding
		schemaBytes, err := json.MarshalIndent(t.inputSchema, "", "  ")
		if err == nil {
			return fmt.Sprintf("%s\n\nInput schema (JSON):\n%s", t.description, string(schemaBytes))
		}
	}
	return t.description
}

// Call executes the MCP tool using mark3labs/mcp-go client.
func (t *MCPTool) Call(ctx context.Context, input string) (string, error) {
	// Parse input JSON
	var arguments map[string]interface{}
	if input != "" {
		if err := json.Unmarshal([]byte(input), &arguments); err != nil {
			return "", fmt.Errorf("invalid JSON input for tool %s: %v. Please retry with correct JSON format", t.name, err)
		}
	}

	// Determine the actual tool name to call
	toolName := t.toolName
	if toolName == "" {
		toolName = t.name
	}

	// Create MCP tool call request
	callRequest := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: arguments,
		},
	}

	// Execute the tool
	result, err := t.client.CallTool(ctx, callRequest)
	if err != nil {
		return "", fmt.Errorf("tool %s execution failed: %v", t.name, err)
	}

	// Format the result
	return t.formatResult(result), nil
}

// formatResult converts the MCP tool result to a string format suitable for LangChain.
func (t *MCPTool) formatResult(result *mcp.CallToolResult) string {
	if result == nil {
		return "Tool executed successfully with no output"
	}

	if len(result.Content) == 0 {
		return "Tool executed successfully with no content"
	}

	// Process content array
	var parts []string
	for _, content := range result.Content {
		switch textContent := content.(type) {
		case mcp.TextContent:
			parts = append(parts, textContent.Text)
		case mcp.ImageContent:
			parts = append(parts, fmt.Sprintf("[Image: %s, MIME: %s]", textContent.Data, textContent.MIMEType))
		case mcp.AudioContent:
			parts = append(parts, fmt.Sprintf("[Audio: MIME: %s]", textContent.MIMEType))
		default:
			// Handle unknown content types
			if data, err := json.MarshalIndent(content, "", "  "); err == nil {
				parts = append(parts, string(data))
			} else {
				parts = append(parts, "[Unknown content type]")
			}
		}
	}

	if len(parts) == 0 {
		return "Tool executed successfully"
	}

	// Join all parts with newlines
	result_str := ""
	for i, part := range parts {
		if i > 0 {
			result_str += "\n"
		}
		result_str += part
	}

	return result_str
}
