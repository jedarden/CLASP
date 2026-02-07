# CLASP API Types: Claude Code Tools Reference

**Version**: 2.1.34+
**Last Updated**: 2026-02-07
**Bead**: bd-1yb

## Overview

This document provides a complete reference of all Claude Code tools supported by CLASP, including their type definitions, parameter schemas, and usage examples. All tools are defined in `internal/translator/claude_code_tools.go` and transformed to OpenAI-compatible schemas for provider compatibility.

---

## Tool Constants

```go
const (
    ToolRead            = "Read"
    ToolWrite           = "Write"
    ToolEdit            = "Edit"
    ToolGlob            = "Glob"
    ToolGrep            = "Grep"
    ToolBash            = "Bash"
    ToolWebFetch        = "WebFetch"
    ToolWebSearch       = "WebSearch"
    ToolLSP             = "LSP"
    ToolNotebookEdit    = "NotebookEdit"
    ToolTask            = "Task"
    ToolSkill           = "Skill"
    ToolAskUserQuestion = "AskUserQuestion"
    ToolEnterPlanMode   = "EnterPlanMode"
    ToolExitPlanMode    = "ExitPlanMode"
    ToolTaskOutput      = "TaskOutput"
    ToolTaskStop        = "TaskStop"
    ToolTaskCreate      = "TaskCreate"
    ToolTaskGet         = "TaskGet"
    ToolTaskUpdate      = "TaskUpdate"
    ToolTaskList        = "TaskList"
)
```

---

## Core File Operations

### Read Tool

**Description**: Read file contents from the filesystem. Supports text files, images (PNG, JPG), PDFs, and Jupyter notebooks.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "file_path": {
      "type": "string",
      "description": "The absolute path to the file to read"
    },
    "offset": {
      "type": "integer",
      "description": "Line number to start reading from (1-based). Optional."
    },
    "limit": {
      "type": "integer",
      "description": "Maximum number of lines to read. Optional."
    },
    "pages": {
      "type": "string",
      "description": "Page range for PDF files (e.g., '1-5', '3', '10-20'). Maximum 20 pages per request. Optional, only applies to PDF files."
    }
  },
  "required": ["file_path"],
  "additionalProperties": false
}
```

**Examples**:
```javascript
// Read entire file
const content = await Read({ file_path: "/path/to/file.txt" });

// Read specific line range
const excerpt = await Read({
  file_path: "/path/to/file.txt",
  offset: 100,
  limit: 50
});

// Read PDF pages 1-5
const pdfPages = await Read({
  file_path: "/path/to/document.pdf",
  pages: "1-5"
});
```

**Version Added**: 2.0.0
**Version Updated**: 2.1.30 (added `pages` parameter for PDFs)

---

### Write Tool

**Description**: Write content to a file, creating it if it doesn't exist or overwriting if it does. The file must have been read first if it exists.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "file_path": {
      "type": "string",
      "description": "The absolute path to the file to write"
    },
    "content": {
      "type": "string",
      "description": "The content to write to the file"
    }
  },
  "required": ["file_path", "content"],
  "additionalProperties": false
}
```

**Example**:
```javascript
await Write({
  file_path: "/path/to/file.txt",
  content: "Hello, world!"
});
```

---

### Edit Tool

**Description**: Edit a file by replacing exact string matches. The old_string must be unique in the file.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "file_path": {
      "type": "string",
      "description": "The absolute path to the file to edit"
    },
    "old_string": {
      "type": "string",
      "description": "The exact string to find and replace"
    },
    "new_string": {
      "type": "string",
      "description": "The replacement string"
    },
    "replace_all": {
      "type": "boolean",
      "description": "Replace all occurrences instead of just the first. Optional, defaults to false."
    }
  },
  "required": ["file_path", "old_string", "new_string"],
  "additionalProperties": false
}
```

**Example**:
```javascript
// Replace single occurrence
await Edit({
  file_path: "/path/to/file.txt",
  old_string: "foo",
  new_string: "bar"
});

