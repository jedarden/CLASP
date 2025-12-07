// Package claudecode manages Claude Code CLI installation and launching.
package claudecode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Manager handles Claude Code CLI installation and launching.
type Manager struct {
	cacheDir    string
	proxyURL    string
	verbose     bool
	installOpts InstallOptions
}

// InstallOptions configures the installation behavior.
type InstallOptions struct {
	ForceUpdate bool
	SkipCheck   bool
}

// InstallStatus represents the current Claude Code installation state.
type InstallStatus struct {
	Installed       bool   `json:"installed"`
	Version         string `json:"version"`
	Path            string `json:"path"`
	NeedsUpdate     bool   `json:"needs_update"`
	LatestVersion   string `json:"latest_version,omitempty"`
	LastChecked     int64  `json:"last_checked"`
	InstallMethod   string `json:"install_method"` // npm, npx, binary
}

// NewManager creates a new Claude Code manager.
func NewManager(proxyURL string, verbose bool) *Manager {
	cacheDir := filepath.Join(os.Getenv("HOME"), ".clasp", "cache")
	os.MkdirAll(cacheDir, 0755)

	return &Manager{
		cacheDir: cacheDir,
		proxyURL: proxyURL,
		verbose:  verbose,
	}
}

// SetInstallOptions sets installation options.
func (m *Manager) SetInstallOptions(opts InstallOptions) {
	m.installOpts = opts
}

// CheckInstallation checks if Claude Code is installed and its version.
// Priority: 1) Bundled in node_modules 2) Global npm 3) npx fallback
func (m *Manager) CheckInstallation() (*InstallStatus, error) {
	status := &InstallStatus{
		LastChecked: time.Now().Unix(),
	}

	// First, check for Claude Code bundled with clasp-ai (in node_modules)
	bundledPath := m.findBundledClaude()
	if bundledPath != "" {
		status.Installed = true
		status.Path = bundledPath
		status.InstallMethod = "bundled"

		// Get version
		version, err := m.getClaudeVersion(bundledPath)
		if err == nil {
			status.Version = version
		}

		if m.verbose {
			fmt.Printf("[CLASP] Using bundled Claude Code at %s (version %s)\n", bundledPath, status.Version)
		}

		return status, nil
	}

	// Check for global claude command
	claudePath, err := exec.LookPath("claude")
	if err == nil {
		status.Installed = true
		status.Path = claudePath
		status.InstallMethod = "npm"

		// Get version
		version, err := m.getClaudeVersion(claudePath)
		if err == nil {
			status.Version = version
		}

		if m.verbose {
			fmt.Printf("[CLASP] Found Claude Code at %s (version %s)\n", claudePath, status.Version)
		}

		return status, nil
	}

	// Check if we can use npx
	npxPath, err := exec.LookPath("npx")
	if err == nil {
		// Try npx @anthropic-ai/claude-code
		cmd := exec.Command(npxPath, "-y", "@anthropic-ai/claude-code", "--version")
		output, err := cmd.Output()
		if err == nil {
			status.Installed = true
			status.InstallMethod = "npx"
			status.Version = strings.TrimSpace(string(output))

			if m.verbose {
				fmt.Printf("[CLASP] Claude Code available via npx (version %s)\n", status.Version)
			}

			return status, nil
		}
	}

	// Not installed
	status.Installed = false
	if m.verbose {
		fmt.Println("[CLASP] Claude Code not found")
	}

	return status, nil
}

// findBundledClaude looks for Claude Code in the clasp-ai package's node_modules.
func (m *Manager) findBundledClaude() string {
	// Get the path to the current executable
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}

	// Resolve any symlinks
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return ""
	}

	// Get the directory containing the executable
	exeDir := filepath.Dir(exePath)

	// Look for node_modules/.bin/claude relative to package root
	// The executable is in bin/, so package root is one level up
	packageRoot := filepath.Dir(exeDir)

	// Check multiple possible locations
	possiblePaths := []string{
		// When installed via npm (clasp-ai/node_modules/.bin/claude)
		filepath.Join(packageRoot, "node_modules", ".bin", "claude"),
		// When installed globally (../node_modules/@anthropic-ai/claude-code/...)
		filepath.Join(packageRoot, "..", "@anthropic-ai", "claude-code", "cli.js"),
		// In global node_modules
		filepath.Join(filepath.Dir(packageRoot), "@anthropic-ai", "claude-code", "cli.js"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// getClaudeVersion retrieves the version of Claude Code.
func (m *Manager) getClaudeVersion(claudePath string) (string, error) {
	cmd := exec.Command(claudePath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse version from output (e.g., "claude 1.0.0" or "1.0.0")
	version := strings.TrimSpace(string(output))

	// Extract version number using regex
	re := regexp.MustCompile(`\d+\.\d+\.\d+`)
	match := re.FindString(version)
	if match != "" {
		return match, nil
	}

	return version, nil
}

// Install installs Claude Code using npm.
func (m *Manager) Install() error {
	fmt.Println("[CLASP] Installing Claude Code...")

	// Check for npm
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm not found. Please install Node.js first: https://nodejs.org")
	}

	// Install globally
	cmd := exec.Command(npmPath, "install", "-g", "@anthropic-ai/claude-code")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Claude Code: %w", err)
	}

	fmt.Println("[CLASP] Claude Code installed successfully!")
	return nil
}

// Update updates Claude Code to the latest version.
func (m *Manager) Update() error {
	fmt.Println("[CLASP] Updating Claude Code...")

	npmPath, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm not found")
	}

	cmd := exec.Command(npmPath, "update", "-g", "@anthropic-ai/claude-code")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update Claude Code: %w", err)
	}

	fmt.Println("[CLASP] Claude Code updated successfully!")
	return nil
}

