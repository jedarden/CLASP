package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/session"
	"github.com/jedarden/clasp/pkg/models"
)

type responsesStateMock struct {
	mu       sync.Mutex
	requests []models.ResponsesRequest
	respond  func(int, models.ResponsesRequest) (int, string)
}

func (m *responsesStateMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var request models.ResponsesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	m.requests = append(m.requests, request)
	callNumber := len(m.requests)
	status, body := m.respond(callNumber, request)
	m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func (m *responsesStateMock) requestsSnapshot() []models.ResponsesRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.ResponsesRequest(nil), m.requests...)
}

func newResponsesStateHandler(t *testing.T, upstreamURL string, trackerTTL time.Duration) *Handler {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Provider = config.ProviderOpenAI
	cfg.OpenAIBaseURL = upstreamURL
	cfg.OpenAIAPIKey = "test-key"
	cfg.DefaultModel = "gpt-5"
	cfg.HTTPClientTimeoutSec = 5

	handler, err := NewHandler(cfg)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	tracker := session.NewTracker(trackerTTL)
	handler.SetSessionTracker(tracker)
	t.Cleanup(tracker.Stop)
	return handler
}

func responsesStateRequest(t *testing.T, handler *Handler, sessionID string, request models.AnthropicRequest) (int, models.AnthropicResponse) {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal Anthropic request: %v", err)
	}
	httpRequest := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	httpRequest.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		httpRequest.Header.Set(responsesSessionIDHeader, sessionID)
	}
	recorder := httptest.NewRecorder()
	handler.HandleMessages(recorder, httpRequest)

	var response models.AnthropicResponse
	if recorder.Code == http.StatusOK && !request.Stream {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode Anthropic response: %v; body=%q", err, recorder.Body.String())
		}
	}
	return recorder.Code, response
}

func completedResponsesStateResponse(id string) string {
	return fmt.Sprintf(`{"id":%q,"object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":"ok"}]}`, id)
}

func TestResponsesStateContinuationAndToolResult(t *testing.T) {
	mock := &responsesStateMock{respond: func(number int, _ models.ResponsesRequest) (int, string) {
		if number == 1 {
			return http.StatusOK, `{"id":"resp_tool_1","object":"response","status":"completed","output":[{"type":"function_call","call_id":"fc_weather","name":"get_weather","arguments":"{\"city\":\"Boston\"}"}]}`
		}
		return http.StatusOK, completedResponsesStateResponse("resp_tool_2")
	}}
	upstream := httptest.NewServer(mock)
	defer upstream.Close()
	handler := newResponsesStateHandler(t, upstream.URL, time.Hour)

	first := models.AnthropicRequest{
		Model:     "gpt-5",
		MaxTokens: 32,
		Messages:  []models.AnthropicMessage{{Role: "user", Content: "What is the weather?"}},
	}
	if status, _ := responsesStateRequest(t, handler, "weather-conversation", first); status != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", status)
	}

	continued := models.AnthropicRequest{
		Model:     "gpt-5",
		MaxTokens: 32,
		Messages: []models.AnthropicMessage{
			{Role: "user", Content: "What is the weather?"},
			{Role: "assistant", Content: []models.ContentBlock{{Type: "tool_use", ID: "toolu_weather", Name: "get_weather", Input: map[string]interface{}{"city": "Boston"}}}},
			{Role: "user", Content: []models.ContentBlock{{Type: "tool_result", ToolUseID: "toolu_weather", Content: "72F and sunny"}}},
		},
	}
	if status, _ := responsesStateRequest(t, handler, "weather-conversation", continued); status != http.StatusOK {
		t.Fatalf("continuation status = %d, want 200", status)
	}

	requests := mock.requestsSnapshot()
	if len(requests) != 2 {
		t.Fatalf("upstream request count = %d, want 2", len(requests))
	}
	second := requests[1]
	if second.PreviousResponseID != "resp_tool_1" {
		t.Fatalf("continuation previous_response_id = %q, want resp_tool_1", second.PreviousResponseID)
	}
	if len(second.Input) != 1 || second.Input[0].Type != "function_call_output" {
		t.Fatalf("continuation input = %#v, want one function_call_output", second.Input)
	}
	if second.Input[0].CallID != "fc_weather" || second.Input[0].Output == nil || *second.Input[0].Output != "72F and sunny" {
		t.Fatalf("tool result = %#v, want translated call ID and output", second.Input[0])
	}
}

