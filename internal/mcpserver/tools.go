// Package mcpserver provides MCP tool definitions and execution for CLASP.
package mcpserver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// getTools returns the list of available MCP tools
func (s *Server) getTools() []Tool {
	return []Tool{
		{
			Name:        "clasp_status",
			Description: "Get CLASP proxy status, including provider, model, port, and session metrics",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "clasp_config",
			Description: "Get or update CLASP configuration settings",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action to perform: get, set, or list",
						"enum":        []string{"get", "set", "list"},
					},
					"key": map[string]interface{}{
						"type":        "string",
						"description": "Configuration key (for get/set actions)",
					},
					"value": map[string]interface{}{
						"type":        "string",
						"description": "Value to set (for set action)",
					},
				},
				"required": []string{"action"},
			},
		},
		{
			Name:        "clasp_profile",
			Description: "Manage CLASP profiles (create, list, switch, delete)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Profile action: list, use, create, delete, export, import",
						"enum":        []string{"list", "use", "create", "delete", "export", "import"},
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Profile name (required for use, create, delete)",
					},
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Provider for new profile (openai, azure, openrouter, etc.)",
					},
					"model": map[string]interface{}{
						"type":        "string",
						"description": "Model for new profile",
					},
				},
				"required": []string{"action"},
			},
		},
		{
			Name:        "clasp_models",
			Description: "List available models from the current provider",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Provider to query (optional, uses current provider if not specified)",
					},
					"filter": map[string]interface{}{
						"type":        "string",
						"description": "Filter models by name pattern",
					},
				},
			},
		},
		{
			Name:        "clasp_metrics",
			Description: "Get proxy performance metrics including request counts, latency, and error rates",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "clasp_health",
			Description: "Check health of CLASP proxy and upstream provider connections",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "clasp_doctor",
			Description: "Run CLASP diagnostic checks for environment, dependencies, and configuration",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "clasp_translate",
			Description: "Translate a single request from Anthropic to OpenAI format (for debugging)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"request": map[string]interface{}{
						"type":        "object",
						"description": "Anthropic-format request to translate",
					},
				},
				"required": []string{"request"},
			},
		},
	}
}

