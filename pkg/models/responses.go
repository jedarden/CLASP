// Package models defines shared types for the CLASP proxy.
package models

// OpenAI Responses API Types
// See: https://platform.openai.com/docs/api-reference/responses

// ResponsesRequest represents an OpenAI Responses API request.
type ResponsesRequest struct {
	Model              string               `json:"model"`
	Input              []ResponsesInput     `json:"input"`
	PreviousResponseID string               `json:"previous_response_id,omitempty"`
	Tools              []ResponsesTool      `json:"tools,omitempty"`
	ToolChoice         interface{}          `json:"tool_choice,omitempty"`
	MaxOutputTokens    int                  `json:"max_output_tokens,omitempty"`
	Stream             bool                 `json:"stream,omitempty"`
	Temperature        *float64             `json:"temperature,omitempty"`
	TopP               *float64             `json:"top_p,omitempty"`
	Background         bool                 `json:"background,omitempty"`
	Metadata           map[string]string    `json:"metadata,omitempty"`
	Reasoning          *ResponsesReasoning  `json:"reasoning,omitempty"`
	Instructions       string               `json:"instructions,omitempty"`
}

// ResponsesReasoning represents the nested reasoning configuration for Responses API.
// The Responses API requires reasoning parameters under this nested object.
type ResponsesReasoning struct {
	Effort string `json:"effort,omitempty"` // "low", "medium", "high"
}

// ResponsesInput represents an input item in a Responses request.
// Supports multiple item types: message, function_call, function_call_output, item_reference
type ResponsesInput struct {
	Type    string      `json:"type"` // "message", "function_call", "function_call_output", "item_reference"
	Role    string      `json:"role,omitempty"`
	Content interface{} `json:"content,omitempty"` // string or []ResponsesContentPart
	ID      string      `json:"id,omitempty"`      // For item_reference

	// Function call fields (type: "function_call")
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`

	// Function call output fields (type: "function_call_output")
	Output string `json:"output,omitempty"`
}

// ResponsesContentPart represents a content part in Responses input.
// Note: Responses API uses "input_text" and "input_image" for user/input content types,
// and "output_text" for assistant/output content types.
type ResponsesContentPart struct {
	Type     string             `json:"type"` // "input_text", "input_image", "output_text", "input_audio", "text", "image_url"
	Text     string             `json:"text,omitempty"`
	ImageURL *ImageURL          `json:"image_url,omitempty"`
	Audio    *ResponsesAudioPart `json:"input_audio,omitempty"`
}

// ResponsesAudioPart represents audio input in Responses API.
type ResponsesAudioPart struct {
	Data   string `json:"data"`   // Base64-encoded audio
	Format string `json:"format"` // "wav", "mp3", etc.
}

// ResponsesTool represents a tool definition in Responses API.
// Note: Responses API supports both the Chat Completions format (with nested function)
// and a flattened format where name/description/parameters are at the top level.
type ResponsesTool struct {
	Type        string                  `json:"type"` // "function", "code_interpreter", "file_search", "mcp", "custom"
	// Top-level fields for Responses API flattened format
	Name        string                  `json:"name,omitempty"`
	Description string                  `json:"description,omitempty"`
	Parameters  interface{}             `json:"parameters,omitempty"`
	// Nested function for backwards compatibility with Chat Completions format
	Function   *ResponsesFunction      `json:"function,omitempty"`
	MCPServer  *ResponsesMCPServer     `json:"mcp_server,omitempty"`
}

// ResponsesFunction represents a function tool in Responses API.
type ResponsesFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Strict      bool        `json:"strict,omitempty"`
}

// ResponsesMCPServer represents an MCP server tool in Responses API.
type ResponsesMCPServer struct {
	URL                 string   `json:"url"`
	AllowedTools        []string `json:"allowed_tools,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
}

