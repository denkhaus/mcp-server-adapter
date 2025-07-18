#!/usr/bin/env node
/**
 * Simple MCP Server for testing purposes (Node.js version).
 * Implements basic MCP protocol over stdio.
 */

const readline = require('readline');

class SimpleMCPServer {
    constructor() {
        this.tools = [
            {
                name: "greet",
                description: "Greet someone with a message",
                inputSchema: {
                    type: "object",
                    properties: {
                        name: {
                            type: "string",
                            description: "Name to greet"
                        }
                    },
                    required: ["name"]
                }
            },
            {
                name: "multiply",
                description: "Multiply two numbers",
                inputSchema: {
                    type: "object",
                    properties: {
                        a: { type: "number", description: "First number" },
                        b: { type: "number", description: "Second number" }
                    },
                    required: ["a", "b"]
                }
            },
            {
                name: "timestamp",
                description: "Get current timestamp",
                inputSchema: {
                    type: "object",
                    properties: {}
                }
            }
        ];
    }

    handleRequest(request) {
        const { method, params = {}, id = 1 } = request;

        try {
            switch (method) {
                case "tools/list":
                    return {
                        jsonrpc: "2.0",
                        id,
                        result: {
                            tools: this.tools
                        }
                    };

                case "tools/call":
                    const { name: toolName, arguments: args = {} } = params;
                    
                    switch (toolName) {
                        case "greet":
                            const name = args.name || "World";
                            return {
                                jsonrpc: "2.0",
                                id,
                                result: {
                                    content: [
                                        {
                                            type: "text",
                                            text: `Hello, ${name}! Greetings from Node.js MCP server.`
                                        }
                                    ]
                                }
                            };

                        case "multiply":
                            const a = args.a || 0;
                            const b = args.b || 0;
                            const result = a * b;
                            return {
                                jsonrpc: "2.0",
                                id,
                                result: {
                                    content: [
                                        {
                                            type: "text",
                                            text: `Result: ${a} × ${b} = ${result}`
                                        }
                                    ]
                                }
                            };

                        case "timestamp":
                            const now = new Date().toISOString();
                            return {
                                jsonrpc: "2.0",
                                id,
                                result: {
                                    content: [
                                        {
                                            type: "text",
                                            text: `Current timestamp: ${now}`
                                        }
                                    ]
                                }
                            };

                        default:
                            return {
                                jsonrpc: "2.0",
                                id,
                                error: {
                                    code: -32601,
                                    message: `Tool not found: ${toolName}`
                                }
                            };
                    }

                case "initialize":
                    return {
                        jsonrpc: "2.0",
                        id,
                        result: {
                            protocolVersion: "2024-11-05",
                            capabilities: {
                                tools: {}
                            },
                            serverInfo: {
                                name: "simple-mcp-server-js",
                                version: "1.0.0"
                            }
                        }
                    };

                default:
                    return {
                        jsonrpc: "2.0",
                        id,
                        error: {
                            code: -32601,
                            message: `Method not found: ${method}`
                        }
                    };
            }
        } catch (error) {
            return {
                jsonrpc: "2.0",
                id,
                error: {
                    code: -32603,
                    message: `Internal error: ${error.message}`
                }
            };
        }
    }

    run() {
        console.error("Simple MCP Server (Node.js) starting...");
        console.error(`PID: ${process.pid}`);

        const rl = readline.createInterface({
            input: process.stdin,
            output: process.stdout,
            terminal: false
        });

        rl.on('line', (line) => {
            line = line.trim();
            if (!line) return;

            try {
                const request = JSON.parse(line);
                const response = this.handleRequest(request);
                console.log(JSON.stringify(response));
            } catch (error) {
                const errorResponse = {
                    jsonrpc: "2.0",
                    id: null,
                    error: {
                        code: -32700,
                        message: `Parse error: ${error.message}`
                    }
                };
                console.log(JSON.stringify(errorResponse));
            }
        });

        rl.on('close', () => {
            console.error("Server shutting down...");
        });

        process.on('SIGINT', () => {
            console.error("Received SIGINT, shutting down...");
            rl.close();
            process.exit(0);
        });
    }
}

if (require.main === module) {
    const server = new SimpleMCPServer();
    server.run();
}

module.exports = SimpleMCPServer;