func TestResponsesStateFailuresDoNotAdvanceChain(t *testing.T) {
	mock := &responsesStateMock{respond: func(number int, _ models.ResponsesRequest) (int, string) {
		switch number {
		case 1:
			return http.StatusOK, completedResponsesStateResponse("resp_good")
		case 2:
			return http.StatusOK, `{"id":"resp_failed","object":"response","status":"failed","output":[]}`
		default:
			return http.StatusOK, completedResponsesStateResponse("resp_recovered")
		}
	}}
	upstream := httptest.NewServer(mock)
	defer upstream.Close()
	handler := newResponsesStateHandler(t, upstream.URL, time.Hour)

	request := func(messages []models.AnthropicMessage) models.AnthropicRequest {
		return models.AnthropicRequest{Model: "gpt-5", MaxTokens: 32, Messages: messages}
	}
	firstMessage := models.AnthropicMessage{Role: "user", Content: "start"}
	if status, _ := responsesStateRequest(t, handler, "failure-conversation", request([]models.AnthropicMessage{firstMessage})); status != http.StatusOK {
		t.Fatalf("initial request status = %d", status)
	}
	if status, _ := responsesStateRequest(t, handler, "failure-conversation", request([]models.AnthropicMessage{
		firstMessage,
		{Role: "assistant", Content: "old answer"},
		{Role: "user", Content: "this turn fails"},
	})); status != http.StatusOK {
		t.Fatalf("failed response status = %d", status)
	}
	if status, _ := responsesStateRequest(t, handler, "failure-conversation", request([]models.AnthropicMessage{
		firstMessage,
		{Role: "assistant", Content: "old answer"},
		{Role: "user", Content: "retry after failure"},
	})); status != http.StatusOK {
		t.Fatalf("recovery response status = %d", status)
	}

	requests := mock.requestsSnapshot()
	if len(requests) != 3 {
		t.Fatalf("upstream request count = %d, want 3", len(requests))
	}
	if requests[1].PreviousResponseID != "resp_good" || requests[2].PreviousResponseID != "resp_good" {
		t.Fatalf("failed response advanced state: previous IDs were %q and %q", requests[1].PreviousResponseID, requests[2].PreviousResponseID)
	}
}

func TestResponsesStateExpirationRetriesWithoutStaleID(t *testing.T) {
	mock := &responsesStateMock{respond: func(number int, _ models.ResponsesRequest) (int, string) {
		if number == 2 {
			return http.StatusBadRequest, `{"error":{"code":"response_not_found","message":"previous response has expired"}}`
		}
		return http.StatusOK, completedResponsesStateResponse(fmt.Sprintf("resp_%d", number))
	}}
	upstream := httptest.NewServer(mock)
	defer upstream.Close()
	handler := newResponsesStateHandler(t, upstream.URL, time.Hour)

	first := models.AnthropicMessage{Role: "user", Content: "start"}
	if status, _ := responsesStateRequest(t, handler, "expiration-conversation", models.AnthropicRequest{Model: "gpt-5", MaxTokens: 32, Messages: []models.AnthropicMessage{first}}); status != http.StatusOK {
		t.Fatalf("initial request status = %d", status)
	}
	continued := []models.AnthropicMessage{first, {Role: "assistant", Content: "old answer"}, {Role: "user", Content: "continue"}}
	if status, _ := responsesStateRequest(t, handler, "expiration-conversation", models.AnthropicRequest{Model: "gpt-5", MaxTokens: 32, Messages: continued}); status != http.StatusOK {
		t.Fatalf("expired-state recovery status = %d", status)
	}

	requests := mock.requestsSnapshot()
	if len(requests) != 3 {
		t.Fatalf("upstream request count = %d, want initial, expired, and recovery", len(requests))
	}
	if requests[1].PreviousResponseID != "resp_1" {
		t.Fatalf("expired request previous_response_id = %q, want resp_1", requests[1].PreviousResponseID)
	}
	if requests[2].PreviousResponseID != "" || len(requests[2].Input) != 3 {
		t.Fatalf("recovery request = %#v, want full history without previous_response_id", requests[2])
	}
}

