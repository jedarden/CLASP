// CLASP - Claude Language Agent Super Proxy
// A Go proxy that translates Claude/Anthropic API calls to OpenAI-compatible endpoints.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/proxy"
	"github.com/joho/godotenv"
)

var (
	version = "v0.7.4"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 0, "Port to listen on (overrides CLASP_PORT)")
	provider := flag.String("provider", "", "LLM provider (openai, azure, openrouter, custom)")
	model := flag.String("model", "", "Default model to use")
	debug := flag.Bool("debug", false, "Enable debug logging (requests and responses)")
	rateLimit := flag.Bool("rate-limit", false, "Enable rate limiting")
	rateLimitReqs := flag.Int("rate-limit-requests", 0, "Requests per window (default: 60)")
	rateLimitWindow := flag.Int("rate-limit-window", 0, "Window in seconds (default: 60)")
	rateLimitBurst := flag.Int("rate-limit-burst", 0, "Burst allowance (default: 10)")
	cache := flag.Bool("cache", false, "Enable response caching")
	cacheMaxSize := flag.Int("cache-max-size", 0, "Maximum cache entries (default: 1000)")
	cacheTTL := flag.Int("cache-ttl", 0, "Cache TTL in seconds (default: 3600)")
	multiProvider := flag.Bool("multi-provider", false, "Enable multi-provider tier routing")
	fallback := flag.Bool("fallback", false, "Enable fallback routing")
	auth := flag.Bool("auth", false, "Enable API key authentication")
	authAPIKey := flag.String("auth-api-key", "", "API key for authentication (required with -auth)")
	queueEnabled := flag.Bool("queue", false, "Enable request queuing during outages")
	queueMaxSize := flag.Int("queue-max-size", 0, "Maximum queued requests (default: 100)")
	queueMaxWait := flag.Int("queue-max-wait", 0, "Queue timeout in seconds (default: 30)")
	circuitBreaker := flag.Bool("circuit-breaker", false, "Enable circuit breaker pattern")
	cbThreshold := flag.Int("cb-threshold", 0, "Circuit breaker failure threshold (default: 5)")
	cbRecovery := flag.Int("cb-recovery", 0, "Circuit breaker success recovery threshold (default: 2)")
	cbTimeout := flag.Int("cb-timeout", 0, "Circuit breaker timeout in seconds (default: 30)")
	showVersion := flag.Bool("version", false, "Show version information")
	help := flag.Bool("help", false, "Show help message")

	flag.Parse()

	if *showVersion {
		fmt.Printf("CLASP %s\n", version)
		os.Exit(0)
	}

	if *help {
		printHelp()
		os.Exit(0)
	}

	// Load .env file if it exists
	envPaths := []string{
		".env",
		filepath.Join(os.Getenv("HOME"), ".clasp", ".env"),
	}
	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Load(path); err == nil {
				log.Printf("[CLASP] Loaded environment from %s", path)
			}
		}
	}

	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("[CLASP] Configuration error: %v", err)
	}

	// Apply command line overrides
	if *port > 0 {
		cfg.Port = *port
	}
	if *provider != "" {
		cfg.Provider = config.ProviderType(*provider)
	}
	if *model != "" {
		cfg.DefaultModel = *model
	}
	if *debug {
		cfg.Debug = true
		cfg.DebugRequests = true
		cfg.DebugResponses = true
	}
	if *rateLimit {
		cfg.RateLimitEnabled = true
	}
	if *rateLimitReqs > 0 {
		cfg.RateLimitRequests = *rateLimitReqs
	}
	if *rateLimitWindow > 0 {
		cfg.RateLimitWindow = *rateLimitWindow
	}
	if *rateLimitBurst > 0 {
		cfg.RateLimitBurst = *rateLimitBurst
	}
	if *cache {
		cfg.CacheEnabled = true
	}
	if *cacheMaxSize > 0 {
		cfg.CacheMaxSize = *cacheMaxSize
	}
	if *cacheTTL > 0 {
		cfg.CacheTTL = *cacheTTL
	}
	if *multiProvider {
		cfg.MultiProviderEnabled = true
	}
	if *fallback {
		cfg.FallbackEnabled = true
	}
	if *auth {
		cfg.AuthEnabled = true
	}
	if *authAPIKey != "" {
		cfg.AuthAPIKey = *authAPIKey
	}
	if *queueEnabled {
		cfg.QueueEnabled = true
	}
	if *queueMaxSize > 0 {
		cfg.QueueMaxSize = *queueMaxSize
	}
	if *queueMaxWait > 0 {
		cfg.QueueMaxWaitSeconds = *queueMaxWait
	}
	if *circuitBreaker {
		cfg.CircuitBreakerEnabled = true
	}
	if *cbThreshold > 0 {
		cfg.CircuitBreakerThreshold = *cbThreshold
	}
	if *cbRecovery > 0 {
		cfg.CircuitBreakerRecovery = *cbRecovery
	}
	if *cbTimeout > 0 {
		cfg.CircuitBreakerTimeoutSec = *cbTimeout
	}

	// Validate authentication configuration
	if cfg.AuthEnabled && cfg.AuthAPIKey == "" {
		log.Fatalf("[CLASP] Authentication enabled but no API key provided. Set CLASP_AUTH_API_KEY or use -auth-api-key flag.")
	}

	// Create and start server
	server, err := proxy.NewServer(cfg)
	if err != nil {
		log.Fatalf("[CLASP] Failed to create server: %v", err)
	}

	printBanner()

	if err := server.Start(); err != nil {
		log.Fatalf("[CLASP] Server error: %v", err)
	}
}

