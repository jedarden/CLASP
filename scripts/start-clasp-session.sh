#!/bin/bash
# CLASP Tmux Session Starter
# Creates a tmux session using phonetic alphabet naming and starts the launcher

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Phonetic alphabet for tmux sessions (same as start.sh)
PHONETIC_NAMES=("alpha" "bravo" "charlie" "delta" "echo" "foxtrot" "golf" "hotel" "india" "juliet" "kilo" "lima" "mike" "november" "oscar" "papa" "quebec" "romeo" "sierra" "tango" "uniform" "victor" "whiskey" "xray" "yankee" "zulu")

# Configuration
WORKSPACE="/workspaces/ord-options-testing"
CLASP_DIR="$WORKSPACE/CLASP"
LAUNCHER_SCRIPT="$CLASP_DIR/scripts/launcher.sh"

# Function to find next available tmux session name
find_tmux_session() {
    for name in "${PHONETIC_NAMES[@]}"; do
        if ! tmux has-session -t "$name" 2>/dev/null; then
            echo "$name"
            return 0
        fi
    done
    # If all phonetic names are taken, use a timestamp
    echo "clasp-$(date +%s)"
    return 0
}

# Print banner
echo -e "${MAGENTA}${BOLD}"
echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║             CLASP - Claude Language Agent Super Proxy            ║"
echo "║                      Tmux Session Starter                        ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Check if launcher script exists
if [ ! -x "$LAUNCHER_SCRIPT" ]; then
    echo -e "${RED}Error: Launcher script not found or not executable at $LAUNCHER_SCRIPT${NC}"
    exit 1
fi

# Ensure .clasp directory exists for logs
mkdir -p "$WORKSPACE/.clasp/logs"

# Find available session name
SESSION_NAME=$(find_tmux_session)

echo -e "${BLUE}Creating tmux session:${NC} $SESSION_NAME"
echo -e "${BLUE}Working directory:${NC} $CLASP_DIR"
echo ""

# Create tmux session
if tmux new-session -d -s "$SESSION_NAME" -c "$CLASP_DIR" "$LAUNCHER_SCRIPT"; then
    echo -e "${GREEN}${BOLD}✓ CLASP autonomous agent started!${NC}"
    echo ""
    echo -e "${CYAN}Session Details:${NC}"
    echo -e "  Name: ${BOLD}$SESSION_NAME${NC}"
    echo -e "  Launcher: $LAUNCHER_SCRIPT"
    echo -e "  Logs: $WORKSPACE/.clasp/logs/"
    echo ""
    echo -e "${YELLOW}Commands:${NC}"
    echo -e "  Attach:   ${WHITE}tmux attach -t $SESSION_NAME${NC}"
    echo -e "  Detach:   ${WHITE}Ctrl+B, D${NC} (while attached)"
    echo -e "  Kill:     ${WHITE}tmux kill-session -t $SESSION_NAME${NC}"
    echo -e "  List:     ${WHITE}tmux ls${NC}"
    echo ""

    # Ask if user wants to attach
    read -p "$(echo -e ${CYAN}Attach to session now? [Y/n]: ${NC})" -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]] || [[ -z $REPLY ]]; then
        tmux attach -t "$SESSION_NAME"
    else
        echo -e "${GREEN}Session running in background.${NC}"
        echo -e "${YELLOW}Run 'tmux attach -t $SESSION_NAME' to connect.${NC}"
    fi
else
    echo -e "${RED}Failed to create tmux session${NC}"
    exit 1
fi