// Replace all occurrences
await Edit({
  file_path: "/path/to/file.txt",
  old_string: "color",
  new_string: "colour",
  replace_all: true
});
```

---

## File Search & Discovery

### Glob Tool

**Description**: Find files matching a glob pattern. Supports patterns like `**/*.js` or `src/**/*.ts`. Returns matching file paths sorted by modification time.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "pattern": {
      "type": "string",
      "description": "The glob pattern to match files against"
    },
    "path": {
      "type": "string",
      "description": "The directory to search in. Optional, defaults to current working directory."
    }
  },
  "required": ["pattern"],
  "additionalProperties": false
}
```

**Example**:
```javascript
// Find all TypeScript files
const tsFiles = await Glob({
  pattern: "**/*.ts",
  path: "/project/src"
});
```

---

### Grep Tool

**Description**: Search for patterns in files using regex. Built on ripgrep for fast searching. Returns matching file paths or content depending on output_mode.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "pattern": {
      "type": "string",
      "description": "The regular expression pattern to search for"
    },
    "path": {
      "type": "string",
      "description": "File or directory to search in. Optional, defaults to current directory."
    },
    "glob": {
      "type": "string",
      "description": "Glob pattern to filter files (e.g., '*.js', '*.{ts,tsx}'). Optional."
    },
    "type": {
      "type": "string",
      "description": "File type to search (e.g., 'js', 'py', 'rust'). Optional."
    },
    "output_mode": {
      "type": "string",
      "enum": ["content", "files_with_matches", "count"],
      "description": "Output mode: 'content' shows matching lines, 'files_with_matches' shows file paths, 'count' shows match counts. Optional, defaults to 'files_with_matches'."
    },
    "-i": {
      "type": "boolean",
      "description": "Case insensitive search. Optional."
    },
    "-n": {
      "type": "boolean",
      "description": "Show line numbers. Optional, defaults to true."
    },
    "-A": {
      "type": "integer",
      "description": "Lines to show after match. Optional."
    },
    "-B": {
      "type": "integer",
      "description": "Lines to show before match. Optional."
    },
    "-C": {
      "type": "integer",
      "description": "Lines to show before and after match. Optional."
    },
    "multiline": {
      "type": "boolean",
      "description": "Enable multiline mode for cross-line patterns. Optional."
    },
    "head_limit": {
      "type": "integer",
      "description": "Limit output to first N entries. Optional."
    },
    "offset": {
      "type": "integer",
      "description": "Skip first N entries before applying head_limit. Optional."
    }
  },
  "required": ["pattern"],
  "additionalProperties": false
}
```

**Example**:
```javascript
// Search for function definitions in Python files
const matches = await Grep({
  pattern: "^def \\w+",
  type: "py",
  output_mode: "content",
  "-n": true
});
```

---

## Command Execution

### Bash Tool

**Description**: Execute bash commands in a persistent shell session. Use for git, npm, docker, and other terminal operations.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "command": {
      "type": "string",
      "description": "The bash command to execute"
    },
    "description": {
      "type": "string",
      "description": "Short description of what the command does (5-10 words). Optional."
    },
    "timeout": {
      "type": "integer",
      "description": "Timeout in milliseconds (max 600000). Optional, defaults to 120000."
    },
    "run_in_background": {
      "type": "boolean",
      "description": "Run command in background. Optional."
    },
    "dangerouslyDisableSandbox": {
      "type": "boolean",
      "description": "Disable sandbox mode. Optional."
    }
  },
  "required": ["command"],
  "additionalProperties": false
}
```

**Example**:
```javascript
// Run command with description
await Bash({
  command: "npm install",
  description: "Install npm dependencies"
});

// Run in background
const taskId = await Bash({
  command: "npm run build",
  run_in_background: true
});
```

**Version Updated**: 2.1.23 (timeout display), 2.0.71 (background support)

---

## Web Operations

### WebSearch Tool

