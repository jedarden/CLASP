package tests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jedarden/clasp/internal/proxy"
)

// TestCostTracker_NewCostTracker tests creation of a new cost tracker.
func TestCostTracker_NewCostTracker(t *testing.T) {
	tracker := proxy.NewCostTracker()
	if tracker == nil {
		t.Fatal("Expected non-nil cost tracker")
	}

	summary := tracker.GetSummary()
	if summary.TotalCostUSD != 0 {
		t.Errorf("Expected zero total cost, got %f", summary.TotalCostUSD)
	}
	if summary.TotalRequests != 0 {
		t.Errorf("Expected zero requests, got %d", summary.TotalRequests)
	}
}

// TestCostTracker_RecordUsage tests recording token usage.
func TestCostTracker_RecordUsage(t *testing.T) {
	tracker := proxy.NewCostTracker()

	// Record usage for gpt-4o (priced at $2.50/$10.00 per 1M tokens)
	tracker.RecordUsage("openai", "gpt-4o", 1000, 500)

	summary := tracker.GetSummary()

	// Check requests counted
	if summary.TotalRequests != 1 {
		t.Errorf("Expected 1 request, got %d", summary.TotalRequests)
	}

	// Check tokens counted
	if summary.TotalInputTokens != 1000 {
		t.Errorf("Expected 1000 input tokens, got %d", summary.TotalInputTokens)
	}
	if summary.TotalOutputTokens != 500 {
		t.Errorf("Expected 500 output tokens, got %d", summary.TotalOutputTokens)
	}

	// Check cost calculation
	// Input: 1000 tokens * $2.50/1M = $0.0025
	// Output: 500 tokens * $10.00/1M = $0.005
	// Total: $0.0075
	expectedCost := 0.0075
	tolerance := 0.0001
	if summary.TotalCostUSD < expectedCost-tolerance || summary.TotalCostUSD > expectedCost+tolerance {
		t.Errorf("Expected cost ~%f, got %f", expectedCost, summary.TotalCostUSD)
	}
}

// TestCostTracker_MultipleRecords tests recording multiple usages.
func TestCostTracker_MultipleRecords(t *testing.T) {
	tracker := proxy.NewCostTracker()

	// Record multiple usages
	tracker.RecordUsage("openai", "gpt-4o", 1000, 500)
	tracker.RecordUsage("openai", "gpt-4o-mini", 2000, 1000)
	tracker.RecordUsage("openrouter", "openai/gpt-4o", 500, 250)

	summary := tracker.GetSummary()

	// Check total requests
	if summary.TotalRequests != 3 {
		t.Errorf("Expected 3 requests, got %d", summary.TotalRequests)
	}

	// Check tokens
	expectedInputTokens := int64(1000 + 2000 + 500)
	if summary.TotalInputTokens != expectedInputTokens {
		t.Errorf("Expected %d input tokens, got %d", expectedInputTokens, summary.TotalInputTokens)
	}

	expectedOutputTokens := int64(500 + 1000 + 250)
	if summary.TotalOutputTokens != expectedOutputTokens {
		t.Errorf("Expected %d output tokens, got %d", expectedOutputTokens, summary.TotalOutputTokens)
	}

	// Check by-provider breakdown
	if len(summary.ByProvider) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(summary.ByProvider))
	}
	if _, ok := summary.ByProvider["openai"]; !ok {
		t.Error("Expected openai provider in breakdown")
	}
	if _, ok := summary.ByProvider["openrouter"]; !ok {
		t.Error("Expected openrouter provider in breakdown")
	}

	// Check by-model breakdown
	if len(summary.ByModel) != 3 {
		t.Errorf("Expected 3 models, got %d", len(summary.ByModel))
	}
}

// TestCostTracker_ProviderBreakdown tests provider-specific tracking.
func TestCostTracker_ProviderBreakdown(t *testing.T) {
	tracker := proxy.NewCostTracker()

	tracker.RecordUsage("openai", "gpt-4o", 1000, 500)
	tracker.RecordUsage("openai", "gpt-4o", 1000, 500)
	tracker.RecordUsage("anthropic", "claude-3-opus-20240229", 500, 200)

	summary := tracker.GetSummary()

	// Check OpenAI breakdown
	openai, ok := summary.ByProvider["openai"]
	if !ok {
		t.Fatal("Expected openai in provider breakdown")
	}
	if openai.Requests != 2 {
		t.Errorf("Expected 2 OpenAI requests, got %d", openai.Requests)
	}
	if openai.InputTokens != 2000 {
		t.Errorf("Expected 2000 OpenAI input tokens, got %d", openai.InputTokens)
	}

	// Check Anthropic breakdown
	anthropic, ok := summary.ByProvider["anthropic"]
	if !ok {
		t.Fatal("Expected anthropic in provider breakdown")
	}
	if anthropic.Requests != 1 {
		t.Errorf("Expected 1 Anthropic request, got %d", anthropic.Requests)
	}
}

