// Package setup provides interactive configuration wizards for first-run experience.
package setup

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DiagnosticResult represents the result of a single diagnostic check.
type DiagnosticResult struct {
	Name    string
	Status  string // "ok", "warning", "error"
	Message string
	Fix     string // Suggested fix if status is not ok
}

// Doctor runs comprehensive diagnostics on the CLASP installation.
type Doctor struct {
	results []DiagnosticResult
	verbose bool
}

// NewDoctor creates a new Doctor instance.
func NewDoctor(verbose bool) *Doctor {
	return &Doctor{
		verbose: verbose,
	}
}

// Run executes all diagnostic checks and returns results.
func (d *Doctor) Run() []DiagnosticResult {
	d.results = nil

	// System checks
	d.checkPlatform()
	d.checkNodeVersion()
	d.checkGoVersion()

	// Configuration checks
	d.checkConfigDirectory()
	d.checkConfigFile()
	d.checkProfiles()

	// API Key checks
	d.checkAPIKeys()

	// Provider connectivity checks
	d.checkProviderConnectivity()

	// Network checks
	d.checkPortAvailability(8080)
	d.checkPortAvailability(8081)

	// Claude Code checks
	d.checkClaudeCodeInstallation()

	// Binary checks
	d.checkBinaryVersion()

	return d.results
}

// checkPlatform verifies the platform is supported.
func (d *Doctor) checkPlatform() {
	platform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	supportedPlatforms := []string{
		"darwin/amd64", "darwin/arm64",
		"linux/amd64", "linux/arm64",
		"windows/amd64",
	}

	supported := false
	for _, p := range supportedPlatforms {
		if p == platform {
			supported = true
			break
		}
	}

	if supported {
		d.addResult("Platform", "ok", fmt.Sprintf("Running on %s", platform), "")
	} else {
		d.addResult("Platform", "warning", fmt.Sprintf("Platform %s may not be fully supported", platform),
			"CLASP is primarily tested on Linux, macOS, and Windows x64/arm64")
	}
}

// checkNodeVersion checks if Node.js is installed and version is sufficient.
func (d *Doctor) checkNodeVersion() {
	cmd := exec.Command("node", "--version")
	output, err := cmd.Output()
	if err != nil {
		d.addResult("Node.js", "warning", "Node.js not found",
			"Install Node.js 18+ from https://nodejs.org/ for npx support")
		return
	}

	version := strings.TrimSpace(string(output))
	// Parse version number (e.g., v18.17.0)
	if strings.HasPrefix(version, "v") {
		majorStr := strings.Split(version[1:], ".")[0]
		var major int
		fmt.Sscanf(majorStr, "%d", &major)
		if major >= 18 {
			d.addResult("Node.js", "ok", fmt.Sprintf("Node.js %s installed", version), "")
		} else {
			d.addResult("Node.js", "warning", fmt.Sprintf("Node.js %s is older than recommended (18+)", version),
				"Update Node.js to version 18 or newer for best compatibility")
		}
	}
}

// checkGoVersion checks if Go is available (optional).
func (d *Doctor) checkGoVersion() {
	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err != nil {
		// Go is optional, only needed for building from source
		if d.verbose {
			d.addResult("Go", "ok", "Not installed (not required for binary distribution)", "")
		}
		return
	}

	version := strings.TrimSpace(string(output))
	d.addResult("Go", "ok", version, "")
}

// checkConfigDirectory verifies the config directory exists.
func (d *Doctor) checkConfigDirectory() {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".clasp")

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		d.addResult("Config Directory", "warning", fmt.Sprintf("Directory %s does not exist", configDir),
			"Run 'clasp -setup' to create configuration")
	} else {
		d.addResult("Config Directory", "ok", fmt.Sprintf("Directory %s exists", configDir), "")
	}
}

