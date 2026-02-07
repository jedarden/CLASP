# Claude Code 2.1.34 Tool Call Analysis

**Date**: 2026-02-07
**Bead ID**: bd-1u7
**Analyzing**: Claude Code CLI v2.1.34 and recent releases (2.1.20-2.1.34)

## Executive Summary

Claude Code 2.1.34 is a **minor bugfix release** with **no new tool calls** introduced in this specific version. However, the recent 2.1.x series (particularly 2.1.20-2.1.33) introduced significant new tools and changes to existing tools that CLASP needs to support.

**Key Findings:**
- **No new tools in v2.1.34** - only bugfixes
- **Major new tools in 2.1.20-2.1.33**: Task orchestration system, memory features, agent teams
- **Significant parameter changes** to Task tool (max_turns, model, resume)
- **New hook events**: TeammateIdle, TaskCompleted
- **Deprecated features**: npm installation (warned in 2.1.15)

---

## v2.1.34 Specific Changes (Feb 2025)

### What's Changed
1. **Fixed crash** when agent teams setting changed between renders
2. **Fixed security bug** where commands excluded from sandboxing could bypass Bash ask permission rule when `autoAllowBashIfSandboxed` was enabled

**Impact on CLASP**: No tool changes. Security fix only.

---

## New Tools Introduced in 2.1.x Series

### 1. **TaskOutput Tool** (v2.0.64)

Retrieves output from running or completed background tasks (agents, shells, remote sessions).

**Parameters:**
```typescript
{
  task_id: string;           // ID of task to retrieve output from
  block: boolean;            // Wait for completion (default: true)
  timeout: number;           // Max wait time in ms (default: 30000)
}
```

**Response:**
```typescript
{
  status: string;            // "running", "completed", "failed", "cancelled"
  output?: string;           // Task output (if completed)
  error?: string;            // Error message (if failed)
}
```

**Example Usage:**
```javascript
// Start a background task
const taskId = await Task({ /* ... */ });

// Later, retrieve the output
const result = await TaskOutput({
  task_id: taskId,
  block: true,
  timeout: 60000
});
```

**CLASP Support Requirement**: **HIGH PRIORITY** - Critical for background agent workflows

---

### 2. **ExitPlanMode Tool** (v2.0.0+)

Signals completion of plan mode and requests user approval to proceed with implementation.

**Parameters:**
```typescript
{
  allowedPrompts?: string[];  // Optional: Permission categories for implementation
  pushToRemote?: boolean;     // Optional: Push plan to remote session
  remoteSessionId?: string;   // Optional: Remote session identifier
}
```

**Known Issues:**
- GitHub issue #6109: System-level ExitPlanMode incorrectly invoked from subagents
- Auto-approval bypasses plan mode restrictions in some scenarios

**CLASP Support Requirement**: **HIGH PRIORITY** - Core to plan mode workflow

---

### 3. **AskUserQuestion Tool** (v2.0.21+)

Interactive tool for asking users questions during execution.

**Parameters:**
```typescript
{
  questions: Array<{
    question: string;         // Question text (must end with "?")
    header: string;           // Short chip/tag (max 12 chars)
    options: Array<{
      label: string;          // Short option label (1-5 words)
      description: string;    // Detailed explanation
    }>;
    multiSelect: boolean;     // Allow multiple selections
  }>;
}
```

**Features:**
- Auto-submit single-select questions on last option
- Support for "Other" free-text input
- Multi-select with checkbox-style selection

**Example Usage:**
```javascript
const response = await AskUserQuestion({
  questions: [{
    question: "Which deployment strategy should we use?",
    header: "Strategy",
    options: [
      { label: "Blue-Green", description: "Zero-downtime deployment with full infrastructure duplication" },
      { label: "Rolling", description: "Gradual replacement of instances" },
      { label: "Canary", description: "Deploy to small subset first" }
    ],
    multiSelect: false
  }]
});
```

**CLASP Support Requirement**: **HIGH PRIORITY** - Interactive agent workflows

---

## Significant Parameter Changes to Existing Tools

### Task Tool (Enhanced in v2.1.30)

