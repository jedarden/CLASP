// Package logging provides log management for CLASP, including file-based logging
// when running in Claude Code mode to prevent TUI corruption.
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logFile     *os.File
	logFilePath string
	mu          sync.Mutex
	isFileMode  bool
)

// GetLogPath returns the path to the CLASP log file.
func GetLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clasp", "logs", "clasp.log")
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
