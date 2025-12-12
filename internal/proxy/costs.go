// Package proxy implements the HTTP proxy server.
package proxy

import (
	"sync"
	"sync/atomic"
	"time"
)

// ModelPricing holds pricing information for a model (per 1M tokens).
type ModelPricing struct {
	InputPer1M  float64 // Cost per 1 million input tokens
	OutputPer1M float64 // Cost per 1 million output tokens
}

// CostTracker tracks API costs across providers and models.
type CostTracker struct {
	mu sync.RWMutex

	// Total costs in cents (using int64 for atomic operations, stored as microcents)
	totalInputCostMicro  int64 // Microcents (1 cent = 1,000,000 microcents)
	totalOutputCostMicro int64

	// Per-provider costs
	providerCosts map[string]*ProviderCost

	// Per-model costs
	modelCosts map[string]*ModelCost

	// Request tracking
	totalRequests int64

	// Start time for calculating rates
	startTime time.Time

	// Custom pricing overrides
	customPricing map[string]ModelPricing
}

// ProviderCost tracks costs for a specific provider.
type ProviderCost struct {
	InputCostMicro  int64
	OutputCostMicro int64
	InputTokens     int64
	OutputTokens    int64
	Requests        int64
}

// ModelCost tracks costs for a specific model.
type ModelCost struct {
	InputCostMicro  int64
	OutputCostMicro int64
	InputTokens     int64
	OutputTokens    int64
	Requests        int64
}

// Default pricing per 1M tokens (in USD cents * 100 for precision)
// These are approximate prices as of late 2024 and should be configurable
var defaultPricing = map[string]ModelPricing{
	// OpenAI models
	"gpt-4o":        {InputPer1M: 250, OutputPer1M: 1000},  // $2.50/$10.00
	"gpt-4o-mini":   {InputPer1M: 15, OutputPer1M: 60},     // $0.15/$0.60
	"gpt-4-turbo":   {InputPer1M: 1000, OutputPer1M: 3000}, // $10.00/$30.00
	"gpt-4":         {InputPer1M: 3000, OutputPer1M: 6000}, // $30.00/$60.00
	"gpt-3.5-turbo": {InputPer1M: 50, OutputPer1M: 150},    // $0.50/$1.50
	"o1-preview":    {InputPer1M: 1500, OutputPer1M: 6000}, // $15.00/$60.00
	"o1-mini":       {InputPer1M: 300, OutputPer1M: 1200},  // $3.00/$12.00

	// Anthropic models (via passthrough)
	"claude-3-opus-20240229":     {InputPer1M: 1500, OutputPer1M: 7500}, // $15.00/$75.00
	"claude-3-sonnet-20240229":   {InputPer1M: 300, OutputPer1M: 1500},  // $3.00/$15.00
	"claude-3-haiku-20240307":    {InputPer1M: 25, OutputPer1M: 125},    // $0.25/$1.25
	"claude-3-5-sonnet-20241022": {InputPer1M: 300, OutputPer1M: 1500},  // $3.00/$15.00
	"claude-3-5-haiku-20241022":  {InputPer1M: 100, OutputPer1M: 500},   // $1.00/$5.00

	// OpenRouter specific (anthropic prefixed)
	"anthropic/claude-3-opus":   {InputPer1M: 1500, OutputPer1M: 7500},
	"anthropic/claude-3-sonnet": {InputPer1M: 300, OutputPer1M: 1500},
	"anthropic/claude-3-haiku":  {InputPer1M: 25, OutputPer1M: 125},
	"openai/gpt-4o":             {InputPer1M: 250, OutputPer1M: 1000},
	"openai/gpt-4-turbo":        {InputPer1M: 1000, OutputPer1M: 3000},

	// Default for unknown models (conservative estimate)
	"default": {InputPer1M: 100, OutputPer1M: 300},
}

// NewCostTracker creates a new cost tracker.
func NewCostTracker() *CostTracker {
	return &CostTracker{
		providerCosts: make(map[string]*ProviderCost),
		modelCosts:    make(map[string]*ModelCost),
		startTime:     time.Now(),
		customPricing: make(map[string]ModelPricing),
	}
}

