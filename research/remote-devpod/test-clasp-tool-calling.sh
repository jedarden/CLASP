#!/bin/bash
# CLASP Tool Calling Test Script
# Tests the enhanced tool schema filtering with a realistic multi-agent prompt
#
# This script:
# 1. Creates a CLASP profile with OpenAI configuration + embedded API key
# 2. Launches CLASP with that profile (which auto-launches Claude Code)
# 3. Pipes the research prompt to test Task tool spawning
# 4. Captures tmux output and monitors logs to verify tool calls work
#
# PASS: Task tool calls succeed, agents spawn without "Invalid tool parameters"
# FAIL: Errors about missing optional parameters (model, resume, run_in_background)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLASP_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
SESSION_NAME="clasp-test-$(date +%s)"
LOG_FILE="$SCRIPT_DIR/clasp-test-$(date +%Y%m%d-%H%M%S).log"
PROMPT_FILE="$SCRIPT_DIR/test-prompt.txt"
RESULTS_FILE="$SCRIPT_DIR/test-results-$(date +%Y%m%d-%H%M%S).txt"
PROFILE_NAME="test-openai"
CLASP_CONFIG_DIR="$HOME/.clasp"
PROFILE_DIR="$CLASP_CONFIG_DIR/profiles"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== CLASP Tool Calling Test ===${NC}"
echo "Session: $SESSION_NAME"
echo "Log file: $LOG_FILE"
echo "Results: $RESULTS_FILE"

# Check if CLASP binary exists
if [[ ! -x "$CLASP_DIR/bin/clasp" ]]; then
    echo -e "${RED}Error: CLASP binary not found at $CLASP_DIR/bin/clasp${NC}"
    echo "Build CLASP first: cd $CLASP_DIR && go build -o bin/clasp ./cmd/clasp"
    exit 1
fi

# Check for .env file with API key
if [[ ! -f "$CLASP_DIR/.env" ]]; then
    echo -e "${RED}Error: .env file not found at $CLASP_DIR/.env${NC}"
    echo "Create .env with: OPENAI_API_KEY=sk-..."
    exit 1
fi

# Load environment
set -a
source "$CLASP_DIR/.env"
set +a

if [[ -z "$OPENAI_API_KEY" ]]; then
    echo -e "${RED}Error: OPENAI_API_KEY not set in .env${NC}"
    exit 1
fi

# Create CLASP profile directory
mkdir -p "$PROFILE_DIR"

# Create test profile with OpenAI configuration
echo -e "${YELLOW}Creating CLASP profile: $PROFILE_NAME${NC}"
cat > "$PROFILE_DIR/${PROFILE_NAME}.json" << EOF
{
  "name": "$PROFILE_NAME",
  "description": "Test profile for tool calling validation",
  "provider": "openai",
  "api_key": "$OPENAI_API_KEY",
  "default_model": "gpt-5.1-codex",
  "port": 8080,
  "claude_code": {
    "auto_launch": true,
    "skip_permissions": true
  },
  "created_at": "$(date -Iseconds)",
  "updated_at": "$(date -Iseconds)"
}
EOF

# Set as active profile
cat > "$CLASP_CONFIG_DIR/config.json" << EOF
{
  "active_profile": "$PROFILE_NAME",
  "last_used": "$(date -Iseconds)"
}
EOF

echo "  Profile created at: $PROFILE_DIR/${PROFILE_NAME}.json"

# The test prompt - spawns 3 agents concurrently (tests Task tool with optional params)
cat > "$PROMPT_FILE" << 'PROMPT_EOF'
Create a new folder in research/remote-devpod and spawn 3 agents at the same time to conduct deep research so that I can use the devpod.sh desktop instance to spawn a remote programming environment on my desktop and then later on connect to that environment from my laptop to continue working in the same workspace.

The 3 agents should research:
1. DevPod architecture and workspace persistence
2. Multi-device connection strategies (desktop to laptop)
3. devpod.sh script creation for remote access setup

Each agent should save findings to separate markdown files in the research/remote-devpod folder.
PROMPT_EOF

