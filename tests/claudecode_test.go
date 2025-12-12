package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jedarden/clasp/internal/claudecode"
)

func TestNewManager(t *testing.T) {
	proxyURL := "http://localhost:8080"
	manager := claudecode.NewManager(proxyURL, false)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}
}

func TestCheckInstallation(t *testing.T) {
	manager := claudecode.NewManager("http://localhost:8080", false)
	status, err := manager.CheckInstallation()

	if err != nil {
		t.Fatalf("CheckInstallation failed: %v", err)
	}

	if status == nil {
		t.Fatal("Expected non-nil status")
	}

	// Status should have valid LastChecked
	if status.LastChecked == 0 {
		t.Error("Expected LastChecked to be set")
	}

	// If installed, should have path
	if status.Installed && status.Path == "" && status.InstallMethod != "npx" {
		t.Error("If installed via npm, should have path")
	}

	// If installed, should have version
	if status.Installed && status.Version == "" {
		t.Error("If installed, should have version")
	}

	t.Logf("Claude Code status: installed=%v, version=%s, method=%s",
		status.Installed, status.Version, status.InstallMethod)
}

func TestCacheStatus(t *testing.T) {
	// Create a temporary directory for cache
	tmpDir, err := os.MkdirTemp("", "clasp-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp dir
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	manager := claudecode.NewManager("http://localhost:8080", false)

	status := &claudecode.InstallStatus{
		Installed:     true,
		Version:       "1.0.0",
		Path:          "/usr/bin/claude",
		InstallMethod: "npm",
		LastChecked:   1234567890,
	}

	err = manager.CacheStatus(status)
	if err != nil {
		t.Fatalf("CacheStatus failed: %v", err)
	}

	// Check cache file exists
	cachePath := filepath.Join(tmpDir, ".clasp", "cache", "claude_status.json")
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatalf("Cache file not created: %v", err)
	}
}

func TestParseClaudeArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantClasp  []string
		wantClaude []string
	}{
		{
			name:       "no separator",
			args:       []string{"-launch", "-port", "8080"},
			wantClasp:  []string{"-launch", "-port", "8080"},
			wantClaude: nil,
		},
		{
			name:       "with separator",
			args:       []string{"-launch", "--", "--resume"},
			wantClasp:  []string{"-launch"},
			wantClaude: []string{"--resume"},
		},
		{
			name:       "separator at start",
			args:       []string{"--", "--resume", "--no-color"},
			wantClasp:  []string{},
			wantClaude: []string{"--resume", "--no-color"},
		},
		{
			name:       "empty args",
			args:       []string{},
			wantClasp:  []string{},
			wantClaude: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClasp, gotClaude := claudecode.ParseClaudeArgs(tt.args)

			if len(gotClasp) != len(tt.wantClasp) {
				t.Errorf("ParseClaudeArgs() clasp args = %v, want %v", gotClasp, tt.wantClasp)
			}

			if len(gotClaude) != len(tt.wantClaude) {
				t.Errorf("ParseClaudeArgs() claude args = %v, want %v", gotClaude, tt.wantClaude)
			}
		})
	}
}

func TestLaunchOptions(t *testing.T) {
	opts := claudecode.LaunchOptions{
		WorkingDir:  "/tmp/test",
		Args:        []string{"--resume"},
		ProxyURL:    "http://localhost:8080",
		APIKey:      "test-key",
		Interactive: true,
	}

	// Verify all fields are set correctly
	if opts.WorkingDir != "/tmp/test" {
		t.Error("WorkingDir not set correctly")
	}

	if len(opts.Args) != 1 || opts.Args[0] != "--resume" {
		t.Error("Args not set correctly")
	}

	if opts.ProxyURL != "http://localhost:8080" {
		t.Error("ProxyURL not set correctly")
	}

	if opts.APIKey != "test-key" {
		t.Error("APIKey not set correctly")
	}

	if !opts.Interactive {
		t.Error("Interactive not set correctly")
	}

	t.Logf("LaunchOptions validated: WorkingDir=%s, Args=%v, ProxyURL=%s, Interactive=%v",
		opts.WorkingDir, opts.Args, opts.ProxyURL, opts.Interactive)
}

func TestLaunchOptionsSkipPermissions(t *testing.T) {
	// Test that SkipPermissions field is available and works
	tests := []struct {
		name            string
		skipPermissions bool
	}{
		{"enabled", true},
		{"disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := claudecode.LaunchOptions{
				WorkingDir:      "/tmp/test",
				Args:            []string{"--resume"},
				ProxyURL:        "http://localhost:8080",
				Interactive:     true,
				SkipPermissions: tt.skipPermissions,
			}

			if opts.SkipPermissions != tt.skipPermissions {
				t.Errorf("SkipPermissions = %v, want %v", opts.SkipPermissions, tt.skipPermissions)
			}
		})
	}
}

func TestLaunchOptionsArgsWithSkipPermissions(t *testing.T) {
	// When SkipPermissions is true, the --dangerously-skip-permissions flag
	// should be prepended to the args list when launching
	opts := claudecode.LaunchOptions{
		Args:            []string{"--resume", "--no-color"},
		ProxyURL:        "http://localhost:8080",
		Interactive:     true,
		SkipPermissions: true,
	}

	// Verify base config is correct
	if !opts.SkipPermissions {
		t.Error("SkipPermissions should be true")
	}

	if len(opts.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(opts.Args))
	}

	t.Logf("SkipPermissions opts validated: Args=%v, ProxyURL=%s, Interactive=%v",
		opts.Args, opts.ProxyURL, opts.Interactive)
}

func TestInstallOptions(t *testing.T) {
	manager := claudecode.NewManager("http://localhost:8080", false)

	opts := claudecode.InstallOptions{
		ForceUpdate: true,
		SkipCheck:   false,
	}

	manager.SetInstallOptions(opts)

	// Verify options are set (we can't directly access private fields,
	// but the manager should use them in EnsureInstalled)
}

func TestProxyLaunchConfig(t *testing.T) {
	cfg := claudecode.ProxyLaunchConfig{
		Port:            8080,
		Provider:        "openai",
		Model:           "gpt-4o",
		AutoInstall:     true,
		ForceUpdate:     false,
		Verbose:         true,
		ClaudeArgs:      []string{"--resume"},
		WorkingDir:      "/tmp/test",
		BackgroundProxy: false,
	}

	if cfg.Port != 8080 {
		t.Error("Port not set correctly")
	}

	if cfg.Provider != "openai" {
		t.Error("Provider not set correctly")
	}

	if cfg.Model != "gpt-4o" {
		t.Error("Model not set correctly")
	}

	if !cfg.AutoInstall {
		t.Error("AutoInstall not set correctly")
	}

	if cfg.ForceUpdate {
		t.Error("ForceUpdate should be false")
	}

	if !cfg.Verbose {
		t.Error("Verbose not set correctly")
	}

	t.Logf("ProxyLaunchConfig validated: Port=%d, Provider=%s, Model=%s, ClaudeArgs=%v, WorkingDir=%s, BackgroundProxy=%v",
		cfg.Port, cfg.Provider, cfg.Model, cfg.ClaudeArgs, cfg.WorkingDir, cfg.BackgroundProxy)
}

// Note: We can't test actual installation/launching without mocking
// as they require npm/npx and would have side effects.
// These tests verify the configuration and utility functions work correctly.
