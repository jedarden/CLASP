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
		"clasp_translate_response",
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
		Name:      "clasp_config",
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

func TestCallToolTranslate(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	// Create a sample Anthropic request
	anthropicReq := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello, world!",
			},
		},
	}
	anthropicReqJSON, _ := json.Marshal(anthropicReq)

	params := CallToolParams{
		Name:      "clasp_translate",
		Arguments: json.RawMessage(`{"request":` + string(anthropicReqJSON) + `,"model":"gpt-4o"}`),
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      11,
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
		t.Fatal("Translation should not be an error")
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	// Parse the result to verify translation occurred
	var translated map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &translated); err != nil {
		t.Fatalf("Failed to parse translation result: %v", err)
	}

	if format, ok := translated["format"].(string); !ok || format != "chat_completions" {
		t.Errorf("Expected format 'chat_completions', got %v", translated["format"])
	}

	if translated["translated"] == nil {
		t.Error("Expected translated request in result")
	}

	translatedRequest, ok := translated["translated"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected translated request to be a map, got: %T", translated["translated"])
	}

	if model, ok := translatedRequest["model"].(string); !ok || model != "gpt-4o" {
		t.Errorf("Expected translated.model to be 'gpt-4o', got: %v", translatedRequest["model"])
	}
	if maxTokens, ok := translatedRequest["max_tokens"].(float64); !ok || maxTokens != 1024 {
		t.Errorf("Expected translated.max_tokens to be 1024, got: %v", translatedRequest["max_tokens"])
	}

	messages, ok := translatedRequest["messages"].([]interface{})
	if !ok || len(messages) != 1 {
		t.Fatalf("Expected one translated message, got: %v", translatedRequest["messages"])
	}
	message, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected translated message to be a map, got: %T", messages[0])
	}
	if role, ok := message["role"].(string); !ok || role != "user" {
		t.Errorf("Expected translated message role 'user', got: %v", message["role"])
	}
	if content, ok := message["content"].(string); !ok || content != "Hello, world!" {
		t.Errorf("Expected translated message content 'Hello, world!', got: %v", message["content"])
	}
}

func TestCallToolTranslateWithResponsesAPI(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	// Create a sample Anthropic request with tools
	anthropicReq := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "What's the weather?",
			},
		},
		"tools": []map[string]interface{}{
			{
				"name":        "get_weather",
				"description": "Get current weather",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "City name",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}
	anthropicReqJSON, _ := json.Marshal(anthropicReq)

	params := CallToolParams{
		Name:      "clasp_translate",
		Arguments: json.RawMessage(`{"request":` + string(anthropicReqJSON) + `,"model":"gpt-4o","use_responses_api":true}`),
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      12,
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
		t.Fatal("Translation should not be an error")
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	// Parse the result to verify translation occurred
	var translated map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &translated); err != nil {
		t.Fatalf("Failed to parse translation result: %v", err)
	}

	if format, ok := translated["format"].(string); !ok || format != "responses_api" {
		t.Errorf("Expected format 'responses_api', got %v", translated["format"])
	}

	translatedRequest, ok := translated["translated"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected translated request to be a map, got: %T", translated["translated"])
	}
	if model, ok := translatedRequest["model"].(string); !ok || model != "gpt-4o" {
		t.Errorf("Expected translated.model to be 'gpt-4o', got: %v", translatedRequest["model"])
	}
	if maxOutputTokens, ok := translatedRequest["max_output_tokens"].(float64); !ok || maxOutputTokens != 1024 {
		t.Errorf("Expected translated.max_output_tokens to be 1024, got: %v", translatedRequest["max_output_tokens"])
	}

	input, ok := translatedRequest["input"].([]interface{})
	if !ok || len(input) != 1 {
		t.Fatalf("Expected one translated input item, got: %v", translatedRequest["input"])
	}
	inputMessage, ok := input[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected translated input item to be a map, got: %T", input[0])
	}
	if inputType, ok := inputMessage["type"].(string); !ok || inputType != "message" {
		t.Errorf("Expected translated input type 'message', got: %v", inputMessage["type"])
	}
	if role, ok := inputMessage["role"].(string); !ok || role != "user" {
		t.Errorf("Expected translated input role 'user', got: %v", inputMessage["role"])
	}
	if content, ok := inputMessage["content"].(string); !ok || content != "What's the weather?" {
		t.Errorf("Expected translated input content 'What's the weather?', got: %v", inputMessage["content"])
	}

	tools, ok := translatedRequest["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("Expected one translated tool, got: %v", translatedRequest["tools"])
	}
	tool, ok := tools[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected translated tool to be a map, got: %T", tools[0])
	}
	if toolType, ok := tool["type"].(string); !ok || toolType != "function" {
		t.Errorf("Expected translated tool type 'function', got: %v", tool["type"])
	}
	if toolName, ok := tool["name"].(string); !ok || toolName != "get_weather" {
		t.Errorf("Expected translated tool name 'get_weather', got: %v", tool["name"])
	}
}

