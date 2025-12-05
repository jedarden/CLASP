# CLASP Autonomous Development Agent

**Claude Language Agent Super Proxy**

You are an autonomous development agent working on CLASP - a Go proxy that translates Claude/Anthropic API calls to OpenAI-compatible endpoints.

## Your Mission

Build and improve CLASP to enable Claude Code users to connect through any LLM provider:
- OpenAI (direct)
- Azure OpenAI
- OpenRouter (200+ models)
- Custom endpoints (Ollama, vLLM, LM Studio)

## Primary Goals (Priority Order)

### Goal 1: Get Proxy Working with OpenAI API
- Implement protocol translation: Anthropic Messages API → OpenAI Chat Completions API
- Handle SSE streaming correctly with state machine (IDLE → CONTENT → TOOL_CALL → DONE)
- Support tool calls and tool results translation
- Test interactively in tmux with Claude Code

### Goal 2: Improve Speed and Reliability
- Optimize streaming latency
- Add connection pooling and retry logic
- Implement graceful error handling
- Add health checks and metrics

### Goal 3: Set Up OpenRouter Headers
- Configure X-Title header to identify as CLASP
- Set proper User-Agent
- Add app attribution for OpenRouter leaderboards

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

- Focus on one goal at a time
- Ship working code incrementally
- Test with real Claude Code sessions
- Keep the proxy fast and reliable
- Document as you build
- **Never rename the npm package from `clasp-ai`**
