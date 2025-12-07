// CLASP - Claude Language Agent Super Proxy
// A Go proxy that translates Claude/Anthropic API calls to OpenAI-compatible endpoints.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jedarden/clasp/internal/claudecode"
	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/logging"
	"github.com/jedarden/clasp/internal/proxy"
	"github.com/jedarden/clasp/internal/setup"
	"github.com/jedarden/clasp/internal/statusline"
	"github.com/joho/godotenv"
)

var (
	version = "v0.39.1"
)

func main() {
	// Check for subcommands first (before flag parsing)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "profile":
			handleProfileCommand(os.Args[2:])
			return
		case "status":
			handleStatusCommand(os.Args[2:])
			return
		case "logs":
			handleLogsCommand(os.Args[2:])
			return
		case "use":
			// Quick alias: clasp use <profile>
			if len(os.Args) > 2 {
				wizard := setup.NewWizard()
				if err := wizard.RunProfileUse(os.Args[2]); err != nil {
					log.Fatalf("[CLASP] %v", err)
				}
			} else {
				fmt.Println("Usage: clasp use <profile-name>")
			}
			return
		case "help", "-h", "--help":
			// Show help without starting the proxy
			printHelp()
			return
		case "version", "-v":
			// Show version without starting the proxy
			fmt.Printf("CLASP %s\n", version)
			return
		}
	}

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
	httpTimeout := flag.Int("http-timeout", 0, "HTTP client timeout in seconds for upstream requests (default: 300)")
	showVersion := flag.Bool("version", false, "Show version information")
	help := flag.Bool("help", false, "Show help message")
	runSetup := flag.Bool("setup", false, "Run interactive setup wizard")
	configure := flag.Bool("configure", false, "Run interactive setup wizard (alias for -setup)")
	listModels := flag.Bool("models", false, "List available models from provider")

	// Claude Code management flags
	launchClaude := flag.Bool("launch", false, "Start proxy and launch Claude Code (default behavior)")
	updateClaude := flag.Bool("update-claude", false, "Update Claude Code to latest version")
	claudeStatus := flag.Bool("claude-status", false, "Check Claude Code installation status")
	proxyOnly := flag.Bool("proxy-only", false, "Run proxy only without launching Claude Code")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	skipPermissions := flag.Bool("skip-permissions", false, "Auto-approve all Claude Code operations (--dangerously-skip-permissions)")
	withPrompts := flag.Bool("with-prompts", false, "Force standard mode with confirmation prompts (overrides profile setting)")

	// Profile management flags
	profileName := flag.String("profile", "", "Use a specific profile")

	// Direct API key flag (with security warning)
	directAPIKey := flag.String("api-key", "", "API key for provider (⚠️ visible in shell history)")

	flag.Parse()

	// Warn about API key in command line (security risk)
	if *directAPIKey != "" {
		fmt.Print(setup.WarnCLIAPIKey())
	}

	// Apply profile if specified
	if *profileName != "" {
		pm := setup.NewProfileManager()
		profile, err := pm.GetProfile(*profileName)
		if err != nil {
			log.Fatalf("[CLASP] Profile '%s' not found. Run 'clasp profile list' to see available profiles.", *profileName)
		}
		if err := pm.ApplyProfileToEnv(profile); err != nil {
			log.Fatalf("[CLASP] Failed to apply profile: %v", err)
		}
		log.Printf("[CLASP] Using profile: %s", *profileName)
	}

	if *showVersion {
		fmt.Printf("CLASP %s\n", version)
		os.Exit(0)
	}

	if *help {
		printHelp()
		os.Exit(0)
	}

	// Handle setup command
	if *runSetup || *configure {
		wizard := setup.NewWizard()
		if _, err := wizard.Run(); err != nil {
			log.Fatalf("[CLASP] Setup failed: %v", err)
		}
		os.Exit(0)
	}

	// Handle models command
	if *listModels {
		if err := listAvailableModels(); err != nil {
			log.Fatalf("[CLASP] Failed to list models: %v", err)
		}
		os.Exit(0)
	}

	// Handle Claude Code status check
	if *claudeStatus {
		manager := claudecode.NewManager("", *verbose)
		status, err := manager.CheckInstallation()
		if err != nil {
			log.Fatalf("[CLASP] Error checking Claude Code: %v", err)
		}

		fmt.Println("")
		if status.Installed {
			fmt.Printf("Claude Code: Installed\n")
			fmt.Printf("  Version: %s\n", status.Version)
			fmt.Printf("  Path: %s\n", status.Path)
			fmt.Printf("  Method: %s\n", status.InstallMethod)

			// Check for updates
			needsUpdate, latestVersion, err := manager.CheckForUpdates(status.Version)
			if err == nil {
				if needsUpdate {
					fmt.Printf("  Update Available: %s\n", latestVersion)
				} else {
					fmt.Printf("  Status: Up to date\n")
				}
			}
		} else {
			fmt.Println("Claude Code: Not installed")
			fmt.Println("  Run 'clasp -launch' to install and launch Claude Code")
		}
		fmt.Println("")
		os.Exit(0)
	}

	// Handle Claude Code update
	if *updateClaude {
		manager := claudecode.NewManager("", *verbose)
		if err := manager.Update(); err != nil {
			log.Fatalf("[CLASP] Failed to update Claude Code: %v", err)
		}
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

	// Try to load saved config from ~/.clasp/config.json
	if err := setup.ApplyConfigToEnv(); err == nil {
		log.Printf("[CLASP] Loaded configuration from %s", setup.GetConfigPath())
	}

	// Check if setup is needed (no API keys configured)
	if setup.NeedsSetup() {
		fmt.Println("")
		fmt.Println("No configuration found. Starting interactive setup...")
		fmt.Println("")

		wizard := setup.NewWizard()
		if _, err := wizard.Run(); err != nil {
			log.Fatalf("[CLASP] Setup failed: %v", err)
		}

		// Reload env after setup
		if err := setup.ApplyConfigToEnv(); err != nil {
			log.Fatalf("[CLASP] Failed to apply configuration: %v", err)
		}
	}

	// Apply direct API key if provided (sets appropriate env var based on provider)
	if *directAPIKey != "" {
		// Determine provider and set appropriate env var
		providerToUse := *provider
		if providerToUse == "" {
			providerToUse = os.Getenv("PROVIDER")
		}
		if providerToUse == "" {
			providerToUse = "openai" // Default to OpenAI
		}

		switch providerToUse {
		case "openai":
			os.Setenv("OPENAI_API_KEY", *directAPIKey)
		case "azure":
			os.Setenv("AZURE_API_KEY", *directAPIKey)
		case "openrouter":
			os.Setenv("OPENROUTER_API_KEY", *directAPIKey)
		case "anthropic":
			os.Setenv("ANTHROPIC_API_KEY", *directAPIKey)
		case "custom":
			os.Setenv("CUSTOM_API_KEY", *directAPIKey)
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

	// Enable debug file logging if debug is enabled (from either -debug flag or CLASP_DEBUG env var)
	if cfg.Debug {
		if err := logging.EnableDebugLogging(); err != nil {
			log.Printf("[CLASP] Warning: Could not enable debug file logging: %v", err)
		} else {
			log.Printf("[CLASP] Debug logging enabled: %s", logging.GetDebugLogPath())
		}
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
	if *httpTimeout > 0 {
		cfg.HTTPClientTimeoutSec = *httpTimeout
	}

	// Validate authentication configuration
	if cfg.AuthEnabled && cfg.AuthAPIKey == "" {
		log.Fatalf("[CLASP] Authentication enabled but no API key provided. Set CLASP_AUTH_API_KEY or use -auth-api-key flag.")
	}

	// Create server with version for status line
	server, err := proxy.NewServerWithVersion(cfg, version)
	if err != nil {
		log.Fatalf("[CLASP] Failed to create server: %v", err)
	}

	// By default, launch Claude Code with the proxy (unless -proxy-only is specified)
	// The -launch flag is kept for backwards compatibility but is now the default behavior
	shouldLaunchClaude := !*proxyOnly
	if *launchClaude {
		shouldLaunchClaude = true // Explicit -launch always launches
	}

	if shouldLaunchClaude {
		// Configure logging to file to prevent TUI corruption
		if err := logging.ConfigureForClaudeCode(); err != nil {
			// Fall back to quiet mode if file logging fails
			logging.ConfigureQuiet()
		}
		defer logging.Close()

		printBanner()
		fmt.Println("")

		// Get Claude args (everything after "--")
		_, claudeArgs := claudecode.ParseClaudeArgs(flag.Args())

		// Start the proxy server in a goroutine
		serverErrCh := make(chan error, 1)
		go func() {
			serverErrCh <- server.Start()
		}()

		// Wait briefly for proxy to start
		time.Sleep(500 * time.Millisecond)

		// Check if proxy started successfully by hitting health endpoint
		proxyURL := fmt.Sprintf("http://localhost:%d", cfg.Port)
		healthURL := proxyURL + "/health"

		for i := 0; i < 10; i++ {
			resp, err := http.Get(healthURL)
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				break
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(200 * time.Millisecond)
		}

		fmt.Printf("[CLASP] Proxy started on %s\n", proxyURL)

		// Set up signal handling for graceful shutdown
		ctx, cancel := context.WithCancel(context.Background())
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigCh
			fmt.Println("\n[CLASP] Shutting down...")
			cancel()
		}()

		// Determine SkipPermissions setting
		// Priority: CLI flag > profile setting > default (false)
		shouldSkipPermissions := false

		// First, check if profile has this setting
		pm := setup.NewProfileManager()
		if activeProfile, err := pm.GetActiveProfile(); err == nil && activeProfile != nil {
			if activeProfile.ClaudeCode != nil {
				shouldSkipPermissions = activeProfile.ClaudeCode.SkipPermissions
			}
		}

		// CLI flags override profile setting
		if *skipPermissions {
			shouldSkipPermissions = true
		}
		if *withPrompts {
			shouldSkipPermissions = false
		}

		// Launch Claude Code
		manager := claudecode.NewManager(proxyURL, *verbose)
		launchOpts := claudecode.LaunchOptions{
			WorkingDir:      "",
			Args:            claudeArgs,
			ProxyURL:        proxyURL,
			Interactive:     true,
			SkipPermissions: shouldSkipPermissions,
		}

		claudeErrCh := make(chan error, 1)
		go func() {
			claudeErrCh <- manager.Launch(launchOpts)
		}()

		// Wait for either Claude to exit or context cancellation
		select {
		case err := <-claudeErrCh:
			if err != nil {
				log.Printf("[CLASP] Claude Code exited with error: %v", err)
			}
		case <-ctx.Done():
			// Shutdown requested
		case err := <-serverErrCh:
			log.Printf("[CLASP] Proxy server error: %v", err)
		}

		fmt.Println("[CLASP] Session ended")
		os.Exit(0)
	}

	// Standard proxy-only mode
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

Usage: clasp [options] [-- claude-args...]
       clasp <command> [arguments]

Quick Start:
  clasp                     Start proxy AND launch Claude Code (default)
  clasp -proxy-only         Start proxy only (no Claude Code)
  clasp status              Show current configuration status
  clasp use <profile>       Switch to a different profile

Profile Management:
  clasp profile create      Create new profile interactively
  clasp profile list        List all profiles
  clasp profile show        Show current profile details
  clasp profile use <name>  Switch to a profile
  clasp profile edit <name> Edit a profile
  clasp profile delete <n>  Delete a profile
  clasp profile export <n>  Export profile to file
  clasp profile import <f>  Import profile from file

Setup & Configuration:
  -setup                    Run interactive setup wizard
  -configure                Alias for -setup
  -models                   List available models from provider
  -profile <name>           Use a specific profile for this session

Claude Code Management:
  -proxy-only               Run proxy only without launching Claude Code
  -launch                   Explicit flag to launch Claude Code (now default)
  -claude-status            Check Claude Code installation status
  -update-claude            Update Claude Code to latest version
  -skip-permissions         Auto-approve all operations (no confirmation prompts)
  -with-prompts             Force standard mode with prompts (overrides profile)
  -verbose                  Enable verbose output

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

  HTTP Client:
    CLASP_HTTP_TIMEOUT             Request timeout in seconds (default: 300 = 5 min)

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
  # Default: clasp starts proxy and launches Claude Code together
  OPENAI_API_KEY=sk-xxx clasp

  # Pass arguments to Claude Code after "--"
  OPENAI_API_KEY=sk-xxx clasp -- --resume

  # Run proxy only (no Claude Code launch)
  OPENAI_API_KEY=sk-xxx clasp -proxy-only

  # Auto-approve mode (no confirmation prompts - for trusted environments)
  OPENAI_API_KEY=sk-xxx clasp -skip-permissions

  # Force prompts even if profile has auto-approve enabled
  OPENAI_API_KEY=sk-xxx clasp -with-prompts

  # Manual integration (set ANTHROPIC_BASE_URL to point to CLASP)
  ANTHROPIC_BASE_URL=http://localhost:8080 claude

  # Check Claude Code installation status
  clasp -claude-status

  # Update Claude Code to latest version
  clasp -update-claude

Permission Modes:
  Auto-approve mode (-skip-permissions) uses Claude Code's --dangerously-skip-permissions
  flag, which disables all confirmation prompts for file edits, commands, etc.
  This is recommended for trusted development environments and CI/CD pipelines.

  The permission mode can be configured in a profile during 'clasp profile create'
  and overridden per-session with -skip-permissions or -with-prompts flags.

For more information: https://github.com/jedarden/CLASP
`, version)
}

// handleProfileCommand handles all profile subcommands.
func handleProfileCommand(args []string) {
	if len(args) == 0 {
		printProfileHelp()
		return
	}

	wizard := setup.NewWizard()

	switch args[0] {
	case "create":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		if _, err := wizard.RunProfileCreate(name); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "list":
		if err := wizard.RunProfileList(); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "show":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		if err := wizard.RunProfileShow(name); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "use":
		if len(args) < 2 {
			fmt.Println("Usage: clasp profile use <name>")
			os.Exit(1)
		}
		if err := wizard.RunProfileUse(args[1]); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "delete":
		if len(args) < 2 {
			fmt.Println("Usage: clasp profile delete <name>")
			os.Exit(1)
		}
		if err := wizard.RunProfileDelete(args[1]); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "edit":
		// Edit is essentially create with overwrite
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		if _, err := wizard.RunProfileCreate(name); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "export":
		if len(args) < 2 {
			fmt.Println("Usage: clasp profile export <name> [output-file]")
			os.Exit(1)
		}
		outputPath := ""
		if len(args) > 2 {
			outputPath = args[2]
		}
		if err := wizard.RunProfileExport(args[1], outputPath); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "import":
		if len(args) < 2 {
			fmt.Println("Usage: clasp profile import <file> [new-name]")
			os.Exit(1)
		}
		newName := ""
		if len(args) > 2 {
			newName = args[2]
		}
		if err := wizard.RunProfileImport(args[1], newName); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "help", "-h", "--help":
		printProfileHelp()

	default:
		fmt.Printf("Unknown profile command: %s\n", args[0])
		printProfileHelp()
		os.Exit(1)
	}
}

// handleStatusCommand shows current CLASP status.
func handleStatusCommand(args []string) {
	// Check for flags
	verbose := false
	showAll := false
	cleanup := false
	var port int

	for i, arg := range args {
		switch arg {
		case "-v", "--verbose":
			verbose = true
		case "-a", "--all":
			showAll = true
		case "--cleanup":
			cleanup = true
		case "-p", "--port":
			if i+1 < len(args) {
				if p, err := strconv.Atoi(args[i+1]); err == nil {
					port = p
				}
			}
		}
	}

	// Handle cleanup command
	if cleanup {
		cleaned, err := statusline.CleanupStaleInstances()
		if err != nil {
			fmt.Printf("Error cleaning up stale instances: %v\n", err)
			os.Exit(1)
		}
		if cleaned > 0 {
			fmt.Printf("Cleaned up %d stale status file(s)\n", cleaned)
		} else {
			fmt.Println("No stale instances to clean up")
		}
		return
	}

	// Handle --all flag to show all instances
	if showAll {
		instances, err := statusline.ListAllInstances()
		if err != nil {
			fmt.Printf("Error listing instances: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(statusline.FormatAllInstancesTable(instances))
		return
	}

	pm := setup.NewProfileManager()

	// Get active profile (may be nil if not configured)
	activeProfile, _ := pm.GetActiveProfile()

	fmt.Println("")
	fmt.Printf("CLASP %s\n", version)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Check if proxy is running (from status file)
	var proxyStatus *statusline.Status
	var err error
	if port > 0 {
		proxyStatus, err = statusline.ReadStatusFromPort(port)
	} else {
		proxyStatus, err = statusline.ReadStatusFromFile()
	}

	if err == nil && proxyStatus != nil && proxyStatus.Running {
		// Verify the process is still running
		isRunning := false
		if proxyStatus.PID > 0 {
			process, err := os.FindProcess(proxyStatus.PID)
			if err == nil {
				err = process.Signal(syscall.Signal(0))
				isRunning = err == nil
			}
		}

		if isRunning {
			// Show running proxy status
			fmt.Println("")
			fmt.Println("🟢 Proxy Running")
			fmt.Println(statusline.FormatStatusLine(proxyStatus, verbose))

			if verbose {
				fmt.Println("")
				fmt.Printf("  Version:    %s\n", proxyStatus.Version)
				fmt.Printf("  PID:        %d\n", proxyStatus.PID)
				fmt.Printf("  Started:    %s\n", proxyStatus.StartTime.Format("2006-01-02 15:04:05"))
				if proxyStatus.Fallback != "" {
					fmt.Printf("  Fallback:   %s\n", proxyStatus.Fallback)
				}
			}
			fmt.Println("")
		} else {
			fmt.Println("")
			fmt.Println("⚪ Proxy Not Running (stale status file)")
			fmt.Println("  Run 'clasp status --cleanup' to clean up stale files")
			fmt.Println("")
		}
	} else {
		fmt.Println("")
		fmt.Println("⚪ Proxy Not Running")
		fmt.Println("")
	}

	// Show profile configuration
	fmt.Println("Configuration:")
	if activeProfile != nil {
		fmt.Printf("  Profile:    %s\n", activeProfile.Name)
		fmt.Printf("  Provider:   %s\n", activeProfile.Provider)

		if activeProfile.DefaultModel != "" {
			fmt.Printf("  Model:      %s\n", activeProfile.DefaultModel)
		}

		// Show tier mappings if configured
		if len(activeProfile.TierMappings) > 0 {
			fmt.Println("")
			fmt.Println("  Model Routing:")
			for tier, mapping := range activeProfile.TierMappings {
				fmt.Printf("    %s → %s\n", tier, mapping.Model)
			}
		}

		// Show configured port
		if activeProfile.Port > 0 {
			fmt.Printf("\n  Port:       %d\n", activeProfile.Port)
		}

		// Show features
		var features []string
		if activeProfile.RateLimitEnabled {
			features = append(features, "rate-limit")
		}
		if activeProfile.CacheEnabled {
			features = append(features, "cache")
		}
		if activeProfile.CircuitBreakerEnabled {
			features = append(features, "circuit-breaker")
		}
		if len(features) > 0 {
			fmt.Printf("\n  Features:   %s\n", strings.Join(features, ", "))
		}
	} else {
		fmt.Println("  No profile configured.")
	}

	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  clasp                    Start proxy and Claude Code (default)")
	fmt.Println("  clasp -proxy-only        Start proxy only")
	fmt.Println("  clasp profile list       Show all profiles")
	fmt.Println("  clasp profile create     Create new profile")
	fmt.Println("  clasp use <name>         Switch profile")
	fmt.Println("  clasp status -v          Show verbose status with metrics")
	fmt.Println("  clasp status --all       Show all running CLASP instances")
	fmt.Println("  clasp status -p <port>   Show status for specific port")
	fmt.Println("  clasp status --cleanup   Remove stale status files")
	fmt.Println("")
}

// printProfileHelp shows profile command help.
func printProfileHelp() {
	fmt.Print(`
CLASP Profile Management

Usage: clasp profile <command> [arguments]

Commands:
  create [name]        Create a new profile interactively
  list                 List all available profiles
  show [name]          Show profile details (current profile if name omitted)
  use <name>           Switch to a different profile
  edit <name>          Edit an existing profile
  delete <name>        Delete a profile
  export <name> [file] Export profile to JSON file
  import <file> [name] Import profile from JSON file

Quick Commands:
  clasp use <name>     Quick alias for 'clasp profile use'

Examples:
  clasp profile create work
  clasp profile list
  clasp profile use personal
  clasp profile export work ./work-profile.json
  clasp profile import ./shared.json team

Profiles are stored in ~/.clasp/profiles/
`)
}

// handleLogsCommand handles the logs subcommand.
func handleLogsCommand(args []string) {
	logPath := logging.GetLogPath()
	debugLogPath := logging.GetDebugLogPath()

	if len(args) > 0 {
		switch args[0] {
		case "--path", "-p":
			fmt.Printf("Main log:  %s\n", logPath)
			fmt.Printf("Debug log: %s\n", debugLogPath)
			return
		case "--clear", "-c":
			// Clear main log
			if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
				fmt.Printf("Error clearing main logs: %v\n", err)
			} else {
				fmt.Println("Main logs cleared.")
			}
			// Clear debug log
			if err := os.Remove(debugLogPath); err != nil && !os.IsNotExist(err) {
				fmt.Printf("Error clearing debug logs: %v\n", err)
			} else {
				fmt.Println("Debug logs cleared.")
			}
			return
		case "--debug", "-d":
			// Show debug logs
			showLogFile(debugLogPath, "Debug")
			return
		case "--follow", "-f":
			// Follow main log file
			tailLogFile(logPath, "Main")
			return
		case "--follow-debug", "-fd":
			// Follow debug log file
			tailLogFile(debugLogPath, "Debug")
			return
		case "--help", "-h":
			fmt.Print(`
CLASP Logs

Usage: clasp logs [options]

Options:
  --path, -p           Show log file paths
  --clear, -c          Clear all log files
  --debug, -d          Show debug log (request/response details)
  --follow, -f         Follow main log file (like tail -f)
  --follow-debug, -fd  Follow debug log file (like tail -f)
  --help, -h           Show this help

By default, shows the last 50 lines of the main log file.

Log Locations:
  Main log:  ~/.clasp/logs/clasp.log
  Debug log: ~/.clasp/logs/debug.log

When CLASP runs in Claude Code mode (the default), all proxy logs
are written to these files instead of stdout to prevent TUI corruption.

Debug logging captures full request/response payloads. Enable it with:
  clasp --debug
`)
			return
		}
	}

	// Default: show main log file
	showLogFile(logPath, "Main")
}

// showLogFile displays the last 50 lines of a log file.
func showLogFile(logPath string, logType string) {
	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Printf("No %s logs found yet.\n", strings.ToLower(logType))
		fmt.Printf("Log file location: %s\n", logPath)
		fmt.Println("")
		fmt.Println("Logs are created when CLASP runs in Claude Code mode.")
		if logType == "Debug" {
			fmt.Println("Enable debug logging with: clasp --debug")
		} else {
			fmt.Println("Use 'clasp -proxy-only' to see logs in real-time on stdout.")
		}
		return
	}

	// Read and display the last 50 lines
	content, err := os.ReadFile(logPath)
	if err != nil {
		fmt.Printf("Error reading %s logs: %v\n", strings.ToLower(logType), err)
		os.Exit(1)
	}

	lines := strings.Split(string(content), "\n")
	start := 0
	if len(lines) > 50 {
		start = len(lines) - 50
	}

	fmt.Printf("=== CLASP %s Logs (%s) ===\n\n", logType, logPath)
	for _, line := range lines[start:] {
		if line != "" {
			fmt.Println(line)
		}
	}
}

// tailLogFile follows a log file and prints new content (like tail -f).
func tailLogFile(logPath string, logType string) {
	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Printf("No %s logs found yet. Waiting for log file to be created...\n", strings.ToLower(logType))
		fmt.Printf("Log file location: %s\n", logPath)
		fmt.Println("Press Ctrl+C to exit.")
		fmt.Println("")
	}

	fmt.Printf("=== Following CLASP %s Logs (%s) ===\n", logType, logPath)
	fmt.Println("Press Ctrl+C to exit.")
	fmt.Println("")

	// Setup signal handling for clean exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Track file position
	var lastPos int64 = 0
	var lastSize int64 = 0

	// If file exists, seek to end to only show new content
	if info, err := os.Stat(logPath); err == nil {
		lastPos = info.Size()
		lastSize = info.Size()
		// Show last 10 lines initially
		if content, err := os.ReadFile(logPath); err == nil {
			lines := strings.Split(string(content), "\n")
			start := 0
			if len(lines) > 10 {
				start = len(lines) - 10
			}
			for _, line := range lines[start:] {
				if line != "" {
					fmt.Println(line)
				}
			}
		}
	}

	// Poll for new content
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Println("\nStopped following logs.")
			return
		case <-ticker.C:
			info, err := os.Stat(logPath)
			if err != nil {
				continue // File doesn't exist yet
			}

			// File was truncated/rotated
			if info.Size() < lastSize {
				lastPos = 0
				fmt.Println("--- Log file rotated ---")
			}
			lastSize = info.Size()

			// No new content
			if info.Size() <= lastPos {
				continue
			}

			// Read new content
			f, err := os.Open(logPath)
			if err != nil {
				continue
			}

			f.Seek(lastPos, 0)
			buf := make([]byte, info.Size()-lastPos)
			n, err := f.Read(buf)
			f.Close()

			if err == nil && n > 0 {
				fmt.Print(string(buf[:n]))
				lastPos += int64(n)
			}
		}
	}
}

// listAvailableModels lists models from the configured provider.
func listAvailableModels() error {
	// Load .env file if it exists
	envPaths := []string{
		".env",
		filepath.Join(os.Getenv("HOME"), ".clasp", ".env"),
	}
	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			godotenv.Load(path)
		}
	}

	// Try to load saved config
	if err := setup.ApplyConfigToEnv(); err != nil {
		// No saved config, try env
	}

	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("no configuration found. Run 'clasp -setup' first")
	}

	fmt.Println("")
	fmt.Printf("Fetching models from %s...\n", cfg.Provider)
	fmt.Println("")

	wizard := setup.NewWizard()
	models, err := wizard.FetchModelsPublic(string(cfg.Provider), cfg.GetAPIKey(), cfg.CustomBaseURL, cfg.AzureEndpoint)
	if err != nil {
		return fmt.Errorf("failed to fetch models: %w", err)
	}

	if len(models) == 0 {
		fmt.Println("No models found.")
		return nil
	}

	fmt.Printf("Available models (%d):\n", len(models))
	fmt.Println("")
	for _, m := range models {
		fmt.Printf("  %s\n", m)
	}
	fmt.Println("")

	return nil
}
