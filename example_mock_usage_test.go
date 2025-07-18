// Package mcpadapter provides examples of how to use the mock adapter in tests.
package mcpadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tmc/langchaingo/tools"
)

// Example: Testing a service that depends on MCPAdapter
type MCPService struct {
	adapter MCPAdapter
}

func NewMCPService(adapter MCPAdapter) *MCPService {
	return &MCPService{adapter: adapter}
}

func (s *MCPService) StartAndWaitForServers(ctx context.Context, timeout time.Duration) error {
	if err := s.adapter.StartAllServers(ctx); err != nil {
		return err
	}
	return s.adapter.WaitForServersReady(ctx, timeout)
}

func (s *MCPService) GetToolCount(ctx context.Context) (int, error) {
	tools, err := s.adapter.GetAllLangChainTools(ctx)
	if err != nil {
		return 0, err
	}
	return len(tools), nil
}

func (s *MCPService) IsHealthy() bool {
	statuses := s.adapter.GetAllServerStatuses()
	for _, status := range statuses {
		if status != StatusRunning {
			return false
		}
	}
	return len(statuses) > 0
}

// Example test using the mock adapter
func TestMCPService_StartAndWaitForServers_Success(t *testing.T) {
	// Create mock adapter
	mock := NewMockAdapter()
	defer mock.Reset()

	// Set up mock servers
	mock.SetServerStatus("server1", StatusStopped)
	mock.SetServerStatus("server2", StatusStopped)

	// Create service with mock
	service := NewMCPService(mock)

	// Test successful startup
	ctx := context.Background()
	err := service.StartAndWaitForServers(ctx, 1*time.Second)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify that StartAllServers was called by checking individual server calls
	startCalls := mock.GetStartServerCalls()
	if len(startCalls) != 2 {
		t.Errorf("Expected 2 start calls, got %d", len(startCalls))
	}
}

func TestMCPService_StartAndWaitForServers_StartFailure(t *testing.T) {
	// Create mock adapter
	mock := NewMockAdapter()
	defer mock.Reset()

	// Set up mock to fail on start
	expectedError := errors.New("start failed")
	mock.SetError("start", expectedError)
	mock.SetServerStatus("server1", StatusStopped)

	// Create service with mock
	service := NewMCPService(mock)

	// Test startup failure
	ctx := context.Background()
	err := service.StartAndWaitForServers(ctx, 1*time.Second)
	if err == nil {
		t.Error("Expected error, got nil")
	}

	// The error should be from StartAllServers, which wraps the start error
	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to contain %v, got %v", expectedError, err)
	}
}

