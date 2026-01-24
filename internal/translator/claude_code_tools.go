// Package translator handles protocol translation between Anthropic and OpenAI formats.
package translator

import (
	"github.com/jedarden/clasp/pkg/models"
)

// Claude Code tool name constants for detection
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

// IsClaudeCodeTool checks if the tool name is a known Claude Code tool.
func IsClaudeCodeTool(toolName string) bool {
	switch toolName {
	case ToolRead, ToolWrite, ToolEdit, ToolGlob, ToolGrep, ToolBash,
		ToolWebFetch, ToolWebSearch, ToolLSP, ToolNotebookEdit,
		ToolTask, ToolSkill, ToolAskUserQuestion,
		ToolEnterPlanMode, ToolExitPlanMode, ToolTaskOutput, ToolTaskStop,
		ToolTaskCreate, ToolTaskGet, ToolTaskUpdate, ToolTaskList:
		return true
	default:
		return false
	}
}

// GetClaudeCodeToolDefinition returns the proper OpenAI-compatible tool definition
// for a Claude Code tool. This ensures all providers can understand the tool schema.
func GetClaudeCodeToolDefinition(tool models.AnthropicTool) (name, description string, params interface{}) {
	switch tool.Name {
	case ToolRead:
		return transformReadTool(tool)
	case ToolWrite:
		return transformWriteTool(tool)
	case ToolEdit:
		return transformEditTool(tool)
	case ToolGlob:
		return transformGlobTool(tool)
	case ToolGrep:
		return transformGrepTool(tool)
	case ToolBash:
		return transformBashTool(tool)
	case ToolWebFetch:
		return transformWebFetchTool(tool)
	case ToolWebSearch:
		return transformWebSearchTool(tool)
	case ToolLSP:
		return transformLSPTool(tool)
	case ToolNotebookEdit:
		return transformNotebookEditTool(tool)
	case ToolTask:
		return transformTaskTool(tool)
	case ToolSkill:
		return transformSkillTool(tool)
	case ToolAskUserQuestion:
		return transformAskUserQuestionTool(tool)
	case ToolEnterPlanMode:
		return transformEnterPlanModeTool(tool)
	case ToolExitPlanMode:
		return transformExitPlanModeTool(tool)
	case ToolTaskOutput:
		return transformTaskOutputTool(tool)
	case ToolTaskStop:
		return transformTaskStopTool(tool)
	case ToolTaskCreate:
		return transformTaskCreateTool(tool)
	case ToolTaskGet:
		return transformTaskGetTool(tool)
	case ToolTaskUpdate:
		return transformTaskUpdateTool(tool)
	case ToolTaskList:
		return transformTaskListTool(tool)
	default:
		// Pass through unchanged
		return tool.Name, tool.Description, tool.InputSchema
	}
}

// transformReadTool creates an OpenAI-compatible schema for the Read tool.
func transformReadTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "Read",
		"Read file contents from the filesystem. Supports reading text files, images (PNG, JPG), PDFs, and Jupyter notebooks. Returns file content with line numbers.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "The absolute path to the file to read",
				},
				"offset": map[string]interface{}{
					"type":        "integer",
					"description": "Line number to start reading from (1-based). Optional.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of lines to read. Optional.",
				},
			},
			"required":             []string{"file_path"},
			"additionalProperties": false,
		}
}

// transformWriteTool creates an OpenAI-compatible schema for the Write tool.
func transformWriteTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "Write",
		"Write content to a file, creating it if it doesn't exist or overwriting if it does. The file must have been read first if it exists.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "The absolute path to the file to write",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to write to the file",
				},
			},
			"required":             []string{"file_path", "content"},
			"additionalProperties": false,
		}
}

// transformEditTool creates an OpenAI-compatible schema for the Edit tool.
func transformEditTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "Edit",
		"Edit a file by replacing exact string matches. The old_string must be unique in the file.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "The absolute path to the file to edit",
				},
				"old_string": map[string]interface{}{
					"type":        "string",
					"description": "The exact string to find and replace",
				},
				"new_string": map[string]interface{}{
					"type":        "string",
					"description": "The replacement string",
				},
				"replace_all": map[string]interface{}{
					"type":        "boolean",
					"description": "Replace all occurrences instead of just the first. Optional, defaults to false.",
				},
			},
			"required":             []string{"file_path", "old_string", "new_string"},
			"additionalProperties": false,
		}
}

// transformGlobTool creates an OpenAI-compatible schema for the Glob tool.
func transformGlobTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "Glob",
		"Find files matching a glob pattern. Supports patterns like '**/*.js' or 'src/**/*.ts'. Returns matching file paths sorted by modification time.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "The glob pattern to match files against",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The directory to search in. Optional, defaults to current working directory.",
				},
			},
			"required":             []string{"pattern"},
			"additionalProperties": false,
		}
}

