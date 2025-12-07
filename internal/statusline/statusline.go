// Package statusline provides Claude Code status line integration for CLASP.
// It writes proxy status to a file that can be read by a shell script,
// which Claude Code executes to display the current model and provider info.
package statusline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Status represents the current CLASP proxy status.
type Status struct {
	Running       bool      `json:"running"`
	Port          int       `json:"port"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	Requests      int64     `json:"requests"`
	Errors        int64     `json:"errors"`
	CostUSD       float64   `json:"cost_usd"`
	AvgLatencyMs  float64   `json:"avg_latency_ms"`
	StartTime     time.Time `json:"start_time"`
	LastUpdated   time.Time `json:"last_updated"`
	Version       string    `json:"version"`
	CacheEnabled  bool      `json:"cache_enabled"`
	CacheHitRate  float64   `json:"cache_hit_rate"`
	Fallback      string    `json:"fallback,omitempty"`
}

// Manager handles status file updates and script installation.
type Manager struct {
	mu         sync.Mutex
	status     Status
	statusPath string
	scriptPath string
}

// NewManager creates a new status line manager.
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	claspDir := filepath.Join(homeDir, ".clasp")
	claudeDir := filepath.Join(homeDir, ".claude")

	// Ensure directories exist
	if err := os.MkdirAll(claspDir, 0755); err != nil {
		return nil, fmt.Errorf("creating .clasp directory: %w", err)
	}
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return nil, fmt.Errorf("creating .claude directory: %w", err)
	}

	return &Manager{
		statusPath: filepath.Join(claspDir, "status.json"),
		scriptPath: filepath.Join(claudeDir, "clasp-statusline.sh"),
	}, nil
}

// GetStatusPath returns the path to the status file.
func (m *Manager) GetStatusPath() string {
	return m.statusPath
}

// GetScriptPath returns the path to the status line script.
func (m *Manager) GetScriptPath() string {
	return m.scriptPath
}

// UpdateStatus updates the current proxy status.
func (m *Manager) UpdateStatus(s Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s.LastUpdated = time.Now()
	m.status = s

	return m.writeStatus()
}

// UpdateRunning marks the proxy as running or stopped.
func (m *Manager) UpdateRunning(running bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.Running = running
	m.status.LastUpdated = time.Now()

	return m.writeStatus()
}

// UpdateMetrics updates the metrics portion of the status.
func (m *Manager) UpdateMetrics(requests, errors int64, costUSD, avgLatencyMs float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.Requests = requests
	m.status.Errors = errors
	m.status.CostUSD = costUSD
	m.status.AvgLatencyMs = avgLatencyMs
	m.status.LastUpdated = time.Now()

	return m.writeStatus()
}

// UpdateCacheStats updates cache statistics.
func (m *Manager) UpdateCacheStats(enabled bool, hitRate float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.CacheEnabled = enabled
	m.status.CacheHitRate = hitRate
	m.status.LastUpdated = time.Now()

	return m.writeStatus()
}

// SetModel updates the current model.
func (m *Manager) SetModel(model string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.Model = model
	m.status.LastUpdated = time.Now()

	return m.writeStatus()
}

// writeStatus writes the current status to the status file.
func (m *Manager) writeStatus() error {
	data, err := json.MarshalIndent(m.status, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling status: %w", err)
	}

	if err := os.WriteFile(m.statusPath, data, 0644); err != nil {
		return fmt.Errorf("writing status file: %w", err)
	}

	return nil
}

// ClearStatus removes the status file (called on shutdown).
func (m *Manager) ClearStatus() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status.Running = false
	m.status.LastUpdated = time.Now()

	// Write final stopped status
	if err := m.writeStatus(); err != nil {
		return err
	}

	return nil
}

// InstallScript installs the status line shell script for Claude Code.
func (m *Manager) InstallScript() error {
	script := m.generateScript()

	if err := os.WriteFile(m.scriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("writing status line script: %w", err)
	}

	return nil
}

// generateScript creates the shell script that Claude Code will execute.
func (m *Manager) generateScript() string {
	return `#!/bin/bash
# CLASP Status Line for Claude Code
# This script is executed by Claude Code to display the current model/provider info.
# It reads from ~/.clasp/status.json which is updated by the CLASP proxy.

CLASP_STATUS="$HOME/.clasp/status.json"

# Check if CLASP is running
if [ -f "$CLASP_STATUS" ]; then
    RUNNING=$(jq -r '.running // false' "$CLASP_STATUS" 2>/dev/null)

    if [ "$RUNNING" = "true" ]; then
        MODEL=$(jq -r '.model // "unknown"' "$CLASP_STATUS")
        PROVIDER=$(jq -r '.provider // "unknown"' "$CLASP_STATUS")
        REQUESTS=$(jq -r '.requests // 0' "$CLASP_STATUS")
        COST=$(jq -r '.cost_usd // 0' "$CLASP_STATUS")

        # Format cost display
        if [ "$COST" != "null" ] && [ "$COST" != "0" ]; then
            # Format to 2 decimal places
            COST_FMT=$(printf "%.2f" "$COST")
            COST_STR=" | \$$COST_FMT"
        else
            COST_STR=""
        fi

        # Output the status line with ANSI colors
        # Cyan for [CLASP], Yellow for model, default for stats
        echo -e "\033[36m[CLASP]\033[0m \033[33m$MODEL\033[0m ($PROVIDER) | $REQUESTS reqs$COST_STR"
    else
        # CLASP proxy is stopped, fall back to default
        echo "$1" | jq -r '.model.display_name // "Claude"'
    fi
else
    # No status file, fall back to default
    echo "$1" | jq -r '.model.display_name // "Claude"'
fi
`
}

// ConfigureClaudeCode updates Claude Code's settings.json to use the CLASP status line.
func (m *Manager) ConfigureClaudeCode() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")

	// Read existing settings or create new
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			settings = make(map[string]interface{})
		} else {
			return fmt.Errorf("reading settings.json: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			// If settings is malformed, start fresh
			settings = make(map[string]interface{})
		}
	}

	// Add or update statusLine configuration
	settings["statusLine"] = map[string]interface{}{
		"type":    "command",
		"command": m.scriptPath,
		"padding": 0,
	}

	// Write updated settings
	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, output, 0644); err != nil {
		return fmt.Errorf("writing settings.json: %w", err)
	}

	return nil
}

// Setup performs full status line installation and configuration.
func (m *Manager) Setup() error {
	// Install the script
	if err := m.InstallScript(); err != nil {
		return fmt.Errorf("installing status line script: %w", err)
	}

	// Configure Claude Code to use it
	if err := m.ConfigureClaudeCode(); err != nil {
		return fmt.Errorf("configuring Claude Code: %w", err)
	}

	return nil
}

// IsConfigured checks if the status line is already configured.
func (m *Manager) IsConfigured() bool {
	// Check if script exists
	if _, err := os.Stat(m.scriptPath); os.IsNotExist(err) {
		return false
	}

	// Check if Claude Code settings reference it
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}

	statusLine, ok := settings["statusLine"].(map[string]interface{})
	if !ok {
		return false
	}

	command, ok := statusLine["command"].(string)
	return ok && command == m.scriptPath
}

// GetStatus returns the current status.
func (m *Manager) GetStatus() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// ReadStatusFromFile reads the status from the file (for external use like CLI).
func ReadStatusFromFile() (*Status, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	statusPath := filepath.Join(homeDir, ".clasp", "status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No status file means not running
		}
		return nil, fmt.Errorf("reading status file: %w", err)
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("parsing status file: %w", err)
	}

	return &status, nil
}

// FormatStatusLine returns a formatted status line string for terminal display.
func FormatStatusLine(s *Status, verbose bool) string {
	if s == nil || !s.Running {
		return "CLASP proxy is not running"
	}

	if verbose {
		uptime := time.Since(s.StartTime).Round(time.Second)
		costStr := ""
		if s.CostUSD > 0 {
			costStr = fmt.Sprintf(" | Cost: $%.4f", s.CostUSD)
		}
		cacheStr := ""
		if s.CacheEnabled {
			cacheStr = fmt.Sprintf(" | Cache: %.1f%% hit rate", s.CacheHitRate*100)
		}
		return fmt.Sprintf("[CLASP] %s (%s) | Port %d | %d requests | %d errors | Uptime: %s%s%s",
			s.Model, s.Provider, s.Port, s.Requests, s.Errors, uptime, costStr, cacheStr)
	}

	costStr := ""
	if s.CostUSD > 0 {
		costStr = fmt.Sprintf(" | $%.2f", s.CostUSD)
	}
	return fmt.Sprintf("[CLASP] %s (%s) | %d reqs%s", s.Model, s.Provider, s.Requests, costStr)
}
