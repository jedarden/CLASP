// Package models defines shared types for the CLASP proxy.
package models

// AnthropicRequest represents an incoming Anthropic Messages API request.
type AnthropicRequest struct {
	Model         string             `json:"model"`
	System        interface{}        `json:"system,omitempty"` // Can be string or []ContentBlock
	Messages      []AnthropicMessage `json:"messages"`
	MaxTokens     int                `json:"max_tokens,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	TopK          *int               `json:"top_k,omitempty"`
	Tools         []AnthropicTool    `json:"tools,omitempty"`
	ToolChoice    interface{}        `json:"tool_choice,omitempty"`
	Metadata      *Metadata          `json:"metadata,omitempty"`
}

// AnthropicMessage represents a message in Anthropic format.
type AnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []ContentBlock
}

// ContentBlock represents a content block in Anthropic messages.
type ContentBlock struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Source *ImageSource `json:"source,omitempty"`
	// Tool use fields
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Input interface{} `json:"input,omitempty"`
	// Tool result fields
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

// ImageSource represents an image source in Anthropic format.
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// AnthropicTool represents a tool definition in Anthropic format.
type AnthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema"`
}

// Metadata represents request metadata.
type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// OpenAIRequest represents an outgoing OpenAI Chat Completions API request.
type OpenAIRequest struct {
	Model         string          `json:"model"`
	Messages      []OpenAIMessage `json:"messages"`
	MaxTokens     int             `json:"max_tokens,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	Stop          []string        `json:"stop,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	Tools         []OpenAITool    `json:"tools,omitempty"`
	ToolChoice    interface{}     `json:"tool_choice,omitempty"`
	StreamOptions *StreamOptions  `json:"stream_options,omitempty"`
}

// OpenAIMessage represents a message in OpenAI format.
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content,omitempty"` // string or []OpenAIContentPart
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// OpenAIContentPart represents a content part in OpenAI messages.
type OpenAIContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL in OpenAI format.
type ImageURL struct {
	URL string `json:"url"`
}

// OpenAIToolCall represents a tool call in OpenAI format.
type OpenAIToolCall struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function OpenAIFunctionCall   `json:"function"`
	Index    int                  `json:"index,omitempty"`
}

// OpenAIFunctionCall represents a function call in OpenAI format.
type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// OpenAITool represents a tool definition in OpenAI format.
type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

// OpenAIFunction represents a function definition in OpenAI format.
type OpenAIFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters"`
}

// StreamOptions for OpenAI streaming.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// OpenAIStreamChunk represents a streaming chunk from OpenAI.
type OpenAIStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// StreamChoice represents a choice in a streaming chunk.
type StreamChoice struct {
	Index        int          `json:"index"`
	Delta        StreamDelta  `json:"delta"`
	FinishReason string       `json:"finish_reason,omitempty"`
}

// StreamDelta represents the delta content in a streaming chunk.
type StreamDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// Usage represents token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// AnthropicResponse represents a response in Anthropic format.
type AnthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason,omitempty"`
	StopSequence string                  `json:"stop_sequence,omitempty"`
	Usage        *AnthropicUsage         `json:"usage,omitempty"`
}

// AnthropicContentBlock represents a content block in Anthropic response.
type AnthropicContentBlock struct {
	Type  string      `json:"type"`
	Text  string      `json:"text,omitempty"`
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Input interface{} `json:"input,omitempty"`
}

// AnthropicUsage represents usage in Anthropic format.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// SSE Event types for Anthropic streaming.
const (
	EventMessageStart      = "message_start"
	EventContentBlockStart = "content_block_start"
	EventContentBlockDelta = "content_block_delta"
	EventContentBlockStop  = "content_block_stop"
	EventMessageDelta      = "message_delta"
	EventMessageStop       = "message_stop"
	EventPing              = "ping"
)

// MessageStartEvent represents a message_start SSE event.
type MessageStartEvent struct {
	Type    string            `json:"type"`
	Message AnthropicResponse `json:"message"`
}

// ContentBlockStartEvent represents a content_block_start SSE event.
type ContentBlockStartEvent struct {
	Type         string                `json:"type"`
	Index        int                   `json:"index"`
	ContentBlock ContentBlockStartData `json:"content_block"`
}

// ContentBlockStartData represents the content_block in a start event.
type ContentBlockStartData struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// ContentBlockDeltaEvent represents a content_block_delta SSE event.
type ContentBlockDeltaEvent struct {
	Type  string     `json:"type"`
	Index int        `json:"index"`
	Delta DeltaData  `json:"delta"`
}

// DeltaData represents the delta in a content_block_delta event.
type DeltaData struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

// ContentBlockStopEvent represents a content_block_stop SSE event.
type ContentBlockStopEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// MessageDeltaEvent represents a message_delta SSE event.
type MessageDeltaEvent struct {
	Type  string             `json:"type"`
	Delta MessageDeltaData   `json:"delta"`
	Usage *MessageDeltaUsage `json:"usage,omitempty"`
}

// MessageDeltaData represents the delta in a message_delta event.
type MessageDeltaData struct {
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// MessageDeltaUsage represents usage in a message_delta event.
type MessageDeltaUsage struct {
	OutputTokens int `json:"output_tokens"`
}

// MessageStopEvent represents a message_stop SSE event.
type MessageStopEvent struct {
	Type string `json:"type"`
}

// PingEvent represents a ping SSE event.
type PingEvent struct {
	Type string `json:"type"`
}
