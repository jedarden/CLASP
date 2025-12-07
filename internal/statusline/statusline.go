// Package statusline provides Claude Code status line integration for CLASP.
// It writes proxy status to per-port files that can be read by a shell script,
// which Claude Code executes to display the current model and provider info.
// Supports multiple concurrent CLASP instances running on different ports.
package statusline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Status represents the current CLASP proxy status.
type Status struct {
	Running      bool      `json:"running"`
	Port         int       `json:"port"`
	PID          int       `json:"pid"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Requests     int64     `json:"requests"`
	Errors       int64     `json:"errors"`
	CostUSD      float64   `json:"cost_usd"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
	StartTime    time.Time `json:"start_time"`
	LastUpdated  time.Time `json:"last_updated"`
	Version      string    `json:"version"`
	CacheEnabled bool      `json:"cache_enabled"`
	CacheHitRate float64   `json:"cache_hit_rate"`
	Fallback     string    `json:"fallback,omitempty"`
}

// Manager handles status file updates and script installation.
type Manager struct {
	mu         sync.Mutex
	status     Status
	port       int
	statusDir  string
	scriptPath string
}

// NewManager creates a new status line manager.
// Call SetPort() before using to set the port for per-instance status files.
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	claspDir := filepath.Join(homeDir, ".clasp")
	statusDir := filepath.Join(claspDir, "status")
	claudeDir := filepath.Join(homeDir, ".claude")

	// Ensure directories exist
	if err := os.MkdirAll(statusDir, 0755); err != nil {
		return nil, fmt.Errorf("creating .clasp/status directory: %w", err)
	}
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return nil, fmt.Errorf("creating .claude directory: %w", err)
	}

	return &Manager{
		statusDir:  statusDir,
		scriptPath: filepath.Join(claudeDir, "clasp-statusline.sh"),
	}, nil
}

// SetPort sets the port for this manager instance.
// This must be called after the actual port is known (after auto-selection).
func (m *Manager) SetPort(port int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.port = port
	m.status.Port = port
	m.status.PID = os.Getpid()
}

// GetStatusPath returns the path to the status file for the current port.
func (m *Manager) GetStatusPath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.port == 0 {
		// Fallback to legacy path if port not set
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".clasp", "status.json")
	}
	return filepath.Join(m.statusDir, fmt.Sprintf("%d.json", m.port))
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
	s.PID = os.Getpid()
	if m.port != 0 {
		s.Port = m.port
	}
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

// writeStatus writes the current status to the per-port status file.
func (m *Manager) writeStatus() error {
	data, err := json.MarshalIndent(m.status, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling status: %w", err)
	}

	statusPath := filepath.Join(m.statusDir, fmt.Sprintf("%d.json", m.port))
	if m.port == 0 {
		// Fallback to legacy path
		homeDir, _ := os.UserHomeDir()
		statusPath = filepath.Join(homeDir, ".clasp", "status.json")
	}

	if err := os.WriteFile(statusPath, data, 0644); err != nil {
		return fmt.Errorf("writing status file: %w", err)
	}

	return nil
}