func TestResponsesStateExplicitIDsIsolateConcurrentConversations(t *testing.T) {
	mock := &responsesStateMock{respond: func(number int, _ models.ResponsesRequest) (int, string) {
		return http.StatusOK, completedResponsesStateResponse(fmt.Sprintf("resp_%d", number))
	}}
	upstream := httptest.NewServer(mock)
	defer upstream.Close()
	handler := newResponsesStateHandler(t, upstream.URL, time.Hour)

	opening := models.AnthropicMessage{Role: "user", Content: "same opening"}
	base := func(messages []models.AnthropicMessage) models.AnthropicRequest {
		return models.AnthropicRequest{Model: "gpt-5", MaxTokens: 32, Messages: messages}
	}
	if status, _ := responsesStateRequest(t, handler, "conversation-a", base([]models.AnthropicMessage{opening})); status != http.StatusOK {
		t.Fatalf("conversation A initial status = %d", status)
	}
	if status, _ := responsesStateRequest(t, handler, "conversation-b", base([]models.AnthropicMessage{opening})); status != http.StatusOK {
		t.Fatalf("conversation B initial status = %d", status)
	}

	var wait sync.WaitGroup
	wait.Add(2)
	for _, conversation := range []string{"conversation-a", "conversation-b"} {
		conversation := conversation
		go func() {
			defer wait.Done()
			messages := []models.AnthropicMessage{opening, {Role: "assistant", Content: "old answer"}, {Role: "user", Content: conversation}}
			if status, _ := responsesStateRequest(t, handler, conversation, base(messages)); status != http.StatusOK {
				t.Errorf("%s continuation status = %d", conversation, status)
			}
		}()
	}
	wait.Wait()

	requests := mock.requestsSnapshot()
	if len(requests) != 4 {
		t.Fatalf("upstream request count = %d, want 4", len(requests))
	}
	seen := map[string]bool{}
	seen[requests[2].PreviousResponseID] = true
	seen[requests[3].PreviousResponseID] = true
	if !seen["resp_1"] || !seen["resp_2"] {
		t.Fatalf("concurrent conversations shared state: continuation IDs were %q and %q", requests[2].PreviousResponseID, requests[3].PreviousResponseID)
	}
}

func TestResponsesStateStreamingStoresOnlyCompletedResponse(t *testing.T) {
	mock := &responsesStateMock{respond: func(number int, _ models.ResponsesRequest) (int, string) {
		return http.StatusOK, fmt.Sprintf("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_stream_%d\"}}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_stream_%d\",\"status\":\"completed\"}}\n\ndata: [DONE]\n\n", number, number)
	}}
	upstream := httptest.NewServer(mock)
	defer upstream.Close()
	handler := newResponsesStateHandler(t, upstream.URL, time.Hour)

	first := models.AnthropicRequest{Model: "gpt-5", MaxTokens: 32, Stream: true, Messages: []models.AnthropicMessage{{Role: "user", Content: "stream start"}}}
	if status, _ := responsesStateRequest(t, handler, "stream-conversation", first); status != http.StatusOK {
		t.Fatalf("stream initial status = %d", status)
	}
	continued := models.AnthropicRequest{Model: "gpt-5", MaxTokens: 32, Stream: true, Messages: []models.AnthropicMessage{
		{Role: "user", Content: "stream start"},
		{Role: "assistant", Content: "ok"},
		{Role: "user", Content: "continue"},
	}}
	if status, _ := responsesStateRequest(t, handler, "stream-conversation", continued); status != http.StatusOK {
		t.Fatalf("stream continuation status = %d", status)
	}

	requests := mock.requestsSnapshot()
	if len(requests) != 2 || requests[1].PreviousResponseID != "resp_stream_1" {
		t.Fatalf("stream continuation state = %#v, want resp_stream_1", requests)
	}
}