// GetLatestVersion fetches the latest version from npm.
func (m *Manager) GetLatestVersion() (string, error) {
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		return "", fmt.Errorf("npm not found")
	}

	cmd := exec.Command(npmPath, "view", "@anthropic-ai/claude-code", "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest version: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// EnsureInstalled checks if Claude Code is installed and installs it if needed.
func (m *Manager) EnsureInstalled() (*InstallStatus, error) {
	// Check current installation status
	status, err := m.CheckInstallation()
	if err != nil {
		return nil, err
	}

	// If not installed, install it
	if !status.Installed {
		if err := m.Install(); err != nil {
			return nil, err
		}

		// Re-check installation
		status, err = m.CheckInstallation()
		if err != nil {
			return nil, err
		}
	}

	// Check for updates if force update is requested
	if m.installOpts.ForceUpdate && status.Installed {
		if err := m.Update(); err != nil {
			fmt.Printf("[CLASP] Warning: Failed to update Claude Code: %v\n", err)
			// Continue with existing installation
		}

		// Re-check installation
		status, err = m.CheckInstallation()
		if err != nil {
			return nil, err
		}
	}

	return status, nil
}

// LaunchOptions configures how Claude Code is launched.
type LaunchOptions struct {
	WorkingDir      string
	Args            []string
	ProxyURL        string
	APIKey          string // Optional: pre-configured API key for the proxy
	Interactive     bool
	PassthroughEnv  bool // Pass through all environment variables
	SkipPermissions bool // Use --dangerously-skip-permissions flag
}

// Launch starts Claude Code with the CLASP proxy configuration.
func (m *Manager) Launch(opts LaunchOptions) error {
	// Ensure Claude Code is installed
	status, err := m.EnsureInstalled()
	if err != nil {
		return fmt.Errorf("failed to ensure Claude Code installation: %w", err)
	}

	if !status.Installed {
		return fmt.Errorf("Claude Code installation failed")
	}

	// Build the arguments list, prepending --dangerously-skip-permissions if requested
	args := opts.Args
	if opts.SkipPermissions {
		args = append([]string{"--dangerously-skip-permissions"}, args...)
	}

	// Determine the command to run
	var cmd *exec.Cmd

	switch status.InstallMethod {
	case "bundled":
		// Use bundled Claude Code - check if it's a JS file or binary
		if strings.HasSuffix(status.Path, ".js") {
			// Run with node
			nodeArgs := append([]string{status.Path}, args...)
			cmd = exec.Command("node", nodeArgs...)
		} else {
			// Direct binary or symlink
			cmd = exec.Command(status.Path, args...)
		}
	case "npx":
		// Use npx to run Claude Code
		npxArgs := append([]string{"-y", "@anthropic-ai/claude-code"}, args...)
		cmd = exec.Command("npx", npxArgs...)
	default:
		// Use installed claude command
		cmd = exec.Command(status.Path, args...)
	}

	// Set working directory
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	// Set up environment
	env := os.Environ()

	// Configure ANTHROPIC_BASE_URL to point to CLASP proxy
	proxyURL := opts.ProxyURL
	if proxyURL == "" {
		proxyURL = m.proxyURL
	}
	if proxyURL != "" {
		env = setEnvVar(env, "ANTHROPIC_BASE_URL", proxyURL)
		if m.verbose {
			fmt.Printf("[CLASP] Setting ANTHROPIC_BASE_URL=%s\n", proxyURL)
		}
	}

	// Set API key if provided (for authenticated proxy)
	if opts.APIKey != "" {
		env = setEnvVar(env, "ANTHROPIC_API_KEY", opts.APIKey)
	}

	cmd.Env = env

	// Connect stdio for interactive mode
	if opts.Interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if m.verbose {
		fmt.Printf("[CLASP] Launching Claude Code (version %s)...\n", status.Version)
	}

	// Clear terminal before launching Claude Code for clean TUI
	clearTerminal()

	// Run the command
	return cmd.Run()
}

// clearTerminal clears the terminal screen before launching Claude Code.
// This provides a clean slate for Claude Code's TUI interface.
func clearTerminal() {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	default:
		// ANSI escape sequences work on macOS, Linux, and most terminals
		// \033[H - Move cursor to home position (0,0)
		// \033[2J - Clear entire screen
		// \033[3J - Clear scrollback buffer (for complete clean slate)
		fmt.Print("\033[H\033[2J\033[3J")
	}
}

