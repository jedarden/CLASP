// Package logging provides log management for CLASP, including file-based logging
// when running in Claude Code mode to prevent TUI corruption.
// Supports multiple concurrent CLASP instances with session-specific log files.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logFile       *os.File
	logFilePath   string
	debugFile     *os.File
	debugFilePath string
	debugLogger   *log.Logger
	mu            sync.Mutex
	isFileMode    bool
	debugEnabled  bool
	sessionID     string // Unique session identifier for this CLASP instance
	sessionPort   int    // Port for this CLASP instance (used for log file naming)
)

// GenerateSessionID creates a unique session identifier using PID and timestamp.
// Format: <pid>-<timestamp> (e.g., "12345-20251208153045")
func GenerateSessionID() string {
	return fmt.Sprintf("%d-%s", os.Getpid(), time.Now().Format("20060102150405"))
}

// SetSessionPort sets the port for this CLASP instance.
// This should be called after the port is determined to create session-specific logs.
func SetSessionPort(port int) {
	mu.Lock()
	defer mu.Unlock()
	sessionPort = port

	// If logging is already configured, recreate log files with port-based names
	if isFileMode && logFile != nil {
		// Close current log file
		logFile.Close()
		logFile = nil

		// Open new port-specific log file
		newLogPath := GetLogPathForPort(port)
		f, err := os.OpenFile(newLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			logFile = f
			logFilePath = newLogPath
			log.SetOutput(f)
			log.Printf("[CLASP] [session:%s] Switched to port-specific log: %s", sessionID, newLogPath)
		}
	}

	// If debug logging is enabled, recreate debug log file with port-based name
	if debugEnabled && debugFile != nil {
		// Close current debug log file
		debugFile.Close()
		debugFile = nil

		// Open new port-specific debug log file
		newDebugPath := GetDebugLogPathForPort(port)
		f, err := os.OpenFile(newDebugPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			debugFile = f
			debugFilePath = newDebugPath
			debugLogger = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
			debugLogger.Printf("[session:%s] Switched to port-specific debug log: %s", sessionID, newDebugPath)
		}
	}
}

// GetSessionID returns the current session identifier.
func GetSessionID() string {
	mu.Lock()
	defer mu.Unlock()
	return sessionID
}

// GetLogPath returns the path to the CLASP log file.
// Returns a port-specific path if a port has been set, otherwise returns the default path.
func GetLogPath() string {
	mu.Lock()
	port := sessionPort
	mu.Unlock()

	if port > 0 {
		return GetLogPathForPort(port)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clasp", "logs", "clasp.log")
}

// GetLogPathForPort returns the path to the CLASP log file for a specific port.
func GetLogPathForPort(port int) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clasp", "logs", fmt.Sprintf("clasp-%d.log", port))
}

// GetDebugLogPath returns the path to the CLASP debug log file.
// Returns a port-specific path if a port has been set, otherwise returns the default path.
func GetDebugLogPath() string {
	mu.Lock()
	port := sessionPort
	mu.Unlock()

	if port > 0 {
		return GetDebugLogPathForPort(port)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clasp", "logs", "debug.log")
}

// GetDebugLogPathForPort returns the path to the CLASP debug log file for a specific port.
func GetDebugLogPathForPort(port int) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clasp", "logs", fmt.Sprintf("debug-%d.log", port))
}

// ConfigureForClaudeCode configures logging for Claude Code mode.
// All log output is redirected to a file to prevent TUI corruption.
// Generates a unique session ID for this CLASP instance.
func ConfigureForClaudeCode() error {
	mu.Lock()
	defer mu.Unlock()

	// Generate session ID for this instance
	sessionID = GenerateSessionID()

	// Use default log path initially (will switch to port-specific when SetSessionPort is called)
	home, _ := os.UserHomeDir()
	logFilePath = filepath.Join(home, ".clasp", "logs", "clasp.log")
	logDir := filepath.Dir(logFilePath)

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Rotate log if it's too large (>10MB)
	if info, err := os.Stat(logFilePath); err == nil {
		if info.Size() > 10*1024*1024 {
			rotateLog()
		}
	}

	// Open log file for appending
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	logFile = f
	isFileMode = true

	// Redirect standard logger to file with session-aware prefix
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Log session start with session ID
	log.Printf("[CLASP] [session:%s] === Session started (PID: %d) ===", sessionID, os.Getpid())

	return nil
}

// ConfigureForProxyOnly configures logging for proxy-only mode.
// Logs go to stdout for debugging visibility.
func ConfigureForProxyOnly() {
	mu.Lock()
	defer mu.Unlock()

	isFileMode = false
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags)
}

// ConfigureQuiet suppresses all log output (discard mode).
func ConfigureQuiet() {
	mu.Lock()
	defer mu.Unlock()

	isFileMode = false
	log.SetOutput(io.Discard)
}

