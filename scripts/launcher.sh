#!/bin/bash
# CLASP Autonomous Agent Launcher
# Runs Claude Code in a continuous loop with the CLASP development prompt

set -e

# Configuration
WORKSPACE="/workspaces/ord-options-testing"
CLASP_DIR="$WORKSPACE/CLASP"
PROMPT_FILE="$CLASP_DIR/prompt.md"
LOG_DIR="$WORKSPACE/.clasp/logs"
LOOP_DELAY=5  # Seconds between iterations
PARSER_SCRIPT="$CLASP_DIR/scripts/stream-parser.sh"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Iteration counter
ITERATION=0

# Log file for this session
SESSION_ID=$(date +%Y%m%d_%H%M%S)
SESSION_LOG="$LOG_DIR/session_$SESSION_ID.jsonl"

echo -e "${MAGENTA}${BOLD}"
echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║             CLASP - Claude Language Agent Super Proxy            ║"
echo "║                   Autonomous Development Agent                   ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"
echo ""
echo -e "${BLUE}Workspace:${NC} $WORKSPACE"
echo -e "${BLUE}CLASP Dir:${NC} $CLASP_DIR"
echo -e "${BLUE}Session:${NC} $SESSION_ID"
echo -e "${BLUE}Log:${NC} $SESSION_LOG"
echo ""

# Ensure directories exist
mkdir -p "$LOG_DIR"

# Check if prompt file exists
if [ ! -f "$PROMPT_FILE" ]; then
    echo -e "${RED}Error: Prompt file not found at $PROMPT_FILE${NC}"
    exit 1
fi

# Check if parser script exists and is executable
if [ ! -x "$PARSER_SCRIPT" ]; then
    echo -e "${YELLOW}Warning: Parser script not found or not executable${NC}"
    echo -e "${YELLOW}Output will be raw JSON${NC}"
    USE_PARSER=false
else
    USE_PARSER=true
fi

# Function to print iteration header
print_header() {
    local iter=$1
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S %Z')

    echo ""
    echo -e "${GREEN}${BOLD}"
    echo "┌──────────────────────────────────────────────────────────────────┐"
    printf "│  Iteration: %-5d                                                │\n" "$iter"
    printf "│  Time: %-55s │\n" "$timestamp"
    echo "└──────────────────────────────────────────────────────────────────┘"
    echo -e "${NC}"
}

# Function to print iteration footer
print_footer() {
    local iter=$1
    local duration=$2

    echo ""
    echo -e "${YELLOW}────────────────────────────────────────────────────────────────────${NC}"
    echo -e "${YELLOW}Iteration $iter completed in ${duration}s${NC}"
    echo -e "${YELLOW}Waiting ${LOOP_DELAY}s before next iteration...${NC}"
    echo -e "${YELLOW}────────────────────────────────────────────────────────────────────${NC}"
}

# Trap for clean exit
cleanup() {
    echo ""
    echo -e "${RED}${BOLD}Shutting down CLASP agent...${NC}"
    echo -e "${BLUE}Total iterations completed: $ITERATION${NC}"
    exit 0
}

trap cleanup SIGINT SIGTERM

# Main loop
echo -e "${GREEN}Starting autonomous development loop...${NC}"
echo -e "${CYAN}Press Ctrl+C to stop${NC}"
echo ""

while true; do
    ITERATION=$((ITERATION + 1))
    START_TIME=$(date +%s)

    # Print iteration header
    print_header $ITERATION

    # Log iteration start
    echo "{\"event\":\"iteration_start\",\"iteration\":$ITERATION,\"timestamp\":\"$(date -Iseconds)\"}" >> "$SESSION_LOG"

    # Read the prompt
    PROMPT=$(cat "$PROMPT_FILE")

    # Run Claude Code headless with stream-json
    echo -e "${BLUE}Running Claude Code...${NC}"
    echo ""

    if [ "$USE_PARSER" = true ]; then
        # Pipe through parser for human-readable output
        claude --dangerously-skip-permissions \
               --output-format stream-json \
               --verbose \
               --print \
               -p "$PROMPT" \
               2>&1 | tee -a "$SESSION_LOG" | "$PARSER_SCRIPT"
    else
        # Raw JSON output
        claude --dangerously-skip-permissions \
               --output-format stream-json \
               --verbose \
               --print \
               -p "$PROMPT" \
               2>&1 | tee -a "$SESSION_LOG"
    fi

    # Calculate duration
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))

    # Log iteration end
    echo "{\"event\":\"iteration_end\",\"iteration\":$ITERATION,\"duration_secs\":$DURATION,\"timestamp\":\"$(date -Iseconds)\"}" >> "$SESSION_LOG"

    # Print iteration footer
    print_footer $ITERATION $DURATION

    # Wait before next iteration
    sleep $LOOP_DELAY
done