**Description**: Search the web and return results. Provides up-to-date information beyond the model's knowledge cutoff.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "The search query"
    },
    "allowed_domains": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Only include results from these domains. Optional."
    },
    "blocked_domains": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Exclude results from these domains. Optional."
    }
  },
  "required": ["query"],
  "additionalProperties": false
}
```

**Example**:
```javascript
const results = await WebSearch({
  query: "Claude Code 2.1.34 release notes"
});
```

---

### WebFetch Tool

**Description**: Fetch content from a URL and process it with a prompt. Converts HTML to markdown.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "url": {
      "type": "string",
      "description": "The URL to fetch content from"
    },
    "prompt": {
      "type": "string",
      "description": "The prompt to run on the fetched content"
    }
  },
  "required": ["url", "prompt"],
  "additionalProperties": false
}
```

**Example**:
```javascript
const content = await WebFetch({
  url: "https://example.com/article",
  prompt: "Summarize the main points of this article"
});
```

---

## Jupyter Notebooks

### NotebookEdit Tool

**Description**: Edit Jupyter notebook cells. Supports replacing, inserting, or deleting cells in .ipynb files.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "notebook_path": {
      "type": "string",
      "description": "The absolute path to the Jupyter notebook file"
    },
    "new_source": {
      "type": "string",
      "description": "The new source content for the cell"
    },
    "cell_id": {
      "type": "string",
      "description": "The ID of the cell to edit. Optional."
    },
    "cell_type": {
      "type": "string",
      "enum": ["code", "markdown"],
      "description": "The type of cell. Required for insert mode."
    },
    "edit_mode": {
      "type": "string",
      "enum": ["replace", "insert", "delete"],
      "description": "The type of edit. Optional, defaults to replace."
    }
  },
  "required": ["notebook_path", "new_source"],
  "additionalProperties": false
}
```

**Example**:
```javascript
await NotebookEdit({
  notebook_path: "/path/to/notebook.ipynb",
  cell_id: "cell-123",
  new_source: "print('Hello, world!')",
  edit_mode: "replace"
});
```

---

## Language Server Protocol

### LSP Tool

**Description**: Interact with Language Server Protocol servers for code intelligence. Supports operations like go to definition, find references, hover, and call hierarchy.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "operation": {
      "type": "string",
      "enum": [
        "goToDefinition",
        "findReferences",
        "hover",
        "documentSymbol",
        "workspaceSymbol",
        "goToImplementation",
        "prepareCallHierarchy",
        "incomingCalls",
        "outgoingCalls"
      ],
      "description": "The LSP operation to perform"
    },
    "filePath": {
      "type": "string",
      "description": "The file to operate on"
    },
    "line": {
      "type": "integer",
      "description": "Line number (1-based)"
    },
    "character": {
      "type": "integer",
      "description": "Character offset (1-based)"
    }
  },
  "required": ["operation", "filePath", "line", "character"],
  "additionalProperties": false
}
```

**Example**:
```javascript
const definition = await LSP({
  operation: "goToDefinition",
  filePath: "/path/to/file.ts",
  line: 42,
  character: 10
});
```

---

## Agent Orchestration

### Task Tool

**Description**: Launch a specialized agent to handle complex tasks autonomously. Agents have access to various tools and can run in background.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "description": {
      "type": "string",
      "description": "A short (3-5 word) description of the task"
    },
    "prompt": {
      "type": "string",
      "description": "The detailed task for the agent to perform"
    },
    "subagent_type": {
      "type": "string",
      "description": "The type of specialized agent to use (e.g., 'general-purpose', 'Explore', 'Plan', 'Bash')"
    },
    "model": {
      "type": "string",
      "enum": ["sonnet", "opus", "haiku"],
      "description": "Model to use for the agent. Optional."
    },
    "resume": {
      "type": "string",
      "description": "Agent ID to resume from previous execution. Optional."
    },
    "run_in_background": {
      "type": "boolean",
      "description": "Run agent in background. Optional."
    },
    "allowed_tools": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Tools to grant this agent. User will be prompted to approve if not already allowed. Optional."
    },
    "max_turns": {
      "type": "integer",
      "description": "Maximum number of agentic turns before stopping. Optional."
    }
  },
  "required": ["description", "prompt", "subagent_type"],
  "additionalProperties": false
}
```

**Examples**:
```javascript
// Launch with explicit model and turn limit
const result = await Task({
  description: "Analyze security vulnerabilities",
  prompt: "Scan the codebase for common security issues...",
  subagent_type: "Explore",
  model: "sonnet",
  max_turns: 50
});