// rotateLog rotates the log file by renaming it with a timestamp.
func rotateLog() {
	if logFilePath == "" {
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := logFilePath + "." + timestamp

	// Close current log file if open
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}

	// Rename current log file (ignore error - best effort)
	_ = os.Rename(logFilePath, rotatedPath)

	// Keep only last 5 rotated logs
	cleanOldLogs()
}

// cleanOldLogs removes old log files, keeping only the 5 most recent.
func cleanOldLogs() {
	logDir := filepath.Dir(logFilePath)
	pattern := filepath.Join(logDir, "clasp.log.*")

	files, err := filepath.Glob(pattern)
	if err != nil || len(files) <= 5 {
		return
	}

	// Sort by name (timestamp-based, so oldest first)
	// and remove excess files
	for i := 0; i < len(files)-5; i++ {
		os.Remove(files[i])
	}
}

// Close closes the log file if open.
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		log.Printf("[CLASP] [session:%s] === Session ended (PID: %d) ===", sessionID, os.Getpid())
		logFile.Close()
		logFile = nil
	}

	// Also close debug log if open
	if debugFile != nil {
		debugLogger.Printf("[session:%s] === Debug session ended (PID: %d) ===", sessionID, os.Getpid())
		debugFile.Close()
		debugFile = nil
		debugLogger = nil
		debugEnabled = false
	}
}

// IsFileMode returns true if logging to file.
func IsFileMode() bool {
	mu.Lock()
	defer mu.Unlock()
	return isFileMode
}

// GetCurrentLogPath returns the current log file path, or empty if not logging to file.
func GetCurrentLogPath() string {
	mu.Lock()
	defer mu.Unlock()
	if isFileMode {
		return logFilePath
	}
	return ""
}

// EnableDebugLogging enables detailed request/response logging to debug.log.
// Uses session-specific log file if a port has been set.
func EnableDebugLogging() error {
	mu.Lock()
	defer mu.Unlock()

	// Ensure session ID is set
	if sessionID == "" {
		sessionID = GenerateSessionID()
	}

	// Use port-specific path if available, otherwise default path
	if sessionPort > 0 {
		debugFilePath = GetDebugLogPathForPort(sessionPort)
	} else {
		home, _ := os.UserHomeDir()
		debugFilePath = filepath.Join(home, ".clasp", "logs", "debug.log")
	}
	logDir := filepath.Dir(debugFilePath)

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("failed to create debug log directory: %w", err)
	}

	// Rotate debug log if it's too large (>50MB for debug logs)
	if info, err := os.Stat(debugFilePath); err == nil {
		if info.Size() > 50*1024*1024 {
			rotateDebugLog()
		}
	}

	// Open debug log file for appending
	f, err := os.OpenFile(debugFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open debug log file: %w", err)
	}

	debugFile = f
	debugEnabled = true
	debugLogger = log.New(f, "", log.LstdFlags|log.Lmicroseconds)

	// Log debug session start with session ID
	debugLogger.Printf("[session:%s] === Debug session started (PID: %d) ===", sessionID, os.Getpid())

	return nil
}

// DisableDebugLogging disables detailed logging.
func DisableDebugLogging() {
	mu.Lock()
	defer mu.Unlock()

	debugEnabled = false
	if debugFile != nil {
		debugLogger.Printf("=== Debug session ended ===")
		debugFile.Close()
		debugFile = nil
		debugLogger = nil
	}
}

// IsDebugEnabled returns true if debug logging is enabled.
func IsDebugEnabled() bool {
	mu.Lock()
	defer mu.Unlock()
	return debugEnabled
}

// LogDebugRequest logs a full request payload to the debug log.
// The payload is pretty-printed JSON for easier reading.
// Includes session ID for multi-instance tracking.
func LogDebugRequest(direction, endpoint string, payload interface{}) {
	mu.Lock()
	defer mu.Unlock()

	if !debugEnabled || debugLogger == nil {
		return
	}

	jsonData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		debugLogger.Printf("[session:%s] [%s] %s - Error marshaling payload: %v", sessionID, direction, endpoint, err)
		return
	}

	debugLogger.Printf("[session:%s] [%s] %s\n%s\n", sessionID, direction, endpoint, string(jsonData))
}

// LogDebugRequestRaw logs raw request/response data to the debug log.
// Includes session ID for multi-instance tracking.
func LogDebugRequestRaw(direction, endpoint string, data []byte) {
	mu.Lock()
	defer mu.Unlock()

	if !debugEnabled || debugLogger == nil {
		return
	}

	// Try to pretty-print if it's JSON
	var prettyJSON interface{}
	if err := json.Unmarshal(data, &prettyJSON); err == nil {
		jsonData, _ := json.MarshalIndent(prettyJSON, "", "  ")
		debugLogger.Printf("[session:%s] [%s] %s\n%s\n", sessionID, direction, endpoint, string(jsonData))
	} else {
		debugLogger.Printf("[session:%s] [%s] %s\n%s\n", sessionID, direction, endpoint, string(data))
	}
}