func TestCallToolTranslateResponse(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderOpenAI,
	}
	server := NewServer("test-server", cfg)

	// Create a sample OpenAI response
	openaiResp := map[string]interface{}{
		"id":      "chatcmpl-123",
		"object":  "chat.completion",
		"created": 1234567890,
		"model":   "gpt-4o",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Hello! How can I help you today?",
				},
				"finish_reason": "stop",
			},
		},
	}
	openaiRespJSON, _ := json.Marshal(openaiResp)

	params := CallToolParams{
		Name:      "clasp_translate_response",
		Arguments: json.RawMessage(`{"response":` + string(openaiRespJSON) + `,"is_stream":false}`),
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      13,
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
		t.Fatal("Translation should not be an error")
	}
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	// Verify the response contains translated Anthropic format
	var responseInfo map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &responseInfo); err != nil {
		t.Fatalf("Failed to parse response info: %v", err)
	}

	// Check for format field
	if format, ok := responseInfo["format"].(string); !ok || format != "anthropic_message" {
		t.Errorf("Expected format 'anthropic_message', got: %v", responseInfo["format"])
	}

	// Check for translated field
	translated, ok := responseInfo["translated"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected translated field to be a map, got: %T", responseInfo["translated"])
	}

	// Verify basic Anthropic response structure
	if id, ok := translated["id"].(string); !ok || id != "chatcmpl-123" {
		t.Errorf("Expected translated.id to be 'chatcmpl-123', got: %v", id)
	}

	if respType, ok := translated["type"].(string); !ok || respType != "message" {
		t.Errorf("Expected translated.type to be 'message', got: %v", respType)
	}

	if role, ok := translated["role"].(string); !ok || role != "assistant" {
		t.Errorf("Expected translated.role to be 'assistant', got: %v", role)
	}

	if model, ok := translated["model"].(string); !ok || model != "gpt-4o" {
		t.Errorf("Expected translated.model to be 'gpt-4o', got: %v", model)
	}

	// Verify content block with text
	content, ok := translated["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("Expected translated.content to be a non-empty array")
	}

	firstBlock, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected content block to be a map, got: %T", content[0])
	}

	if blockType, ok := firstBlock["type"].(string); !ok || blockType != "text" {
		t.Errorf("Expected content block type 'text', got: %v", blockType)
	}

	if text, ok := firstBlock["text"].(string); !ok || text != "Hello! How can I help you today?" {
		t.Errorf("Expected content block text 'Hello! How can I help you today?', got: %v", text)
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

	// Run handler synchronously (will return on EOF or context timeout)
	done := make(chan struct{})
	go func() {
		server.handleStdio(ctx, input, output)
		close(done)
	}()

	// Wait for handler to finish
	select {
	case <-done:
		// Handler finished
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Handler did not finish in time")
	}

	// Now safe to check output
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
