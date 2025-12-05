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
	version = "v0.2.5"
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
  -provider <name>          LLM provider: openai, azure, openrouter, custom
  -model <model>            Default model to use for all requests
  -debug                    Enable debug logging (full request/response)
  -rate-limit               Enable rate limiting
  -rate-limit-requests <n>  Requests per window (default: 60)
  -rate-limit-window <n>    Window in seconds (default: 60)
  -rate-limit-burst <n>     Burst allowance (default: 10)
  -cache                    Enable response caching
  -cache-max-size <n>       Maximum cache entries (default: 1000)
  -cache-ttl <n>            Cache TTL in seconds (default: 3600)
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

  Custom:
    CUSTOM_BASE_URL      Base URL for OpenAI-compatible endpoint
    CUSTOM_API_KEY       API key (optional for some endpoints)

  Model Mapping:
    CLASP_MODEL          Default model for all requests
    CLASP_MODEL_OPUS     Model to use for Opus tier
    CLASP_MODEL_SONNET   Model to use for Sonnet tier
    CLASP_MODEL_HAIKU    Model to use for Haiku tier

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

Examples:
  # Use OpenAI with GPT-4o
  OPENAI_API_KEY=sk-xxx clasp -model gpt-4o

  # Use Azure OpenAI
  AZURE_API_KEY=xxx AZURE_OPENAI_ENDPOINT=https://xxx.openai.azure.com \
    AZURE_DEPLOYMENT_NAME=gpt-4 clasp -provider azure

  # Use local Ollama
  CUSTOM_BASE_URL=http://localhost:11434/v1 clasp -provider custom -model llama3.1

Claude Code Integration:
  Set ANTHROPIC_BASE_URL to point to CLASP:

  ANTHROPIC_BASE_URL=http://localhost:8080 claude

For more information: https://github.com/jedarden/CLASP
`, version)
}