// LogDebugSSE logs an SSE event to the debug log.
// Includes session ID for multi-instance tracking.
func LogDebugSSE(direction, eventType, data string) {
	mu.Lock()
	defer mu.Unlock()

	if !debugEnabled || debugLogger == nil {
		return
	}

	// Try to pretty-print if the data is JSON
	var prettyJSON interface{}
	if err := json.Unmarshal([]byte(data), &prettyJSON); err == nil {
		jsonData, _ := json.MarshalIndent(prettyJSON, "", "  ")
		debugLogger.Printf("[session:%s] [%s SSE] event: %s\ndata: %s\n", sessionID, direction, eventType, string(jsonData))
	} else {
		debugLogger.Printf("[session:%s] [%s SSE] event: %s\ndata: %s\n", sessionID, direction, eventType, data)
	}
}

// LogDebugMessage logs a simple message to the debug log.
// Includes session ID for multi-instance tracking.
func LogDebugMessage(format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()

	if !debugEnabled || debugLogger == nil {
		return
	}

	// Prepend session ID to format
	sessionFormat := fmt.Sprintf("[session:%s] %s", sessionID, format)
	debugLogger.Printf(sessionFormat, args...)
}

// rotateDebugLog rotates the debug log file by renaming it with a timestamp.
func rotateDebugLog() {
	if debugFilePath == "" {
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := debugFilePath + "." + timestamp

	// Close current debug log file if open
	if debugFile != nil {
		debugFile.Close()
		debugFile = nil
		debugLogger = nil
	}

	// Rename current debug log file (ignore error - best effort)
	_ = os.Rename(debugFilePath, rotatedPath)

	// Keep only last 3 rotated debug logs (they can be large)
	cleanOldDebugLogs()
}

// cleanOldDebugLogs removes old debug log files, keeping only the 3 most recent.
func cleanOldDebugLogs() {
	logDir := filepath.Dir(debugFilePath)
	pattern := filepath.Join(logDir, "debug.log.*")

	files, err := filepath.Glob(pattern)
	if err != nil || len(files) <= 3 {
		return
	}

	// Sort by name (timestamp-based, so oldest first)
	// and remove excess files
	for i := 0; i < len(files)-3; i++ {
		os.Remove(files[i])
	}
}

// GetDebugLogFilePath returns the current debug log file path.
func GetDebugLogFilePath() string {
	mu.Lock()
	defer mu.Unlock()
	if debugEnabled {
		return debugFilePath
	}
	return ""
}

// ListAllLogFiles returns a list of all CLASP log files (main and debug).
// This is useful for the logs command to show logs from all instances.
func ListAllLogFiles() ([]string, error) {
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return nil, homeErr
	}

	logsDir := filepath.Join(home, ".clasp", "logs")

	// Check if logs directory exists
	if _, statErr := os.Stat(logsDir); os.IsNotExist(statErr) {
		return []string{}, nil
	}

	entries, readErr := os.ReadDir(logsDir)
	if readErr != nil {
		return nil, readErr
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && (filepath.Ext(entry.Name()) == ".log" || entry.Name() == "clasp.log" || entry.Name() == "debug.log") {
			files = append(files, filepath.Join(logsDir, entry.Name()))
		}
	}

	return files, nil
}

// GetAllDebugLogPaths returns paths to all debug log files (port-specific and legacy).
func GetAllDebugLogPaths() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	logsDir := filepath.Join(home, ".clasp", "logs")
	pattern := filepath.Join(logsDir, "debug*.log")

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	// Also check for legacy debug.log
	legacyPath := filepath.Join(logsDir, "debug.log")
	if _, err := os.Stat(legacyPath); err == nil {
		// Check if already in list
		found := false
		for _, f := range files {
			if f == legacyPath {
				found = true
				break
			}
		}
		if !found {
			files = append(files, legacyPath)
		}
	}

	return files, nil
}

// GetAllMainLogPaths returns paths to all main log files (port-specific and legacy).
func GetAllMainLogPaths() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	logsDir := filepath.Join(home, ".clasp", "logs")
	pattern := filepath.Join(logsDir, "clasp-*.log")

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	// Also check for legacy clasp.log
	legacyPath := filepath.Join(logsDir, "clasp.log")
	if _, err := os.Stat(legacyPath); err == nil {
		// Check if already in list
		found := false
		for _, f := range files {
			if f == legacyPath {
				found = true
				break
			}
		}
		if !found {
			files = append(files, legacyPath)
		}
	}

	return files, nil
}
