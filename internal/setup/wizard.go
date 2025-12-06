// Package setup provides interactive configuration wizards for first-run experience.
package setup

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/clasp/internal/config"
)

// ConfigFile represents the saved configuration format.
type ConfigFile struct {
	Provider      string            `json:"provider"`
	Model         string            `json:"model,omitempty"`
	APIKey        string            `json:"api_key,omitempty"`
	BaseURL       string            `json:"base_url,omitempty"`
	AzureEndpoint string            `json:"azure_endpoint,omitempty"`
	AzureDeployment string          `json:"azure_deployment,omitempty"`
	ModelAliases  map[string]string `json:"model_aliases,omitempty"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
}

// Wizard handles the interactive setup process.
type Wizard struct {
	reader  *bufio.Reader
	writer  io.Writer
	client  *http.Client
}

// NewWizard creates a new setup wizard.
func NewWizard() *Wizard {
	return &Wizard{
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// NeedsSetup checks if configuration is missing and setup is required.
func NeedsSetup() bool {
	// Check for any provider API key in environment
	if os.Getenv("OPENAI_API_KEY") != "" ||
		os.Getenv("AZURE_API_KEY") != "" ||
		os.Getenv("OPENROUTER_API_KEY") != "" ||
		os.Getenv("ANTHROPIC_API_KEY") != "" ||
		os.Getenv("CUSTOM_API_KEY") != "" {
		return false
	}

	// Check for existing config file
	configPath := GetConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		return false
	}

	return true
}

// GetConfigPath returns the path to the CLASP config file.
func GetConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clasp", "config.json")
}

// GetProfilePath returns the path to a specific profile.
func GetProfilePath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clasp", "profiles", name+".json")
}

// Run executes the interactive setup wizard.
func (w *Wizard) Run() (*config.Config, error) {
	w.printBanner()
	w.println("")
	w.println("Welcome to CLASP! Let's get you set up.")
	w.println("")

	// Step 1: Select provider
	provider, err := w.selectProvider()
	if err != nil {
		return nil, err
	}

	// Step 2: Get API key
	apiKey, err := w.promptAPIKey(provider)
	if err != nil {
		return nil, err
	}

	// Step 3: Get additional config for Azure
	var azureEndpoint, azureDeployment string
	if provider == "azure" {
		azureEndpoint, err = w.promptInput("Azure OpenAI Endpoint", "https://your-resource.openai.azure.com")
		if err != nil {
			return nil, err
		}
		azureDeployment, err = w.promptInput("Azure Deployment Name", "gpt-4")
		if err != nil {
			return nil, err
		}
	}

	// Step 4: Custom base URL
	var baseURL string
	if provider == "custom" {
		baseURL, err = w.promptInput("Custom Base URL", "http://localhost:11434/v1")
		if err != nil {
			return nil, err
		}
	}

	// Step 5: Fetch and select model
	w.println("")
	w.println("Fetching available models...")

	models, err := w.fetchModels(provider, apiKey, baseURL, azureEndpoint)
	if err != nil {
		w.printf("Warning: Could not fetch models: %v\n", err)
		w.println("You can manually specify a model.")
	}

	model, err := w.selectModel(provider, models)
	if err != nil {
		return nil, err
	}

	// Step 6: Save configuration
	w.println("")
	w.println("Saving configuration...")

	configFile := &ConfigFile{
		Provider:        provider,
		Model:           model,
		APIKey:          apiKey,
		BaseURL:         baseURL,
		AzureEndpoint:   azureEndpoint,
		AzureDeployment: azureDeployment,
		CreatedAt:       time.Now().Format(time.RFC3339),
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}

	if err := w.saveConfig(configFile); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	// Set environment variables for current session
	w.setEnvVars(configFile)

	// Build and return config
	cfg := config.DefaultConfig()
	cfg.Provider = config.ProviderType(provider)
	cfg.DefaultModel = model

	switch provider {
	case "openai":
		cfg.OpenAIAPIKey = apiKey
	case "azure":
		cfg.AzureAPIKey = apiKey
		cfg.AzureEndpoint = azureEndpoint
		cfg.AzureDeploymentName = azureDeployment
	case "openrouter":
		cfg.OpenRouterAPIKey = apiKey
	case "anthropic":
		cfg.AnthropicAPIKey = apiKey
	case "custom":
		cfg.CustomAPIKey = apiKey
		cfg.CustomBaseURL = baseURL
	}

	w.println("")
	w.println("Configuration saved successfully!")
	w.printf("Config file: %s\n", GetConfigPath())
	w.println("")
	w.println("CLASP is now ready to use. Run 'clasp' to start the proxy.")
	w.println("")

	return cfg, nil
}

func (w *Wizard) printBanner() {
	w.println(`
╔═══════════════════════════════════════════════════════════════╗
║        CLASP - Claude Language Agent Super Proxy              ║
║                    Interactive Setup                          ║
╚═══════════════════════════════════════════════════════════════╝`)
}

func (w *Wizard) selectProvider() (string, error) {
	w.println("Select your LLM provider:")
	w.println("")
	w.println("  1) OpenAI        - GPT-4, GPT-4o, GPT-4o-mini, etc.")
	w.println("  2) Azure OpenAI  - Azure-hosted OpenAI models")
	w.println("  3) OpenRouter    - 200+ models from multiple providers")
	w.println("  4) Anthropic     - Direct passthrough (no translation)")
	w.println("  5) Custom        - Ollama, vLLM, LM Studio, etc.")
	w.println("")

	for {
		choice, err := w.promptInput("Enter choice [1-5]", "1")
		if err != nil {
			return "", err
		}

		switch choice {
		case "1", "openai":
			return "openai", nil
		case "2", "azure":
			return "azure", nil
		case "3", "openrouter":
			return "openrouter", nil
		case "4", "anthropic":
			return "anthropic", nil
		case "5", "custom":
			return "custom", nil
		default:
			w.println("Invalid choice. Please enter 1-5.")
		}
	}
}

func (w *Wizard) promptAPIKey(provider string) (string, error) {
	var prompt string
	switch provider {
	case "openai":
		prompt = "OpenAI API Key (sk-...)"
	case "azure":
		prompt = "Azure API Key"
	case "openrouter":
		prompt = "OpenRouter API Key (sk-or-...)"
	case "anthropic":
		prompt = "Anthropic API Key (sk-ant-...)"
	case "custom":
		prompt = "API Key (optional, press Enter to skip)"
	default:
		prompt = "API Key"
	}

	w.println("")
	return w.promptSecure(prompt, "")
}

func (w *Wizard) promptInput(prompt, defaultVal string) (string, error) {
	if defaultVal != "" {
		w.printf("%s [%s]: ", prompt, defaultVal)
	} else {
		w.printf("%s: ", prompt)
	}

	input, err := w.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal, nil
	}
	return input, nil
}

func (w *Wizard) promptSecure(prompt, defaultVal string) (string, error) {
	// For now, just use regular input since we can't easily hide input in Go
	// without external dependencies. In production, you'd use terminal.ReadPassword
	return w.promptInput(prompt, defaultVal)
}

func (w *Wizard) println(s string) {
	fmt.Fprintln(w.writer, s)
}

func (w *Wizard) printf(format string, args ...interface{}) {
	fmt.Fprintf(w.writer, format, args...)
}

// OpenAI Models Response
type openAIModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// OpenRouter Models Response
type openRouterModelsResponse struct {
	Data []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
}

func (w *Wizard) fetchModels(provider, apiKey, baseURL, azureEndpoint string) ([]string, error) {
	var url string
	var headers map[string]string

	switch provider {
	case "openai":
		url = "https://api.openai.com/v1/models"
		headers = map[string]string{"Authorization": "Bearer " + apiKey}
	case "openrouter":
		url = "https://openrouter.ai/api/v1/models"
		headers = map[string]string{"Authorization": "Bearer " + apiKey}
	case "custom":
		if baseURL == "" {
			return nil, fmt.Errorf("no base URL provided")
		}
		url = strings.TrimSuffix(baseURL, "/") + "/models"
		if apiKey != "" {
			headers = map[string]string{"Authorization": "Bearer " + apiKey}
		}
	case "anthropic":
		// Return Anthropic's known models
		return []string{
			"claude-3-5-sonnet-20241022",
			"claude-3-5-haiku-20241022",
			"claude-3-opus-20240229",
			"claude-3-sonnet-20240229",
			"claude-3-haiku-20240307",
		}, nil
	case "azure":
		// Azure doesn't have a models endpoint, return common deployments
		return []string{
			"gpt-4",
			"gpt-4o",
			"gpt-4o-mini",
			"gpt-4-turbo",
			"gpt-35-turbo",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var models []string

	switch provider {
	case "openai", "custom":
		var openAIResp openAIModelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
			return nil, err
		}
		for _, m := range openAIResp.Data {
			// Filter for chat models
			if isChatModel(m.ID) {
				models = append(models, m.ID)
			}
		}
	case "openrouter":
		var orResp openRouterModelsResponse
		if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
			return nil, err
		}
		for _, m := range orResp.Data {
			models = append(models, m.ID)
		}
	}

	sort.Strings(models)
	return models, nil
}

// isChatModel filters for likely chat/completion models.
func isChatModel(id string) bool {
	// Include GPT models
	if strings.HasPrefix(id, "gpt-") {
		return true
	}
	// Include o1 models
	if strings.HasPrefix(id, "o1") {
		return true
	}
	// Include chatgpt models
	if strings.HasPrefix(id, "chatgpt") {
		return true
	}
	return false
}

func (w *Wizard) selectModel(provider string, models []string) (string, error) {
	if len(models) == 0 {
		return w.promptInput("Enter model name", getDefaultModel(provider))
	}

	w.println("")
	w.println("Available models:")
	w.println("")

	// Show top models first (prioritize popular ones)
	prioritized := prioritizeModels(models, provider)

	// Display up to 15 models
	displayCount := len(prioritized)
	if displayCount > 15 {
		displayCount = 15
	}

	for i := 0; i < displayCount; i++ {
		w.printf("  %2d) %s\n", i+1, prioritized[i])
	}

	if len(prioritized) > 15 {
		w.printf("\n  ... and %d more. Enter a number or type model name.\n", len(prioritized)-15)
	}

	w.println("")

	defaultModel := getDefaultModel(provider)
	for {
		choice, err := w.promptInput("Select model", defaultModel)
		if err != nil {
			return "", err
		}

		// Check if it's a number
		if num, err := strconv.Atoi(choice); err == nil {
			if num >= 1 && num <= len(prioritized) {
				return prioritized[num-1], nil
			}
			w.printf("Invalid number. Enter 1-%d or a model name.\n", len(prioritized))
			continue
		}

		// It's a model name, validate if possible
		if len(models) > 0 {
			for _, m := range models {
				if strings.EqualFold(m, choice) {
					return m, nil
				}
			}
			// Not in list, but allow it anyway
			w.printf("Note: '%s' not in fetched list. Using anyway.\n", choice)
		}

		return choice, nil
	}
}

func getDefaultModel(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4o"
	case "azure":
		return "gpt-4"
	case "openrouter":
		return "anthropic/claude-3.5-sonnet"
	case "anthropic":
		return "claude-3-5-sonnet-20241022"
	case "custom":
		return "llama3.1"
	default:
		return "gpt-4o"
	}
}

func prioritizeModels(models []string, provider string) []string {
	// Priority models by provider
	var priority []string
	switch provider {
	case "openai":
		priority = []string{
			"gpt-4o", "gpt-4o-mini", "o1-preview", "o1-mini",
			"gpt-4-turbo", "gpt-4", "gpt-3.5-turbo",
		}
	case "openrouter":
		priority = []string{
			"anthropic/claude-3.5-sonnet",
			"anthropic/claude-3.5-haiku",
			"openai/gpt-4o",
			"openai/o1-preview",
			"google/gemini-pro-1.5",
			"meta-llama/llama-3.1-405b-instruct",
		}
	}

	// Build ordered list with priorities first
	result := make([]string, 0, len(models))
	seen := make(map[string]bool)

	// Add priority models first
	for _, p := range priority {
		for _, m := range models {
			if strings.EqualFold(m, p) && !seen[m] {
				result = append(result, m)
				seen[m] = true
				break
			}
		}
	}

	// Add remaining models
	for _, m := range models {
		if !seen[m] {
			result = append(result, m)
		}
	}

	return result
}

func (w *Wizard) saveConfig(cfg *ConfigFile) error {
	configPath := GetConfigPath()
	configDir := filepath.Dir(configPath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	// Marshal with indentation
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Write with restrictive permissions (API keys inside)
	return os.WriteFile(configPath, data, 0600)
}

func (w *Wizard) setEnvVars(cfg *ConfigFile) {
	os.Setenv("PROVIDER", cfg.Provider)

	switch cfg.Provider {
	case "openai":
		os.Setenv("OPENAI_API_KEY", cfg.APIKey)
	case "azure":
		os.Setenv("AZURE_API_KEY", cfg.APIKey)
		os.Setenv("AZURE_OPENAI_ENDPOINT", cfg.AzureEndpoint)
		os.Setenv("AZURE_DEPLOYMENT_NAME", cfg.AzureDeployment)
	case "openrouter":
		os.Setenv("OPENROUTER_API_KEY", cfg.APIKey)
	case "anthropic":
		os.Setenv("ANTHROPIC_API_KEY", cfg.APIKey)
	case "custom":
		os.Setenv("CUSTOM_API_KEY", cfg.APIKey)
		os.Setenv("CUSTOM_BASE_URL", cfg.BaseURL)
	}

	if cfg.Model != "" {
		os.Setenv("CLASP_MODEL", cfg.Model)
	}
}

// LoadConfig loads configuration from the saved config file.
func LoadConfig() (*ConfigFile, error) {
	configPath := GetConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg ConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ApplyConfigToEnv applies saved config to environment variables.
func ApplyConfigToEnv() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	os.Setenv("PROVIDER", cfg.Provider)

	switch cfg.Provider {
	case "openai":
		os.Setenv("OPENAI_API_KEY", cfg.APIKey)
	case "azure":
		os.Setenv("AZURE_API_KEY", cfg.APIKey)
		os.Setenv("AZURE_OPENAI_ENDPOINT", cfg.AzureEndpoint)
		os.Setenv("AZURE_DEPLOYMENT_NAME", cfg.AzureDeployment)
	case "openrouter":
		os.Setenv("OPENROUTER_API_KEY", cfg.APIKey)
	case "anthropic":
		os.Setenv("ANTHROPIC_API_KEY", cfg.APIKey)
	case "custom":
		if cfg.APIKey != "" {
			os.Setenv("CUSTOM_API_KEY", cfg.APIKey)
		}
		os.Setenv("CUSTOM_BASE_URL", cfg.BaseURL)
	}

	if cfg.Model != "" {
		os.Setenv("CLASP_MODEL", cfg.Model)
	}

	return nil
}

// FetchModelsPublic is a public wrapper for fetchModels.
func (w *Wizard) FetchModelsPublic(provider, apiKey, baseURL, azureEndpoint string) ([]string, error) {
	return w.fetchModels(provider, apiKey, baseURL, azureEndpoint)
}
