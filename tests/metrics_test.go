package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/proxy"
)

func newMetricsHTTPServer(t *testing.T, authEnabled, anonymousMetrics bool) *httptest.Server {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "upstream-test-key"
	cfg.AuthEnabled = authEnabled
	cfg.AuthAPIKey = "metrics-test-key"
	cfg.AuthAllowAnonymousMetrics = anonymousMetrics

	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", handler.HandleMetrics)
	mux.HandleFunc("/metrics/prometheus", handler.HandleMetricsPrometheus)
	mux.HandleFunc("/costs", handler.HandleCosts)
	var root http.Handler = mux
	if authEnabled {
		root = proxy.AuthMiddleware(&proxy.AuthConfig{
			Enabled:               true,
			APIKey:                cfg.AuthAPIKey,
			AllowAnonymousMetrics: anonymousMetrics,
		})(root)
	}
	return httptest.NewServer(root)
}

func TestMetricsIntegrationHTTPContract(t *testing.T) {
	server := newMetricsHTTPServer(t, false, false)
	defer server.Close()

	response, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("GET /metrics content type = %q, want application/json", got)
	}
	var payload struct {
		Requests struct {
			Total       int64  `json:"total"`
			Successful  int64  `json:"successful"`
			SuccessRate string `json:"success_rate"`
		} `json:"requests"`
		Performance struct {
			AverageLatency string `json:"avg_latency_ms"`
		} `json:"performance"`
		Provider string `json:"provider"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode /metrics: %v", err)
	}
	if payload.Requests.Total != 0 || payload.Requests.Successful != 0 {
		t.Fatalf("initial request metrics = %+v, want zero counters", payload.Requests)
	}
	if payload.Requests.SuccessRate != "0.00%" || payload.Performance.AverageLatency != "0.00" {
		t.Fatalf("initial derived metrics = %+v, want zero values", payload)
	}
	if payload.Provider != "openai" {
		t.Errorf("provider = %q, want openai", payload.Provider)
	}

	response, err = http.Get(server.URL + "/metrics/prometheus")
	if err != nil {
		t.Fatalf("GET /metrics/prometheus: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics/prometheus status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read Prometheus response: %v", err)
	}
	for _, metric := range []string{"clasp_requests_total", "clasp_latency_avg_ms", "clasp_cost_total_usd"} {
		if !strings.Contains(string(body), metric) {
			t.Errorf("Prometheus output missing %s", metric)
		}
	}
}

func TestMetricsIntegrationAuthentication(t *testing.T) {
	server := newMetricsHTTPServer(t, true, false)
	defer server.Close()
	client := &http.Client{}

	for _, path := range []string{"/metrics", "/metrics/prometheus"} {
		response, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatalf("anonymous GET %s: %v", path, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Errorf("anonymous GET %s status = %d, want %d", path, response.StatusCode, http.StatusUnauthorized)
		}
	}

	for _, path := range []string{"/metrics", "/metrics/prometheus"} {
		request, err := http.NewRequest(http.MethodGet, server.URL+path, http.NoBody)
		if err != nil {
			t.Fatalf("create invalid-key request for %s: %v", path, err)
		}
		request.Header.Set("x-api-key", "wrong-key")
		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("invalid-key GET %s: %v", path, err)
		}
		assertAuthFailure(t, response, "Invalid API key")
	}

	for name, authorization := range map[string]string{
		"x-api-key": "metrics-test-key",
		"bearer":    "Bearer metrics-test-key",
	} {
		for _, path := range []string{"/metrics", "/metrics/prometheus"} {
			request, err := http.NewRequest(http.MethodGet, server.URL+path, http.NoBody)
			if err != nil {
				t.Fatalf("create %s request for %s: %v", name, path, err)
			}
			if name == "x-api-key" {
				request.Header.Set("x-api-key", authorization)
			} else {
				request.Header.Set("Authorization", authorization)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatalf("authenticated %s request for %s: %v", name, path, err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Errorf("authenticated %s request for %s status = %d, want %d", name, path, response.StatusCode, http.StatusOK)
			}
		}
	}

	anonymousServer := newMetricsHTTPServer(t, true, true)
	defer anonymousServer.Close()
	for _, path := range []string{"/metrics", "/metrics/prometheus"} {
		response, err := client.Get(anonymousServer.URL + path)
		if err != nil {
			t.Fatalf("anonymous metrics GET %s: %v", path, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Errorf("allow-anonymous-metrics GET %s status = %d, want %d", path, response.StatusCode, http.StatusOK)
		}
	}
}

func TestMetricsIntegrationConcurrentRequests(t *testing.T) {
	server := newMetricsHTTPServer(t, false, false)
	defer server.Close()

	const workers = 16
	const requestsPerWorker = 20
	client := &http.Client{}
	errors := make(chan error, workers*requestsPerWorker)
	var waitGroup sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		waitGroup.Add(1)
		go func(worker int) {
			defer waitGroup.Done()
			for requestNumber := 0; requestNumber < requestsPerWorker; requestNumber++ {
				path := "/metrics"
				if (worker+requestNumber)%2 == 0 {
					path = "/metrics/prometheus"
				}
				response, err := client.Get(server.URL + path)
				if err != nil {
					errors <- fmt.Errorf("GET %s: %w", path, err)
					continue
				}
				body, readErr := io.ReadAll(response.Body)
				response.Body.Close()
				if response.StatusCode != http.StatusOK {
					errors <- fmt.Errorf("GET %s status = %d: %s", path, response.StatusCode, body)
					continue
				}
				if readErr != nil || len(body) == 0 {
					errors <- fmt.Errorf("GET %s body: %v", path, readErr)
				}
			}
		}(worker)
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