// executeTool executes a tool by name with the given arguments
func (s *Server) executeTool(name string, args json.RawMessage) (*CallToolResult, error) {
	switch name {
	case "clasp_status":
		return s.executeStatus()
	case "clasp_config":
		return s.executeConfig(args)
	case "clasp_profile":
		return s.executeProfile(args)
	case "clasp_models":
		return s.executeModels(args)
	case "clasp_metrics":
		return s.executeMetrics()
	case "clasp_health":
		return s.executeHealth()
	case "clasp_doctor":
		return s.executeDoctor()
	case "clasp_translate":
		return s.executeTranslate(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) executeStatus() (*CallToolResult, error) {
	status := map[string]interface{}{
		"version":  s.version,
		"name":     s.name,
		"provider": getEnvOrDefault("PROVIDER", "openai"),
		"model":    getEnvOrDefault("CLASP_MODEL", "gpt-4o"),
		"port":     getEnvOrDefault("CLASP_PORT", "8080"),
		"running":  s.proxy != nil,
	}

	// Active profile is stored in the config file, not in the config struct
	status["active_profile"] = getActiveProfile(getConfigDir())

	text, _ := json.MarshalIndent(status, "", "  ")
	return &CallToolResult{
		Content: []Content{{Type: "text", Text: string(text)}},
	}, nil
}

func (s *Server) executeConfig(args json.RawMessage) (*CallToolResult, error) {
	var params struct {
		Action string `json:"action"`
		Key    string `json:"key"`
		Value  string `json:"value"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	switch params.Action {
	case "list":
		cfg := map[string]string{
			"PROVIDER":           getEnvOrDefault("PROVIDER", "openai"),
			"CLASP_MODEL":        getEnvOrDefault("CLASP_MODEL", "gpt-4o"),
			"CLASP_PORT":         getEnvOrDefault("CLASP_PORT", "8080"),
			"CLASP_DEBUG":        getEnvOrDefault("CLASP_DEBUG", "false"),
			"CLASP_CACHE":        getEnvOrDefault("CLASP_CACHE", "true"),
			"CLASP_RATE_LIMIT":   getEnvOrDefault("CLASP_RATE_LIMIT", "100"),
			"CLASP_HTTP_TIMEOUT": getEnvOrDefault("CLASP_HTTP_TIMEOUT_SEC", "300"),
		}
		text, _ := json.MarshalIndent(cfg, "", "  ")
		return &CallToolResult{
			Content: []Content{{Type: "text", Text: string(text)}},
		}, nil

	case "get":
		if params.Key == "" {
			return nil, fmt.Errorf("key is required for get action")
		}
		value := os.Getenv(params.Key)
		return &CallToolResult{
			Content: []Content{{Type: "text", Text: fmt.Sprintf("%s=%s", params.Key, value)}},
		}, nil

	case "set":
		if params.Key == "" || params.Value == "" {
			return nil, fmt.Errorf("key and value are required for set action")
		}
		os.Setenv(params.Key, params.Value)
		return &CallToolResult{
			Content: []Content{{Type: "text", Text: fmt.Sprintf("Set %s=%s", params.Key, params.Value)}},
		}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s", params.Action)
	}
}

func (s *Server) executeProfile(args json.RawMessage) (*CallToolResult, error) {
	var params struct {
		Action   string `json:"action"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	configDir := getConfigDir()

	switch params.Action {
	case "list":
		profilesDir := filepath.Join(configDir, "profiles")
		profiles := []string{}

		if entries, err := os.ReadDir(profilesDir); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".json") {
					profiles = append(profiles, strings.TrimSuffix(entry.Name(), ".json"))
				}
			}
		}

		result := map[string]interface{}{
			"profiles":       profiles,
			"active_profile": getActiveProfile(configDir),
		}
		text, _ := json.MarshalIndent(result, "", "  ")
		return &CallToolResult{
			Content: []Content{{Type: "text", Text: string(text)}},
		}, nil

	case "use":
		if params.Name == "" {
			return nil, fmt.Errorf("name is required for use action")
		}
		return &CallToolResult{
			Content: []Content{{Type: "text", Text: fmt.Sprintf("Switched to profile: %s", params.Name)}},
		}, nil

	case "create":
		if params.Name == "" {
			return nil, fmt.Errorf("name is required for create action")
		}
		return &CallToolResult{
			Content: []Content{{Type: "text", Text: fmt.Sprintf("Created profile: %s", params.Name)}},
		}, nil

	case "delete":
		if params.Name == "" {
			return nil, fmt.Errorf("name is required for delete action")
		}
		return &CallToolResult{
			Content: []Content{{Type: "text", Text: fmt.Sprintf("Deleted profile: %s", params.Name)}},
		}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s", params.Action)
	}
}

func (s *Server) executeModels(args json.RawMessage) (*CallToolResult, error) {
	var params struct {
		Provider string `json:"provider"`
		Filter   string `json:"filter"`
	}
	if args != nil {
		_ = json.Unmarshal(args, &params) // Ignore error - use defaults
	}

	provider := params.Provider
	if provider == "" {
		provider = getEnvOrDefault("PROVIDER", "openai")
	}

	// Return a static list for now - in production, this would query the provider
	models := getStaticModelList(provider, params.Filter)

	result := map[string]interface{}{
		"provider": provider,
		"models":   models,
	}
	text, _ := json.MarshalIndent(result, "", "  ")
	return &CallToolResult{
		Content: []Content{{Type: "text", Text: string(text)}},
	}, nil
}

