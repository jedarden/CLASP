// Package logging provides log management for CLASP, including file-based logging
// when running in Claude Code mode to prevent TUI corruption.
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
	logFile      *os.File
	logFilePath  string
	debugFile    *os.File
	debugFilePath string
	debugLogger  *log.Logger
	mu           sync.Mutex
	isFileMode   bool
	debugEnabled bool
)

// GetLogPath returns the path to the CLASP log file.
func GetLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clasp", "logs", "clasp.log")
}

// GetDebugLogPath returns the path to the CLASP debug log file.
func GetDebugLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clasp", "logs", "debug.log")
}

// ConfigureForClaudeCode configures logging for Claude Code mode.
// All log output is redirected to a file to prevent TUI corruption.
func ConfigureForClaudeCode() error {
	mu.Lock()
	defer mu.Unlock()

	logFilePath = GetLogPath()
	logDir := filepath.Dir(logFilePath)

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Rotate log if it's too large (>10MB)
	if info, err := os.Stat(logFilePath); err == nil {
		if info.Size() > 10*1024*1024 {
			rotateLog()
		}
	}

	// Open log file for appending
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	logFile = f
	isFileMode = true

	// Redirect standard logger to file
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Log session start
	log.Printf("[CLASP] === Session started ===")

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

	// Rename current log file
	os.Rename(logFilePath, rotatedPath)

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
		log.Printf("[CLASP] === Session ended ===")
		logFile.Close()
		logFile = nil
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
func EnableDebugLogging() error {
	mu.Lock()
	defer mu.Unlock()

	debugFilePath = GetDebugLogPath()
	logDir := filepath.Dir(debugFilePath)

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug log directory: %w", err)
	}

	// Rotate debug log if it's too large (>50MB for debug logs)
	if info, err := os.Stat(debugFilePath); err == nil {
		if info.Size() > 50*1024*1024 {
			rotateDebugLog()
		}
	}

	// Open debug log file for appending
	f, err := os.OpenFile(debugFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open debug log file: %w", err)
	}

	debugFile = f
	debugEnabled = true
	debugLogger = log.New(f, "", log.LstdFlags|log.Lmicroseconds)

	// Log debug session start
	debugLogger.Printf("=== Debug session started ===")

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
func LogDebugRequest(direction string, endpoint string, payload interface{}) {
	mu.Lock()
	defer mu.Unlock()

	if !debugEnabled || debugLogger == nil {
		return
	}

	jsonData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		debugLogger.Printf("[%s] %s - Error marshaling payload: %v", direction, endpoint, err)
		return
	}

	debugLogger.Printf("[%s] %s\n%s\n", direction, endpoint, string(jsonData))
}

// LogDebugRequestRaw logs raw request/response data to the debug log.
func LogDebugRequestRaw(direction string, endpoint string, data []byte) {
	mu.Lock()
	defer mu.Unlock()

	if !debugEnabled || debugLogger == nil {
		return
	}

	// Try to pretty-print if it's JSON
	var prettyJSON interface{}
	if err := json.Unmarshal(data, &prettyJSON); err == nil {
		jsonData, _ := json.MarshalIndent(prettyJSON, "", "  ")
		debugLogger.Printf("[%s] %s\n%s\n", direction, endpoint, string(jsonData))
	} else {
		debugLogger.Printf("[%s] %s\n%s\n", direction, endpoint, string(data))
	}
}

// LogDebugSSE logs an SSE event to the debug log.
func LogDebugSSE(direction string, eventType string, data string) {
	mu.Lock()
	defer mu.Unlock()

	if !debugEnabled || debugLogger == nil {
		return
	}

	// Try to pretty-print if the data is JSON
	var prettyJSON interface{}
	if err := json.Unmarshal([]byte(data), &prettyJSON); err == nil {
		jsonData, _ := json.MarshalIndent(prettyJSON, "", "  ")
		debugLogger.Printf("[%s SSE] event: %s\ndata: %s\n", direction, eventType, string(jsonData))
	} else {
		debugLogger.Printf("[%s SSE] event: %s\ndata: %s\n", direction, eventType, data)
	}
}

// LogDebugMessage logs a simple message to the debug log.
func LogDebugMessage(format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()

	if !debugEnabled || debugLogger == nil {
		return
	}

	debugLogger.Printf(format, args...)
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

	// Rename current debug log file
	os.Rename(debugFilePath, rotatedPath)

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
