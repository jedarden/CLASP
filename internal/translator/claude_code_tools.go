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
	ToolTodoWrite       = "TodoWrite"
	ToolAskUserQuestion = "AskUserQuestion"
	ToolEnterPlanMode   = "EnterPlanMode"
	ToolExitPlanMode    = "ExitPlanMode"
	ToolTaskOutput      = "TaskOutput"
	ToolKillShell       = "KillShell"
)

// IsClaudeCodeTool checks if the tool name is a known Claude Code tool.
func IsClaudeCodeTool(toolName string) bool {
	switch toolName {
	case ToolRead, ToolWrite, ToolEdit, ToolGlob, ToolGrep, ToolBash,
		ToolWebFetch, ToolWebSearch, ToolLSP, ToolNotebookEdit,
		ToolTask, ToolSkill, ToolTodoWrite, ToolAskUserQuestion,
		ToolEnterPlanMode, ToolExitPlanMode, ToolTaskOutput, ToolKillShell:
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
	case ToolTodoWrite:
		return transformTodoWriteTool(tool)
	case ToolAskUserQuestion:
		return transformAskUserQuestionTool(tool)
	case ToolEnterPlanMode:
		return transformEnterPlanModeTool(tool)
	case ToolExitPlanMode:
		return transformExitPlanModeTool(tool)
	case ToolTaskOutput:
		return transformTaskOutputTool(tool)
	case ToolKillShell:
		return transformKillShellTool(tool)
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

// transformTodoWriteTool creates an OpenAI-compatible schema for the TodoWrite tool.
func transformTodoWriteTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "TodoWrite",
		"Create and manage a structured task list for tracking progress. Helps organize complex tasks and show progress to users.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"todos": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content": map[string]interface{}{
								"type":        "string",
								"description": "The imperative form describing what needs to be done",
							},
							"status": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"pending", "in_progress", "completed"},
								"description": "The current status of the task",
							},
							"activeForm": map[string]interface{}{
								"type":        "string",
								"description": "The present continuous form shown during execution",
							},
						},
						"required": []string{"content", "status", "activeForm"},
					},
					"description": "The updated todo list",
				},
			},
			"required":             []string{"todos"},
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
			"type":                 "object",
			"properties":           map[string]interface{}{},
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
					"description": "Wait for task completion. Optional, defaults to true.",
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Max wait time in ms. Optional, defaults to 30000.",
				},
			},
			"required":             []string{"task_id"},
			"additionalProperties": false,
		}
}

// transformKillShellTool creates an OpenAI-compatible schema for the KillShell tool.
func transformKillShellTool(tool models.AnthropicTool) (string, string, interface{}) {
	return "KillShell",
		"Kill a running background bash shell by its ID.",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"shell_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the background shell to kill",
				},
			},
			"required":             []string{"shell_id"},
			"additionalProperties": false,
		}
}