// checkConfigFile verifies the main config file exists and is valid.
func (d *Doctor) checkConfigFile() {
	configPath := GetConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			d.addResult("Configuration", "warning", "No configuration file found",
				"Run 'clasp -setup' or set API keys via environment variables")
		} else {
			d.addResult("Configuration", "error", fmt.Sprintf("Cannot read config: %v", err),
				"Check file permissions on ~/.clasp/config.json")
		}
		return
	}

	var cfg ConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		d.addResult("Configuration", "error", fmt.Sprintf("Invalid JSON in config file: %v", err),
			"Run 'clasp -setup' to recreate configuration")
		return
	}

	if cfg.Provider == "" {
		d.addResult("Configuration", "warning", "No provider configured",
			"Run 'clasp -setup' to configure a provider")
	} else {
		d.addResult("Configuration", "ok", fmt.Sprintf("Provider: %s, Model: %s", cfg.Provider, cfg.Model), "")
	}
}

// checkProfiles verifies profiles are properly configured.
func (d *Doctor) checkProfiles() {
	pm := NewProfileManager()
	profiles, err := pm.ListProfiles()
	if err != nil {
		if d.verbose {
			d.addResult("Profiles", "warning", "Could not list profiles",
				"Run 'clasp profile create' to create a profile")
		}
		return
	}

	if len(profiles) == 0 {
		d.addResult("Profiles", "ok", "No profiles configured (using environment/config file)", "")
	} else {
		activeProfile := "default"
		if gc, _ := pm.GetGlobalConfig(); gc != nil && gc.ActiveProfile != "" {
			activeProfile = gc.ActiveProfile
		}
		d.addResult("Profiles", "ok", fmt.Sprintf("%d profile(s), active: %s", len(profiles), activeProfile), "")
	}
}

// checkAPIKeys verifies API keys are configured for at least one provider.
func (d *Doctor) checkAPIKeys() {
	providers := []struct {
		name   string
		envVar string
	}{
		{"OpenAI", "OPENAI_API_KEY"},
		{"Azure OpenAI", "AZURE_API_KEY"},
		{"OpenRouter", "OPENROUTER_API_KEY"},
		{"Anthropic", "ANTHROPIC_API_KEY"},
		{"Gemini", "GEMINI_API_KEY"},
		{"DeepSeek", "DEEPSEEK_API_KEY"},
	}

	configuredCount := 0
	configuredProviders := []string{}

	for _, p := range providers {
		if os.Getenv(p.envVar) != "" {
			configuredCount++
			configuredProviders = append(configuredProviders, p.name)
		}
	}

	// Also check config file
	cfg, err := LoadConfig()
	if err == nil && cfg.APIKey != "" {
		if configuredCount == 0 {
			configuredProviders = append(configuredProviders, cfg.Provider+" (from config)")
			configuredCount++
		}
	}

	if configuredCount == 0 {
		d.addResult("API Keys", "error", "No API keys configured",
			"Set an API key: export OPENAI_API_KEY=sk-... or run 'clasp -setup'")
	} else {
		d.addResult("API Keys", "ok",
			fmt.Sprintf("Configured: %s", strings.Join(configuredProviders, ", ")), "")
	}
}

// checkProviderConnectivity tests connectivity to configured providers.
func (d *Doctor) checkProviderConnectivity() {
	client := &http.Client{Timeout: 5 * time.Second}

	// Test OpenAI API reachability (doesn't require auth for this check)
	endpoints := []struct {
		name string
		url  string
	}{
		{"OpenAI API", "https://api.openai.com"},
		{"OpenRouter API", "https://openrouter.ai"},
	}

	for _, ep := range endpoints {
		req, _ := http.NewRequest("HEAD", ep.url, http.NoBody)
		resp, err := client.Do(req)
		if err != nil {
			d.addResult(fmt.Sprintf("Network (%s)", ep.name), "warning",
				fmt.Sprintf("Cannot reach %s: %v", ep.name, err),
				"Check your internet connection and firewall settings")
		} else {
			resp.Body.Close()
			d.addResult(fmt.Sprintf("Network (%s)", ep.name), "ok", "Reachable", "")
		}
	}
}

// checkPortAvailability checks if a port is available for the proxy.
func (d *Doctor) checkPortAvailability(port int) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		d.addResult(fmt.Sprintf("Port %d", port), "warning",
			fmt.Sprintf("Port %d is in use", port),
			fmt.Sprintf("CLASP will automatically find an available port, or specify: clasp -port %d", port+10))
	} else {
		listener.Close()
		if d.verbose {
			d.addResult(fmt.Sprintf("Port %d", port), "ok", "Available", "")
		}
	}
}