// Background execution
const taskId = await Task({
  description: "Run test suite",
  prompt: "Execute all tests and report results",
  subagent_type: "Bash",
  run_in_background: true
});

// Resume previous session
const resumed = await Task({
  description: "Continue analysis",
  prompt: "Continue from where we left off",
  subagent_type: "Explore",
  resume: "previous-session-id"
});
```

**Version Updated**: 2.1.30 (added `model`, `max_turns`, `resume` parameters)

---

### TaskOutput Tool

**Description**: Retrieve output from a running or completed background task (shell, agent, or remote session).

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "task_id": {
      "type": "string",
      "description": "The task ID to get output from"
    },
    "block": {
      "type": "boolean",
      "default": true,
      "description": "Whether to wait for task completion"
    },
    "timeout": {
      "type": "number",
      "default": 30000,
      "minimum": 0,
      "maximum": 600000,
      "description": "Max wait time in ms"
    }
  },
  "required": ["task_id", "block", "timeout"],
  "additionalProperties": false
}
```

**Example**:
```javascript
const result = await TaskOutput({
  task_id: "task-123",
  block: true,
  timeout: 60000
});
// Returns: { status: "completed", output: "..." }
```

**Version Added**: 2.0.64

---

### TaskStop Tool

**Description**: Stop a running background task by its ID. Works with background shells, async agents, and remote sessions.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "task_id": {
      "type": "string",
      "description": "The ID of the background task to stop"
    },
    "shell_id": {
      "type": "string",
      "description": "Deprecated: use task_id instead"
    }
  },
  "additionalProperties": false
}
```

**Example**:
```javascript
await TaskStop({ task_id: "task-123" });
```

---

## Task Management (CLI Only)

> **Note**: The following tools are CLI-only and not available in the Agent SDK.

### TaskCreate Tool

**Description**: Create a structured task for tracking progress. Helps organize complex tasks and show progress to users.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "subject": {
      "type": "string",
      "description": "A brief title for the task in imperative form (e.g., 'Fix authentication bug')"
    },
    "description": {
      "type": "string",
      "description": "A detailed description of what needs to be done, including context and acceptance criteria"
    },
    "activeForm": {
      "type": "string",
      "description": "Present continuous form shown in spinner when in_progress (e.g., 'Fixing authentication bug')"
    },
    "metadata": {
      "type": "object",
      "description": "Arbitrary metadata to attach to the task. Optional."
    }
  },
  "required": ["subject", "description"],
  "additionalProperties": false
}
```

---

### TaskGet Tool

**Description**: Retrieve a task by its ID to get full details including description, status, and dependencies.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "taskId": {
      "type": "string",
      "description": "The ID of the task to retrieve"
    }
  },
  "required": ["taskId"],
  "additionalProperties": false
}
```

---

### TaskUpdate Tool

**Description**: Update a task's status, details, or dependencies. Use to mark tasks as in_progress or completed.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "taskId": {
      "type": "string",
      "description": "The ID of the task to update"
    },
    "status": {
      "type": "string",
      "enum": ["pending", "in_progress", "completed"],
      "description": "New status for the task. Optional."
    },
    "subject": {
      "type": "string",
      "description": "New subject for the task. Optional."
    },
    "description": {
      "type": "string",
      "description": "New description for the task. Optional."
    },
    "activeForm": {
      "type": "string",
      "description": "Present continuous form shown in spinner when in_progress. Optional."
    },
    "owner": {
      "type": "string",
      "description": "New owner for the task. Optional."
    },
    "metadata": {
      "type": "object",
      "description": "Metadata keys to merge into the task. Set a key to null to delete it. Optional."
    },
    "addBlocks": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Task IDs that this task blocks. Optional."
    },
    "addBlockedBy": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Task IDs that block this task. Optional."
    }
  },
  "required": ["taskId"],
  "additionalProperties": false
}
```

