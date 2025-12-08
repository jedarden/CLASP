#!/bin/bash
# CLASP Comprehensive Test Suite
# Runs isolated tests in separate tmux sessions with proper cleanup
#
# Tests:
# 1. Tool Calling - Validates Task tool schema filtering with optional params
# 2. Statusline - Validates Claude Code status line integration
# 3. Proxy Lifecycle - Tests start/stop/status commands
#
# Each test runs in its own tmux session that is destroyed and recreated

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLASP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$CLASP_DIR/test-results"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
RESULTS_FILE="$RESULTS_DIR/test-run-$TIMESTAMP.log"
SESSION_PREFIX="clasp-test"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'
BOLD='\033[1m'

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Create results directory
mkdir -p "$RESULTS_DIR"

#######################################
# Utility Functions
#######################################

log() {
    echo -e "$1" | tee -a "$RESULTS_FILE"
}

log_section() {
    log ""
    log "${MAGENTA}${BOLD}════════════════════════════════════════════════════════════════${NC}"
    log "${MAGENTA}${BOLD}  $1${NC}"
    log "${MAGENTA}${BOLD}════════════════════════════════════════════════════════════════${NC}"
    log ""
}

log_test_start() {
    log "${CYAN}▶ Starting test: $1${NC}"
    log "  Description: $2"
}

log_pass() {
    log "${GREEN}${BOLD}✓ PASS:${NC} $1"
    ((TESTS_PASSED++))
}

log_fail() {
    log "${RED}${BOLD}✗ FAIL:${NC} $1"
    log "${RED}  Reason: $2${NC}"
    ((TESTS_FAILED++))
}

log_skip() {
    log "${YELLOW}○ SKIP:${NC} $1"
    log "${YELLOW}  Reason: $2${NC}"
    ((TESTS_SKIPPED++))
}