// ResponsesResponse represents an OpenAI Responses API response.
type ResponsesResponse struct {
	ID                 string              `json:"id"`
	Object             string              `json:"object"` // "response"
	CreatedAt          int64               `json:"created_at"`
	Model              string              `json:"model"`
	Status             string              `json:"status"` // "completed", "in_progress", "failed", "cancelled"
	Output             []ResponsesItem     `json:"output"`
	Usage              *ResponsesUsage     `json:"usage,omitempty"`
	Metadata           map[string]string   `json:"metadata,omitempty"`
	Error              *ResponsesError     `json:"error,omitempty"`
}

// ResponsesItem represents an output item in Responses API.
// Items are polymorphic: reasoning, message, function_call, function_call_output
type ResponsesItem struct {
	Type      string      `json:"type"` // "reasoning", "message", "function_call", "function_call_output"
	ID        string      `json:"id,omitempty"`

	// Message fields
	Role      string      `json:"role,omitempty"`
	Content   interface{} `json:"content,omitempty"` // string or []ResponsesOutputContentPart

	// Reasoning fields (encrypted, not readable)
	Summary   string      `json:"summary,omitempty"`

	// Function call fields
	CallID    string      `json:"call_id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Arguments string      `json:"arguments,omitempty"`
	Status    string      `json:"status,omitempty"` // "in_progress", "completed", "failed"
	Output    string      `json:"output,omitempty"` // For function_call_output
}

// ResponsesOutputContentPart represents a content part in Responses output.
type ResponsesOutputContentPart struct {
	Type        string `json:"type"` // "text", "refusal", "audio"
	Text        string `json:"text,omitempty"`
	Refusal     string `json:"refusal,omitempty"`
	AudioData   string `json:"audio_data,omitempty"`
	Transcript  string `json:"transcript,omitempty"`
}

// ResponsesUsage represents usage statistics in Responses API.
type ResponsesUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	OutputTokensDetails      *ResponsesTokenDetails `json:"output_tokens_details,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
}

// ResponsesTokenDetails provides detailed token breakdown.
type ResponsesTokenDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

// ResponsesError represents an error in Responses API.
type ResponsesError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ResponsesStreamEvent represents an SSE event from Responses API streaming.
type ResponsesStreamEvent struct {
	Type      string           `json:"type"` // "response.created", "response.output_item.added", etc.
	Response  *ResponsesResponse `json:"response,omitempty"`
	Item      *ResponsesItem     `json:"item,omitempty"`
	Delta     *ResponsesDelta    `json:"delta,omitempty"`
	Error     *ResponsesError    `json:"error,omitempty"`
	Index     int                `json:"index,omitempty"`
}

// ResponsesDelta represents incremental content in streaming.
type ResponsesDelta struct {
	Type    string `json:"type"` // "text_delta", "refusal_delta", "function_call_arguments_delta"
	Text    string `json:"text,omitempty"`
	Refusal string `json:"refusal,omitempty"`
	CallID  string `json:"call_id,omitempty"`
	Delta   string `json:"delta,omitempty"` // For function call arguments
}

// Responses API SSE Event Types
const (
	// Response lifecycle events
	EventResponseCreated   = "response.created"
	EventResponseInProgress = "response.in_progress"
	EventResponseCompleted = "response.completed"
	EventResponseFailed    = "response.failed"
	EventResponseCancelled = "response.cancelled"

	// Output item events
	EventOutputItemAdded   = "response.output_item.added"
	EventOutputItemDone    = "response.output_item.done"

	// Content events
	EventContentPartAdded  = "response.content_part.added"
	EventContentPartDelta  = "response.content_part.delta"
	EventContentPartDone   = "response.content_part.done"

	// Function call events
	EventFunctionCallArgs  = "response.function_call_arguments.delta"
	EventFunctionCallDone  = "response.function_call_arguments.done"

	// Rate limit event
	EventRateLimitsUpdated = "rate_limits.updated"
)
