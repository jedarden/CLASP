// CLASP - Claude Language Agent Super Proxy
// Subcommand handlers and utility functions
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/jedarden/clasp/internal/claudecode"
	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/logging"
	"github.com/jedarden/clasp/internal/mcpserver"
	"github.com/jedarden/clasp/internal/proxy"
	"github.com/jedarden/clasp/internal/setup"
	"github.com/jedarden/clasp/internal/statusline"
)

// printBanner displays the CLASP banner.
func printBanner() {
	fmt.Println(`
╔═══════════════════════════════════════════════════════════════╗
║        CLASP - Claude Language Agent Super Proxy              ║
║        Translate Claude API calls to any LLM provider         ║
╚═══════════════════════════════════════════════════════════════╝`)
}

// printHelp shows the main help message.
func printHelp() {
	fmt.Printf(`CLASP - Claude Language Agent Super Proxy %s

Usage: clasp [options] [-- claude-args...]
       clasp <command> [arguments]

Quick Start:
  clasp                     Launch with profile selector (if profiles exist)
  clasp -profile <name>     Start with a specific profile (skip selector)
  clasp -proxy-only         Start proxy only (no Claude Code, no selector)
  clasp status              Show current configuration status
  clasp use <profile>       Switch to a different profile
  clasp doctor              Run diagnostics and troubleshooting
  clasp mcp                 Start as MCP server (for tool integration)
  clasp update              Update CLASP to the latest version

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
func showLogFile(logPath, logType string) {
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
func tailLogFile(logPath, logType string) {
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

			_, _ = f.Seek(lastPos, 0)
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
			_ = godotenv.Load(path)
		}
	}

	// Try to load saved config (ignore error if no saved config)
	_ = setup.ApplyConfigToEnv()

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

// handleMCPCommand starts the MCP server mode.
func handleMCPCommand(args []string) {
	// Parse MCP-specific flags
	transport := "stdio" // Default to stdio for MCP
	httpAddr := ""

	for i, arg := range args {
		switch arg {
		case "-t", "--transport":
			if i+1 < len(args) {
				transport = args[i+1]
			}
		case "-a", "--addr":
			if i+1 < len(args) {
				httpAddr = args[i+1]
			}
		case "-h", "--help":
			printMCPHelp()
			return
		}
	}

	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		// Create minimal config for MCP mode
		cfg = &config.Config{
			Provider: config.ProviderType(os.Getenv("PROVIDER")),
		}
		if cfg.Provider == "" {
			cfg.Provider = config.ProviderOpenAI
		}
	}

	// Create MCP server
	server := mcpserver.NewServer("clasp", cfg)

	// Run the MCP server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	switch transport {
	case "stdio":
		if err := server.Run(ctx); err != nil && err != context.Canceled {
			cancel()
			log.Printf("[MCP] Server error: %v", err)
			os.Exit(1)
		}
	case "http":
		if httpAddr == "" {
			httpAddr = ":8081"
		}
		if err := server.RunHTTP(ctx, httpAddr); err != nil && err != context.Canceled {
			cancel()
			log.Printf("[MCP] HTTP server error: %v", err)
			os.Exit(1)
		}
	default:
		cancel()
		log.Printf("[MCP] Unknown transport: %s (use 'stdio' or 'http')", transport)
		os.Exit(1)
	}
}

// printMCPHelp prints help for the MCP command.
func printMCPHelp() {
	fmt.Printf(`CLASP MCP Server Mode %s

Start CLASP as an MCP (Model Context Protocol) server for integration
with Claude Code and other MCP-compatible clients.

Usage: clasp mcp [options]

Options:
  -t, --transport <type>   Transport type: stdio (default) or http
  -a, --addr <addr>        HTTP address (default: :8081, only for http transport)
  -h, --help               Show this help

Transport Types:
  stdio    Standard input/output (default, for local process communication)
  http     HTTP server with SSE support (for remote connections)

Available MCP Tools:
  clasp_status      Get proxy status, provider, model, and session info
  clasp_config      Get or update CLASP configuration
  clasp_profile     Manage profiles (list, create, switch, delete)
  clasp_models      List available models from the provider
  clasp_metrics     Get proxy performance metrics
  clasp_health      Check proxy and provider health
  clasp_doctor      Run diagnostic checks
  clasp_translate   Translate Anthropic to OpenAI format (debug)

Examples:
  # Start as stdio MCP server (for Claude Code integration)
  clasp mcp

  # Start as HTTP MCP server on port 8081
  clasp mcp -t http

  # Start as HTTP MCP server on custom port
  clasp mcp -t http -a :9090

MCP Client Configuration:
  To add CLASP as an MCP server in Claude Code:

  claude mcp add clasp npx clasp-ai mcp

  Or for HTTP mode:
  claude mcp add clasp-http --url http://localhost:8081/mcp

For more information: https://github.com/jedarden/CLASP
`, version)
}

// handleUpdateCommand handles the self-update command.
func handleUpdateCommand(args []string) {
	// Parse flags
	checkOnly := false
	forceUpdate := false

	for _, arg := range args {
		switch arg {
		case "-c", "--check":
			checkOnly = true
		case "-f", "--force":
			forceUpdate = true
		case "-h", "--help":
			printUpdateHelp()
			return
		}
	}

	fmt.Println("")
	fmt.Printf("CLASP %s\n", version)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("")

	// Check for latest version from npm
	fmt.Println("Checking for updates...")
	cmd := exec.Command("npm", "view", "clasp-ai", "version")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("")
		fmt.Println("❌ Could not check for updates.")
		fmt.Println("")
		fmt.Println("Make sure you have Node.js and npm installed:")
		fmt.Println("  • Node.js: https://nodejs.org/")
		fmt.Println("")
		os.Exit(1)
	}

	latestVersion := strings.TrimSpace(string(output))
	currentVersion := strings.TrimPrefix(version, "v")

	fmt.Printf("  Current version: %s\n", version)
	fmt.Printf("  Latest version:  v%s\n", latestVersion)
	fmt.Println("")

	// Compare versions
	if currentVersion == latestVersion {
		fmt.Println("✅ You're running the latest version!")
		fmt.Println("")
		return
	}

	if checkOnly {
		fmt.Printf("🔄 Update available: %s → v%s\n", version, latestVersion)
		fmt.Println("")
		fmt.Println("Run 'clasp update' to update.")
		fmt.Println("")
		return
	}

	// Prompt for confirmation unless force flag is set
	if !forceUpdate {
		fmt.Printf("🔄 Update available: %s → v%s\n", version, latestVersion)
		fmt.Println("")
		fmt.Print("Do you want to update now? [y/N]: ")

		var response string
		_, _ = fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "y" && response != "yes" {
			fmt.Println("")
			fmt.Println("Update canceled.")
			fmt.Println("")
			return
		}
	}

	fmt.Println("")
	fmt.Println("Updating CLASP...")
	fmt.Println("")

	// Determine update method based on how clasp was installed
	// Check if we're running from npx cache or global install
	execPath, err := os.Executable()
	if err != nil {
		execPath = ""
	}

	var updateCmd *exec.Cmd
	if strings.Contains(execPath, ".npm/_npx") || strings.Contains(execPath, "npx") {
		// Running via npx - clear npx cache for fresh install on next run
		fmt.Println("Detected npx installation.")
		fmt.Println("")
		fmt.Println("To update, run:")
		fmt.Println("  npx clasp-ai@latest")
		fmt.Println("")
		fmt.Println("The latest version will be automatically downloaded on next run.")
		fmt.Println("")
		return
	}

	// Try global npm update first
	updateCmd = exec.Command("npm", "update", "-g", "clasp-ai")
	updateCmd.Stdout = os.Stdout
	updateCmd.Stderr = os.Stderr

	if err := updateCmd.Run(); err != nil {
		// Try alternative methods
		fmt.Println("")
		fmt.Println("⚠️  Global update failed. Trying alternative methods...")
		fmt.Println("")

		// Try npm install -g
		updateCmd = exec.Command("npm", "install", "-g", "clasp-ai@latest")
		updateCmd.Stdout = os.Stdout
		updateCmd.Stderr = os.Stderr

		if err := updateCmd.Run(); err != nil {
			fmt.Println("")
			fmt.Println("❌ Update failed.")
			fmt.Println("")
			fmt.Println("Try updating manually:")
			fmt.Println("  npm install -g clasp-ai@latest")
			fmt.Println("")
			fmt.Println("If you encounter permission errors, try:")
			fmt.Println("  sudo npm install -g clasp-ai@latest")
			fmt.Println("")
			os.Exit(1)
		}
	}

	fmt.Println("")
	fmt.Printf("✅ Successfully updated to v%s!\n", latestVersion)
	fmt.Println("")
	fmt.Println("Restart CLASP to use the new version.")
	fmt.Println("")
}

// printUpdateHelp prints help for the update command.
func printUpdateHelp() {
	fmt.Printf(`CLASP Self-Update %s

Update CLASP to the latest version.

Usage: clasp update [options]

Options:
  -c, --check    Check for updates without installing
  -f, --force    Update without confirmation prompt
  -h, --help     Show this help

Examples:
  clasp update            Update to latest version (with confirmation)
  clasp update --check    Check if updates are available
  clasp update --force    Update without asking for confirmation

Alternative Update Methods:
  If 'clasp update' fails, you can update manually:

  # Global installation
  npm install -g clasp-ai@latest

  # npx (no install needed, always runs latest)
  npx clasp-ai@latest

  # With sudo (if you get permission errors)
  sudo npm install -g clasp-ai@latest

For more information: https://github.com/jedarden/CLASP
`, version)
}

// handleClaudeStatus handles the Claude Code status check.
func handleClaudeStatus(verbose bool) {
	manager := claudecode.NewManager("", verbose)
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
}

// handleClaudeUpdate handles the Claude Code update.
func handleClaudeUpdate(verbose bool) {
	manager := claudecode.NewManager("", verbose)
	if err := manager.Update(); err != nil {
		log.Fatalf("[CLASP] Failed to update Claude Code: %v", err)
	}
}

// applyFlagOverrides applies command line flag overrides to the config.
func applyFlagOverrides(cfg *config.Config, flags *Flags) {
	if flags.Port > 0 {
		cfg.Port = flags.Port
	}
	if flags.Provider != "" {
		cfg.Provider = config.ProviderType(flags.Provider)
	}
	if flags.Model != "" {
		cfg.DefaultModel = flags.Model
	}
	if flags.Debug {
		cfg.Debug = true
		cfg.DebugRequests = true
		cfg.DebugResponses = true
	}

	if flags.RateLimit {
		cfg.RateLimitEnabled = true
	}
	if flags.RateLimitReqs > 0 {
		cfg.RateLimitRequests = flags.RateLimitReqs
	}
	if flags.RateLimitWindow > 0 {
		cfg.RateLimitWindow = flags.RateLimitWindow
	}
	if flags.RateLimitBurst > 0 {
		cfg.RateLimitBurst = flags.RateLimitBurst
	}
	if flags.Cache {
		cfg.CacheEnabled = true
	}
	if flags.CacheMaxSize > 0 {
		cfg.CacheMaxSize = flags.CacheMaxSize
	}
	if flags.CacheTTL > 0 {
		cfg.CacheTTL = flags.CacheTTL
	}
	if flags.MultiProvider {
		cfg.MultiProviderEnabled = true
	}
	if flags.Fallback {
		cfg.FallbackEnabled = true
	}
	if flags.Auth {
		cfg.AuthEnabled = true
	}
	if flags.AuthAPIKey != "" {
		cfg.AuthAPIKey = flags.AuthAPIKey
	}
	if flags.QueueEnabled {
		cfg.QueueEnabled = true
	}
	if flags.QueueMaxSize > 0 {
		cfg.QueueMaxSize = flags.QueueMaxSize
	}
	if flags.QueueMaxWait > 0 {
		cfg.QueueMaxWaitSeconds = flags.QueueMaxWait
	}
	if flags.CircuitBreaker {
		cfg.CircuitBreakerEnabled = true
	}
	if flags.CBThreshold > 0 {
		cfg.CircuitBreakerThreshold = flags.CBThreshold
	}
	if flags.CBRecovery > 0 {
		cfg.CircuitBreakerRecovery = flags.CBRecovery
	}
	if flags.CBTimeout > 0 {
		cfg.CircuitBreakerTimeoutSec = flags.CBTimeout
	}
	if flags.HTTPTimeout > 0 {
		cfg.HTTPClientTimeoutSec = flags.HTTPTimeout
	}
}

// applyDirectAPIKey applies the direct API key if provided.
func applyDirectAPIKey(flags *Flags) {
	if flags.DirectAPIKey == "" {
		return
	}

	// Determine provider and set appropriate env var
	providerToUse := flags.Provider
	if providerToUse == "" {
		providerToUse = os.Getenv("PROVIDER")
	}
	if providerToUse == "" {
		providerToUse = "openai" // Default to OpenAI
	}

	switch providerToUse {
	case "openai":
		os.Setenv("OPENAI_API_KEY", flags.DirectAPIKey)
	case "azure":
		os.Setenv("AZURE_API_KEY", flags.DirectAPIKey)
	case "openrouter":
		os.Setenv("OPENROUTER_API_KEY", flags.DirectAPIKey)
	case "anthropic":
		os.Setenv("ANTHROPIC_API_KEY", flags.DirectAPIKey)
	case "custom":
		os.Setenv("CUSTOM_API_KEY", flags.DirectAPIKey)
	}
}

// loadEnvFiles loads environment from .env files.
func loadEnvFiles() {
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
}

// runProxyWithClaude starts the proxy and launches Claude Code.
func runProxyWithClaude(cfg *config.Config, flags *Flags) {
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
	server, err := proxy.NewServerWithVersion(cfg, version)
	if err != nil {
		log.Fatalf("[CLASP] Failed to create server: %v", err)
	}

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.Start()
	}()

	// Wait briefly for proxy to start
	time.Sleep(500 * time.Millisecond)

	// Check if proxy started successfully by hitting health endpoint
	proxyURL := fmt.Sprintf("http://localhost:%d", cfg.Port)
	healthURL := proxyURL + "/health"

	httpClient := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 10; i++ {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, healthURL, http.NoBody)
		if reqErr != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		resp, err := httpClient.Do(req)
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
	if flags.SkipPermissions {
		shouldSkipPermissions = true
	}
	if flags.WithPrompts {
		shouldSkipPermissions = false
	}

	// Launch Claude Code
	manager := claudecode.NewManager(proxyURL, flags.Verbose)
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
	logging.Close()
}

// runProxyOnly starts the proxy in standalone mode.
func runProxyOnly(cfg *config.Config) {
	printBanner()

	server, err := proxy.NewServerWithVersion(cfg, version)
	if err != nil {
		log.Fatalf("[CLASP] Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("[CLASP] Server error: %v", err)
	}
}
