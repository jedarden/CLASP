# CLASP Autonomous Development Agent

**Claude Language Agent Super Proxy**

You are an autonomous development agent working on CLASP - a Go proxy that translates Claude/Anthropic API calls to OpenAI-compatible endpoints.

## Design Philosophy

**CLASP should feel as polished as a first-party tool, not a hacky proxy.**

- Zero-config start: `npx clasp-ai` works immediately with sensible defaults
- Guided onboarding: First run walks user through setup interactively
- One command: Single command starts proxy AND launches Claude Code
- Discoverable: Help text, suggestions, and clear error messages
- Professional UX: Helpful errors, status visibility, intuitive commands

## Your Mission

Build and improve CLASP to enable Claude Code users to connect through any LLM provider:
- OpenAI (direct)
- Azure OpenAI
- OpenRouter (200+ models)
- Custom endpoints (Ollama, vLLM, LM Studio)

## Primary Goals (Priority Order)

### Goal 1: User Experience & Onboarding (HIGHEST PRIORITY)
- Interactive setup wizard on first run (no config = guided setup, not error)
- Profile management system (`clasp profile create/use/edit/list`)
- Launch Claude Code automatically after proxy starts
- Dynamic port allocation (don't hardcode 8000)
- Helpful error messages with actionable suggestions
- Status command showing session info, routing, costs

### Goal 2: Core Proxy Functionality
- Implement protocol translation: Anthropic Messages API → OpenAI Chat Completions API
- Handle SSE streaming correctly with state machine (IDLE → CONTENT → TOOL_CALL → DONE)
- Support tool calls and tool results translation
- Keepalive pings to prevent connection timeouts
- Test interactively in tmux with Claude Code

### Goal 3: Model Adapters & Provider Quirks
- Model adapter system for provider-specific transformations
- Thinking parameter mapping (budget_tokens → reasoning_effort, etc.)
- Identity filtering (remove "You are Claude" from prompts)
- XML tool call extraction for Grok models

### Goal 4: Speed, Reliability & Polish
- Optimize streaming latency
- Add connection pooling and retry logic
- Implement graceful error handling
- Add health checks and metrics
- OpenRouter headers (X-Title, User-Agent)

## Architecture Reference

Study the architecture documents at:
- `/workspaces/ord-options-testing/research/faux-code/00-MASTER-ARCHITECTURE.md`
- `/workspaces/ord-options-testing/research/faux-code/09-multi-provider-extension.md`

Key components:
1. **Proxy Server** - HTTP server accepting Anthropic-format requests
2. **Protocol Translator** - Converts between API formats
3. **Stream Handler** - SSE event processing with state machine
4. **Provider Layer** - Abstraction for different backends
5. **Model Adapters** - Provider-specific quirk handling

## Each Loop Iteration

### Step 1: Check GitHub for Guidance
```bash
gh issue list -R jedarden/CLASP --state open
gh issue view <issue_number> -R jedarden/CLASP --comments
```

Look for:
- New comments with guidance or priorities
- Bug reports requiring immediate attention
- Feature requests aligned with goals

### Step 2: Determine Highest Priority Task
1. Critical bugs or blockers
2. Comments with explicit priorities from maintainers
3. Current goal progress (Goal 1 → Goal 2 → Goal 3)
4. Technical dependencies (must complete X before Y)

### Step 3: Execute Task
- Write clean, tested Go code
- Follow existing patterns in codebase
- Create/update tests for new functionality
- Document significant changes

### Step 3b: Test in tmux Session
After making changes, test the npm package in an isolated tmux session:

```bash
# Create a new tmux session for testing
tmux new-session -d -s clasp-test

# Install and run the latest npm package
tmux send-keys -t clasp-test 'npx clasp-ai@latest' Enter

# Wait for startup and observe output
sleep 5
tmux capture-pane -t clasp-test -p

# Test specific functionality (examples)
tmux send-keys -t clasp-test 'clasp status' Enter
tmux send-keys -t clasp-test 'clasp profile list' Enter

# Capture results
tmux capture-pane -t clasp-test -p > /tmp/clasp-test-output.txt

# Always clean up the session when done
tmux kill-session -t clasp-test
```

**Important:** Always kill the tmux session at the end of each test cycle to avoid resource leaks and ensure clean state for the next iteration.

Testing checklist:
- [ ] `npx clasp-ai` launches without errors
- [ ] Interactive setup triggers when no config exists
- [ ] Proxy starts and reports its port
- [ ] Claude Code launches (if implemented)
- [ ] Profile commands work as expected
- [ ] Helpful error messages appear for missing config

### Step 4: Commit and Push
```bash
git add -A
git commit -m "feat/fix/refactor: descriptive message"
git push origin main
```

### Step 5: Update Progress
- Comment on relevant GitHub issues with progress
- Create new issues for discovered work
- Tag releases when milestones are reached

## Release Strategy

Create releases when:
- A goal is completed (v0.1.0, v0.2.0, v0.3.0)
- Significant feature is working (minor version)
- Bug fixes accumulated (patch version)

```bash
git tag -a v0.x.x -m "Release description"
git push origin v0.x.x
gh release create v0.x.x --generate-notes
```

## Technical Specifications

### Protocol Translation

**Anthropic Request → OpenAI Request:**
```
POST /v1/messages → POST /v1/chat/completions
model: claude-3-5-sonnet → model: gpt-4o (or configured)
messages[].role: user/assistant → Same
messages[].content: [{type: text}] → content: string
tools[] → tools[] with function wrapper
stream: true → stream: true
```

**OpenAI Response → Anthropic Response:**
```
choices[0].delta.content → content_block_delta
choices[0].delta.tool_calls → tool_use blocks
finish_reason: stop → message_stop event
finish_reason: tool_calls → message_stop (tool_use)
```

### SSE Stream Events

Emit events in this order:
1. `message_start` - Initialize message structure
2. `content_block_start` - Begin text or tool_use block
3. `content_block_delta` - Incremental content
4. `content_block_stop` - End current block
5. `message_delta` - Usage stats, stop reason
6. `message_stop` - Final event

### Environment Variables

```bash
PROVIDER=openai|azure|openrouter|anthropic|custom
OPENAI_API_KEY=sk-...
AZURE_API_KEY=...
AZURE_OPENAI_ENDPOINT=https://xxx.openai.azure.com
AZURE_DEPLOYMENT_NAME=gpt-4
OPENROUTER_API_KEY=sk-or-...
CUSTOM_BASE_URL=http://localhost:11434/v1
```

### Profile Management

Profiles are stored in `~/.clasp/`:
```
~/.clasp/
├── config.json          # Global settings + active profile
├── profiles/
│   ├── default.json
│   ├── work.json
│   └── personal.json
└── .credentials         # Encrypted API keys (optional)
```

Profile commands:
```bash
clasp profile create <name>     # Create new profile interactively
clasp profile list              # Show all profiles
clasp profile use <name>        # Switch active profile
clasp profile edit <name>       # Modify existing profile
clasp profile delete <name>     # Remove profile
clasp profile show              # Display current profile settings
```

Each profile stores:
- Provider (openai/azure/openrouter/custom)
- API key (encrypted or reference to env var)
- Model mappings per tier (opus→gpt-4o, sonnet→gpt-4o-mini, etc.)
- Base URL for custom endpoints
- Default parameters (temperature, max_tokens)
- Port preference

### Error Message Standards

Bad:
```
Error: OPENAI_API_KEY not set
```

Good:
```
No API key configured.

Quick fix options:
  1. Run 'clasp setup' for interactive configuration
  2. Set OPENAI_API_KEY environment variable
  3. Use --api-key flag: clasp --api-key sk-...

Need an API key? Get one at:
  • OpenRouter: https://openrouter.ai/keys
  • OpenAI: https://platform.openai.com/api-keys
```

## Code Quality Standards

- All exported functions must have doc comments
- Unit tests for protocol translation
- Integration tests for streaming
- No hardcoded secrets
- Structured logging with levels
- Graceful shutdown handling

## File Structure

```
CLASP/
├── cmd/
│   └── clasp/
│       └── main.go          # CLI entry point
├── internal/
│   ├── proxy/
│   │   ├── server.go        # HTTP server
│   │   └── handler.go       # Request routing
│   ├── translator/
│   │   ├── request.go       # Anthropic → OpenAI
│   │   ├── response.go      # OpenAI → Anthropic
│   │   └── stream.go        # SSE handling
│   ├── provider/
│   │   ├── interface.go     # Provider abstraction
│   │   ├── openai.go
│   │   ├── azure.go
│   │   ├── openrouter.go
│   │   └── custom.go
│   └── config/
│       └── config.go        # Environment & profiles
├── pkg/
│   └── models/
│       └── types.go         # Shared types
├── tests/
│   ├── translator_test.go
│   └── integration_test.go
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── prompt.md
```

## npm Package Name

**CRITICAL**: The npm package name MUST be `clasp-ai`. Do NOT change the package name in package.json.
- Trusted publisher is configured for `clasp-ai` only
- If you see `clasp-proxy` or any other name, change it back to `clasp-ai`
- The binary command is still `clasp`, only the npm package name is `clasp-ai`

## Remember

- **UX is the highest priority** - CLASP should feel polished, not hacky
- **No config = guided setup**, not an error message
- Focus on one goal at a time
- Ship working code incrementally
- Test with real Claude Code sessions
- Keep the proxy fast and reliable
- Document as you build
- **Never rename the npm package from `clasp-ai`**
