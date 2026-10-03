package logging

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLeveledLoggingUsesStructuredPrefixes(t *testing.T) {
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	defer func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	}()

	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetFlags(0)

	mu.Lock()
	oldSessionID := sessionID
	sessionID = "test-session"
	mu.Unlock()
	defer func() {
		mu.Lock()
		sessionID = oldSessionID
		mu.Unlock()
	}()

	tests := []struct {
		name  string
		level string
		log   func(string, ...interface{})
	}{
		{name: "info", level: "INFO", log: Info},
		{name: "warn", level: "WARN", log: Warn},
		{name: "error", level: "ERROR", log: Error},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output.Reset()
			test.log("request %d failed", 42)

			got := output.String()
			want := "[CLASP] [" + test.level + "] [session:test-session] request 42 failed\n"
			if got != want {
				t.Fatalf("structured log = %q, want %q", got, want)
			}
		})
	}
}

func TestDebugUsesDebugLoggerAndLevel(t *testing.T) {
	var output bytes.Buffer

	mu.Lock()
	oldSessionID := sessionID
	oldDebugEnabled := debugEnabled
	oldDebugLogger := debugLogger
	sessionID = "debug-session"
	debugEnabled = true
	debugLogger = log.New(&output, "", 0)
	mu.Unlock()
	defer func() {
		mu.Lock()
		sessionID = oldSessionID
		debugEnabled = oldDebugEnabled
		debugLogger = oldDebugLogger
		mu.Unlock()
	}()

	Debug("payload hidden: %s", "redacted")

	got := output.String()
	if !strings.Contains(got, "[CLASP] [DEBUG] [session:debug-session] payload hidden: redacted") {
		t.Fatalf("debug log = %q, missing structured debug fields", got)
	}
}

func TestDebugDoesNotWriteWhenDisabled(t *testing.T) {
	var output bytes.Buffer

	mu.Lock()
	oldDebugEnabled := debugEnabled
	oldDebugLogger := debugLogger
	debugEnabled = false
	debugLogger = log.New(&output, "", 0)
	mu.Unlock()
	defer func() {
		mu.Lock()
		debugEnabled = oldDebugEnabled
		debugLogger = oldDebugLogger
		mu.Unlock()
	}()

	Debug("not emitted")

	if output.Len() != 0 {
		t.Fatalf("disabled debug log wrote %q", output.String())
	}
}