// LaunchWithProxy starts CLASP proxy and then launches Claude Code.
func (m *Manager) LaunchWithProxy(proxyPort int, opts LaunchOptions) error {
	// Set the proxy URL
	opts.ProxyURL = fmt.Sprintf("http://localhost:%d", proxyPort)

	// Launch Claude Code
	return m.Launch(opts)
}

// setEnvVar sets or updates an environment variable in the env slice.
func setEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = key + "=" + value
			return env
		}
	}
	return append(env, key+"="+value)
}

// CacheStatus saves installation status to cache.
func (m *Manager) CacheStatus(status *InstallStatus) error {
	cachePath := filepath.Join(m.cacheDir, "claude_status.json")

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// LoadCachedStatus loads installation status from cache.
func (m *Manager) LoadCachedStatus() (*InstallStatus, error) {
	cachePath := filepath.Join(m.cacheDir, "claude_status.json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var status InstallStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}

	// Check if cache is stale (older than 24 hours)
	if time.Since(time.Unix(status.LastChecked, 0)) > 24*time.Hour {
		return nil, fmt.Errorf("cache stale")
	}

	return &status, nil
}

// CheckForUpdates checks if a newer version of Claude Code is available.
func (m *Manager) CheckForUpdates(currentVersion string) (bool, string, error) {
	latestVersion, err := m.GetLatestVersion()
	if err != nil {
		return false, "", err
	}

	if latestVersion != currentVersion {
		return true, latestVersion, nil
	}

	return false, latestVersion, nil
}

// ProxyLaunchConfig holds configuration for launching with the proxy.
type ProxyLaunchConfig struct {
	Port             int
	Provider         string
	Model            string
	AutoInstall      bool
	ForceUpdate      bool
	Verbose          bool
	ClaudeArgs       []string
	WorkingDir       string
	BackgroundProxy  bool // Run proxy in background
}

// RunWithClaudeCode starts the CLASP proxy and launches Claude Code in one operation.
// This is the main entry point for the "clasp" command when used to launch Claude.
func RunWithClaudeCode(cfg ProxyLaunchConfig) error {
	proxyURL := fmt.Sprintf("http://localhost:%d", cfg.Port)

	manager := NewManager(proxyURL, cfg.Verbose)
	manager.SetInstallOptions(InstallOptions{
		ForceUpdate: cfg.ForceUpdate,
	})

	// Check/install Claude Code
	fmt.Println("")
	fmt.Println("[CLASP] Checking Claude Code installation...")

	status, err := manager.EnsureInstalled()
	if err != nil {
		return fmt.Errorf("failed to set up Claude Code: %w", err)
	}

	fmt.Printf("[CLASP] Claude Code %s ready\n", status.Version)

	// Cache the status
	manager.CacheStatus(status)

	// Check for updates (non-blocking, just informational)
	go func() {
		needsUpdate, latestVersion, err := manager.CheckForUpdates(status.Version)
		if err == nil && needsUpdate {
			fmt.Printf("\n[CLASP] Note: Claude Code %s is available (current: %s). Run 'clasp -update-claude' to update.\n", latestVersion, status.Version)
		}
	}()

	fmt.Printf("[CLASP] Starting proxy on %s...\n", proxyURL)
	fmt.Println("[CLASP] Launching Claude Code...")
	fmt.Println("")

	// Launch Claude Code
	launchOpts := LaunchOptions{
		WorkingDir:  cfg.WorkingDir,
		Args:        cfg.ClaudeArgs,
		ProxyURL:    proxyURL,
		Interactive: true,
	}

	return manager.Launch(launchOpts)
}

// ParseClaudeArgs separates CLASP args from Claude Code args.
// Everything after "--" is passed to Claude Code.
func ParseClaudeArgs(args []string) (claspArgs, claudeArgs []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

// SpawnProxyBackground starts the proxy in a background process.
func SpawnProxyBackground(port int, provider, model string) (*exec.Cmd, error) {
	// Get the path to the current executable
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	// Build arguments for proxy-only mode
	args := []string{
		"-proxy-only",
		"-port", fmt.Sprintf("%d", port),
	}
	if provider != "" {
		args = append(args, "-provider", provider)
	}
	if model != "" {
		args = append(args, "-model", model)
	}

	cmd := exec.Command(exePath, args...)

	// Detach from parent process
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start proxy: %w", err)
	}

	// Wait briefly to ensure proxy started
	time.Sleep(500 * time.Millisecond)

	return cmd, nil
}