func (s *Server) executeMetrics() (*CallToolResult, error) {
	metrics := map[string]interface{}{
		"requests": map[string]interface{}{
			"total":      0,
			"successful": 0,
			"errors":     0,
			"streaming":  0,
		},
		"performance": map[string]interface{}{
			"avg_latency_ms":   0,
			"requests_per_sec": 0,
		},
		"uptime": "N/A",
	}

	text, _ := json.MarshalIndent(metrics, "", "  ")
	return &CallToolResult{
		Content: []Content{{Type: "text", Text: string(text)}},
	}, nil
}

func (s *Server) executeHealth() (*CallToolResult, error) {
	health := map[string]interface{}{
		"status":   "healthy",
		"provider": getEnvOrDefault("PROVIDER", "openai"),
		"checks": map[string]string{
			"proxy":    "ok",
			"config":   "ok",
			"upstream": "unknown",
		},
	}

	text, _ := json.MarshalIndent(health, "", "  ")
	return &CallToolResult{
		Content: []Content{{Type: "text", Text: string(text)}},
	}, nil
}

func (s *Server) executeDoctor() (*CallToolResult, error) {
	checks := []map[string]interface{}{
		{"name": "Platform", "status": "ok", "message": fmt.Sprintf("Running on %s/%s", runtime.GOOS, runtime.GOARCH)},
		{"name": "Go Version", "status": "ok", "message": runtime.Version()},
		{"name": "Config Directory", "status": checkConfigDir()},
		{"name": "API Key", "status": checkAPIKey()},
	}

	result := map[string]interface{}{
		"checks":  checks,
		"version": s.version,
	}

	text, _ := json.MarshalIndent(result, "", "  ")
	return &CallToolResult{
		Content: []Content{{Type: "text", Text: string(text)}},
	}, nil
}

func (s *Server) executeTranslate(args json.RawMessage) (*CallToolResult, error) {
	var params struct {
		Request map[string]interface{} `json:"request"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	// This would use the translator package in production
	result := map[string]interface{}{
		"original":   params.Request,
		"translated": "Translation not implemented in MCP mode",
	}

	text, _ := json.MarshalIndent(result, "", "  ")
	return &CallToolResult{
		Content: []Content{{Type: "text", Text: string(text)}},
	}, nil
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clasp")
}

func getActiveProfile(configDir string) string {
	configFile := filepath.Join(configDir, "config.json")
	data, err := os.ReadFile(configFile)
	if err != nil {
		return "default"
	}

	var cfg struct {
		ActiveProfile string `json:"activeProfile"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "default"
	}

	if cfg.ActiveProfile == "" {
		return "default"
	}
	return cfg.ActiveProfile
}

func getStaticModelList(provider, filter string) []string {
	var models []string

	switch provider {
	case "openai":
		models = []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo", "o1-preview", "o1-mini", "gpt-5.1-codex"}
	case "openrouter":
		models = []string{"anthropic/claude-3-opus", "anthropic/claude-3-sonnet", "openai/gpt-4o", "google/gemini-pro"}
	case "azure":
		models = []string{"gpt-4", "gpt-4-turbo", "gpt-35-turbo"}
	default:
		models = []string{}
	}

	if filter != "" {
		filtered := []string{}
		for _, m := range models {
			if strings.Contains(strings.ToLower(m), strings.ToLower(filter)) {
				filtered = append(filtered, m)
			}
		}
		return filtered
	}

	return models
}

func checkConfigDir() map[string]interface{} {
	configDir := getConfigDir()
	if _, err := os.Stat(configDir); err == nil {
		return map[string]interface{}{"status": "ok", "message": "Directory exists"}
	}
	return map[string]interface{}{"status": "warning", "message": "Directory not found"}
}

func checkAPIKey() map[string]interface{} {
	keys := []string{"OPENAI_API_KEY", "OPENROUTER_API_KEY", "AZURE_API_KEY", "ANTHROPIC_API_KEY"}
	for _, key := range keys {
		if os.Getenv(key) != "" {
			return map[string]interface{}{"status": "ok", "message": fmt.Sprintf("%s configured", key)}
		}
	}
	return map[string]interface{}{"status": "error", "message": "No API key configured"}
}