// transformGrepTool creates an OpenAI-compatible schema for the Grep tool.
func transformGrepTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "Grep",
		"Search for patterns in files using regex. Built on ripgrep for fast searching. Returns matching file paths or content depending on output_mode.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "The regular expression pattern to search for",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File or directory to search in. Optional, defaults to current directory.",
				},
				"glob": map[string]interface{}{
					"type":        "string",
					"description": "Glob pattern to filter files (e.g., '*.js', '*.{ts,tsx}'). Optional.",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "File type to search (e.g., 'js', 'py', 'rust'). Optional.",
				},
				"output_mode": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"content", "files_with_matches", "count"},
					"description": "Output mode: 'content' shows matching lines, 'files_with_matches' shows file paths, 'count' shows match counts. Optional, defaults to 'files_with_matches'.",
				},
				"-i": map[string]interface{}{
					"type":        "boolean",
					"description": "Case insensitive search. Optional.",
				},
				"-n": map[string]interface{}{
					"type":        "boolean",
					"description": "Show line numbers. Optional, defaults to true.",
				},
				"-A": map[string]interface{}{
					"type":        "integer",
					"description": "Lines to show after match. Optional.",
				},
				"-B": map[string]interface{}{
					"type":        "integer",
					"description": "Lines to show before match. Optional.",
				},
				"-C": map[string]interface{}{
					"type":        "integer",
					"description": "Lines to show before and after match. Optional.",
				},
				"multiline": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable multiline mode for cross-line patterns. Optional.",
				},
				"head_limit": map[string]interface{}{
					"type":        "integer",
					"description": "Limit output to first N entries. Optional.",
				},
				"offset": map[string]interface{}{
					"type":        "integer",
					"description": "Skip first N entries before applying head_limit. Optional.",
				},
			},
			"required":             []string{"pattern"},
			"additionalProperties": false,
		}
}

// transformBashTool creates an OpenAI-compatible schema for the Bash tool.
func transformBashTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "Bash",
		"Execute bash commands in a persistent shell session. Use for git, npm, docker, and other terminal operations. Commands timeout after 2 minutes by default.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The bash command to execute",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Short description of what the command does (5-10 words). Optional.",
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Timeout in milliseconds (max 600000). Optional, defaults to 120000.",
				},
				"run_in_background": map[string]interface{}{
					"type":        "boolean",
					"description": "Run command in background. Optional.",
				},
				"dangerouslyDisableSandbox": map[string]interface{}{
					"type":        "boolean",
					"description": "Disable sandbox mode. Optional.",
				},
			},
			"required":             []string{"command"},
			"additionalProperties": false,
		}
}

// transformWebFetchTool creates an OpenAI-compatible schema for the WebFetch tool.
func transformWebFetchTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "WebFetch",
		"Fetch content from a URL and process it with a prompt. Converts HTML to markdown. Results may be summarized if content is large.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "The URL to fetch content from",
				},
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "The prompt to run on the fetched content",
				},
			},
			"required":             []string{"url", "prompt"},
			"additionalProperties": false,
		}
}

// transformWebSearchTool creates an OpenAI-compatible schema for the WebSearch tool.
func transformWebSearchTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "WebSearch",
		"Search the web and return results. Provides up-to-date information beyond the model's knowledge cutoff.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query",
				},
				"allowed_domains": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Only include results from these domains. Optional.",
				},
				"blocked_domains": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Exclude results from these domains. Optional.",
				},
			},
			"required":             []string{"query"},
			"additionalProperties": false,
		}
}

// transformLSPTool creates an OpenAI-compatible schema for the LSP tool.
func transformLSPTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "LSP",
		"Interact with Language Server Protocol servers for code intelligence. Supports operations like go to definition, find references, hover, and call hierarchy.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"goToDefinition", "findReferences", "hover", "documentSymbol", "workspaceSymbol", "goToImplementation", "prepareCallHierarchy", "incomingCalls", "outgoingCalls"},
					"description": "The LSP operation to perform",
				},
				"filePath": map[string]interface{}{
					"type":        "string",
					"description": "The file to operate on",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Line number (1-based)",
				},
				"character": map[string]interface{}{
					"type":        "integer",
					"description": "Character offset (1-based)",
				},
			},
			"required":             []string{"operation", "filePath", "line", "character"},
			"additionalProperties": false,
		}
}

