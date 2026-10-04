package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/clasp/internal/config"
)

type metricsTestProvider struct{}

func (metricsTestProvider) Name() string { return "test-provider" }

func (metricsTestProvider) GetHeaders(string) http.Header { return http.Header{} }

func (metricsTestProvider) GetEndpointURL() string { return "http://provider.invalid/v1" }

func (metricsTestProvider) TransformModelID(modelID string) string { return modelID }

func (metricsTestProvider) SupportsStreaming() bool { return true }

func (metricsTestProvider) RequiresTransformation() bool { return true }

func newMetricsTestHandler() *Handler {
	return &Handler{
		cfg:         config.DefaultConfig(),
		provider:    metricsTestProvider{},
		metrics:     &Metrics{StartTime: time.Now()},
		costTracker: NewCostTracker(),
	}
}

func decodeMetricsResponse(t *testing.T, handler http.HandlerFunc) map[string]interface{} {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	handler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	return response
}

func metricMap(t *testing.T, response map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	value, ok := response[key].(map[string]interface{})
	if !ok {
		t.Fatalf("response[%q] = %#v, want object", key, response[key])
	}
	return value
}

func metricInt(t *testing.T, values map[string]interface{}, key string) int64 {
	t.Helper()
	value, ok := values[key].(float64)
	if !ok {
		t.Fatalf("metric %q = %#v, want number", key, values[key])
	}
	return int64(value)
}

func TestHandleMetricsJSONContract(t *testing.T) {
	handler := newMetricsTestHandler()
	atomic.StoreInt64(&handler.metrics.TotalRequests, 10)
	atomic.StoreInt64(&handler.metrics.SuccessRequests, 8)
	atomic.StoreInt64(&handler.metrics.ErrorRequests, 2)
	atomic.StoreInt64(&handler.metrics.StreamRequests, 3)
	atomic.StoreInt64(&handler.metrics.ToolCallRequests, 4)
	atomic.StoreInt64(&handler.metrics.TotalLatencyMs, 1200)
	handler.costTracker.RecordUsage("test-provider", "gpt-4o", 100, 50)

	response := decodeMetricsResponse(t, handler.HandleMetrics)
	requests := metricMap(t, response, "requests")
	performance := metricMap(t, response, "performance")

	for _, key := range []string{"total", "successful", "errors", "streaming", "tool_calls"} {
		if _, ok := requests[key]; !ok {
			t.Errorf("requests missing %q", key)
		}
	}
	if got := metricInt(t, requests, "total"); got != 10 {
		t.Errorf("requests.total = %d, want 10", got)
	}
	if got := metricInt(t, requests, "successful"); got != 8 {
		t.Errorf("requests.successful = %d, want 8", got)
	}
	if got := metricInt(t, requests, "errors"); got != 2 {
		t.Errorf("requests.errors = %d, want 2", got)
	}
	if got := requests["success_rate"]; got != "80.00%" {
		t.Errorf("requests.success_rate = %#v, want %q", got, "80.00%")
	}
	if got := performance["avg_latency_ms"]; got != "150.00" {
		t.Errorf("performance.avg_latency_ms = %#v, want %q", got, "150.00")
	}
	if _, err := strconv.ParseFloat(performance["requests_per_sec"].(string), 64); err != nil {
		t.Fatalf("performance.requests_per_sec is not numeric: %v", err)
	}
	if response["provider"] != "test-provider" {
		t.Errorf("provider = %#v, want test-provider", response["provider"])
	}
	if _, ok := response["uptime"].(string); !ok {
		t.Errorf("uptime = %#v, want string", response["uptime"])
	}

	costs := metricMap(t, response, "costs")
	if got := costs["total_input_tokens"]; got != float64(100) {
		t.Errorf("costs.total_input_tokens = %#v, want 100", got)
	}
}

func TestHandleMetricsCostResetDoesNotResetRequestMetrics(t *testing.T) {
	handler := newMetricsTestHandler()
	atomic.StoreInt64(&handler.metrics.TotalRequests, 4)
	atomic.StoreInt64(&handler.metrics.SuccessRequests, 3)
	handler.costTracker.RecordUsage("test-provider", "gpt-4o", 100, 50)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/costs?action=reset", http.NoBody)
	handler.HandleCosts(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "reset") {
		t.Fatalf("reset response = %q, want reset confirmation", recorder.Body.String())
	}

	response := decodeMetricsResponse(t, handler.HandleMetrics)
	requests := metricMap(t, response, "requests")
	if got := metricInt(t, requests, "total"); got != 4 {
		t.Errorf("requests.total after cost reset = %d, want 4", got)
	}
	if got := metricMap(t, response, "costs")["total_input_tokens"]; got != float64(0) {
		t.Errorf("costs.total_input_tokens after reset = %#v, want 0", got)
	}
}

func TestHandleMetricsPrometheusContract(t *testing.T) {
	handler := newMetricsTestHandler()
	atomic.StoreInt64(&handler.metrics.TotalRequests, 7)
	atomic.StoreInt64(&handler.metrics.SuccessRequests, 5)
	atomic.StoreInt64(&handler.metrics.ErrorRequests, 2)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics/prometheus", http.NoBody)
	handler.HandleMetricsPrometheus(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("content type = %q, want Prometheus content type", got)
	}

	body := recorder.Body.String()
	for _, metric := range []string{
		"clasp_requests_total",
		"clasp_requests_successful",
		"clasp_requests_errors",
		"clasp_requests_streaming",
		"clasp_requests_tool_calls",
		"clasp_latency_total_ms",
		"clasp_uptime_seconds",
		"clasp_latency_avg_ms",
		"clasp_requests_per_second",
	} {
		if !strings.Contains(body, metric+"{") {
			t.Errorf("Prometheus output missing %s", metric)
		}
	}
	if !strings.Contains(body, `provider="test-provider"`) {
		t.Error("Prometheus output missing provider label")
	}
	if !strings.Contains(body, "# HELP") || !strings.Contains(body, "# TYPE") {
		t.Error("Prometheus output missing HELP/TYPE comments")
	}
}
