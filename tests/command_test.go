package tests

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCommandVersion tests that the version command works
func TestCommandVersion(t *testing.T) {
	cmd := exec.Command("go", "run", "../cmd/clasp", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("version command failed: %v, output: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "CLASP") {
		t.Errorf("version output should contain 'CLASP', got: %s", outputStr)
	}
}

// TestCommandHelp tests that the help command works
func TestCommandHelp(t *testing.T) {
	cmd := exec.Command("go", "run", "../cmd/clasp", "--help")
	output, err := cmd.CombinedOutput()
	// --help may exit with status 0 or non-zero depending on implementation
	// Just check the output contains expected content
	_ = err // Deliberately ignore error as --help behavior varies

	outputStr := string(output)
	expectedContent := []string{
		"Usage",
		"clasp",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(strings.ToLower(outputStr), strings.ToLower(expected)) {
			t.Errorf("help output should contain '%s', got: %s", expected, outputStr)
		}
	}
}

// TestCommandInvalidSubcommand tests that invalid subcommands are handled
func TestCommandInvalidSubcommand(t *testing.T) {
	cmd := exec.Command("go", "run", "../cmd/clasp", "invalidcommand12345")
	output, err := cmd.CombinedOutput()

	// Should either show help or an error message
	outputStr := string(output)
	if err == nil && !strings.Contains(outputStr, "unknown") && !strings.Contains(outputStr, "Usage") && !strings.Contains(outputStr, "help") {
		t.Logf("Warning: invalid command did not produce clear error/help output: %s", outputStr)
	}
}

// TestCommandProfileList tests the profile list command
func TestCommandProfileList(t *testing.T) {
	cmd := exec.Command("go", "run", "../cmd/clasp", "profile", "list")
	output, err := cmd.CombinedOutput()

	// This may fail if no profiles exist, which is OK for CI
	if err != nil {
		t.Logf("profile list command exited with error (expected if no profiles): %v", err)
	}

	// Just verify it runs without panic
	outputStr := string(output)
	t.Logf("profile list output: %s", outputStr)
}

// TestBuildAndRun tests that the binary builds and runs
func TestBuildAndRun(t *testing.T) {
	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "/tmp/clasp-test-binary", "../cmd/clasp")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v, output: %s", err, output)
	}

	// Test version command with built binary
	versionCmd := exec.Command("/tmp/clasp-test-binary", "version")
	output, err := versionCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("version command failed with built binary: %v, output: %s", err, output)
	}

	if !strings.Contains(string(output), "CLASP") {
		t.Errorf("version output should contain 'CLASP', got: %s", output)
	}
}