// checkClaudeCodeInstallation verifies Claude Code CLI is installed.
func (d *Doctor) checkClaudeCodeInstallation() {
	// Check for claude command
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		d.addResult("Claude Code", "warning", "Claude Code CLI not found in PATH",
			"Run 'clasp' to automatically install Claude Code, or install with: npm install -g @anthropic-ai/claude-code")
		return
	}

	// Get version
	cmd := exec.Command(claudePath, "--version")
	output, err := cmd.Output()
	if err != nil {
		d.addResult("Claude Code", "warning", fmt.Sprintf("Found at %s but could not get version", claudePath),
			"Try reinstalling: npm install -g @anthropic-ai/claude-code")
		return
	}

	version := strings.TrimSpace(string(output))
	d.addResult("Claude Code", "ok", fmt.Sprintf("Version %s at %s", version, claudePath), "")
}

// checkBinaryVersion verifies the CLASP binary version matches package.
func (d *Doctor) checkBinaryVersion() {
	// Check local binary
	binaryPath, err := os.Executable()
	if err != nil {
		return
	}

	// Get npm package version if available
	cmd := exec.Command("npm", "view", "clasp-ai", "version")
	output, err := cmd.Output()
	if err == nil {
		npmVersion := strings.TrimSpace(string(output))
		d.addResult("Package Version", "ok", fmt.Sprintf("npm: v%s", npmVersion), "")
	}

	d.addResult("Binary Path", "ok", binaryPath, "")
}

// addResult adds a diagnostic result.
func (d *Doctor) addResult(name, status, message, fix string) {
	d.results = append(d.results, DiagnosticResult{
		Name:    name,
		Status:  status,
		Message: message,
		Fix:     fix,
	})
}

// PrintResults formats and prints diagnostic results.
func (d *Doctor) PrintResults(w io.Writer) {
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "╔═══════════════════════════════════════════════════════════════╗")
	fmt.Fprintln(w, "║              CLASP Diagnostics                                ║")
	fmt.Fprintln(w, "╚═══════════════════════════════════════════════════════════════╝")
	fmt.Fprintln(w, "")

	okCount := 0
	warningCount := 0
	errorCount := 0

	for _, r := range d.results {
		var icon string
		switch r.Status {
		case "ok":
			icon = "✓"
			okCount++
		case "warning":
			icon = "⚠"
			warningCount++
		case "error":
			icon = "✗"
			errorCount++
		}

		fmt.Fprintf(w, "%s %s: %s\n", icon, r.Name, r.Message)
		if r.Fix != "" && r.Status != "ok" {
			fmt.Fprintf(w, "  └─ Fix: %s\n", r.Fix)
		}
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if errorCount > 0 {
		fmt.Fprintf(w, "Summary: %d error(s), %d warning(s), %d ok\n", errorCount, warningCount, okCount)
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Fix the errors above to ensure CLASP works correctly.")
	} else if warningCount > 0 {
		fmt.Fprintf(w, "Summary: %d warning(s), %d ok\n", warningCount, okCount)
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "CLASP should work, but consider addressing the warnings.")
	} else {
		fmt.Fprintf(w, "Summary: All %d checks passed!\n", okCount)
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "CLASP is ready to use. Run 'clasp' to start.")
	}
	fmt.Fprintln(w, "")
}

// HasErrors returns true if any errors were found.
func (d *Doctor) HasErrors() bool {
	for _, r := range d.results {
		if r.Status == "error" {
			return true
		}
	}
	return false
}

// QuickCheck performs a minimal set of checks for startup validation.
func QuickCheck() (bool, string) {
	// Check if any API key is configured
	envVars := []string{
		"OPENAI_API_KEY",
		"AZURE_API_KEY",
		"OPENROUTER_API_KEY",
		"ANTHROPIC_API_KEY",
		"GEMINI_API_KEY",
		"DEEPSEEK_API_KEY",
	}

	for _, env := range envVars {
		if os.Getenv(env) != "" {
			return true, ""
		}
	}

	// Check config file
	cfg, err := LoadConfig()
	if err == nil && cfg.APIKey != "" {
		return true, ""
	}

	return false, "No API key configured. Run 'clasp -setup' or set an API key environment variable."
}