// TestCostTracker_ModelBreakdown tests model-specific tracking.
func TestCostTracker_ModelBreakdown(t *testing.T) {
	tracker := proxy.NewCostTracker()

	tracker.RecordUsage("openai", "gpt-4o", 1000, 500)
	tracker.RecordUsage("openai", "gpt-4o-mini", 5000, 2000)

	summary := tracker.GetSummary()

	// Check gpt-4o breakdown
	gpt4o, ok := summary.ByModel["gpt-4o"]
	if !ok {
		t.Fatal("Expected gpt-4o in model breakdown")
	}
	if gpt4o.InputTokens != 1000 {
		t.Errorf("Expected 1000 gpt-4o input tokens, got %d", gpt4o.InputTokens)
	}

	// Check gpt-4o-mini breakdown
	gpt4oMini, ok := summary.ByModel["gpt-4o-mini"]
	if !ok {
		t.Fatal("Expected gpt-4o-mini in model breakdown")
	}
	if gpt4oMini.InputTokens != 5000 {
		t.Errorf("Expected 5000 gpt-4o-mini input tokens, got %d", gpt4oMini.InputTokens)
	}

	// gpt-4o-mini should be cheaper
	if gpt4oMini.TotalCostUSD >= gpt4o.TotalCostUSD {
		t.Errorf("Expected gpt-4o-mini to be cheaper than gpt-4o")
	}
}

// TestCostTracker_CustomPricing tests custom pricing configuration.
func TestCostTracker_CustomPricing(t *testing.T) {
	tracker := proxy.NewCostTracker()

	// Set custom pricing for a model
	tracker.SetCustomPricing("custom-model", proxy.ModelPricing{
		InputPer1M:  100, // $1.00 per 1M input tokens
		OutputPer1M: 200, // $2.00 per 1M output tokens
	})

	tracker.RecordUsage("custom", "custom-model", 1000000, 500000)

	summary := tracker.GetSummary()

	// Expected: $1.00 input + $1.00 output = $2.00
	expectedCost := 2.0
	tolerance := 0.01
	if summary.TotalCostUSD < expectedCost-tolerance || summary.TotalCostUSD > expectedCost+tolerance {
		t.Errorf("Expected cost ~%f, got %f", expectedCost, summary.TotalCostUSD)
	}
}

// TestCostTracker_GetPricing tests pricing retrieval.
func TestCostTracker_GetPricing(t *testing.T) {
	tracker := proxy.NewCostTracker()

	// Test known model
	pricing := tracker.GetPricing("gpt-4o")
	if pricing.InputPer1M != 250 || pricing.OutputPer1M != 1000 {
		t.Errorf("Unexpected gpt-4o pricing: input=%f, output=%f", pricing.InputPer1M, pricing.OutputPer1M)
	}

	// Test unknown model (should return default)
	pricing = tracker.GetPricing("unknown-model-xyz")
	if pricing.InputPer1M == 0 || pricing.OutputPer1M == 0 {
		t.Error("Expected non-zero default pricing for unknown model")
	}
}

// TestCostTracker_Reset tests resetting cost data.
func TestCostTracker_Reset(t *testing.T) {
	tracker := proxy.NewCostTracker()

	tracker.RecordUsage("openai", "gpt-4o", 1000, 500)
	tracker.RecordUsage("openai", "gpt-4o", 1000, 500)

	summary := tracker.GetSummary()
	if summary.TotalRequests != 2 {
		t.Errorf("Expected 2 requests before reset, got %d", summary.TotalRequests)
	}

	tracker.Reset()

	summary = tracker.GetSummary()
	if summary.TotalRequests != 0 {
		t.Errorf("Expected 0 requests after reset, got %d", summary.TotalRequests)
	}
	if summary.TotalCostUSD != 0 {
		t.Errorf("Expected 0 cost after reset, got %f", summary.TotalCostUSD)
	}
	if len(summary.ByProvider) != 0 {
		t.Errorf("Expected 0 providers after reset, got %d", len(summary.ByProvider))
	}
	if len(summary.ByModel) != 0 {
		t.Errorf("Expected 0 models after reset, got %d", len(summary.ByModel))
	}
}

