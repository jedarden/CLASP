package proxy

import (
	"net/http/httptest"
	"testing"
)

func TestSetRequestCostHeader(t *testing.T) {
	tracker := NewCostTracker()
	tracker.SetCustomPricing("test-model", ModelPricing{InputPer1M: 250, OutputPer1M: 1000})
	response := httptest.NewRecorder()

	setRequestCostHeader(response, tracker, "test-model", 100, 50)

	if got, want := response.Header().Get(requestCostHeader), "0.000750"; got != want {
		t.Fatalf("%s = %q, want %q", requestCostHeader, got, want)
	}
}

func TestStreamingRequestCostTrailer(t *testing.T) {
	tracker := NewCostTracker()
	tracker.SetCustomPricing("test-model", ModelPricing{InputPer1M: 250, OutputPer1M: 1000})
	response := httptest.NewRecorder()

	announceRequestCostTrailer(response)
	_, _ = response.Write([]byte("data: response\n\n"))
	setRequestCostHeader(response, tracker, "test-model", 100, 50)

	if got, want := response.Result().Trailer.Get(requestCostHeader), "0.000750"; got != want {
		t.Fatalf("streaming %s trailer = %q, want %q", requestCostHeader, got, want)
	}
}

func TestAnthropicStreamUsage(t *testing.T) {
	var usage anthropicStreamUsage
	usage.consume([]byte("event: message_start\r\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":123}}}\r\n\r\n"))
	usage.consume([]byte("event: message_delta\r\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":45}}\r\n\r\n"))

	if !usage.complete() {
		t.Fatal("expected complete usage after message_start and message_delta")
	}
	if usage.inputTokens != 123 || usage.outputTokens != 45 {
		t.Fatalf("usage = (%d, %d), want (123, 45)", usage.inputTokens, usage.outputTokens)
	}
}
