// Package tests provides comprehensive tests for Claude Code tool support.
//
// This file tests the new Claude Code tool support including:
// 1. Tool detection (IsClaudeCodeTool)
// 2. Tool definition transformation (GetClaudeCodeToolDefinition)
// 3. Parameter schema validation for all Claude Code tools
// 4. New Task-related tools (TaskOutput, TaskStop, TaskCreate, TaskGet, TaskUpdate, TaskList)
// 5. Backward compatibility with older Claude Code versions
package tests

import (
	"testing"

	"github.com/jedarden/clasp/internal/translator"
	"github.com/jedarden/clasp/pkg/models"
)

// =============================================================================
// SECTION 1: Tool Detection Tests
// =============================================================================

// TestIsClaudeCodeTool_KnownTools verifies that all known Claude Code tools
// are correctly identified.
func TestIsClaudeCodeTool_KnownTools(t *testing.T) {
	knownTools := []string{
		"Read", "Write", "Edit", "Glob", "Grep", "Bash",
		"WebFetch", "WebSearch", "LSP", "NotebookEdit",
		"Task", "Skill", "AskUserQuestion",
		"EnterPlanMode", "ExitPlanMode", "TaskOutput", "TaskStop",
		"TaskCreate", "TaskGet", "TaskUpdate", "TaskList",
	}

	for _, toolName := range knownTools {
		t.Run(toolName, func(t *testing.T) {
			if !translator.IsClaudeCodeTool(toolName) {
				t.Errorf("IsClaudeCodeTool(%q) = false, want true", toolName)
			}
		})
	}
}

// TestIsClaudeCodeTool_UnknownTools verifies that unknown tools are not
// identified as Claude Code tools.
func TestIsClaudeCodeTool_UnknownTools(t *testing.T) {
	unknownTools := []string{
		"get_weather",
		"calculate_tax",
		"send_email",
		"custom_function",
		"analyze_data",
		"", // empty string
	}

	for _, toolName := range unknownTools {
		t.Run(toolName, func(t *testing.T) {
			if translator.IsClaudeCodeTool(toolName) {
				t.Errorf("IsClaudeCodeTool(%q) = true, want false", toolName)
			}
		})
	}
}

// TestIsClaudeCodeTool_CaseSensitive verifies that tool detection is case-sensitive.
func TestIsClaudeCodeTool_CaseSensitive(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"Read", true},
		{"read", false},      // lowercase
		{"READ", false},      // uppercase
		{"ReAd", false},      // mixed case
		{"Bash", true},
		{"bash", false},
		{"BASH", false},
		{"WebSearch", true},
		{"websearch", false},
		{"WEBSEARCH", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := translator.IsClaudeCodeTool(tc.input)
			if result != tc.expected {
				t.Errorf("IsClaudeCodeTool(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

// =============================================================================
// SECTION 2: Tool Definition Transformation Tests - Core Tools
// =============================================================================

// TestGetClaudeCodeToolDefinition_Read verifies the Read tool definition.
func TestGetClaudeCodeToolDefinition_Read(t *testing.T) {
	tool := models.AnthropicTool{
		Name:        "Read",
		Description: "Read file contents",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"file_path"},
		},
	}

	name, _, params := translator.GetClaudeCodeToolDefinition(tool)

	// Verify basic fields
	if name != "Read" {
		t.Errorf("name = %q, want 'Read'", name)
	}

	// Verify parameters structure
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		t.Fatal("params is not a map")
	}

	// Check type
	if paramsMap["type"] != "object" {
		t.Errorf("params.type = %v, want 'object'", paramsMap["type"])
	}

	// Check additionalProperties is false
	if paramsMap["additionalProperties"] != false {
		t.Errorf("params.additionalProperties = %v, want false", paramsMap["additionalProperties"])
	}

	// Check properties exist
	props, ok := paramsMap["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("params.properties is not a map")
	}

	// Verify file_path property
	filePath, ok := props["file_path"].(map[string]interface{})
	if !ok {
		t.Fatal("file_path property not found or not a map")
	}
	if filePath["type"] != "string" {
		t.Errorf("file_path.type = %v, want 'string'", filePath["type"])
	}

	// Verify required array
	required, ok := paramsMap["required"].([]string)
	if !ok {
		t.Fatal("required is not a string array")
	}
	if len(required) != 1 || required[0] != "file_path" {
		t.Errorf("required = %v, want [file_path]", required)
	}
}