// ClearStatus removes the status file (called on shutdown).
func (m *Manager) ClearStatus() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove the per-port status file
	statusPath := filepath.Join(m.statusDir, fmt.Sprintf("%d.json", m.port))
	if m.port == 0 {
		homeDir, _ := os.UserHomeDir()
		statusPath = filepath.Join(homeDir, ".clasp", "status.json")
	}

	return os.Remove(statusPath)
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
// It detects the active CLASP instance from ANTHROPIC_BASE_URL.
func (m *Manager) generateScript() string {
	return `#!/bin/bash
# CLASP Status Line for Claude Code
# This script is executed by Claude Code to display the current model/provider info.
# It reads from ~/.clasp/status/<port>.json based on ANTHROPIC_BASE_URL.

STATUS_DIR="$HOME/.clasp/status"

# Get port from ANTHROPIC_BASE_URL environment variable
if [ -n "$ANTHROPIC_BASE_URL" ]; then
    # Extract port from URL (e.g., http://localhost:8080 → 8080)
    PORT=$(echo "$ANTHROPIC_BASE_URL" | grep -oE ':[0-9]+' | tr -d ':' | head -1)

    if [ -n "$PORT" ]; then
        STATUS_FILE="$STATUS_DIR/${PORT}.json"

        if [ -f "$STATUS_FILE" ]; then
            RUNNING=$(jq -r '.running // false' "$STATUS_FILE" 2>/dev/null)

            if [ "$RUNNING" = "true" ]; then
                # Verify the process is still running
                PID=$(jq -r '.pid // 0' "$STATUS_FILE" 2>/dev/null)
                if [ "$PID" != "0" ] && kill -0 "$PID" 2>/dev/null; then
                    MODEL=$(jq -r '.model // "unknown"' "$STATUS_FILE")
                    PROVIDER=$(jq -r '.provider // "unknown"' "$STATUS_FILE")
                    REQUESTS=$(jq -r '.requests // 0' "$STATUS_FILE")
                    COST=$(jq -r '.cost_usd // 0' "$STATUS_FILE")

                    # Format cost display
                    if [ "$COST" != "null" ] && [ "$COST" != "0" ]; then
                        COST_FMT=$(printf "%.2f" "$COST")
                        COST_STR=" | \$$COST_FMT"
                    else
                        COST_STR=""
                    fi

                    # Output status with ANSI colors
                    echo -e "\033[36m[CLASP:$PORT]\033[0m \033[33m$MODEL\033[0m ($PROVIDER) | $REQUESTS reqs$COST_STR"
                    exit 0
                fi
            fi
        fi
    fi
fi

# Fallback: check legacy status file
LEGACY_STATUS="$HOME/.clasp/status.json"
if [ -f "$LEGACY_STATUS" ]; then
    RUNNING=$(jq -r '.running // false' "$LEGACY_STATUS" 2>/dev/null)

    if [ "$RUNNING" = "true" ]; then
        MODEL=$(jq -r '.model // "unknown"' "$LEGACY_STATUS")
        PROVIDER=$(jq -r '.provider // "unknown"' "$LEGACY_STATUS")
        REQUESTS=$(jq -r '.requests // 0' "$LEGACY_STATUS")
        COST=$(jq -r '.cost_usd // 0' "$LEGACY_STATUS")

        if [ "$COST" != "null" ] && [ "$COST" != "0" ]; then
            COST_FMT=$(printf "%.2f" "$COST")
            COST_STR=" | \$$COST_FMT"
        else
            COST_STR=""
        fi

        echo -e "\033[36m[CLASP]\033[0m \033[33m$MODEL\033[0m ($PROVIDER) | $REQUESTS reqs$COST_STR"
        exit 0
    fi
fi

# No CLASP running, fall back to default
echo "$1" | jq -r '.model.display_name // "Claude"'
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

// InstanceInfo represents a running CLASP instance.
type InstanceInfo struct {
	Port       int       `json:"port"`
	PID        int       `json:"pid"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	Requests   int64     `json:"requests"`
	CostUSD    float64   `json:"cost_usd"`
	StartTime  time.Time `json:"start_time"`
	Uptime     string    `json:"uptime"`
	IsRunning  bool      `json:"is_running"`
	StatusFile string    `json:"status_file"`
}

// ListAllInstances returns information about all CLASP instances.
// It reads all status files and checks if the processes are still running.
func ListAllInstances() ([]InstanceInfo, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	statusDir := filepath.Join(homeDir, ".clasp", "status")

	// Ensure directory exists
	if _, err := os.Stat(statusDir); os.IsNotExist(err) {
		return []InstanceInfo{}, nil
	}

	entries, err := os.ReadDir(statusDir)
	if err != nil {
		return nil, fmt.Errorf("reading status directory: %w", err)
	}

	var instances []InstanceInfo

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		statusPath := filepath.Join(statusDir, entry.Name())
		data, err := os.ReadFile(statusPath)
		if err != nil {
			continue
		}

		var status Status
		if err := json.Unmarshal(data, &status); err != nil {
			continue
		}

		// Check if the process is still running (and not a zombie)
		isRunning := isProcessAlive(status.PID)

		uptime := ""
		if !status.StartTime.IsZero() {
			uptime = time.Since(status.StartTime).Round(time.Second).String()
		}

		instances = append(instances, InstanceInfo{
			Port:       status.Port,
			PID:        status.PID,
			Provider:   status.Provider,
			Model:      status.Model,
			Requests:   status.Requests,
			CostUSD:    status.CostUSD,
			StartTime:  status.StartTime,
			Uptime:     uptime,
			IsRunning:  isRunning,
			StatusFile: statusPath,
		})
	}

	// Also check legacy status file
	legacyPath := filepath.Join(homeDir, ".clasp", "status.json")
	if data, err := os.ReadFile(legacyPath); err == nil {
		var status Status
		if json.Unmarshal(data, &status) == nil && status.Running {
			isRunning := isProcessAlive(status.PID)

			uptime := ""
			if !status.StartTime.IsZero() {
				uptime = time.Since(status.StartTime).Round(time.Second).String()
			}

			instances = append(instances, InstanceInfo{
				Port:       status.Port,
				PID:        status.PID,
				Provider:   status.Provider,
				Model:      status.Model,
				Requests:   status.Requests,
				CostUSD:    status.CostUSD,
				StartTime:  status.StartTime,
				Uptime:     uptime,
				IsRunning:  isRunning,
				StatusFile: legacyPath,
			})
		}
	}

	return instances, nil
}