**New Parameters:**
```typescript
{
  description: string;        // Required: Task description
  subagent_type: string;      // 'general-purpose', 'Explore', 'Plan', 'Bash', etc.
  model?: string;             // NEW: 'sonnet', 'opus', 'haiku' (default: inherits from parent)
  max_turns?: number;         // NEW: Maximum agent turns (default: 9007199254740991)
  resume?: string;            // NEW: Resume previous agent by ID
  run_in_background?: boolean; // Launch in background (returns task_id)
}
```

**Changes from v2.0.x:**
1. **model parameter** - Explicitly choose model for subagent (v2.1.0+)
2. **max_turns parameter** - Limit agent iterations (v2.1.30+)
3. **resume parameter** - Continue previous sessions (v2.1.0+)
4. **run_in_background** - Async execution with task_id (v2.0.60+)
5. **Token count, tool uses, duration metrics** added to results (v2.1.30)

**Example Usage:**
```javascript
// Launch with explicit model and turn limit
const result = await Task({
  description: "Analyze codebase for security vulnerabilities",
  subagent_type: "Explore",
  model: "sonnet",
  max_turns: 50
});

// Background execution
const taskId = await Task({
  description: "Run full test suite",
  subagent_type: "Bash",
  run_in_background: true
});

// Resume previous session
const resumed = await Task({
  description: "Continue security analysis",
  subagent_type: "Explore",
  resume: "previous-session-id"
});
```

**CLASP Support Requirement**: **CRITICAL** - Core agent spawning mechanism

---

### Read Tool (Enhanced in v2.1.30)

**New Parameters:**
```typescript
{
  file_path: string;          // Required: Absolute path
  offset?: number;            // Start reading from line number
  limit?: number;             // Maximum lines to read (default: 2000)
  pages?: string;             // NEW: PDF page range (e.g., "1-5", "3", "10-20")
}
```

**Changes:**
- **pages parameter** for PDFs - Support reading specific page ranges (v2.1.30)
- Large PDFs (>10 pages) return lightweight reference instead of full content when @-mentioned
- Maximum 20 pages per request

**Example Usage:**
```javascript
// Read PDF pages 1-5
const content = await Read({
  file_path: "/path/to/document.pdf",
  pages: "1-5"
});

// Read single page
const page10 = await Read({
  file_path: "/path/to/document.pdf",
  pages: "10"
});
```

**CLASP Support Requirement**: **MEDIUM** - PDF handling optimization

---

### Bash Tool (Enhanced in v2.1.30)

**New Parameter Behavior:**
```typescript
{
  command: string;            // Required: Command to execute
  timeout?: number;           // Max wait time in ms
  description?: string;       // NEW: Descriptive text for progress messages
  run_in_background?: boolean; // Run as background job
}
```

**Changes:**
- **Progress messages** based on last 5 lines of output (v1.0.48)
- **Timeout duration** now displayed alongside elapsed time (v2.1.23)
- **Background job support** with task_id tracking (v2.0.71)

**CLASP Support Requirement**: **LOW** - UX improvements only

---

## New Hook Events

### v2.1.33 New Hook Events

1. **TeammateIdle** - Triggered when multi-agent teammate goes idle
2. **TaskCompleted** - Triggered when background task completes

**Hook Input Schema:**
```typescript
// TeammateIdle
{
  agent_type: string;         // Type of agent that went idle
  agent_id: string;           // Agent identifier
  idle_duration: number;      // Time since last activity
}

// TaskCompleted
{
  task_id: string;            // Completed task ID
  status: "completed" | "failed" | "cancelled";
  output?: string;            // Task output
  error?: string;             // Error if failed
}
```

**CLASP Support Requirement**: **MEDIUM** - Multi-agent workflows

---

## Deprecated Tools / Features

### 1. **npm Installation** (Deprecated v2.1.15)

**Status:** Deprecated with warning
**Replacement:** Native installers (Homebrew, native binaries)
**Action:** Users should run `claude install` or use native installers

### 2. **Job/CronJob Resources** (Prohibited in ArgoCD)

**Status:** Prohibited for GitOps deployments
**Reason:** Non-idempotent, causes ArgoCD sync issues
**Replacement:** Idempotent Deployments with restart-on-config-change

### 3. **Output Styles** (Deprecated v2.0.30, Un-deprecated v2.0.32)

**Status:** Deprecated then un-deprecated based on community feedback
**Current:** Still supported
**Recommendations:** Use system prompts, CLAUDE.md, or plugins instead

