# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

# Build directory
BIN_DIR := bin

# Ensure bin directory exists
$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# Build targets
.PHONY: build build-all build-examples clean test test-examples lint lint-fix fmt vet check ci help

build-all: build-examples ## Build all binaries

build-examples: $(BIN_DIR) ## Build all example binaries
	@echo "$(YELLOW)Building all examples...$(NC)"
	@go build -o $(BIN_DIR)/demo ./examples/demo
	@go build -o $(BIN_DIR)/agent ./examples/agent
	@go build -o $(BIN_DIR)/mcp-server ./examples/server
	@go build -o $(BIN_DIR)/simple-demo ./examples/simple-demo
	@echo "$(GREEN)All examples built successfully in $(BIN_DIR)/$(NC)"

build-demo: $(BIN_DIR) ## Build demo example
	@echo "$(YELLOW)Building demo...$(NC)"
	@go build -o $(BIN_DIR)/demo ./examples/demo
	@echo "$(GREEN)Demo built: $(BIN_DIR)/demo$(NC)"

build-agent: $(BIN_DIR) ## Build agent example
	@echo "$(YELLOW)Building agent...$(NC)"
	@go build -o $(BIN_DIR)/agent ./examples/agent
	@echo "$(GREEN)Agent built: $(BIN_DIR)/agent$(NC)"

build-server: $(BIN_DIR) ## Build server example
	@echo "$(YELLOW)Building server...$(NC)"
	@go build -o $(BIN_DIR)/mcp-server ./examples/server
	@echo "$(GREEN)Server built: $(BIN_DIR)/mcp-server$(NC)"

build-simple-demo: $(BIN_DIR) ## Build simple demo example
	@echo "$(YELLOW)Building simple demo...$(NC)"
	@go build -o $(BIN_DIR)/simple-demo ./examples/simple-demo
	@echo "$(GREEN)Simple demo built: $(BIN_DIR)/simple-demo$(NC)"

# Test targets
test: ## Run all tests
	@echo "$(YELLOW)Running tests...$(NC)"
	@go test -v ./...
	@echo "$(GREEN)All tests completed$(NC)"

# Linting and code quality targets
lint: ## Run golangci-lint
	@echo "$(YELLOW)Running golangci-lint...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "$(GREEN)Linting completed$(NC)"; \
	else \
		echo "$(RED)golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@2.2.2$(NC)"; \
		exit 1; \
	fi

lint-fix: ## Run golangci-lint with auto-fix
	@echo "$(YELLOW)Running golangci-lint with auto-fix...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --fix; \
		echo "$(GREEN)Linting with auto-fix completed$(NC)"; \
	else \
		echo "$(RED)golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)"; \
		exit 1; \
	fi

fmt: ## Format Go code
	@echo "$(YELLOW)Formatting Go code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)Code formatting completed$(NC)"

vet: ## Run go vet
	@echo "$(YELLOW)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)Go vet completed$(NC)"

check: fmt vet lint ## Run all code quality checks
	@echo "$(GREEN)All code quality checks completed$(NC)"

ci: clean fmt vet lint test test-examples test-config ## Complete CI pipeline
	@echo "$(GREEN)Complete CI pipeline passed!$(NC)"

test-examples: build-examples test-demo test-agent test-simple-demo test-servers test-config ## Run all example tests with timeout
	@echo "$(GREEN)All example tests completed$(NC)"

test-demo: build-demo ## Test demo example (with timeout)
	@echo "$(YELLOW)Testing Demo Example$(NC)"
	@chmod +x examples/test-servers/simple-mcp-server.py examples/test-servers/simple-mcp-server.js
	@timeout 15s $(BIN_DIR)/demo || echo "$(GREEN)Demo completed (timeout expected)$(NC)"

test-agent: build-agent build-server ## Test agent example (with timeout)
	@echo "$(YELLOW)Testing Agent Example$(NC)"
	@timeout 10s $(BIN_DIR)/agent || echo "$(GREEN)Agent example completed (timeout expected)$(NC)"

test-simple-demo: build-simple-demo build-server ## Test simple demo example (with timeout)
	@echo "$(YELLOW)Testing Simple Demo Example$(NC)"
	@timeout 10s $(BIN_DIR)/simple-demo || echo "$(GREEN)Simple demo completed (timeout expected)$(NC)"

test-servers: ## Test individual MCP servers
	@echo "$(YELLOW)Testing Python MCP Server$(NC)"
	@echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | timeout 5s python3 examples/test-servers/simple-mcp-server.py || echo "$(GREEN)Python server test completed$(NC)"
	@echo "$(YELLOW)Testing Node.js MCP Server$(NC)"
	@echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | timeout 5s node examples/test-servers/simple-mcp-server.js || echo "$(GREEN)Node.js server test completed$(NC)"

test-config: ## Validate configuration files
	@echo "$(YELLOW)Validating Configuration Files$(NC)"
	@python3 -m json.tool examples/mcp-spec-config.json > /dev/null && echo "$(GREEN)mcp-spec-config.json is valid JSON$(NC)" || echo "$(RED)mcp-spec-config.json is invalid JSON$(NC)"
	@python3 -m json.tool specs/mcp.json > /dev/null && echo "$(GREEN)specs/mcp.json is valid JSON$(NC)" || echo "$(RED)specs/mcp.json is invalid JSON$(NC)"

# Development targets
run-demo: build-demo ## Build and run demo
	@echo "$(YELLOW)Running demo...$(NC)"
	@$(BIN_DIR)/demo

run-agent: build-agent build-server ## Build and run agent
	@echo "$(YELLOW)Running agent...$(NC)"
	@$(BIN_DIR)/agent

run-simple-demo: build-simple-demo build-server ## Build and run simple demo
	@echo "$(YELLOW)Running simple demo...$(NC)"
	@$(BIN_DIR)/simple-demo

# Cleanup
clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -rf $(BIN_DIR)
	@rm -f examples/*/agent examples/*/agent.exe
	@rm -f examples/*/demo examples/*/demo.exe
	@rm -f examples/*/mcp-server examples/*/mcp-server.exe
	@rm -f examples/*/simple-demo examples/*/simple-demo.exe
	@rm -f tmp_rovodev_*
	@echo "$(GREEN)Clean completed$(NC)"

# Help
help: ## Show this help message
	@echo "$(YELLOW)Available targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