---

### TaskList Tool

**Description**: List all tasks to see available work, check progress, or find blocked tasks.

**Parameters**:
```json
{
  "type": "object",
  "properties": {},
  "additionalProperties": false
}
```

---

## Interactive Features

### AskUserQuestion Tool

**Description**: Ask the user questions to gather preferences, clarify instructions, or get decisions on implementation choices.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "questions": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "question": {
            "type": "string",
            "description": "The complete question to ask (must end with '?')"
          },
          "header": {
            "type": "string",
            "description": "Short label (max 12 chars)"
          },
          "options": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "label": {"type": "string"},
                "description": {"type": "string"}
              },
              "required": ["label", "description"]
            },
            "description": "Available choices (2-4 options)"
          },
          "multiSelect": {
            "type": "boolean",
            "description": "Allow multiple selections"
          }
        },
        "required": ["question", "header", "options", "multiSelect"]
      },
      "description": "Questions to ask (1-4)"
    },
    "answers": {
      "type": "object",
      "description": "User answers from permission component. Optional."
    },
    "metadata": {
      "type": "object",
      "properties": {
        "source": {
          "type": "string",
          "description": "Optional identifier for the source of this question. Used for analytics."
        }
      },
      "description": "Optional metadata for tracking and analytics purposes."
    }
  },
  "required": ["questions"],
  "additionalProperties": false
}
```

**Example**:
```javascript
const response = await AskUserQuestion({
  questions: [{
    question: "Which deployment strategy should we use?",
    header: "Strategy",
    options: [
      {
        label: "Blue-Green",
        description: "Zero-downtime deployment with full infrastructure duplication"
      },
      {
        label: "Rolling",
        description: "Gradual replacement of instances"
      },
      {
        label: "Canary",
        description: "Deploy to small subset first"
      }
    ],
    multiSelect: false
  }]
});
```

**Version Added**: 2.0.21

---

## Plan Mode

### EnterPlanMode Tool

**Description**: Transition into plan mode to explore the codebase and design an implementation approach for user approval before coding.

**Parameters**:
```json
{
  "type": "object",
  "properties": {},
  "additionalProperties": false
}
```

**Example**:
```javascript
await EnterPlanMode();
```

---

### ExitPlanMode Tool

**Description**: Signal that planning is complete and the plan file is ready for user approval.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "allowedPrompts": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "tool": {
            "type": "string",
            "enum": ["Bash"],
            "description": "The tool this prompt applies to"
          },
          "prompt": {
            "type": "string",
            "description": "Semantic description of the action"
          }
        },
        "required": ["tool", "prompt"]
      },
      "description": "Prompt-based permissions needed to implement the plan. Optional."
    },
    "pushToRemote": {
      "type": "boolean",
      "description": "Whether to push the plan to a remote Claude.ai session. Optional."
    },
    "remoteSessionId": {
      "type": "string",
      "description": "The remote session ID if pushed to remote. Optional."
    },
    "remoteSessionTitle": {
      "type": "string",
      "description": "The remote session title if pushed to remote. Optional."
    },
    "remoteSessionUrl": {
      "type": "string",
      "description": "The remote session URL if pushed to remote. Optional."
    }
  },
  "additionalProperties": false
}
```

**Example**:
```javascript
await ExitPlanMode({
  allowedPrompts: [{
    tool: "Bash",
    prompt: "Run tests"
  }],
  pushToRemote: false
});
```

**Known Issues**: GitHub issue #6109 - System-level ExitPlanMode incorrectly invoked from subagents

---

## Skills

### Skill Tool

**Description**: Execute a skill (slash command) within the conversation. Skills provide specialized capabilities.

**Parameters**:
```json
{
  "type": "object",
  "properties": {
    "skill": {
      "type": "string",
      "description": "The skill name (e.g., 'commit', 'review-pr', 'pdf')"
    },
    "args": {
      "type": "string",
      "description": "Arguments for the skill. Optional."
    }
  },
  "required": ["skill"],
  "additionalProperties": false
}
```

