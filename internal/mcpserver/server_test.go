package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jedarden/clasp/internal/config"
)

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	if server == nil {
		t.Fatal("Expected server to be created")
	}
	if server.name != "test-server" {
		t.Errorf("Expected name 'test-server', got '%s'", server.name)
	}
	if server.version != Version {
		t.Errorf("Expected version '%s', got '%s'", Version, server.version)
	}
}

func TestInitialize(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  paramsJSON,
	}

	resp := server.handleRequest(req)

	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
	if resp.ID != 1 {
		t.Errorf("Expected ID 1, got %v", resp.ID)
	}

	result, ok := resp.Result.(InitializeResult)
	if !ok {
		t.Fatal("Expected InitializeResult")
	}
	if result.ServerInfo.Name != "test-server" {
		t.Errorf("Expected server name 'test-server', got '%s'", result.ServerInfo.Name)
	}
	if result.Capabilities.Tools == nil {
		t.Error("Expected tools capability")
	}
}

func TestListTools(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp := server.handleRequest(req)

	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(ListToolsResult)
	if !ok {
		t.Fatal("Expected ListToolsResult")
	}
	if len(result.Tools) == 0 {
		t.Error("Expected at least one tool")
	}

	// Check for expected tools
	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"clasp_status",
		"clasp_config",
		"clasp_profile",
		"clasp_models",
		"clasp_metrics",
		"clasp_health",
		"clasp_doctor",
		"clasp_translate",
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Expected tool '%s' not found", expected)
		}
	}
}

func TestCallToolStatus(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	params := CallToolParams{
		Name: "clasp_status",
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := server.handleRequest(req)

	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(*CallToolResult)
	if !ok {
		t.Fatal("Expected CallToolResult")
	}
	if len(result.Content) == 0 {
		t.Error("Expected content in result")
	}
	if result.IsError {
		t.Error("Result should not be an error")
	}
}

func TestCallToolConfig(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	// Test list action
	params := CallToolParams{
		Name: "clasp_config",
		Arguments: json.RawMessage(`{"action": "list"}`),
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := server.handleRequest(req)

	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(*CallToolResult)
	if !ok {
		t.Fatal("Expected CallToolResult")
	}
	if len(result.Content) == 0 {
		t.Error("Expected content in result")
	}
}

func TestCallToolHealth(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	params := CallToolParams{
		Name: "clasp_health",
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := server.handleRequest(req)

	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(*CallToolResult)
	if !ok {
		t.Fatal("Expected CallToolResult")
	}
	if result.IsError {
		t.Error("Health check should not be an error")
	}
}

func TestCallToolDoctor(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	params := CallToolParams{
		Name: "clasp_doctor",
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := server.handleRequest(req)

	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
}

func TestCallToolUnknown(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	params := CallToolParams{
		Name: "unknown_tool",
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := server.handleRequest(req)

	if resp == nil {
		t.Fatal("Expected response")
	}

	result, ok := resp.Result.(CallToolResult)
	if !ok {
		t.Fatal("Expected CallToolResult")
	}
	if !result.IsError {
		t.Error("Expected error for unknown tool")
	}
}

func TestPing(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "ping",
	}

	resp := server.handleRequest(req)

	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
}

func TestMethodNotFound(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "unknown_method",
	}

	resp := server.handleRequest(req)

	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.Error == nil {
		t.Fatal("Expected error for unknown method")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("Expected error code %d, got %d", MethodNotFound, resp.Error.Code)
	}
}

func TestInvalidJSONRPCVersion(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	req := &JSONRPCRequest{
		JSONRPC: "1.0", // Invalid version
		ID:      10,
		Method:  "ping",
	}

	resp := server.handleRequest(req)

	if resp == nil {
		t.Fatal("Expected response")
	}
	if resp.Error == nil {
		t.Fatal("Expected error for invalid JSON-RPC version")
	}
	if resp.Error.Code != InvalidRequest {
		t.Errorf("Expected error code %d, got %d", InvalidRequest, resp.Error.Code)
	}
}

func TestStdioHandler(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	// Create request
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ping",
	}
	reqJSON, _ := json.Marshal(req)
	reqJSON = append(reqJSON, '\n')

	// Create input/output buffers
	input := bytes.NewReader(reqJSON)
	output := &bytes.Buffer{}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run handler (will timeout on EOF)
	go server.handleStdio(ctx, input, output)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Check output
	if output.Len() == 0 {
		t.Error("Expected response output")
	}

	// Parse response
	var resp JSONRPCResponse
	if err := json.NewDecoder(output).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("Unexpected error: %v", resp.Error)
	}
}
