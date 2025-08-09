// Package mcpadapter provides tests for the mock implementation.
package mcpadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tmc/langchaingo/tools"
)

func TestMockAdapter_BasicOperations(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	// Test initial state
	status := mock.GetServerStatusByName("test-server")
	if status != StatusStopped {
		t.Errorf("Expected initial status to be stopped, got %s", status.String())
	}

	// Test setting server status
	mock.SetServerStatus("test-server", StatusRunning)
	status = mock.GetServerStatusByName("test-server")
	if status != StatusRunning {
		t.Errorf("Expected status to be running, got %s", status.String())
	}

	// Test getting all statuses
	mock.SetServerStatus("server1", StatusRunning)
	mock.SetServerStatus("server2", StatusError)
	statuses := mock.GetAllServerStatuses()

	if len(statuses) != 3 {
		t.Errorf("Expected 3 servers, got %d", len(statuses))
	}

	if statuses["server1"] != StatusRunning {
		t.Errorf("Expected server1 to be running, got %s", statuses["server1"].String())
	}

	if statuses["server2"] != StatusError {
		t.Errorf("Expected server2 to be error, got %s", statuses["server2"].String())
	}
}

const testServerName = "test-server"

func TestMockAdapter_StartServer(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	ctx := context.Background()

	// Test successful start
	err := mock.StartServer(ctx, testServerName)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Check that the call was tracked
	calls := mock.GetStartServerCalls()
	if len(calls) != 1 || calls[0] != testServerName {
		t.Errorf("Expected one call to %s, got %v", testServerName, calls)
	}

	// Check that status was set to running
	status := mock.GetServerStatusByName(testServerName)
	if status != StatusRunning {
		t.Errorf("Expected status to be running, got %s", status.String())
	}
}