// SetCustomPricing sets custom pricing for a model.
func (ct *CostTracker) SetCustomPricing(model string, pricing ModelPricing) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.customPricing[model] = pricing
}

// GetPricing returns the pricing for a model.
func (ct *CostTracker) GetPricing(model string) ModelPricing {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	// Check custom pricing first
	if pricing, ok := ct.customPricing[model]; ok {
		return pricing
	}

	// Check default pricing
	if pricing, ok := defaultPricing[model]; ok {
		return pricing
	}

	// Return default pricing for unknown models
	return defaultPricing["default"]
}

// RecordUsage records token usage for cost tracking.
func (ct *CostTracker) RecordUsage(provider, model string, inputTokens, outputTokens int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	pricing := ct.getPricingLocked(model)

	// Calculate costs in microcents (1 cent = 1,000,000 microcents)
	// Cost = (tokens / 1,000,000) * (cents per 1M tokens) * 1,000,000 microcents/cent
	// Simplified: inputCost = tokens * centsPerM (since the million cancels out)
	inputCostMicro := int64(inputTokens) * int64(pricing.InputPer1M)
	outputCostMicro := int64(outputTokens) * int64(pricing.OutputPer1M)

	// Update totals
	atomic.AddInt64(&ct.totalInputCostMicro, inputCostMicro)
	atomic.AddInt64(&ct.totalOutputCostMicro, outputCostMicro)
	atomic.AddInt64(&ct.totalRequests, 1)

	// Update provider costs
	pc, ok := ct.providerCosts[provider]
	if !ok {
		pc = &ProviderCost{}
		ct.providerCosts[provider] = pc
	}
	atomic.AddInt64(&pc.InputCostMicro, inputCostMicro)
	atomic.AddInt64(&pc.OutputCostMicro, outputCostMicro)
	atomic.AddInt64(&pc.InputTokens, int64(inputTokens))
	atomic.AddInt64(&pc.OutputTokens, int64(outputTokens))
	atomic.AddInt64(&pc.Requests, 1)

	// Update model costs
	mc, ok := ct.modelCosts[model]
	if !ok {
		mc = &ModelCost{}
		ct.modelCosts[model] = mc
	}
	atomic.AddInt64(&mc.InputCostMicro, inputCostMicro)
	atomic.AddInt64(&mc.OutputCostMicro, outputCostMicro)
	atomic.AddInt64(&mc.InputTokens, int64(inputTokens))
	atomic.AddInt64(&mc.OutputTokens, int64(outputTokens))
	atomic.AddInt64(&mc.Requests, 1)
}

func (ct *CostTracker) getPricingLocked(model string) ModelPricing {
	// Check custom pricing first (already holding lock)
	if pricing, ok := ct.customPricing[model]; ok {
		return pricing
	}

	// Check default pricing
	if pricing, ok := defaultPricing[model]; ok {
		return pricing
	}

	return defaultPricing["default"]
}

// CostSummary represents a summary of costs.
type CostSummary struct {
	TotalCostUSD      float64                    `json:"total_cost_usd"`
	InputCostUSD      float64                    `json:"input_cost_usd"`
	OutputCostUSD     float64                    `json:"output_cost_usd"`
	TotalRequests     int64                      `json:"total_requests"`
	TotalInputTokens  int64                      `json:"total_input_tokens"`
	TotalOutputTokens int64                      `json:"total_output_tokens"`
	CostPerRequest    float64                    `json:"avg_cost_per_request_usd"`
	CostPerHour       float64                    `json:"cost_per_hour_usd"`
	Uptime            string                     `json:"uptime"`
	ByProvider        map[string]ProviderSummary `json:"by_provider"`
	ByModel           map[string]ModelSummary    `json:"by_model"`
}

// ProviderSummary provides cost summary for a provider.
type ProviderSummary struct {
	TotalCostUSD  float64 `json:"total_cost_usd"`
	InputCostUSD  float64 `json:"input_cost_usd"`
	OutputCostUSD float64 `json:"output_cost_usd"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	Requests      int64   `json:"requests"`
}

// ModelSummary provides cost summary for a model.
type ModelSummary struct {
	TotalCostUSD  float64 `json:"total_cost_usd"`
	InputCostUSD  float64 `json:"input_cost_usd"`
	OutputCostUSD float64 `json:"output_cost_usd"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
	Requests      int64   `json:"requests"`
}