func printBanner() {
	fmt.Println(`
╔═══════════════════════════════════════════════════════════════╗
║        CLASP - Claude Language Agent Super Proxy              ║
║        Translate Claude API calls to any LLM provider         ║
╚═══════════════════════════════════════════════════════════════╝`)
}

func printHelp() {
	fmt.Printf(`CLASP - Claude Language Agent Super Proxy %s

Usage: clasp [options]

Options:
  -port <port>              Port to listen on (default: 8080, or CLASP_PORT env)
  -provider <name>          LLM provider: openai, azure, openrouter, anthropic, custom
  -model <model>            Default model to use for all requests
  -debug                    Enable debug logging (full request/response)
  -rate-limit               Enable rate limiting
  -rate-limit-requests <n>  Requests per window (default: 60)
  -rate-limit-window <n>    Window in seconds (default: 60)
  -rate-limit-burst <n>     Burst allowance (default: 10)
  -cache                    Enable response caching
  -cache-max-size <n>       Maximum cache entries (default: 1000)
  -cache-ttl <n>            Cache TTL in seconds (default: 3600)
  -multi-provider           Enable multi-provider tier routing
  -fallback                 Enable fallback routing for auto-failover
  -auth                     Enable API key authentication for proxy access
  -auth-api-key <key>       API key for authentication (required with -auth)
  -queue                    Enable request queuing during provider outages
  -queue-max-size <n>       Maximum queued requests (default: 100)
  -queue-max-wait <n>       Queue timeout in seconds (default: 30)
  -circuit-breaker          Enable circuit breaker pattern
  -cb-threshold <n>         Failures before opening circuit (default: 5)
  -cb-recovery <n>          Successes to close circuit (default: 2)
  -cb-timeout <n>           Circuit breaker timeout in seconds (default: 30)
  -version                  Show version information
  -help                     Show this help message

Environment Variables:
  PROVIDER           LLM provider (openai, azure, openrouter, anthropic, custom)

  OpenAI:
    OPENAI_API_KEY       Your OpenAI API key
    OPENAI_BASE_URL      Custom base URL (default: https://api.openai.com/v1)

  Azure:
    AZURE_API_KEY            Your Azure API key
    AZURE_OPENAI_ENDPOINT    Azure OpenAI endpoint URL
    AZURE_DEPLOYMENT_NAME    Azure deployment name
    AZURE_API_VERSION        API version (default: 2024-02-15-preview)

  OpenRouter:
    OPENROUTER_API_KEY   Your OpenRouter API key

  Anthropic (Passthrough Mode):
    ANTHROPIC_API_KEY    Your Anthropic API key (passthrough - no translation)

  Custom:
    CUSTOM_BASE_URL      Base URL for OpenAI-compatible endpoint
    CUSTOM_API_KEY       API key (optional for some endpoints)

  Model Mapping:
    CLASP_MODEL          Default model for all requests
    CLASP_MODEL_OPUS     Model to use for Opus tier
    CLASP_MODEL_SONNET   Model to use for Sonnet tier
    CLASP_MODEL_HAIKU    Model to use for Haiku tier

  Multi-Provider Routing (route different tiers to different providers):
    CLASP_MULTI_PROVIDER           Enable multi-provider routing (true/1)
    CLASP_OPUS_PROVIDER            Provider for Opus tier (openai/openrouter/anthropic/custom)
    CLASP_OPUS_MODEL               Model for Opus tier
    CLASP_OPUS_API_KEY             API key for Opus tier (optional, inherits from main)
    CLASP_OPUS_BASE_URL            Base URL for Opus tier (optional)
    CLASP_SONNET_PROVIDER          Provider for Sonnet tier
    CLASP_SONNET_MODEL             Model for Sonnet tier
    CLASP_SONNET_API_KEY           API key for Sonnet tier
    CLASP_SONNET_BASE_URL          Base URL for Sonnet tier
    CLASP_HAIKU_PROVIDER           Provider for Haiku tier
    CLASP_HAIKU_MODEL              Model for Haiku tier
    CLASP_HAIKU_API_KEY            API key for Haiku tier
    CLASP_HAIKU_BASE_URL           Base URL for Haiku tier

  Server:
    CLASP_PORT           Port to listen on (default: 8080)
    CLASP_LOG_LEVEL      Logging level (debug, info, minimal)

  Debug:
    CLASP_DEBUG            Enable all debug logging (true/1)
    CLASP_DEBUG_REQUESTS   Log incoming/outgoing requests (true/1)
    CLASP_DEBUG_RESPONSES  Log responses (true/1)

  Rate Limiting:
    CLASP_RATE_LIMIT           Enable rate limiting (true/1)
    CLASP_RATE_LIMIT_REQUESTS  Requests per window (default: 60)
    CLASP_RATE_LIMIT_WINDOW    Window in seconds (default: 60)
    CLASP_RATE_LIMIT_BURST     Burst allowance (default: 10)

  Caching:
    CLASP_CACHE              Enable response caching (true/1)
    CLASP_CACHE_MAX_SIZE     Maximum cache entries (default: 1000)
    CLASP_CACHE_TTL          Cache TTL in seconds (default: 3600)

  Authentication (secure the proxy with an API key):
    CLASP_AUTH                         Enable authentication (true/1)
    CLASP_AUTH_API_KEY                 API key required for access
    CLASP_AUTH_ALLOW_ANONYMOUS_HEALTH  Allow /health without auth (default: true)
    CLASP_AUTH_ALLOW_ANONYMOUS_METRICS Allow /metrics without auth (default: false)

  Fallback Routing (auto-failover to backup provider):
    CLASP_FALLBACK           Enable global fallback routing (true/1)
    CLASP_FALLBACK_PROVIDER  Fallback provider (openai/openrouter/custom)
    CLASP_FALLBACK_MODEL     Model to use with fallback provider
    CLASP_FALLBACK_API_KEY   API key for fallback (optional, inherits from main)
    CLASP_FALLBACK_BASE_URL  Base URL for fallback (optional)

  Tier-Specific Fallback (per-tier fallback within multi-provider):
    CLASP_OPUS_FALLBACK_PROVIDER    Fallback provider for Opus tier
    CLASP_OPUS_FALLBACK_MODEL       Fallback model for Opus tier
    CLASP_SONNET_FALLBACK_PROVIDER  Fallback provider for Sonnet tier
    CLASP_SONNET_FALLBACK_MODEL     Fallback model for Sonnet tier
    CLASP_HAIKU_FALLBACK_PROVIDER   Fallback provider for Haiku tier
    CLASP_HAIKU_FALLBACK_MODEL      Fallback model for Haiku tier

  Request Queue (buffer requests during provider outages):
    CLASP_QUEUE                Enable request queuing (true/1)
    CLASP_QUEUE_MAX_SIZE       Maximum queued requests (default: 100)
    CLASP_QUEUE_MAX_WAIT       Queue timeout in seconds (default: 30)
    CLASP_QUEUE_RETRY_DELAY    Retry delay in milliseconds (default: 1000)
    CLASP_QUEUE_MAX_RETRIES    Maximum retries per request (default: 3)

  Circuit Breaker (prevent cascade failures):
    CLASP_CIRCUIT_BREAKER          Enable circuit breaker (true/1)
    CLASP_CIRCUIT_BREAKER_THRESHOLD Failures before opening (default: 5)
    CLASP_CIRCUIT_BREAKER_RECOVERY  Successes to close (default: 2)
    CLASP_CIRCUIT_BREAKER_TIMEOUT   Timeout in seconds (default: 30)

  Model Aliasing (create custom model names):
    CLASP_ALIAS_<name>=<model>     Define a model alias (e.g., CLASP_ALIAS_FAST=gpt-4o-mini)
    CLASP_MODEL_ALIASES            Comma-separated aliases (e.g., fast:gpt-4o-mini,smart:gpt-4o)

Endpoints:
  /v1/messages         - Anthropic Messages API endpoint (main proxy)
  /health              - Health check endpoint
  /metrics             - JSON metrics endpoint
  /metrics/prometheus  - Prometheus format metrics
  /costs               - Cost tracking summary (GET=summary, POST?action=reset=reset)

Cost Tracking:
  CLASP automatically tracks API costs based on token usage.
  View costs at /costs endpoint or in /metrics and /metrics/prometheus.
  Pricing is based on public rates for supported models.
  Costs are tracked per-provider and per-model.

Examples:
  # Use OpenAI with GPT-4o
  OPENAI_API_KEY=sk-xxx clasp -model gpt-4o

  # Use Azure OpenAI
  AZURE_API_KEY=xxx AZURE_OPENAI_ENDPOINT=https://xxx.openai.azure.com \
    AZURE_DEPLOYMENT_NAME=gpt-4 clasp -provider azure

  # Use local Ollama
  CUSTOM_BASE_URL=http://localhost:11434/v1 clasp -provider custom -model llama3.1

  # Anthropic Passthrough (direct to Anthropic API, no translation)
  ANTHROPIC_API_KEY=sk-ant-xxx clasp -provider anthropic
  # Use original Claude models without translation - requests pass through unchanged

  # Multi-provider with Anthropic tier (passthrough for opus, translate for others)
  ANTHROPIC_API_KEY=sk-ant-xxx OPENAI_API_KEY=sk-xxx \
    CLASP_MULTI_PROVIDER=true \
    CLASP_OPUS_PROVIDER=anthropic CLASP_OPUS_MODEL=claude-3-opus-20240229 \
    CLASP_SONNET_PROVIDER=openai CLASP_SONNET_MODEL=gpt-4o \
    clasp -multi-provider

  # Multi-provider: Opus->OpenAI, Sonnet->OpenRouter, Haiku->local
  OPENAI_API_KEY=sk-xxx OPENROUTER_API_KEY=sk-or-xxx \
    CLASP_MULTI_PROVIDER=true \
    CLASP_OPUS_PROVIDER=openai CLASP_OPUS_MODEL=gpt-4o \
    CLASP_SONNET_PROVIDER=openrouter CLASP_SONNET_MODEL=anthropic/claude-3-sonnet \
    CLASP_HAIKU_PROVIDER=custom CLASP_HAIKU_BASE_URL=http://localhost:11434/v1 CLASP_HAIKU_MODEL=llama3.1 \
    clasp -multi-provider

  # Fallback routing: OpenAI primary with OpenRouter fallback
  OPENAI_API_KEY=sk-xxx OPENROUTER_API_KEY=sk-or-xxx \
    CLASP_FALLBACK=true \
    CLASP_FALLBACK_PROVIDER=openrouter \
    CLASP_FALLBACK_MODEL=openai/gpt-4o \
    clasp -fallback

  # Secure proxy with API key authentication
  OPENAI_API_KEY=sk-xxx clasp -auth -auth-api-key "my-secret-key"

  # Then use with x-api-key header:
  curl -H "x-api-key: my-secret-key" http://localhost:8080/v1/messages ...

  # Enable circuit breaker to prevent cascade failures
  OPENAI_API_KEY=sk-xxx clasp -circuit-breaker

  # Request queuing with circuit breaker for maximum resilience
  OPENAI_API_KEY=sk-xxx clasp -queue -circuit-breaker

  # Model aliasing: create stable custom model names
  OPENAI_API_KEY=sk-xxx \
    CLASP_ALIAS_FAST=gpt-4o-mini \
    CLASP_ALIAS_SMART=gpt-4o \
    CLASP_ALIAS_BEST=o1-preview \
    clasp

  # Or use comma-separated format
  OPENAI_API_KEY=sk-xxx CLASP_MODEL_ALIASES="fast:gpt-4o-mini,smart:gpt-4o" clasp

Claude Code Integration:
  Set ANTHROPIC_BASE_URL to point to CLASP:

  ANTHROPIC_BASE_URL=http://localhost:8080 claude

For more information: https://github.com/jedarden/CLASP
`, version)
}