// TestCostTracker_GetTotalCostUSD tests the helper method.
func TestCostTracker_GetTotalCostUSD(t *testing.T) {
	tracker := proxy.NewCostTracker()

	tracker.RecordUsage("openai", "gpt-4o", 1000, 500)

	totalCost := tracker.GetTotalCostUSD()
	summary := tracker.GetSummary()

	if totalCost != summary.TotalCostUSD {
		t.Errorf("GetTotalCostUSD mismatch: %f vs %f", totalCost, summary.TotalCostUSD)
	}
}

// TestCostTracker_CostPerRequest tests average cost calculation.
func TestCostTracker_CostPerRequest(t *testing.T) {
	tracker := proxy.NewCostTracker()

	// Record 10 identical requests
	for i := 0; i < 10; i++ {
		tracker.RecordUsage("openai", "gpt-4o", 1000, 500)
	}

	summary := tracker.GetSummary()

	// Check cost per request
	expectedPerRequest := summary.TotalCostUSD / float64(summary.TotalRequests)
	tolerance := 0.0001
	if summary.CostPerRequest < expectedPerRequest-tolerance || summary.CostPerRequest > expectedPerRequest+tolerance {
		t.Errorf("Expected cost per request ~%f, got %f", expectedPerRequest, summary.CostPerRequest)
	}
}

// TestCostTracker_CostPerHour tests hourly rate calculation.
func TestCostTracker_CostPerHour(t *testing.T) {
	tracker := proxy.NewCostTracker()

	// Record some usage
	tracker.RecordUsage("openai", "gpt-4o", 1000, 500)

	// Wait a tiny bit to ensure non-zero uptime
	time.Sleep(10 * time.Millisecond)

	summary := tracker.GetSummary()

	// Cost per hour should be positive
	if summary.CostPerHour <= 0 {
		t.Errorf("Expected positive cost per hour, got %f", summary.CostPerHour)
	}
}

// TestCostTracker_Concurrency tests concurrent access.
func TestCostTracker_Concurrency(t *testing.T) {
	tracker := proxy.NewCostTracker()

	// Run concurrent recordings
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				tracker.RecordUsage("openai", "gpt-4o", 100, 50)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	summary := tracker.GetSummary()

	// Should have 1000 total requests
	if summary.TotalRequests != 1000 {
		t.Errorf("Expected 1000 requests, got %d", summary.TotalRequests)
	}
}

// TestCostTracker_ClaudePricing tests Claude model pricing.
func TestCostTracker_ClaudePricing(t *testing.T) {
	tracker := proxy.NewCostTracker()

	// Claude models have specific pricing
	pricing := tracker.GetPricing("claude-3-opus-20240229")
	if pricing.InputPer1M != 1500 || pricing.OutputPer1M != 7500 {
		t.Errorf("Unexpected Claude Opus pricing: input=%f, output=%f", pricing.InputPer1M, pricing.OutputPer1M)
	}

	pricing = tracker.GetPricing("claude-3-haiku-20240307")
	if pricing.InputPer1M != 25 || pricing.OutputPer1M != 125 {
		t.Errorf("Unexpected Claude Haiku pricing: input=%f, output=%f", pricing.InputPer1M, pricing.OutputPer1M)
	}
}

// TestCostSummary_Serialization tests JSON serialization of cost summary.
func TestCostSummary_Serialization(t *testing.T) {
	tracker := proxy.NewCostTracker()

	tracker.RecordUsage("openai", "gpt-4o", 1000, 500)
	tracker.RecordUsage("anthropic", "claude-3-opus-20240229", 2000, 1000)

	summary := tracker.GetSummary()

	// Marshal to JSON
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("Failed to marshal summary: %v", err)
	}

	// Unmarshal and verify
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal summary: %v", err)
	}

	// Check required fields exist
	requiredFields := []string{
		"total_cost_usd",
		"input_cost_usd",
		"output_cost_usd",
		"total_requests",
		"by_provider",
		"by_model",
	}

	for _, field := range requiredFields {
		if _, ok := decoded[field]; !ok {
			t.Errorf("Expected field %s in serialized summary", field)
		}
	}
}

// TestCostTracker_ZeroDivision tests edge cases with zero values.
func TestCostTracker_ZeroDivision(t *testing.T) {
	tracker := proxy.NewCostTracker()

	// Get summary without any usage recorded
	summary := tracker.GetSummary()

	// Should not panic and should return zero values
	if summary.CostPerRequest != 0 {
		t.Errorf("Expected zero cost per request, got %f", summary.CostPerRequest)
	}
}