// CleanupStaleInstances removes status files for processes that are no longer running.
func CleanupStaleInstances() (int, error) {
	instances, err := ListAllInstances()
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, inst := range instances {
		if !inst.IsRunning {
			if err := os.Remove(inst.StatusFile); err == nil {
				cleaned++
			}
		}
	}

	return cleaned, nil
}

// isProcessAlive checks if a process is running and not a zombie.
// On Linux, zombie processes have state 'Z' in /proc/[pid]/stat.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	// First check if process exists via signal 0
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}

	// On Linux, check /proc/[pid]/stat for zombie state
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		// If we can't read /proc, fall back to just the signal check (non-Linux)
		return true
	}

	// /proc/[pid]/stat format: pid (comm) state ...
	// The state is the third field after the command name in parentheses
	content := string(data)
	// Find the closing parenthesis of the command name
	closeParenIdx := strings.LastIndex(content, ")")
	if closeParenIdx == -1 || closeParenIdx+2 >= len(content) {
		return true // Can't parse, assume alive
	}

	// State is the first field after ") "
	remainder := strings.TrimSpace(content[closeParenIdx+1:])
	fields := strings.Fields(remainder)
	if len(fields) == 0 {
		return true // Can't parse, assume alive
	}

	state := fields[0]
	// 'Z' means zombie, 'X' means dead
	return state != "Z" && state != "X"
}

// ReadStatusFromFile reads the status from the file (for external use like CLI).
// If port is 0, it reads from the legacy status file.
func ReadStatusFromFile() (*Status, error) {
	return ReadStatusFromPort(0)
}

// ReadStatusFromPort reads the status for a specific port.
func ReadStatusFromPort(port int) (*Status, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	var statusPath string
	if port == 0 {
		// Try to find the first running instance
		instances, err := ListAllInstances()
		if err != nil {
			return nil, err
		}
		for _, inst := range instances {
			if inst.IsRunning {
				statusPath = inst.StatusFile
				break
			}
		}
		if statusPath == "" {
			// Fall back to legacy path
			statusPath = filepath.Join(homeDir, ".clasp", "status.json")
		}
	} else {
		statusPath = filepath.Join(homeDir, ".clasp", "status", fmt.Sprintf("%d.json", port))
	}

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
		return fmt.Sprintf("[CLASP:%d] %s (%s) | PID %d | %d requests | %d errors | Uptime: %s%s%s",
			s.Port, s.Model, s.Provider, s.PID, s.Requests, s.Errors, uptime, costStr, cacheStr)
	}

	costStr := ""
	if s.CostUSD > 0 {
		costStr = fmt.Sprintf(" | $%.2f", s.CostUSD)
	}
	return fmt.Sprintf("[CLASP:%d] %s (%s) | %d reqs%s", s.Port, s.Model, s.Provider, s.Requests, costStr)
}

// FormatAllInstancesTable formats all instances as a table for CLI output.
func FormatAllInstancesTable(instances []InstanceInfo) string {
	if len(instances) == 0 {
		return "No CLASP instances found."
	}

	// Filter to only running instances
	var running []InstanceInfo
	for _, inst := range instances {
		if inst.IsRunning {
			running = append(running, inst)
		}
	}

	if len(running) == 0 {
		return "No running CLASP instances found."
	}

	// Header
	output := "\nCLASP Instances\n"
	output += strings.Repeat("─", 70) + "\n"
	output += fmt.Sprintf("%-7s %-12s %-20s %-10s %-10s %-10s\n",
		"Port", "Provider", "Model", "Requests", "Cost", "Uptime")
	output += strings.Repeat("─", 70) + "\n"

	for _, inst := range running {
		model := inst.Model
		if len(model) > 18 {
			model = model[:15] + "..."
		}
		costStr := fmt.Sprintf("$%.2f", inst.CostUSD)
		output += fmt.Sprintf("%-7d %-12s %-20s %-10d %-10s %-10s\n",
			inst.Port, inst.Provider, model, inst.Requests, costStr, inst.Uptime)
	}

	return output
}
