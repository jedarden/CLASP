// Package mcpserver provides MCP (Model Context Protocol) server functionality for CLASP.
// This allows CLASP to be used as an MCP server for tool integration with LLM applications.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/proxy"
)

// Version is the MCP server version
const Version = "0.48.9"

// Server represents the MCP server
type Server struct {
	name    string
	version string
	proxy   *proxy.Server
	config  *config.Config
	mu      sync.RWMutex

	// Session management
	sessions map[string]*Session
}

// Session represents an MCP client session
type Session struct {
	ID        string
	StartedAt time.Time
	Writer    io.Writer
	Reader    io.Reader
}

// NewServer creates a new MCP server instance
func NewServer(name string, cfg *config.Config) *Server {
	return &Server{
		name:     name,
		version:  Version,
		config:   cfg,
		sessions: make(map[string]*Session),
	}
}

// SetProxy sets the proxy server for API forwarding
func (s *Server) SetProxy(p *proxy.Server) {
	s.proxy = p
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// Implementation represents server implementation details
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCapabilities represents the capabilities this server provides
type ServerCapabilities struct {
	Tools     *ToolCapability     `json:"tools,omitempty"`
	Resources *ResourceCapability `json:"resources,omitempty"`
	Prompts   *PromptCapability   `json:"prompts,omitempty"`
}

// ToolCapability represents tool-related capabilities
type ToolCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourceCapability represents resource-related capabilities
type ResourceCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptCapability represents prompt-related capabilities
type PromptCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializeParams represents initialization parameters from client
type InitializeParams struct {
	ProtocolVersion string            `json:"protocolVersion"`
	ClientInfo      Implementation    `json:"clientInfo"`
	Capabilities    ClientCapabilities `json:"capabilities"`
}

// ClientCapabilities represents client capabilities
type ClientCapabilities struct {
	Roots    *RootsCapability    `json:"roots,omitempty"`
	Sampling *SamplingCapability `json:"sampling,omitempty"`
}

// RootsCapability represents roots-related capabilities
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability represents sampling-related capabilities
type SamplingCapability struct{}

// InitializeResult represents the initialization response
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	Instructions    string             `json:"instructions,omitempty"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"inputSchema"`
}

// ListToolsResult represents the response to tools/list
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams represents parameters for tools/call
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResult represents the result of a tool call
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content represents content in a tool result
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Run starts the MCP server on stdio
func (s *Server) Run(ctx context.Context) error {
	log.Printf("[MCP] Starting MCP server %s v%s on stdio", s.name, s.version)
	return s.handleStdio(ctx, os.Stdin, os.Stdout)
}

// RunHTTP starts the MCP server on HTTP
func (s *Server) RunHTTP(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleHTTP)
	mux.HandleFunc("/mcp/sse", s.handleSSE)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("[MCP] Starting MCP HTTP server on %s", addr)
	return server.ListenAndServe()
}

func (s *Server) handleStdio(ctx context.Context, r io.Reader, w io.Writer) error {
	decoder := json.NewDecoder(r)
	encoder := json.NewEncoder(w)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var req JSONRPCRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			s.sendError(encoder, nil, ParseError, "Parse error", err.Error())
			continue
		}

		resp := s.handleRequest(&req)
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				log.Printf("[MCP] Error encoding response: %v", err)
			}
		}
	}
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeHTTPError(w, nil, ParseError, "Parse error", err.Error())
		return
	}

	resp := s.handleRequest(&req)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"version\":\"%s\"}\n\n", s.version)
	flusher.Flush()

	// Keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleRequest(req *JSONRPCRequest) *JSONRPCResponse {
	if req.JSONRPC != "2.0" {
		return s.errorResponse(req.ID, InvalidRequest, "Invalid JSON-RPC version", nil)
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification, no response needed
		return nil
	case "tools/list":
		return s.handleListTools(req)
	case "tools/call":
		return s.handleCallTool(req)
	case "ping":
		return s.handlePing(req)
	case "shutdown":
		return s.handleShutdown(req)
	default:
		return s.errorResponse(req.ID, MethodNotFound, fmt.Sprintf("Method not found: %s", req.Method), nil)
	}
}

func (s *Server) handleInitialize(req *JSONRPCRequest) *JSONRPCResponse {
	var params InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.errorResponse(req.ID, InvalidParams, "Invalid params", err.Error())
		}
	}

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: Implementation{
			Name:    s.name,
			Version: s.version,
		},
		Capabilities: ServerCapabilities{
			Tools: &ToolCapability{
				ListChanged: false,
			},
		},
		Instructions: "CLASP MCP Server provides proxy management and API translation tools for Claude Code.",
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleListTools(req *JSONRPCRequest) *JSONRPCResponse {
	tools := s.getTools()
	result := ListToolsResult{Tools: tools}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handleCallTool(req *JSONRPCRequest) *JSONRPCResponse {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.errorResponse(req.ID, InvalidParams, "Invalid params", err.Error())
	}

	result, err := s.executeTool(params.Name, params.Arguments)
	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: CallToolResult{
				Content: []Content{{Type: "text", Text: err.Error()}},
				IsError: true,
			},
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *Server) handlePing(req *JSONRPCRequest) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{},
	}
}

func (s *Server) handleShutdown(req *JSONRPCRequest) *JSONRPCResponse {
	log.Printf("[MCP] Shutdown requested")
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{},
	}
}

func (s *Server) errorResponse(id interface{}, code int, message string, data interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func (s *Server) sendError(encoder *json.Encoder, id interface{}, code int, message string, data interface{}) {
	resp := s.errorResponse(id, code, message, data)
	encoder.Encode(resp)
}

func (s *Server) writeHTTPError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(s.errorResponse(id, code, message, data))
}