func TestMCPService_StartAndWaitForServers_WaitTimeout(t *testing.T) {
	// Create mock adapter
	mock := NewMockAdapter()
	defer mock.Reset()

	// Set up servers that won't be ready (one stuck in starting state)
	mock.SetServerStatus("server1", StatusRunning)
	mock.SetServerStatus("server2", StatusStarting) // This one never becomes ready

	// Test wait timeout directly to avoid StartAllServers changing status
	ctx := context.Background()
	err := mock.WaitForServersReady(ctx, 50*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestMCPService_GetToolCount_Success(t *testing.T) {
	// Create mock adapter
	mock := NewMockAdapter()
	defer mock.Reset()

	// Set up mock servers with tools
	mock.SetServerStatus("server1", StatusRunning)
	mock.SetServerStatus("server2", StatusRunning)

	tool1 := NewMockTool("tool1", "First tool")
	tool2 := NewMockTool("tool2", "Second tool")
	tool3 := NewMockTool("tool3", "Third tool")

	mock.SetServerTools("server1", []tools.Tool{tool1, tool2})
	mock.SetServerTools("server2", []tools.Tool{tool3})

	// Create service with mock
	service := NewMCPService(mock)

	// Test getting tool count
	ctx := context.Background()
	count, err := service.GetToolCount(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 tools (server1 has 2 tools, server2 has 1 tool), got %d", count)
	}

	// Verify that GetAllLangChainTools was called
	callCount := mock.GetGetAllToolsCalls()
	if callCount != 1 {
		t.Errorf("Expected 1 call to GetAllLangChainTools, got %d", callCount)
	}
}

func TestMCPService_GetToolCount_Error(t *testing.T) {
	// Create mock adapter
	mock := NewMockAdapter()
	defer mock.Reset()

	// Set up mock to fail on getting tools
	expectedError := errors.New("tools failed")
	mock.SetError("alltools", expectedError)

	// Create service with mock
	service := NewMCPService(mock)

	// Test getting tool count with error
	ctx := context.Background()
	count, err := service.GetToolCount(ctx)
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}

	if count != 0 {
		t.Errorf("Expected count 0 on error, got %d", count)
	}
}

func TestMCPService_IsHealthy(t *testing.T) {
	// Create mock adapter
	mock := NewMockAdapter()
	defer mock.Reset()

	// Create service with mock
	service := NewMCPService(mock)

	// Test with no servers (should be unhealthy)
	if service.IsHealthy() {
		t.Error("Expected service to be unhealthy with no servers")
	}

	// Test with all servers running (should be healthy)
	mock.SetServerStatus("server1", StatusRunning)
	mock.SetServerStatus("server2", StatusRunning)

	if !service.IsHealthy() {
		t.Error("Expected service to be healthy with all servers running")
	}

	// Test with one server not running (should be unhealthy)
	mock.SetServerStatus("server2", StatusError)

	if service.IsHealthy() {
		t.Error("Expected service to be unhealthy with one server in error state")
	}
}

// Example: Testing complex scenarios with multiple mock configurations
func TestMCPService_ComplexScenario(t *testing.T) {
	// Create mock adapter
	mock := NewMockAdapter()
	defer mock.Reset()

	// Set up a complex scenario with multiple servers in different states
	mock.SetServerStatus("critical-server", StatusRunning)
	mock.SetServerStatus("optional-server", StatusError)
	mock.SetServerStatus("backup-server", StatusRunning)

	// Set up tools for running servers
	criticalTool := NewMockTool("critical-tool", "Critical functionality")
	backupTool := NewMockTool("backup-tool", "Backup functionality")

	mock.SetServerTools("critical-server", []tools.Tool{criticalTool})
	mock.SetServerTools("backup-server", []tools.Tool{backupTool})

	// Create service with mock
	service := NewMCPService(mock)

	// Test that service is unhealthy due to optional-server error
	if service.IsHealthy() {
		t.Error("Expected service to be unhealthy due to server error")
	}

	// Test that we can still get tools from running servers
	ctx := context.Background()
	count, err := service.GetToolCount(ctx)
	if err != nil {
		t.Errorf("Expected no error getting tools, got %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 tools from running servers, got %d", count)
	}

	// Fix the optional server and test again
	mock.SetServerStatus("optional-server", StatusRunning)
	optionalTool := NewMockTool("optional-tool", "Optional functionality")
	mock.SetServerTools("optional-server", []tools.Tool{optionalTool})

	if !service.IsHealthy() {
		t.Error("Expected service to be healthy after fixing server")
	}

	// Tool count should now include the optional server
	count, err = service.GetToolCount(ctx)
	if err != nil {
		t.Errorf("Expected no error getting tools, got %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 tools after fixing server, got %d", count)
	}
}

// Example: Testing with delays and timeouts
func TestMCPService_WithDelaysAndTimeouts(t *testing.T) {
	// Create mock adapter
	mock := NewMockAdapter()
	defer mock.Reset()

	// Set up servers with start delay
	mock.SetServerStatus("slow-server", StatusStopped)
	mock.SetStartDelay(100 * time.Millisecond)

	// Create service with mock
	service := NewMCPService(mock)

	// Test with sufficient timeout
	ctx := context.Background()
	start := time.Now()
	err := service.StartAndWaitForServers(ctx, 500*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error with sufficient timeout, got %v", err)
	}

	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected at least 100ms delay, got %v", elapsed)
	}

	// Reset and test with insufficient timeout
	mock.Reset()
	mock.SetServerStatus("slow-server", StatusStarting) // Keep it in starting state

	// Test wait timeout directly to avoid StartAllServers changing status
	err = mock.WaitForServersReady(ctx, 50*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error with insufficient timeout")
	}
}

// Example: Testing call tracking and verification
func TestMCPService_CallTracking(t *testing.T) {
	// Create mock adapter
	mock := NewMockAdapter()
	defer mock.Reset()

	// Set up servers
	mock.SetServerStatus("server1", StatusStopped)
	mock.SetServerStatus("server2", StatusStopped)

	tool1 := NewMockTool("tool1", "Tool 1")
	tool2 := NewMockTool("tool2", "Tool 2")
	mock.SetServerTools("server1", []tools.Tool{tool1})
	mock.SetServerTools("server2", []tools.Tool{tool2})

	// Create service with mock
	service := NewMCPService(mock)

	ctx := context.Background()

	// Perform operations
	_ = service.StartAndWaitForServers(ctx, 1*time.Second)
	_, _ = service.GetToolCount(ctx)
	_, _ = service.GetToolCount(ctx) // Call twice

	// Verify call tracking
	startCalls := mock.GetStartServerCalls()
	expectedStartCalls := []string{"server1", "server2"}

	if len(startCalls) != len(expectedStartCalls) {
		t.Errorf("Expected %d start calls, got %d", len(expectedStartCalls), len(startCalls))
	}

	for _, expected := range expectedStartCalls {
		found := false
		for _, actual := range startCalls {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected start call for %s not found in %v", expected, startCalls)
		}
	}

	// Verify GetAllLangChainTools was called twice
	allToolsCalls := mock.GetGetAllToolsCalls()
	if allToolsCalls != 2 {
		t.Errorf("Expected 2 calls to GetAllLangChainTools, got %d", allToolsCalls)
	}
}
