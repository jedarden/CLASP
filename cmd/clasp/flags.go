// CLASP - Claude Language Agent Super Proxy
// Command line flag definitions and parsing
package main

import (
	"flag"
	"fmt"

	"github.com/jedarden/clasp/internal/setup"
)

// Flags holds all command line flag values
type Flags struct {
	// Basic options
	Port         int
	Provider     string
	Model        string
	Debug        bool
	ShowVersion  bool
	Help         bool
	RunSetup     bool
	Configure    bool
	ListModels   bool

	// Rate limiting
	RateLimit       bool
	RateLimitReqs   int
	RateLimitWindow int
	RateLimitBurst  int

	// Caching
	Cache        bool
	CacheMaxSize int
	CacheTTL     int

	// Routing
	MultiProvider bool
	Fallback      bool

	// Authentication
	Auth        bool
	AuthAPIKey  string

	// Request queue
	QueueEnabled bool
	QueueMaxSize int
	QueueMaxWait int

	// Circuit breaker
	CircuitBreaker bool
	CBThreshold    int
	CBRecovery     int
	CBTimeout      int

	// HTTP client
	HTTPTimeout int

	// Claude Code management
	LaunchClaude     bool
	UpdateClaude     bool
	ClaudeStatus     bool
	ProxyOnly        bool
	Verbose          bool
	SkipPermissions  bool
	WithPrompts      bool

	// Profile management
	ProfileName string

	// Direct API key (with security warning)
	DirectAPIKey string
}

// ParseFlags parses command line flags and returns a Flags struct
func ParseFlags() *Flags {
	f := &Flags{}

	flag.IntVar(&f.Port, "port", 0, "Port to listen on (overrides CLASP_PORT)")
	flag.StringVar(&f.Provider, "provider", "", "LLM provider (openai, azure, openrouter, custom)")
	flag.StringVar(&f.Model, "model", "", "Default model to use")
	flag.BoolVar(&f.Debug, "debug", false, "Enable debug logging (requests and responses)")

	flag.BoolVar(&f.RateLimit, "rate-limit", false, "Enable rate limiting")
	flag.IntVar(&f.RateLimitReqs, "rate-limit-requests", 0, "Requests per window (default: 60)")
	flag.IntVar(&f.RateLimitWindow, "rate-limit-window", 0, "Window in seconds (default: 60)")
	flag.IntVar(&f.RateLimitBurst, "rate-limit-burst", 0, "Burst allowance (default: 10)")

	flag.BoolVar(&f.Cache, "cache", false, "Enable response caching")
	flag.IntVar(&f.CacheMaxSize, "cache-max-size", 0, "Maximum cache entries (default: 1000)")
	flag.IntVar(&f.CacheTTL, "cache-ttl", 0, "Cache TTL in seconds (default: 3600)")

	flag.BoolVar(&f.MultiProvider, "multi-provider", false, "Enable multi-provider tier routing")
	flag.BoolVar(&f.Fallback, "fallback", false, "Enable fallback routing")

	flag.BoolVar(&f.Auth, "auth", false, "Enable API key authentication")
	flag.StringVar(&f.AuthAPIKey, "auth-api-key", "", "API key for authentication (required with -auth)")

	flag.BoolVar(&f.QueueEnabled, "queue", false, "Enable request queuing during outages")
	flag.IntVar(&f.QueueMaxSize, "queue-max-size", 0, "Maximum queued requests (default: 100)")
	flag.IntVar(&f.QueueMaxWait, "queue-max-wait", 0, "Queue timeout in seconds (default: 30)")

	flag.BoolVar(&f.CircuitBreaker, "circuit-breaker", false, "Enable circuit breaker pattern")
	flag.IntVar(&f.CBThreshold, "cb-threshold", 0, "Circuit breaker failure threshold (default: 5)")
	flag.IntVar(&f.CBRecovery, "cb-recovery", 0, "Circuit breaker success recovery threshold (default: 2)")
	flag.IntVar(&f.CBTimeout, "cb-timeout", 0, "Circuit breaker timeout in seconds (default: 30)")

	flag.IntVar(&f.HTTPTimeout, "http-timeout", 0, "HTTP client timeout in seconds for upstream requests (default: 300)")

	flag.BoolVar(&f.ShowVersion, "version", false, "Show version information")
	flag.BoolVar(&f.Help, "help", false, "Show help message")
	flag.BoolVar(&f.RunSetup, "setup", false, "Run interactive setup wizard")
	flag.BoolVar(&f.Configure, "configure", false, "Run interactive setup wizard (alias for -setup)")
	flag.BoolVar(&f.ListModels, "models", false, "List available models from provider")

	// Claude Code management flags
	flag.BoolVar(&f.LaunchClaude, "launch", false, "Start proxy and launch Claude Code (default behavior)")
	flag.BoolVar(&f.UpdateClaude, "update-claude", false, "Update Claude Code to latest version")
	flag.BoolVar(&f.ClaudeStatus, "claude-status", false, "Check Claude Code installation status")
	flag.BoolVar(&f.ProxyOnly, "proxy-only", false, "Run proxy only without launching Claude Code")
	flag.BoolVar(&f.Verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&f.SkipPermissions, "skip-permissions", false, "Auto-approve all Claude Code operations (--dangerously-skip-permissions)")
	flag.BoolVar(&f.WithPrompts, "with-prompts", false, "Force standard mode with confirmation prompts (overrides profile setting)")

	// Profile management flags
	flag.StringVar(&f.ProfileName, "profile", "", "Use a specific profile")

	// Direct API key flag (with security warning)
	flag.StringVar(&f.DirectAPIKey, "api-key", "", "API key for provider (visible in shell history)")

	flag.Parse()

	// Warn about API key in command line (security risk)
	if f.DirectAPIKey != "" {
		fmt.Print(setup.WarnCLIAPIKey())
	}

	return f
}