// GetSummary returns the current cost summary.
func (ct *CostTracker) GetSummary() CostSummary {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	inputCostMicro := atomic.LoadInt64(&ct.totalInputCostMicro)
	outputCostMicro := atomic.LoadInt64(&ct.totalOutputCostMicro)
	totalRequests := atomic.LoadInt64(&ct.totalRequests)

	// Convert microcents to USD
	// microcents / 1,000,000 = cents
	// cents / 100 = dollars
	inputCostUSD := float64(inputCostMicro) / 100000000.0
	outputCostUSD := float64(outputCostMicro) / 100000000.0
	totalCostUSD := inputCostUSD + outputCostUSD

	uptime := time.Since(ct.startTime)

	summary := CostSummary{
		TotalCostUSD:  totalCostUSD,
		InputCostUSD:  inputCostUSD,
		OutputCostUSD: outputCostUSD,
		TotalRequests: totalRequests,
		Uptime:        uptime.String(),
		ByProvider:    make(map[string]ProviderSummary),
		ByModel:       make(map[string]ModelSummary),
	}

	// Calculate total tokens
	var totalInputTokens, totalOutputTokens int64
	for _, pc := range ct.providerCosts {
		totalInputTokens += atomic.LoadInt64(&pc.InputTokens)
		totalOutputTokens += atomic.LoadInt64(&pc.OutputTokens)
	}
	summary.TotalInputTokens = totalInputTokens
	summary.TotalOutputTokens = totalOutputTokens

	// Cost per request
	if totalRequests > 0 {
		summary.CostPerRequest = totalCostUSD / float64(totalRequests)
	}

	// Cost per hour
	hours := uptime.Hours()
	if hours > 0 {
		summary.CostPerHour = totalCostUSD / hours
	}

	// Provider breakdown
	for provider, pc := range ct.providerCosts {
		inputUSD := float64(atomic.LoadInt64(&pc.InputCostMicro)) / 100000000.0
		outputUSD := float64(atomic.LoadInt64(&pc.OutputCostMicro)) / 100000000.0
		summary.ByProvider[provider] = ProviderSummary{
			TotalCostUSD:  inputUSD + outputUSD,
			InputCostUSD:  inputUSD,
			OutputCostUSD: outputUSD,
			InputTokens:   atomic.LoadInt64(&pc.InputTokens),
			OutputTokens:  atomic.LoadInt64(&pc.OutputTokens),
			Requests:      atomic.LoadInt64(&pc.Requests),
		}
	}

	// Model breakdown
	for model, mc := range ct.modelCosts {
		inputUSD := float64(atomic.LoadInt64(&mc.InputCostMicro)) / 100000000.0
		outputUSD := float64(atomic.LoadInt64(&mc.OutputCostMicro)) / 100000000.0
		summary.ByModel[model] = ModelSummary{
			TotalCostUSD:  inputUSD + outputUSD,
			InputCostUSD:  inputUSD,
			OutputCostUSD: outputUSD,
			InputTokens:   atomic.LoadInt64(&mc.InputTokens),
			OutputTokens:  atomic.LoadInt64(&mc.OutputTokens),
			Requests:      atomic.LoadInt64(&mc.Requests),
		}
	}

	return summary
}

// GetTotalCostUSD returns the total cost in USD.
func (ct *CostTracker) GetTotalCostUSD() float64 {
	inputCostMicro := atomic.LoadInt64(&ct.totalInputCostMicro)
	outputCostMicro := atomic.LoadInt64(&ct.totalOutputCostMicro)
	return float64(inputCostMicro+outputCostMicro) / 100000000.0
}

// Reset resets all cost tracking data.
func (ct *CostTracker) Reset() {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	atomic.StoreInt64(&ct.totalInputCostMicro, 0)
	atomic.StoreInt64(&ct.totalOutputCostMicro, 0)
	atomic.StoreInt64(&ct.totalRequests, 0)
	ct.providerCosts = make(map[string]*ProviderCost)
	ct.modelCosts = make(map[string]*ModelCost)
	ct.startTime = time.Now()
}