// TestGetClaudeCodeToolDefinition_Write verifies the Write tool definition.
func TestGetClaudeCodeToolDefinition_Write(t *testing.T) {
	tool := models.AnthropicTool{Name: "Write", Description: "Write file", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	required, _ := paramsMap["required"].([]string)

	// Both file_path and content should be required
	if len(required) != 2 {
		t.Fatalf("expected 2 required params, got %d", len(required))
	}

	expectedRequired := map[string]bool{"file_path": true, "content": true}
	for _, r := range required {
		if !expectedRequired[r] {
			t.Errorf("unexpected required param: %s", r)
		}
	}
}

// TestGetClaudeCodeToolDefinition_Bash verifies the Bash tool definition.
func TestGetClaudeCodeToolDefinition_Bash(t *testing.T) {
	tool := models.AnthropicTool{Name: "Bash", Description: "Execute command", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	props, _ := paramsMap["properties"].(map[string]interface{})

	// Verify command is required
	required, _ := paramsMap["required"].([]string)
	if len(required) != 1 || required[0] != "command" {
		t.Errorf("required = %v, want [command]", required)
	}

	// Verify optional parameters
	optionalParams := []string{"description", "timeout", "run_in_background", "dangerouslyDisableSandbox"}
	for _, param := range optionalParams {
		if _, ok := props[param]; !ok {
			t.Errorf("missing optional parameter: %s", param)
		}
	}

	// Verify timeout defaults to 120000ms
	if timeout, ok := props["timeout"].(map[string]interface{}); ok {
		if timeout["type"] != "integer" {
			t.Errorf("timeout type = %v, want 'integer'", timeout["type"])
		}
	}
}

// TestGetClaudeCodeToolDefinition_Grep verifies the Grep tool definition.
func TestGetClaudeCodeToolDefinition_Grep(t *testing.T) {
	tool := models.AnthropicTool{Name: "Grep", Description: "Search files", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	props, _ := paramsMap["properties"].(map[string]interface{})

	// Verify pattern is required
	required, _ := paramsMap["required"].([]string)
	if len(required) != 1 || required[0] != "pattern" {
		t.Errorf("required = %v, want [pattern]", required)
	}

	// Verify output_mode enum
	if outputMode, ok := props["output_mode"].(map[string]interface{}); ok {
		enum, ok := outputMode["enum"].([]string)
		if !ok {
			t.Error("output_mode.enum is not a string array")
		} else {
			expectedEnum := []string{"content", "files_with_matches", "count"}
			if len(enum) != len(expectedEnum) {
				t.Errorf("output_mode.enum length = %d, want %d", len(enum), len(expectedEnum))
			}
		}
	}

	// Verify context flags
	contextFlags := []string{"-A", "-B", "-C"}
	for _, flag := range contextFlags {
		if flagProp, ok := props[flag].(map[string]interface{}); ok {
			if flagProp["type"] != "integer" {
				t.Errorf("%s type = %v, want 'integer'", flag, flagProp["type"])
			}
		}
	}
}

// =============================================================================
// SECTION 3: Tool Definition Transformation Tests - Web Tools
// =============================================================================

// TestGetClaudeCodeToolDefinition_WebFetch verifies the WebFetch tool.
func TestGetClaudeCodeToolDefinition_WebFetch(t *testing.T) {
	tool := models.AnthropicTool{Name: "WebFetch", Description: "Fetch URL", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	required, _ := paramsMap["required"].([]string)

	// Both url and prompt should be required
	if len(required) != 2 {
		t.Fatalf("expected 2 required params, got %d", len(required))
	}

	requiredMap := make(map[string]bool)
	for _, r := range required {
		requiredMap[r] = true
	}

	if !requiredMap["url"] {
		t.Error("url should be required")
	}
	if !requiredMap["prompt"] {
		t.Error("prompt should be required")
	}
}

// TestGetClaudeCodeToolDefinition_WebSearch verifies the WebSearch tool.
func TestGetClaudeCodeToolDefinition_WebSearch(t *testing.T) {
	tool := models.AnthropicTool{Name: "WebSearch", Description: "Search web", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	props, _ := paramsMap["properties"].(map[string]interface{})

	// Verify query is required
	required, _ := paramsMap["required"].([]string)
	if len(required) != 1 || required[0] != "query" {
		t.Errorf("required = %v, want [query]", required)
	}

	// Verify domain filters are arrays
	for _, prop := range []string{"allowed_domains", "blocked_domains"} {
		if domainProp, ok := props[prop].(map[string]interface{}); ok {
			if domainProp["type"] != "array" {
				t.Errorf("%s type = %v, want 'array'", prop, domainProp["type"])
			}
			// Check items type
			if items, ok := domainProp["items"].(map[string]interface{}); ok {
				if items["type"] != "string" {
					t.Errorf("%s.items.type = %v, want 'string'", prop, items["type"])
				}
			}
		}
	}
}

// =============================================================================
// SECTION 4: Tool Definition Transformation Tests - Agent/Task Tools
// =============================================================================

// TestGetClaudeCodeToolDefinition_Task verifies the Task tool.
func TestGetClaudeCodeToolDefinition_Task(t *testing.T) {
	tool := models.AnthropicTool{Name: "Task", Description: "Launch agent", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	required, _ := paramsMap["required"].([]string)

	// description, prompt, subagent_type should be required
	if len(required) != 3 {
		t.Fatalf("expected 3 required params, got %d: %v", len(required), required)
	}

	props, _ := paramsMap["properties"].(map[string]interface{})

	// Verify model enum
	if model, ok := props["model"].(map[string]interface{}); ok {
		enum, ok := model["enum"].([]string)
		if !ok {
			t.Error("model.enum is not a string array")
		} else {
			expectedEnum := []string{"sonnet", "opus", "haiku"}
			if len(enum) != len(expectedEnum) {
				t.Errorf("model.enum length = %d, want %d", len(enum), len(expectedEnum))
			}
		}
	}

	// Verify allowed_tools is array of strings
	if allowedTools, ok := props["allowed_tools"].(map[string]interface{}); ok {
		if allowedTools["type"] != "array" {
			t.Errorf("allowed_tools type = %v, want 'array'", allowedTools["type"])
		}
	}
}

// TestGetClaudeCodeToolDefinition_Skill verifies the Skill tool.
func TestGetClaudeCodeToolDefinition_Skill(t *testing.T) {
	tool := models.AnthropicTool{Name: "Skill", Description: "Execute skill", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	required, _ := paramsMap["required"].([]string)

	// Only skill should be required
	if len(required) != 1 || required[0] != "skill" {
		t.Errorf("required = %v, want [skill]", required)
	}

	props, _ := paramsMap["properties"].(map[string]interface{})

	// Verify args is optional
	if _, ok := props["args"]; !ok {
		t.Error("args should be an optional parameter")
	}
}

// =============================================================================
// SECTION 5: Tool Definition Transformation Tests - Plan Mode Tools
// =============================================================================

// TestGetClaudeCodeToolDefinition_EnterPlanMode verifies the EnterPlanMode tool.
func TestGetClaudeCodeToolDefinition_EnterPlanMode(t *testing.T) {
	tool := models.AnthropicTool{Name: "EnterPlanMode", Description: "Enter plan mode", InputSchema: map[string]interface{}{}}

	name, _, params := translator.GetClaudeCodeToolDefinition(tool)

	if name != "EnterPlanMode" {
		t.Errorf("name = %q, want 'EnterPlanMode'", name)
	}

	paramsMap, _ := params.(map[string]interface{})

	// Should have empty properties
	props, ok := paramsMap["properties"].(map[string]interface{})
	if !ok || len(props) != 0 {
		t.Errorf("EnterPlanMode should have empty properties, got %v", props)
	}

	// No required parameters
	if required, ok := paramsMap["required"]; ok {
		t.Errorf("EnterPlanMode should have no required params, got %v", required)
	}
}

// TestGetClaudeCodeToolDefinition_ExitPlanMode verifies the ExitPlanMode tool.
func TestGetClaudeCodeToolDefinition_ExitPlanMode(t *testing.T) {
	tool := models.AnthropicTool{Name: "ExitPlanMode", Description: "Exit plan mode", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	props, _ := paramsMap["properties"].(map[string]interface{})

	// Verify allowedPrompts structure
	if allowedPrompts, ok := props["allowedPrompts"].(map[string]interface{}); ok {
		if allowedPrompts["type"] != "array" {
			t.Errorf("allowedPrompts type = %v, want 'array'", allowedPrompts["type"])
		}
	}

	// Verify remote session parameters
	remoteParams := []string{"pushToRemote", "remoteSessionId", "remoteSessionTitle", "remoteSessionUrl"}
	for _, param := range remoteParams {
		if _, ok := props[param]; !ok {
			t.Errorf("missing parameter: %s", param)
		}
	}
}

// =============================================================================
// SECTION 6: Tool Definition Transformation Tests - New Task Management Tools
// =============================================================================

// TestGetClaudeCodeToolDefinition_TaskOutput verifies the TaskOutput tool.
func TestGetClaudeCodeToolDefinition_TaskOutput(t *testing.T) {
	tool := models.AnthropicTool{Name: "TaskOutput", Description: "Get task output", InputSchema: map[string]interface{}{}}

	name, _, params := translator.GetClaudeCodeToolDefinition(tool)

	if name != "TaskOutput" {
		t.Errorf("name = %q, want 'TaskOutput'", name)
	}

	paramsMap, _ := params.(map[string]interface{})
	required, _ := paramsMap["required"].([]string)

	// task_id, block, timeout should be required
	if len(required) != 3 {
		t.Fatalf("expected 3 required params, got %d: %v", len(required), required)
	}

	requiredMap := make(map[string]bool)
	for _, r := range required {
		requiredMap[r] = true
	}

	expectedRequired := []string{"task_id", "block", "timeout"}
	for _, exp := range expectedRequired {
		if !requiredMap[exp] {
			t.Errorf("%s should be required", exp)
		}
	}

	props, _ := paramsMap["properties"].(map[string]interface{})

	// Verify timeout has default and constraints
	if timeout, ok := props["timeout"].(map[string]interface{}); ok {
		if timeout["type"] != "number" {
			t.Errorf("timeout type = %v, want 'number'", timeout["type"])
		}
		// Default can be int or float64 depending on JSON unmarshaling
		if def, ok := timeout["default"].(int); !ok || def != 30000 {
			if defFloat, ok := timeout["default"].(float64); !ok || defFloat != 30000 {
				t.Errorf("timeout default = %v (type %T), want 30000", timeout["default"], timeout["default"])
			}
		}
		// Minimum and maximum can be int or float64
		if min, ok := timeout["minimum"].(int); !ok || min != 0 {
			if minFloat, ok := timeout["minimum"].(float64); !ok || minFloat != 0 {
				t.Errorf("timeout minimum = %v, want 0", timeout["minimum"])
			}
		}
		if max, ok := timeout["maximum"].(int); !ok || max != 600000 {
			if maxFloat, ok := timeout["maximum"].(float64); !ok || maxFloat != 600000 {
				t.Errorf("timeout maximum = %v, want 600000", timeout["maximum"])
			}
		}
	}

	// Verify block has default
	if block, ok := props["block"].(map[string]interface{}); ok {
		if def, ok := block["default"].(bool); !ok || !def {
			t.Errorf("block default = %v, want true", def)
		}
	}
}

// TestGetClaudeCodeToolDefinition_TaskStop verifies the TaskStop tool.
func TestGetClaudeCodeToolDefinition_TaskStop(t *testing.T) {
	tool := models.AnthropicTool{Name: "TaskStop", Description: "Stop task", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	props, _ := paramsMap["properties"].(map[string]interface{})

	// Verify both task_id and shell_id exist
	if _, ok := props["task_id"]; !ok {
		t.Error("missing task_id parameter")
	}
	if _, ok := props["shell_id"]; !ok {
		t.Error("missing shell_id parameter (deprecated)")
	}

	// No required parameters in schema (both are optional per spec)
	// This is because the tool can be called with either parameter
}

// TestGetClaudeCodeToolDefinition_TaskCreate verifies the TaskCreate tool.
func TestGetClaudeCodeToolDefinition_TaskCreate(t *testing.T) {
	tool := models.AnthropicTool{Name: "TaskCreate", Description: "Create task", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	required, _ := paramsMap["required"].([]string)

	// subject and description should be required
	if len(required) != 2 {
		t.Fatalf("expected 2 required params, got %d: %v", len(required), required)
	}

	requiredMap := make(map[string]bool)
	for _, r := range required {
		requiredMap[r] = true
	}

	if !requiredMap["subject"] {
		t.Error("subject should be required")
	}
	if !requiredMap["description"] {
		t.Error("description should be required")
	}

	props, _ := paramsMap["properties"].(map[string]interface{})

	// Verify optional parameters
	optionalParams := []string{"activeForm", "metadata"}
	for _, param := range optionalParams {
		if _, ok := props[param]; !ok {
			t.Errorf("missing optional parameter: %s", param)
		}
	}

	// Verify metadata is object
	if metadata, ok := props["metadata"].(map[string]interface{}); ok {
		if metadata["type"] != "object" {
			t.Errorf("metadata type = %v, want 'object'", metadata["type"])
		}
	}
}

// TestGetClaudeCodeToolDefinition_TaskGet verifies the TaskGet tool.
func TestGetClaudeCodeToolDefinition_TaskGet(t *testing.T) {
	tool := models.AnthropicTool{Name: "TaskGet", Description: "Get task", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	required, _ := paramsMap["required"].([]string)

	// Only taskId should be required
	if len(required) != 1 || required[0] != "taskId" {
		t.Errorf("required = %v, want [taskId]", required)
	}

	props, _ := paramsMap["properties"].(map[string]interface{})

	// Verify taskId property
	if taskId, ok := props["taskId"].(map[string]interface{}); ok {
		if taskId["type"] != "string" {
			t.Errorf("taskId type = %v, want 'string'", taskId["type"])
		}
	}
}

// TestGetClaudeCodeToolDefinition_TaskUpdate verifies the TaskUpdate tool.
func TestGetClaudeCodeToolDefinition_TaskUpdate(t *testing.T) {
	tool := models.AnthropicTool{Name: "TaskUpdate", Description: "Update task", InputSchema: map[string]interface{}{}}

	_, _, params := translator.GetClaudeCodeToolDefinition(tool)

	paramsMap, _ := params.(map[string]interface{})
	required, _ := paramsMap["required"].([]string)

	// Only taskId should be required (all others are optional)
	if len(required) != 1 || required[0] != "taskId" {
		t.Errorf("required = %v, want [taskId]", required)
	}

	props, _ := paramsMap["properties"].(map[string]interface{})

	// Verify status enum
	if status, ok := props["status"].(map[string]interface{}); ok {
		enum, ok := status["enum"].([]string)
		if !ok {
			t.Error("status.enum is not a string array")
		} else {
			expectedEnum := []string{"pending", "in_progress", "completed"}
			if len(enum) != len(expectedEnum) {
				t.Errorf("status.enum length = %d, want %d", len(enum), len(expectedEnum))
			}
		}
	}

	// Verify array parameters
	arrayParams := []string{"addBlocks", "addBlockedBy"}
	for _, param := range arrayParams {
		if arrayProp, ok := props[param].(map[string]interface{}); ok {
			if arrayProp["type"] != "array" {
				t.Errorf("%s type = %v, want 'array'", param, arrayProp["type"])
			}
			// Check items type
			if items, ok := arrayProp["items"].(map[string]interface{}); ok {
				if items["type"] != "string" {
					t.Errorf("%s.items.type = %v, want 'string'", param, items["type"])
				}
			}
		}
	}
}

// TestGetClaudeCodeToolDefinition_TaskList verifies the TaskList tool.
func TestGetClaudeCodeToolDefinition_TaskList(t *testing.T) {
	tool := models.AnthropicTool{Name: "TaskList", Description: "List tasks", InputSchema: map[string]interface{}{}}

	name, _, params := translator.GetClaudeCodeToolDefinition(tool)

	if name != "TaskList" {
		t.Errorf("name = %q, want 'TaskList'", name)
	}

	paramsMap, _ := params.(map[string]interface{})

	// Should have empty properties (lists all tasks)
	props, ok := paramsMap["properties"].(map[string]interface{})
	if !ok || len(props) != 0 {
		t.Errorf("TaskList should have empty properties, got %v", props)
	}

	// No required parameters
	if required, ok := paramsMap["required"]; ok {
		t.Errorf("TaskList should have no required params, got %v", required)
	}
}

// =============================================================================
// SECTION 7: Backward Compatibility Tests
// =============================================================================

// TestBackwardCompatibility_OldTools verifies that tools from older
// Claude Code versions are still handled correctly.
func TestBackwardCompatibility_OldTools(t *testing.T) {
	// These are tools that have existed in Claude Code for a while
	oldTools := []string{
		"Read", "Write", "Edit", "Bash", "Glob", "Grep",
		"WebFetch", "WebSearch",
	}

	for _, toolName := range oldTools {
		t.Run(toolName, func(t *testing.T) {
			// Verify tool is detected
			if !translator.IsClaudeCodeTool(toolName) {
				t.Errorf("IsClaudeCodeTool(%q) should return true for backward compatibility", toolName)
			}

			// Verify tool definition is available
			tool := models.AnthropicTool{
				Name:        toolName,
				Description: "Test",
				InputSchema: map[string]interface{}{"type": "object"},
			}

			name, _, params := translator.GetClaudeCodeToolDefinition(tool)

			if name != toolName {
				t.Errorf("tool name changed for %q (backward compatibility)", toolName)
			}

			if params == nil {
				t.Errorf("params nil for %s", toolName)
			}
		})
	}
}

// =============================================================================
// SECTION 8: Error Handling Tests
// =============================================================================

// TestGetClaudeCodeToolDefinition_UnknownTool verifies that unknown tools
// are passed through unchanged.
func TestGetClaudeCodeToolDefinition_UnknownTool(t *testing.T) {
	tool := models.AnthropicTool{
		Name:        "unknown_custom_tool",
		Description: "Custom tool",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}

	name, desc, params := translator.GetClaudeCodeToolDefinition(tool)

	// Should pass through unchanged
	if name != "unknown_custom_tool" {
		t.Errorf("name = %q, want 'unknown_custom_tool'", name)
	}

	if desc != "Custom tool" {
		t.Errorf("description = %q, want 'Custom tool'", desc)
	}

	// Params should be unchanged
	if params == nil {
		t.Error("params should not be nil for unknown tools")
	}
}

// TestGetClaudeCodeToolDefinition_EmptySchema verifies handling of tools
// with empty or minimal schemas.
func TestGetClaudeCodeToolDefinition_EmptySchema(t *testing.T) {
	tool := models.AnthropicTool{
		Name:        "Read",
		Description: "Read file",
		InputSchema: map[string]interface{}{},
	}

	// Should not panic
	name, _, params := translator.GetClaudeCodeToolDefinition(tool)

	if name != "Read" {
		t.Errorf("name = %q, want 'Read'", name)
	}

	if params == nil {
		t.Error("params should not be nil even with empty input schema")
	}
}

// TestGetClaudeCodeToolDefinition_NilSchema verifies handling of nil schemas.
func TestGetClaudeCodeToolDefinition_NilSchema(t *testing.T) {
	tool := models.AnthropicTool{
		Name:        "Write",
		Description: "Write file",
		InputSchema: nil,
	}

	// Should not panic
	name, _, params := translator.GetClaudeCodeToolDefinition(tool)

	if name != "Write" {
		t.Errorf("name = %q, want 'Write'", name)
	}

	// Nil schema should be handled gracefully
	_ = params
}

// =============================================================================
// SECTION 9: Integration Tests
// =============================================================================

// TestClaudeCodeToolsInRequest verifies that Claude Code tools are properly
// transformed when included in a full request.
func TestClaudeCodeToolsInRequest(t *testing.T) {
	req := &models.AnthropicRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 4096,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "Read the README.md file"},
		},
		Tools: []models.AnthropicTool{
			{
				Name:        "Read",
				Description: "Read file contents",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"file_path"},
				},
			},
			{
				Name:        "Write",
				Description: "Write content to a file",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_path": map[string]interface{}{"type": "string"},
						"content":   map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"file_path", "content"},
				},
			},
			{
				Name:        "TaskCreate",
				Description: "Create a task",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"subject":     map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"subject", "description"},
				},
			},
		},
	}

	// Transform the request
	result, err := translator.TransformRequest(req, "gpt-4o")
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	// Verify tools are transformed
	if len(result.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(result.Tools))
	}

	// Verify tool names
	expectedNames := []string{"Read", "Write", "TaskCreate"}
	for i, expectedName := range expectedNames {
		if result.Tools[i].Function.Name != expectedName {
			t.Errorf("tool[%d].name = %q, want %q", i, result.Tools[i].Function.Name, expectedName)
		}
	}

	// Verify strict mode is false for all tools (Claude Code compatibility)
	for i, tool := range result.Tools {
		if tool.Function.Strict != false {
			t.Errorf("tool[%d].strict = %v, want false (Claude Code compatibility)", i, tool.Function.Strict)
		}
	}
}

// TestAllToolNamesAreDistinct verifies that all tool name constants
// are unique (no duplicates).
func TestAllToolNamesAreDistinct(t *testing.T) {
	// This test verifies the constants in claude_code_tools.go
	tools := []string{
		translator.ToolRead,
		translator.ToolWrite,
		translator.ToolEdit,
		translator.ToolGlob,
		translator.ToolGrep,
		translator.ToolBash,
		translator.ToolWebFetch,
		translator.ToolWebSearch,
		translator.ToolLSP,
		translator.ToolNotebookEdit,
		translator.ToolTask,
		translator.ToolSkill,
		translator.ToolAskUserQuestion,
		translator.ToolEnterPlanMode,
		translator.ToolExitPlanMode,
		translator.ToolTaskOutput,
		translator.ToolTaskStop,
		translator.ToolTaskCreate,
		translator.ToolTaskGet,
		translator.ToolTaskUpdate,
		translator.ToolTaskList,
	}

	seen := make(map[string]bool)
	for _, tool := range tools {
		if seen[tool] {
			t.Errorf("Duplicate tool name: %s", tool)
		}
		seen[tool] = true
	}

	if len(seen) != len(tools) {
		t.Errorf("Expected %d unique tools, found %d", len(tools), len(seen))
	}
}