func TestMockAdapter_StartServerWithError(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	ctx := context.Background()
	expectedError := errors.New("start failed")

	// Set error for start operation
	mock.SetError("start", expectedError)

	err := mock.StartServer(ctx, "test-server")
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestMockAdapter_StartServerWithDelay(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	ctx := context.Background()
	delay := 50 * time.Millisecond

	// Set start delay
	mock.SetStartDelay(delay)

	start := time.Now()
	err := mock.StartServer(ctx, "test-server")
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if elapsed < delay {
		t.Errorf("Expected delay of at least %v, got %v", delay, elapsed)
	}
}

func TestMockAdapter_StartServerWithCancellation(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	ctx, cancel := context.WithCancel(context.Background())

	// Set a long delay
	mock.SetStartDelay(1 * time.Second)

	// Cancel the context after a short time
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := mock.StartServer(ctx, "test-server")
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestMockAdapter_StartAllServers(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	ctx := context.Background()

	// Set up multiple servers
	mock.SetServerStatus("server1", StatusStopped)
	mock.SetServerStatus("server2", StatusStopped)
	mock.SetServerStatus("server3", StatusStopped)

	err := mock.StartAllServers(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Check that all servers were started
	calls := mock.GetStartServerCalls()
	if len(calls) != 3 {
		t.Errorf("Expected 3 start calls, got %d", len(calls))
	}
}

func TestMockAdapter_StopServer(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	// Set server to running
	mock.SetServerStatus("test-server", StatusRunning)

	err := mock.StopServer("test-server")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Check that the call was tracked
	calls := mock.GetStopServerCalls()
	if len(calls) != 1 || calls[0] != "test-server" {
		t.Errorf("Expected one call to test-server, got %v", calls)
	}

	// Check that status was set to stopped
	status := mock.GetServerStatusByName("test-server")
	if status != StatusStopped {
		t.Errorf("Expected status to be stopped, got %s", status.String())
	}
}

func TestMockAdapter_Close(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	// Set up multiple running servers
	mock.SetServerStatus("server1", StatusRunning)
	mock.SetServerStatus("server2", StatusRunning)

	err := mock.Close()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Check that all servers are stopped
	statuses := mock.GetAllServerStatuses()
	for name, status := range statuses {
		if status != StatusStopped {
			t.Errorf("Expected server %s to be stopped, got %s", name, status.String())
		}
	}
}

func TestMockAdapter_WaitForServersReady(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	ctx := context.Background()

	// Set up servers - some running, some not
	mock.SetServerStatus("server1", StatusRunning)
	mock.SetServerStatus("server2", StatusStarting)

	// This should timeout since server2 is not running
	err := mock.WaitForServersReady(ctx, 50*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	// Now set all servers to running
	mock.SetServerStatus("server2", StatusRunning)

	err = mock.WaitForServersReady(ctx, 100*time.Millisecond)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestMockAdapter_GetLangChainTools(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	ctx := context.Background()

	// Create mock tools
	tool1 := NewMockTool("tool1", "First tool")
	tool2 := NewMockTool("tool2", "Second tool")
	tools := []tools.Tool{tool1, tool2}

	// Set tools for server
	mock.SetServerTools("test-server", tools)

	result, err := mock.GetToolsByServerName(ctx, "test-server")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(result))
	}

	// Check that the call was tracked
	calls := mock.GetGetToolsCalls()
	if len(calls) != 1 || calls[0] != "test-server" {
		t.Errorf("Expected one call to test-server, got %v", calls)
	}
}

func TestMockAdapter_GetLangChainToolsWithError(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	ctx := context.Background()
	expectedError := errors.New("tools failed")

	// Set error for tools operation
	mock.SetError("tools", expectedError)

	_, err := mock.GetToolsByServerName(ctx, "test-server")
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestMockAdapter_GetAllLangChainTools(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	ctx := context.Background()

	// Set up multiple servers with tools
	tool1 := NewMockTool("tool1", "Tool from server1")
	tool2 := NewMockTool("tool2", "Tool from server2")

	mock.SetServerStatus("server1", StatusRunning)
	mock.SetServerStatus("server2", StatusRunning)
	mock.SetServerStatus("server3", StatusStopped) // This one should be ignored

	mock.SetServerTools("server1", []tools.Tool{tool1})
	mock.SetServerTools("server2", []tools.Tool{tool2})
	mock.SetServerTools("server3", []tools.Tool{NewMockTool("tool3", "Should be ignored")})

	result, err := mock.GetAllTools(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(result))
	}

	// Check that tools have server prefixes
	expectedNames := []string{"server1.tool1", "server2.tool2"}
	actualNames := []string{result[0].Name(), result[1].Name()}

	for _, expected := range expectedNames {
		found := false
		for _, actual := range actualNames {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool name %s not found in %v", expected, actualNames)
		}
	}

	// Check that the call was tracked
	callCount := mock.GetGetAllToolsCalls()
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func TestMockAdapter_ConfigWatcher(t *testing.T) {
	mock := NewMockAdapter()
	defer mock.Reset()

	// Test default state
	if mock.IsConfigWatcherRunning() {
		t.Error("Expected config watcher to be false by default")
	}

	// Test setting config watcher
	mock.SetConfigWatcher(true)
	if !mock.IsConfigWatcherRunning() {
		t.Error("Expected config watcher to be true")
	}
}

func TestMockTool_Operations(t *testing.T) {
	tool := NewMockTool("test-tool", "Test tool description")

	// Test basic properties
	if tool.Name() != "test-tool" {
		t.Errorf("Expected name 'test-tool', got %s", tool.Name())
	}

	if tool.Description() != "Test tool description" {
		t.Errorf("Expected description 'Test tool description', got %s", tool.Description())
	}

	// Test default call
	ctx := context.Background()
	result, err := tool.Call(ctx, "test input")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	expectedResult := "Mock result from test-tool"
	if result != expectedResult {
		t.Errorf("Expected result %s, got %s", expectedResult, result)
	}

	// Test custom result
	tool.SetResult("custom result")
	result, err = tool.Call(ctx, "test input")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "custom result" {
		t.Errorf("Expected result 'custom result', got %s", result)
	}

	// Test error
	expectedError := errors.New("tool error")
	tool.SetError(expectedError)
	_, err = tool.Call(ctx, "test input")
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestMockAdapter_Reset(t *testing.T) {
	mock := NewMockAdapter()

	// Set up some state
	mock.SetServerStatus("server1", StatusRunning)
	mock.SetServerTools("server1", []tools.Tool{NewMockTool("tool1", "desc")})
	mock.SetError("start", errors.New("error"))
	mock.SetStartDelay(100 * time.Millisecond)
	mock.SetConfigWatcher(true)

	// Make some calls to track
	ctx := context.Background()
	_ = mock.StartServer(ctx, "server1")
	_ = mock.StopServer("server1")
	_, _ = mock.GetToolsByServerName(ctx, "server1")
	_, _ = mock.GetAllTools(ctx)

	// Verify state exists
	if len(mock.GetAllServerStatuses()) == 0 {
		t.Error("Expected some server statuses before reset")
	}

	if len(mock.GetStartServerCalls()) == 0 {
		t.Error("Expected some start calls before reset")
	}

	// Reset
	mock.Reset()

	// Verify everything is cleared
	if len(mock.GetAllServerStatuses()) != 0 {
		t.Error("Expected no server statuses after reset")
	}

	if len(mock.GetStartServerCalls()) != 0 {
		t.Error("Expected no start calls after reset")
	}

	if len(mock.GetStopServerCalls()) != 0 {
		t.Error("Expected no stop calls after reset")
	}

	if len(mock.GetGetToolsCalls()) != 0 {
		t.Error("Expected no get tools calls after reset")
	}

	if mock.GetGetAllToolsCalls() != 0 {
		t.Error("Expected no get all tools calls after reset")
	}

	if mock.IsConfigWatcherRunning() {
		t.Error("Expected config watcher to be false after reset")
	}
}