---

## Tools CLASP Must Support

### Critical Priority (Core Functionality)

1. **Task** - Agent spawning with model/resume/max_turns parameters
2. **TaskOutput** - Retrieve background task output
3. **ExitPlanMode** - Plan mode completion signaling
4. **AskUserQuestion** - Interactive user prompts
5. **Read** - File reading with PDF page support
6. **Edit** - File editing
7. **Write** - File writing
8. **Bash** - Command execution
9. **Glob** - File pattern matching
10. **Grep** - Content search
11. **TodoWrite** - Task tracking

### High Priority (Important Workflows)

1. **NotebookEdit** - Jupyter notebook cell editing
2. **Skill** - Skill invocation
3. **ExitPlanMode** - Plan workflow completion
4. **TaskStop** - Stop running tasks
5. **WebSearch** - Web search capability

### Medium Priority (Enhanced Features)

1. **Hook events** - TeammateIdle, TaskCompleted
2. **Memory operations** - Automatic memory recording/recall (v2.1.32)
3. **Agent team features** - Multi-agent collaboration (v2.1.32, experimental)
4. **Agent restrictions** - `Task(agent_type)` syntax (v2.1.33)

### Low Priority (Nice to Have)

1. **Progress indicators** - Enhanced UI feedback
2. **External editor support** - Ctrl+G integration
3. **Session management** - Fork, rewind, teleport features

---

## Version-Specific Tool Availability

| Tool | Introduced | Notes |
|------|------------|-------|
| TaskOutput | 2.0.64 | Replaces AgentOutputTool, BashOutputTool |
| ExitPlanMode | 2.0.0 | Core to plan mode |
| AskUserQuestion | 2.0.21 | Interactive questions |
| Task (enhanced) | 2.1.30+ | model, max_turns, resume params |
| Read (PDF pages) | 2.1.30 | pages parameter |
| Task orchestration | 2.1.19+ | TaskCreate, TaskUpdate, TaskGet, TaskList (CLI only) |
| Memory | 2.1.32 | Automatic recording/recall |
| Agent teams | 2.1.32 | Experimental, requires opt-in |

---

## Recommendations for CLASP

### 1. Immediate Actions (v2.1.34 Compatibility)

- **No breaking changes** in v2.1.34
- Ensure **Task tool** supports: `model`, `max_turns`, `resume` parameters
- Implement **TaskOutput tool** for background task retrieval
- Add **pages parameter** to Read tool for PDFs

### 2. Short-term (2.1.30+ Features)

- **AskUserQuestion** tool with multi-select and "Other" option
- **ExitPlanMode** with remote session support
- Hook event handlers for **TeammateIdle**, **TaskCompleted**

### 3. Long-term (Advanced Features)

- **Memory system** integration (automatic memory recording/recall)
- **Agent teams** support (experimental feature)
- **Task orchestration** tools (TaskCreate, TaskUpdate, TaskGet, TaskList)

### 4. Security Considerations

- Monitor ExitPlanMode auto-approval issues (GitHub #6109)
- Ensure sandbox exclusion commands don't bypass permission checks
- Validate agent team permissions when restricting sub-agent spawning

---

## Additional Resources

- **Official Changelog**: https://code.claude.com/docs/en/changelog
- **GitHub Releases**: https://github.com/anthropics/claude-code/releases
- **Issue Tracker**: https://github.com/anthropics/claude-code/issues
- **Agent SDK Docs**: https://platform.claude.com/docs/en/agent-sdk
- **Plugin System**: https://code.claude.com/docs/en/plugins

---

## Sources

- [Claude Code Changelog](https://code.claude.com/docs/en/changelog)
- [GitHub Releases: anthropics/claude-code](https://github.com/anthropics/claude-code/releases)
- [Web Search: Claude Code 2.1.34 Release Notes](https://claude-world.com/articles/claude-code-2134-release/)
- [Issue #6109: ExitPlanMode Incorrectly Invoked](https://github.com/anthropics/claude-code/issues/6109)
- [Issue #9846: AskUserQuestion in Skills](https://github.com/anthropics/claude-code/issues/9846)
- [Issue #10346: Missing AskUserQuestion Docs](https://github.com/anthropics/claude-code/issues/10346)
