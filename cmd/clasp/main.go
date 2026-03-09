// CLASP - Claude Language Agent Super Proxy
// A Go proxy that translates Claude/Anthropic API calls to OpenAI-compatible endpoints.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jedarden/clasp/internal/config"
	"github.com/jedarden/clasp/internal/logging"
	"github.com/jedarden/clasp/internal/setup"
)

var (
	version = "v0.49.0"
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
		case "doctor":
			// Run diagnostics
			verbose := len(os.Args) > 2 && (os.Args[2] == "-v" || os.Args[2] == "--verbose")
			doctor := setup.NewDoctor(verbose)
			doctor.Run()
			doctor.PrintResults(os.Stdout)
			if doctor.HasErrors() {
				os.Exit(1)
			}
			return
		case "mcp":
			// Start MCP server mode
			handleMCPCommand(os.Args[2:])
			return
		case "update":
			// Self-update to latest version
			handleUpdateCommand(os.Args[2:])
			return
		}
	}

	// Parse command line flags
	flags := ParseFlags()

	// Profile selection logic:
	// In proxy-only mode, skip profile selection entirely - config will come from env/flags
	// 1. If --profile flag is set, use that profile
	// 2. If TTY available, show profile selector TUI (with option to create new)
	// 3. Non-TTY: use active profile or first available
	selectedProfileName := selectProfile(flags.ProfileName, flags.ProxyOnly)

	// Apply selected profile if we have one
	// In proxy-only mode with no profile, skip this - config from env/flags is sufficient
	applyProfile(selectedProfileName)

	if flags.ShowVersion {
		fmt.Printf("CLASP %s\n", version)
		os.Exit(0)
	}

	if flags.Help {
		printHelp()
		os.Exit(0)
	}

	// Handle setup command
	if flags.RunSetup || flags.Configure {
		wizard := setup.NewWizard()
		if _, err := wizard.Run(); err != nil {
			log.Fatalf("[CLASP] Setup failed: %v", err)
		}
		os.Exit(0)
	}

	// Handle models command
	if flags.ListModels {
		if err := listAvailableModels(); err != nil {
			log.Fatalf("[CLASP] Failed to list models: %v", err)
		}
		os.Exit(0)
	}

	// Handle Claude Code status check
	if flags.ClaudeStatus {
		handleClaudeStatus(flags.Verbose)
		os.Exit(0)
	}

	// Handle Claude Code update
	if flags.UpdateClaude {
		handleClaudeUpdate(flags.Verbose)
		os.Exit(0)
	}

	// Load .env file if it exists
	loadEnvFiles()

	// Try to load saved config from ~/.clasp/config.json
	if err := setup.ApplyConfigToEnv(); err == nil {
		log.Printf("[CLASP] Loaded configuration from %s", setup.GetConfigPath())
	}

	// Check if setup is needed (no API keys configured)
	// In proxy-only mode, we can proceed without setup if config is provided via env/flags
	// This allows containerized/CI usage without interactive prompts
	if setup.NeedsSetup() && !flags.ProxyOnly {
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
	applyDirectAPIKey(flags)

	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		// In proxy-only mode with no config, provide helpful error message
		if flags.ProxyOnly {
			fmt.Println("")
			fmt.Println("[CLASP] Configuration error: " + err.Error())
			fmt.Println("")
			fmt.Println("For proxy-only mode in containers/CI, provide configuration via:")
			fmt.Println("  Environment variables:")
			fmt.Println("    PROVIDER=openai           # or: azure, openrouter, anthropic, custom")
			fmt.Println("    OPENAI_API_KEY=sk-xxx     # or equivalent for your provider")
			fmt.Println("    CLASP_MODEL=gpt-4o        # (optional) default model")
			fmt.Println("    CLASP_PORT=8080           # (optional) proxy port")
			fmt.Println("")
			fmt.Println("  Or command-line flags:")
			fmt.Println("    clasp -proxy-only -provider openai -model gpt-4o -port 8080")
			fmt.Println("")
			fmt.Println("For interactive setup:")
			fmt.Println("    clasp -setup              # Create a profile interactively")
			fmt.Println("")
			os.Exit(1)
		}
		log.Fatalf("[CLASP] Configuration error: %v", err)
	}

	// Apply command line overrides
	applyFlagOverrides(cfg, flags)

	// Enable debug file logging if debug is enabled (from either -debug flag or CLASP_DEBUG env var)
	if cfg.Debug {
		if debugErr := logging.EnableDebugLogging(); debugErr != nil {
			log.Printf("[CLASP] Warning: Could not enable debug file logging: %v", debugErr)
		} else {
			log.Printf("[CLASP] Debug logging enabled: %s", logging.GetDebugLogPath())
		}
	}

	// Validate authentication configuration
	if cfg.AuthEnabled && cfg.AuthAPIKey == "" {
		log.Fatalf("[CLASP] Authentication enabled but no API key provided. Set CLASP_AUTH_API_KEY or use -auth-api-key flag.")
	}

	// By default, launch Claude Code with the proxy (unless -proxy-only is specified)
	// The -launch flag is kept for backwards compatibility but is now the default behavior
	shouldLaunchClaude := !flags.ProxyOnly
	if flags.LaunchClaude {
		shouldLaunchClaude = true // Explicit -launch always launches
	}

	if shouldLaunchClaude {
		runProxyWithClaude(cfg, flags)
		return
	}

	// Standard proxy-only mode
	runProxyOnly(cfg)
}