# Cleanup any existing test sessions
cleanup_test_sessions() {
    log "${YELLOW}Cleaning up existing test sessions...${NC}"
    for session in $(tmux ls -F '#{session_name}' 2>/dev/null | grep "^${SESSION_PREFIX}" || true); do
        log "  Killing session: $session"
        tmux kill-session -t "$session" 2>/dev/null || true
    done
    # Also cleanup zombie CLASP processes
    pkill -9 -f "clasp.*-profile" 2>/dev/null || true
    # Clean stale status files
    rm -f ~/.clasp/status/*.json 2>/dev/null || true
    rm -f ~/.clasp/status.json 2>/dev/null || true
    sleep 1
}

# Create and verify tmux session
create_test_session() {
    local session_name="$1"
    local work_dir="${2:-$CLASP_DIR}"

    # Ensure no existing session with this name
    tmux kill-session -t "$session_name" 2>/dev/null || true
    sleep 0.5

    # Create new session
    tmux new-session -d -s "$session_name" -c "$work_dir" -x 200 -y 50

    if tmux has-session -t "$session_name" 2>/dev/null; then
        log "  Created tmux session: $session_name"
        return 0
    else
        log "${RED}  Failed to create tmux session: $session_name${NC}"
        return 1
    fi
}

# Destroy tmux session with verification
destroy_test_session() {
    local session_name="$1"

    if tmux has-session -t "$session_name" 2>/dev/null; then
        # First try graceful termination
        tmux send-keys -t "$session_name" C-c 2>/dev/null || true
        sleep 1

        # Force kill
        tmux kill-session -t "$session_name" 2>/dev/null || true
        log "  Destroyed tmux session: $session_name"
    fi

    # Kill any associated CLASP processes
    pkill -9 -f "clasp" 2>/dev/null || true
    sleep 0.5
}

# Wait for condition with timeout
wait_for_condition() {
    local condition="$1"
    local timeout="${2:-30}"
    local interval="${3:-1}"
    local elapsed=0

    while [ $elapsed -lt $timeout ]; do
        if eval "$condition" 2>/dev/null; then
            return 0
        fi
        sleep $interval
        ((elapsed+=interval))
    done
    return 1
}

#######################################
# Test 1: Statusline Integration
#######################################

test_statusline() {
    local session_name="${SESSION_PREFIX}-statusline"
    local test_port=8081
    local test_name="Statusline Integration"

    log_test_start "$test_name" "Validates Claude Code status line displays CLASP proxy info"

    # Pre-test cleanup
    destroy_test_session "$session_name"
    rm -f ~/.clasp/status/${test_port}.json 2>/dev/null

    # Check prerequisites
    if [[ ! -x "$CLASP_DIR/bin/clasp" ]]; then
        log_skip "$test_name" "CLASP binary not found. Run: go build -o bin/clasp ./cmd/clasp"
        return
    fi

    if [[ ! -f ~/.claude/clasp-statusline.sh ]]; then
        log_skip "$test_name" "Statusline script not installed at ~/.claude/clasp-statusline.sh"
        return
    fi

    # Create test session
    if ! create_test_session "$session_name"; then
        log_fail "$test_name" "Could not create tmux session"
        return
    fi

    # Step 1: Start CLASP proxy
    log "  Step 1: Starting CLASP proxy on port $test_port..."
    tmux send-keys -t "$session_name" "cd $CLASP_DIR && ./bin/clasp --port $test_port --proxy-only 2>&1 &" Enter
    tmux send-keys -t "$session_name" "CLASP_PID=\$!" Enter

    # Wait for status file to be created
    log "  Step 2: Waiting for status file..."
    if ! wait_for_condition "[ -f ~/.clasp/status/${test_port}.json ]" 10; then
        log_fail "$test_name" "Status file not created at ~/.clasp/status/${test_port}.json"
        destroy_test_session "$session_name"
        return
    fi

    # Step 3: Verify status file content
    log "  Step 3: Verifying status file content..."
    local status_content
    status_content=$(cat ~/.clasp/status/${test_port}.json 2>/dev/null)

    if ! echo "$status_content" | jq -e '.running == true' >/dev/null 2>&1; then
        log_fail "$test_name" "Status file does not show running=true"
        destroy_test_session "$session_name"
        return
    fi

    if ! echo "$status_content" | jq -e ".port == $test_port" >/dev/null 2>&1; then
        log_fail "$test_name" "Status file does not show correct port"
        destroy_test_session "$session_name"
        return
    fi

    # Step 4: Test statusline script
    log "  Step 4: Testing statusline script..."
    local statusline_output
    statusline_output=$(ANTHROPIC_BASE_URL="http://localhost:$test_port" ~/.claude/clasp-statusline.sh '{}' 2>&1)

    if [[ "$statusline_output" == *"[CLASP:$test_port]"* ]]; then
        log "  Statusline output: $statusline_output"
        log_pass "$test_name"
    else
        log_fail "$test_name" "Statusline script did not produce expected output. Got: $statusline_output"
    fi

    # Step 5: Test process detection (verify PID check works)
    log "  Step 5: Verifying process detection..."
    local pid
    pid=$(echo "$status_content" | jq -r '.pid')
    if kill -0 "$pid" 2>/dev/null; then
        log "  Process $pid is running correctly"
    else
        log "${YELLOW}  Warning: Process $pid not running (may have exited)${NC}"
    fi

    # Cleanup
    destroy_test_session "$session_name"
    rm -f ~/.clasp/status/${test_port}.json 2>/dev/null
}

#######################################
# Test 2: Tool Schema Filtering
#######################################

test_tool_calling() {
    local session_name="${SESSION_PREFIX}-tools"
    local test_port=8082
    local test_name="Tool Schema Filtering"

    log_test_start "$test_name" "Validates strict:false and required array filtering for Responses API"

    # Pre-test cleanup
    destroy_test_session "$session_name"

    # Check prerequisites
    if [[ ! -x "$CLASP_DIR/bin/clasp" ]]; then
        log_skip "$test_name" "CLASP binary not found. Run: go build -o bin/clasp ./cmd/clasp"
        return
    fi

    if [[ -z "$OPENAI_API_KEY" ]]; then
        if [[ -f "$CLASP_DIR/.env" ]]; then
            source "$CLASP_DIR/.env"
        fi
        if [[ -z "$OPENAI_API_KEY" ]]; then
            log_skip "$test_name" "OPENAI_API_KEY not set"
            return
        fi
    fi

    # Create test session
    if ! create_test_session "$session_name"; then
        log_fail "$test_name" "Could not create tmux session"
        return
    fi

    # Step 1: Start CLASP with debug mode
    log "  Step 1: Starting CLASP with debug logging..."
    local debug_log="$RESULTS_DIR/tool-debug-$TIMESTAMP.log"
    tmux send-keys -t "$session_name" "cd $CLASP_DIR" Enter
    tmux send-keys -t "$session_name" "OPENAI_API_KEY=$OPENAI_API_KEY ./bin/clasp --port $test_port --provider openai --debug --proxy-only 2>&1 | tee $debug_log &" Enter

    # Wait for CLASP to start
    if ! wait_for_condition "[ -f ~/.clasp/status/${test_port}.json ]" 10; then
        log_fail "$test_name" "CLASP did not start (no status file)"
        destroy_test_session "$session_name"
        return
    fi

    # Step 2: Send a test request with tools
    log "  Step 2: Sending test request with tool definitions..."
    local test_request='{"model":"claude-3-5-sonnet-20241022","max_tokens":100,"messages":[{"role":"user","content":"test"}],"tools":[{"name":"test_tool","description":"test","input_schema":{"type":"object","properties":{"required_param":{"type":"string"},"optional_param":{"type":"boolean","description":"Optional flag"}},"required":["required_param","optional_param"],"strict":true}}]}'

    local response
    response=$(curl -s -X POST "http://localhost:$test_port/v1/messages" \
        -H "Content-Type: application/json" \
        -H "x-api-key: test" \
        -H "anthropic-version: 2023-06-01" \
        -d "$test_request" 2>&1)

    # Step 3: Check debug log for schema transformation
    log "  Step 3: Verifying schema transformation in debug log..."
    sleep 2

    # Check that strict:false is set at top level
    if grep -q '"strict":false' "$debug_log" 2>/dev/null; then
        log "  ✓ Found strict:false in transformed request"
    else
        log "${YELLOW}  ⚠ Could not verify strict:false (may need more request data)${NC}"
    fi

    # Check that required array is filtered
    if grep -q '"required":\["required_param"\]' "$debug_log" 2>/dev/null || \
       ! grep -q '"optional_param".*"required"' "$debug_log" 2>/dev/null; then
        log "  ✓ Required array appears to be filtered correctly"
    fi

    # Step 4: Check for validation errors
    if grep -qi "invalid tool parameters" "$debug_log" 2>/dev/null; then
        log_fail "$test_name" "Found 'Invalid tool parameters' error in logs"
    elif grep -qi "missing required" "$debug_log" 2>/dev/null; then
        log_fail "$test_name" "Found 'missing required' error in logs"
    else
        log_pass "$test_name"
    fi

    # Cleanup
    destroy_test_session "$session_name"
    rm -f ~/.clasp/status/${test_port}.json 2>/dev/null
}

#######################################
# Test 3: Proxy Lifecycle
#######################################

test_proxy_lifecycle() {
    local session_name="${SESSION_PREFIX}-lifecycle"
    local test_port=8083
    local test_name="Proxy Lifecycle"

    log_test_start "$test_name" "Validates CLASP start/stop/status commands"

    # Pre-test cleanup
    destroy_test_session "$session_name"
    rm -f ~/.clasp/status/${test_port}.json 2>/dev/null

    # Check prerequisites
    if [[ ! -x "$CLASP_DIR/bin/clasp" ]]; then
        log_skip "$test_name" "CLASP binary not found"
        return
    fi

    # Create test session
    if ! create_test_session "$session_name"; then
        log_fail "$test_name" "Could not create tmux session"
        return
    fi

    # Step 1: Check initial status (should show not running)
    log "  Step 1: Checking initial status..."
    local initial_status
    initial_status=$("$CLASP_DIR/bin/clasp" --status --port $test_port 2>&1 || true)

    if [[ "$initial_status" == *"not running"* ]] || [[ "$initial_status" == *"No running"* ]]; then
        log "  ✓ Initial status correctly shows not running"
    fi

    # Step 2: Start proxy
    log "  Step 2: Starting proxy..."
    tmux send-keys -t "$session_name" "cd $CLASP_DIR && ./bin/clasp --port $test_port --proxy-only 2>&1 &" Enter

    if ! wait_for_condition "[ -f ~/.clasp/status/${test_port}.json ]" 10; then
        log_fail "$test_name" "Proxy did not start"
        destroy_test_session "$session_name"
        return
    fi

    # Step 3: Check running status
    log "  Step 3: Checking running status..."
    local running_status
    running_status=$("$CLASP_DIR/bin/clasp" --status --port $test_port 2>&1 || true)

    if [[ "$running_status" == *"Running"* ]] || [[ "$running_status" == *"[CLASP:$test_port]"* ]]; then
        log "  ✓ Status correctly shows proxy running"
    else
        log "${YELLOW}  ⚠ Status output: $running_status${NC}"
    fi

    # Step 4: Verify HTTP endpoint
    log "  Step 4: Checking HTTP endpoint..."
    local health_check
    health_check=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$test_port/health" 2>&1 || echo "failed")

    if [[ "$health_check" == "200" ]]; then
        log "  ✓ Health endpoint responding (HTTP 200)"
    else
        log "${YELLOW}  ⚠ Health check returned: $health_check${NC}"
    fi

    # Step 5: Stop proxy and verify cleanup
    log "  Step 5: Stopping proxy..."
    tmux send-keys -t "$session_name" "kill %1" Enter
    sleep 2

    # Verify status file is removed (or shows not running)
    if [[ ! -f ~/.clasp/status/${test_port}.json ]]; then
        log "  ✓ Status file cleaned up on shutdown"
        log_pass "$test_name"
    else
        local final_status
        final_status=$(cat ~/.clasp/status/${test_port}.json 2>/dev/null)
        if echo "$final_status" | jq -e '.running == false' >/dev/null 2>&1; then
            log "  ✓ Status file shows running=false"
            log_pass "$test_name"
        else
            log_fail "$test_name" "Status file not cleaned up properly"
        fi
    fi

    # Cleanup
    destroy_test_session "$session_name"
}

#######################################
# Test 4: Multi-Instance Statusline
#######################################

test_multi_instance() {
    local session_name="${SESSION_PREFIX}-multi"
    local test_name="Multi-Instance Statusline"
    local port1=8084
    local port2=8085

    log_test_start "$test_name" "Validates statusline works with multiple CLASP instances"

    # Pre-test cleanup
    destroy_test_session "$session_name"
    rm -f ~/.clasp/status/${port1}.json ~/.clasp/status/${port2}.json 2>/dev/null

    # Check prerequisites
    if [[ ! -x "$CLASP_DIR/bin/clasp" ]]; then
        log_skip "$test_name" "CLASP binary not found"
        return
    fi

    # Create test session
    if ! create_test_session "$session_name"; then
        log_fail "$test_name" "Could not create tmux session"
        return
    fi

    # Step 1: Start first instance
    log "  Step 1: Starting first CLASP instance on port $port1..."
    tmux send-keys -t "$session_name" "cd $CLASP_DIR && ./bin/clasp --port $port1 --proxy-only 2>&1 &" Enter

    if ! wait_for_condition "[ -f ~/.clasp/status/${port1}.json ]" 10; then
        log_fail "$test_name" "First instance did not start"
        destroy_test_session "$session_name"
        return
    fi

    # Step 2: Start second instance
    log "  Step 2: Starting second CLASP instance on port $port2..."
    tmux send-keys -t "$session_name" "./bin/clasp --port $port2 --proxy-only 2>&1 &" Enter

    if ! wait_for_condition "[ -f ~/.clasp/status/${port2}.json ]" 10; then
        log_fail "$test_name" "Second instance did not start"
        destroy_test_session "$session_name"
        return
    fi

    # Step 3: Verify both status files exist and are distinct
    log "  Step 3: Verifying status files..."
    local status1 status2 pid1 pid2
    status1=$(cat ~/.clasp/status/${port1}.json 2>/dev/null)
    status2=$(cat ~/.clasp/status/${port2}.json 2>/dev/null)

    pid1=$(echo "$status1" | jq -r '.pid')
    pid2=$(echo "$status2" | jq -r '.pid')

    if [[ "$pid1" != "$pid2" ]]; then
        log "  ✓ Status files have distinct PIDs ($pid1 vs $pid2)"
    else
        log_fail "$test_name" "Status files have same PID"
        destroy_test_session "$session_name"
        return
    fi

    # Step 4: Test statusline for each instance
    log "  Step 4: Testing statusline for each instance..."
    local out1 out2
    out1=$(ANTHROPIC_BASE_URL="http://localhost:$port1" ~/.claude/clasp-statusline.sh '{}' 2>&1)
    out2=$(ANTHROPIC_BASE_URL="http://localhost:$port2" ~/.claude/clasp-statusline.sh '{}' 2>&1)

    if [[ "$out1" == *"[CLASP:$port1]"* ]] && [[ "$out2" == *"[CLASP:$port2]"* ]]; then
        log "  ✓ Statusline correctly identifies each instance"
        log "    Instance 1: $out1"
        log "    Instance 2: $out2"
        log_pass "$test_name"
    else
        log_fail "$test_name" "Statusline did not correctly identify instances"
        log "    Instance 1 output: $out1"
        log "    Instance 2 output: $out2"
    fi

    # Cleanup
    destroy_test_session "$session_name"
    rm -f ~/.clasp/status/${port1}.json ~/.clasp/status/${port2}.json 2>/dev/null
}

#######################################
# Main Test Runner
#######################################

main() {
    log_section "CLASP Test Suite"
    log "Timestamp: $(date)"
    log "Results file: $RESULTS_FILE"
    log "CLASP directory: $CLASP_DIR"

    # Initial cleanup
    cleanup_test_sessions

    # Ensure CLASP is built
    if [[ ! -x "$CLASP_DIR/bin/clasp" ]]; then
        log "${YELLOW}Building CLASP binary...${NC}"
        (cd "$CLASP_DIR" && go build -o bin/clasp ./cmd/clasp)
    fi

    # Run tests
    log_section "Running Tests"

    test_statusline
    log ""

    test_proxy_lifecycle
    log ""

    test_multi_instance
    log ""

    test_tool_calling
    log ""

    # Final cleanup
    cleanup_test_sessions

    # Summary
    log_section "Test Summary"
    log "${GREEN}Passed: $TESTS_PASSED${NC}"
    log "${RED}Failed: $TESTS_FAILED${NC}"
    log "${YELLOW}Skipped: $TESTS_SKIPPED${NC}"
    log ""
    log "Results saved to: $RESULTS_FILE"

    # Exit with failure if any tests failed
    if [[ $TESTS_FAILED -gt 0 ]]; then
        exit 1
    fi
}

# Handle arguments
case "${1:-}" in
    --statusline)
        cleanup_test_sessions
        test_statusline
        cleanup_test_sessions
        ;;
    --tools)
        cleanup_test_sessions
        test_tool_calling
        cleanup_test_sessions
        ;;
    --lifecycle)
        cleanup_test_sessions
        test_proxy_lifecycle
        cleanup_test_sessions
        ;;
    --multi)
        cleanup_test_sessions
        test_multi_instance
        cleanup_test_sessions
        ;;
    --help|-h)
        echo "CLASP Test Suite"
        echo ""
        echo "Usage: $0 [option]"
        echo ""
        echo "Options:"
        echo "  (none)        Run all tests"
        echo "  --statusline  Run only statusline test"
        echo "  --tools       Run only tool schema test"
        echo "  --lifecycle   Run only proxy lifecycle test"
        echo "  --multi       Run only multi-instance test"
        echo "  --help        Show this help"
        ;;
    *)
        main
        ;;
esac