**Example**:
```javascript
await Skill({
  skill: "commit",
  args: "-m 'feat: Add new feature'"
});
```

---

## Type Definitions Reference

### Core Types

All tool definitions use the following core types from `pkg/models/types.go`:

```go
// AnthropicTool represents a tool definition in Anthropic format
type AnthropicTool struct {
    Name        string      `json:"name"`
    Description string      `json:"description,omitempty"`
    InputSchema interface{} `json:"input_schema"`
    Type        string      `json:"type,omitempty"`              // For computer use tools
    CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// OpenAITool represents a tool definition in OpenAI format
type OpenAITool struct {
    Type     string         `json:"type"`
    Function OpenAIFunction `json:"function"`
}

// OpenAIFunction represents a function definition in OpenAI format
type OpenAIFunction struct {
    Name        string      `json:"name"`
    Description string      `json:"description,omitempty"`
    Parameters  interface{} `json:"parameters"`
    Strict      bool        `json:"strict"` // Must be false for optional parameters
}
```

### Tool Detection Function

```go
// IsClaudeCodeTool checks if the tool name is a known Claude Code tool
func IsClaudeCodeTool(toolName string) bool
```

Returns `true` for any of the tool constants listed above.

### Tool Transformation Function

```go
// GetClaudeCodeToolDefinition returns the proper OpenAI-compatible tool definition
func GetClaudeCodeToolDefinition(tool models.AnthropicTool) (name, description string, params interface{})
```

Transforms Anthropic tool definitions to OpenAI-compatible schemas with proper parameter validation.

---

## Version Compatibility Matrix

| Tool | Introduced | Last Updated | Notes |
|------|------------|--------------|-------|
| Read | 2.0.0 | 2.1.30 | Added `pages` parameter for PDFs |
| Write | 2.0.0 | - | Stable |
| Edit | 2.0.0 | - | Stable |
| Glob | 2.0.0 | - | Stable |
| Grep | 2.0.0 | - | Stable |
| Bash | 2.0.0 | 2.1.23 | Background support, timeout display |
| WebSearch | 2.0.0 | - | Stable |
| WebFetch | 2.0.0 | - | Stable |
| LSP | 2.0.0 | - | Stable |
| NotebookEdit | 2.0.0 | - | Stable |
| Task | 2.0.0 | 2.1.30 | Added `model`, `max_turns`, `resume` |
| TaskOutput | 2.0.64 | - | Replaces AgentOutputTool, BashOutputTool |
| TaskStop | 2.0.0 | - | Stable |
| Skill | 2.0.0 | - | Stable |
| AskUserQuestion | 2.0.21 | - | Interactive questions |
| EnterPlanMode | 2.0.0 | - | Stable |
| ExitPlanMode | 2.0.0 | - | Known issue #6109 |
| TaskCreate | 2.1.19+ | - | CLI only |
| TaskGet | 2.1.19+ | - | CLI only |
| TaskUpdate | 2.1.19+ | - | CLI only |
| TaskList | 2.1.19+ | - | CLI only |

---

## Related Documentation

- [OpenAI Chat Completions API](/docs/api-reference/openai-chat-completions.md)
- [Anthropic Messages API](/docs/api-reference/anthropic-messages.md)
- [OpenAI Responses API](/docs/api-reference/openai-responses.md)
- [Tool Call Translation Guide](/docs/translation-guides/tool-calls.md)
- [Streaming Guide](/docs/translation-guides/streaming.md)

---

## Changelog

### 2026-02-07 (bd-1yb)
- Verified all Claude Code 2.1.34 tools are implemented
- Confirmed Task tool has `model`, `max_turns`, `resume` parameters
- Confirmed Read tool has `pages` parameter for PDFs
- Added comprehensive documentation with examples

### Previous Updates
- See [claude-code-2.1.34-tool-analysis.md](/docs/claude-code-2.1.34-tool-analysis.md) for detailed version history