echo ""
echo -e "${YELLOW}Test Configuration:${NC}"
echo "  Profile: $PROFILE_NAME"
echo "  Provider: openai"
echo "  Model: gpt-5.1-codex"
echo "  API Key: ${OPENAI_API_KEY:0:12}..."
echo "  Port: 8080"
echo "  Auto-launch Claude Code: true"
echo "  Skip permissions: true"
echo ""
echo -e "${YELLOW}Test Prompt:${NC}"
head -3 "$PROMPT_FILE"
echo "..."
echo ""

# Initialize results file
cat > "$RESULTS_FILE" << EOF
CLASP Tool Calling Test Results
================================
Date: $(date)
Session: $SESSION_NAME
Profile: $PROFILE_NAME

Configuration:
- Provider: openai
- Model: gpt-4o
- API Key: ${OPENAI_API_KEY:0:12}...
- Port: 8080

Test Prompt:
$(cat "$PROMPT_FILE")

---
EOF

# Create tmux session
echo -e "${YELLOW}Starting test in tmux session: $SESSION_NAME${NC}"
tmux new-session -d -s "$SESSION_NAME" -x 200 -y 50

# Launch CLASP with the test profile
# CLASP will:
# 1. Load the profile configuration
# 2. Start the proxy on port 8080
# 3. Auto-launch Claude Code with ANTHROPIC_BASE_URL=http://localhost:8080
# 4. Pass the prompt via -p flag
tmux send-keys -t "$SESSION_NAME" "cd $CLASP_DIR" Enter
tmux send-keys -t "$SESSION_NAME" "echo '=== CLASP Test Started ===' | tee $LOG_FILE" Enter
tmux send-keys -t "$SESSION_NAME" "echo 'Using profile: $PROFILE_NAME' | tee -a $LOG_FILE" Enter
tmux send-keys -t "$SESSION_NAME" "echo 'Launching CLASP + Claude Code with test prompt...' | tee -a $LOG_FILE" Enter

# Use the profile and pass prompt to Claude Code
tmux send-keys -t "$SESSION_NAME" "./bin/clasp -profile $PROFILE_NAME -debug -- -p \"\$(cat $PROMPT_FILE)\" 2>&1 | tee -a $LOG_FILE" Enter

echo ""
echo -e "${GREEN}Test launched!${NC}"
echo ""
echo -e "${BLUE}To monitor the test:${NC}"
echo "  tmux attach -t $SESSION_NAME     # Watch live execution"
echo "  tail -f $LOG_FILE                # Watch CLASP logs"
echo ""
echo -e "${BLUE}To capture results after completion:${NC}"
echo "  tmux capture-pane -t $SESSION_NAME -p >> $RESULTS_FILE"
echo ""
echo -e "${YELLOW}Expected behavior (PASS):${NC}"
echo "  - CLASP proxy starts on port 8080"
echo "  - Claude Code launches and receives prompt"
echo "  - Task tool calls succeed (3 agents spawn concurrently)"
echo "  - No 'Invalid tool parameters' errors in logs"
echo "  - Research files created in research/remote-devpod/"
echo ""
echo -e "${RED}Failure indicators (FAIL):${NC}"
echo "  - 'Invalid tool parameters' in logs"
echo "  - 'missing required field' errors"
echo "  - Task agents fail to spawn"
echo "  - 'strict' mode validation errors"
echo ""
echo -e "${BLUE}To verify success after test completes:${NC}"
echo "  grep -i 'invalid\|error\|required' $LOG_FILE"
echo "  ls -la $SCRIPT_DIR/*.md"
echo ""
echo "To kill session: tmux kill-session -t $SESSION_NAME"
echo "To cleanup profile: rm $PROFILE_DIR/${PROFILE_NAME}.json"

# Optional: Auto-attach to session
if [[ "${1:-}" == "--attach" ]]; then
    echo ""
    echo "Attaching to session..."
    sleep 2
    tmux attach -t "$SESSION_NAME"
fi