// transformNotebookEditTool creates an OpenAI-compatible schema for the NotebookEdit tool.
func transformNotebookEditTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "NotebookEdit",
		"Edit Jupyter notebook cells. Supports replacing, inserting, or deleting cells in .ipynb files.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"notebook_path": map[string]interface{}{
					"type":        "string",
					"description": "The absolute path to the Jupyter notebook file",
				},
				"new_source": map[string]interface{}{
					"type":        "string",
					"description": "The new source content for the cell",
				},
				"cell_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the cell to edit. Optional.",
				},
				"cell_type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"code", "markdown"},
					"description": "The type of cell. Required for insert mode.",
				},
				"edit_mode": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"replace", "insert", "delete"},
					"description": "The type of edit. Optional, defaults to replace.",
				},
			},
			"required":             []string{"notebook_path", "new_source"},
			"additionalProperties": false,
		}
}

// transformTaskTool creates an OpenAI-compatible schema for the Task tool.
func transformTaskTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "Task",
		"Launch a specialized agent to handle complex tasks autonomously. Agents have access to various tools and can run in background.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": map[string]interface{}{
					"type":        "string",
					"description": "A short (3-5 word) description of the task",
				},
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "The detailed task for the agent to perform",
				},
				"subagent_type": map[string]interface{}{
					"type":        "string",
					"description": "The type of specialized agent to use",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"sonnet", "opus", "haiku"},
					"description": "Model to use for the agent. Optional.",
				},
				"resume": map[string]interface{}{
					"type":        "string",
					"description": "Agent ID to resume from previous execution. Optional.",
				},
				"run_in_background": map[string]interface{}{
					"type":        "boolean",
					"description": "Run agent in background. Optional.",
				},
				"allowed_tools": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Tools to grant this agent. User will be prompted to approve if not already allowed. Optional.",
				},
				"max_turns": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of agentic turns before stopping. Optional.",
				},
			},
			"required":             []string{"description", "prompt", "subagent_type"},
			"additionalProperties": false,
		}
}

// transformSkillTool creates an OpenAI-compatible schema for the Skill tool.
func transformSkillTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "Skill",
		"Execute a skill (slash command) within the conversation. Skills provide specialized capabilities.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"skill": map[string]interface{}{
					"type":        "string",
					"description": "The skill name (e.g., 'commit', 'review-pr', 'pdf')",
				},
				"args": map[string]interface{}{
					"type":        "string",
					"description": "Arguments for the skill. Optional.",
				},
			},
			"required":             []string{"skill"},
			"additionalProperties": false,
		}
}

// transformAskUserQuestionTool creates an OpenAI-compatible schema for the AskUserQuestion tool.
func transformAskUserQuestionTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "AskUserQuestion",
		"Ask the user questions to gather preferences, clarify instructions, or get decisions on implementation choices.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"questions": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"question": map[string]interface{}{
								"type":        "string",
								"description": "The complete question to ask",
							},
							"header": map[string]interface{}{
								"type":        "string",
								"description": "Short label (max 12 chars)",
							},
							"options": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"label":       map[string]interface{}{"type": "string"},
										"description": map[string]interface{}{"type": "string"},
									},
									"required": []string{"label", "description"},
								},
								"description": "Available choices (2-4 options)",
							},
							"multiSelect": map[string]interface{}{
								"type":        "boolean",
								"description": "Allow multiple selections",
							},
						},
						"required": []string{"question", "header", "options", "multiSelect"},
					},
					"description": "Questions to ask (1-4)",
				},
				"answers": map[string]interface{}{
					"type":        "object",
					"description": "User answers from permission component. Optional.",
				},
				"metadata": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"source": map[string]interface{}{
							"type":        "string",
							"description": "Optional identifier for the source of this question. Used for analytics.",
						},
					},
					"description": "Optional metadata for tracking and analytics purposes.",
				},
			},
			"required":             []string{"questions"},
			"additionalProperties": false,
		}
}

// transformEnterPlanModeTool creates an OpenAI-compatible schema for the EnterPlanMode tool.
func transformEnterPlanModeTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "EnterPlanMode",
		"Transition into plan mode to explore the codebase and design an implementation approach for user approval before coding.",
		map[string]interface{}{
			"type":                 "object",
			"properties":           map[string]interface{}{},
			"additionalProperties": false,
		}
}

