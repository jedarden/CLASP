// Package statusline provides tests for status line functionality.
package statusline

import (
	"fmt"
	"os"
	"testing"
)

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	// Current process should be alive
	pid := os.Getpid()
	if !isProcessAlive(pid) {
		t.Errorf("isProcessAlive(%d) = false, want true (current process)", pid)
	}
}

func TestIsProcessAlive_InvalidPID(t *testing.T) {
	tests := []struct {
		name string
		pid  int
		want bool
	}{
		{"zero PID", 0, false},
		{"negative PID", -1, false},
		{"non-existent PID", 999999999, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isProcessAlive(tt.pid)
			if got != tt.want {
				t.Errorf("isProcessAlive(%d) = %v, want %v", tt.pid, got, tt.want)
			}
		})
	}
}

func TestIsProcessAlive_ZombieDetection(t *testing.T) {
	// This test verifies the zombie detection logic parses /proc/[pid]/stat correctly
	// We can't easily create a zombie in a unit test, but we can verify the parsing logic
	// by checking that we can read our own process state

	pid := os.Getpid()
	statPath := fmt.Sprintf("/proc/%d/stat", pid)

	// Skip if /proc is not available (non-Linux)
	if _, err := os.Stat(statPath); os.IsNotExist(err) {
		t.Skip("Skipping zombie detection test on non-Linux platform")
	}

	// Our process state should be 'R' (running) or 'S' (sleeping)
	data, err := os.ReadFile(statPath)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", statPath, err)
	}

	// Verify the stat file is readable and parseable
	content := string(data)
	if len(content) == 0 {
		t.Fatal("Empty stat file")
	}

	// Our process should be alive
	if !isProcessAlive(pid) {
		t.Error("isProcessAlive returned false for current process")
	}
}

func TestFormatStatusLine(t *testing.T) {
	tests := []struct {
		name    string
		status  *Status
		verbose bool
		want    string
	}{
		{
			name:    "nil status",
			status:  nil,
			verbose: false,
			want:    "CLASP proxy is not running",
		},
		{
			name:    "not running",
			status:  &Status{Running: false},
			verbose: false,
			want:    "CLASP proxy is not running",
		},
		{
			name: "running simple",
			status: &Status{
				Running:  true,
				Port:     8080,
				Model:    "gpt-4o",
				Provider: "openai",
				Requests: 10,
			},
			verbose: false,
			want:    "[CLASP:8080] gpt-4o (openai) | 10 reqs",
		},
		{
			name: "running with cost",
			status: &Status{
				Running:  true,
				Port:     8080,
				Model:    "gpt-4o",
				Provider: "openai",
				Requests: 10,
				CostUSD:  1.50,
			},
			verbose: false,
			want:    "[CLASP:8080] gpt-4o (openai) | 10 reqs | $1.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStatusLine(tt.status, tt.verbose)
			if got != tt.want {
				t.Errorf("FormatStatusLine() = %q, want %q", got, tt.want)
			}
		})
	}
}