// transformExitPlanModeTool creates an OpenAI-compatible schema for the ExitPlanMode tool.
func transformExitPlanModeTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "ExitPlanMode",
		"Signal that planning is complete and the plan file is ready for user approval.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"allowedPrompts": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"tool": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"Bash"},
								"description": "The tool this prompt applies to",
							},
							"prompt": map[string]interface{}{
								"type":        "string",
								"description": "Semantic description of the action",
							},
						},
						"required": []string{"tool", "prompt"},
					},
					"description": "Prompt-based permissions needed to implement the plan. Optional.",
				},
				"pushToRemote": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to push the plan to a remote Claude.ai session. Optional.",
				},
				"remoteSessionId": map[string]interface{}{
					"type":        "string",
					"description": "The remote session ID if pushed to remote. Optional.",
				},
				"remoteSessionTitle": map[string]interface{}{
					"type":        "string",
					"description": "The remote session title if pushed to remote. Optional.",
				},
				"remoteSessionUrl": map[string]interface{}{
					"type":        "string",
					"description": "The remote session URL if pushed to remote. Optional.",
				},
			},
			"additionalProperties": false,
		}
}

// transformTaskOutputTool creates an OpenAI-compatible schema for the TaskOutput tool.
func transformTaskOutputTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "TaskOutput",
		"Retrieve output from a running or completed background task (shell, agent, or remote session).",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id": map[string]interface{}{
					"type":        "string",
					"description": "The task ID to get output from",
				},
				"block": map[string]interface{}{
					"type":        "boolean",
					"default":     true,
					"description": "Whether to wait for task completion",
				},
				"timeout": map[string]interface{}{
					"type":        "number",
					"default":     30000,
					"minimum":     0,
					"maximum":     600000,
					"description": "Max wait time in ms",
				},
			},
			"required":             []string{"task_id", "block", "timeout"},
			"additionalProperties": false,
		}
}

// transformTaskStopTool creates an OpenAI-compatible schema for the TaskStop tool.
func transformTaskStopTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "TaskStop",
		"Stop a running background task by its ID. Works with background shells, async agents, and remote sessions.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the background task to stop",
				},
				"shell_id": map[string]interface{}{
					"type":        "string",
					"description": "Deprecated: use task_id instead",
				},
			},
			"additionalProperties": false,
		}
}

// transformTaskCreateTool creates an OpenAI-compatible schema for the TaskCreate tool.
func transformTaskCreateTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "TaskCreate",
		"Create a structured task for tracking progress. Helps organize complex tasks and show progress to users.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"subject": map[string]interface{}{
					"type":        "string",
					"description": "A brief title for the task in imperative form (e.g., 'Fix authentication bug')",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "A detailed description of what needs to be done, including context and acceptance criteria",
				},
				"activeForm": map[string]interface{}{
					"type":        "string",
					"description": "Present continuous form shown in spinner when in_progress (e.g., 'Fixing authentication bug')",
				},
				"metadata": map[string]interface{}{
					"type":        "object",
					"description": "Arbitrary metadata to attach to the task. Optional.",
				},
			},
			"required":             []string{"subject", "description"},
			"additionalProperties": false,
		}
}

// transformTaskGetTool creates an OpenAI-compatible schema for the TaskGet tool.
func transformTaskGetTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "TaskGet",
		"Retrieve a task by its ID to get full details including description, status, and dependencies.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"taskId": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the task to retrieve",
				},
			},
			"required":             []string{"taskId"},
			"additionalProperties": false,
		}
}

// transformTaskUpdateTool creates an OpenAI-compatible schema for the TaskUpdate tool.
func transformTaskUpdateTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "TaskUpdate",
		"Update a task's status, details, or dependencies. Use to mark tasks as in_progress or completed.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"taskId": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the task to update",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"pending", "in_progress", "completed"},
					"description": "New status for the task. Optional.",
				},
				"subject": map[string]interface{}{
					"type":        "string",
					"description": "New subject for the task. Optional.",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "New description for the task. Optional.",
				},
				"activeForm": map[string]interface{}{
					"type":        "string",
					"description": "Present continuous form shown in spinner when in_progress. Optional.",
				},
				"owner": map[string]interface{}{
					"type":        "string",
					"description": "New owner for the task. Optional.",
				},
				"metadata": map[string]interface{}{
					"type":        "object",
					"description": "Metadata keys to merge into the task. Set a key to null to delete it. Optional.",
				},
				"addBlocks": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Task IDs that this task blocks. Optional.",
				},
				"addBlockedBy": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Task IDs that block this task. Optional.",
				},
			},
			"required":             []string{"taskId"},
			"additionalProperties": false,
		}
}

// transformTaskListTool creates an OpenAI-compatible schema for the TaskList tool.
func transformTaskListTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "TaskList",
		"List all tasks to see available work, check progress, or find blocked tasks.",
		map[string]interface{}{
			"type":                 "object",
			"properties":           map[string]interface{}{},
			"additionalProperties": false,
		}
}
