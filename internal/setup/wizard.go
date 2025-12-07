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
	Provider         string            `json:"provider"`
	Model            string            `json:"model,omitempty"`
	APIKey           string            `json:"api_key,omitempty"`
	BaseURL          string            `json:"base_url,omitempty"`
	AzureEndpoint    string            `json:"azure_endpoint,omitempty"`
	AzureDeployment  string            `json:"azure_deployment,omitempty"`
	ModelAliases     map[string]string `json:"model_aliases,omitempty"`
	CreatedAt        string            `json:"created_at"`
	UpdatedAt        string            `json:"updated_at"`
	// Claude Code settings
	ClaudeCodeConfig *ClaudeCodeConfig `json:"claude_code,omitempty"`
}

// ClaudeCodeConfig represents Claude Code CLI configuration.
type ClaudeCodeConfig struct {
	// SkipPermissions enables --dangerously-skip-permissions flag (default: true)
	SkipPermissions bool `json:"skip_permissions"`
	// AdditionalArgs contains extra arguments to pass to Claude Code
	AdditionalArgs []string `json:"additional_args,omitempty"`
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

	// Step 5: Fetch and select model with API key validation
	w.println("")
	w.println("Fetching available models...")

	var models []string
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		var fetchErr error
		models, fetchErr = w.fetchModels(provider, apiKey, baseURL, azureEndpoint)
		if fetchErr == nil {
			break // Success
		}

		// Check for 401 (invalid API key) errors
		if strings.Contains(fetchErr.Error(), "401") || strings.Contains(fetchErr.Error(), "Unauthorized") || strings.Contains(fetchErr.Error(), "Invalid API key") || strings.Contains(fetchErr.Error(), "Incorrect API key") {
			w.println("")
			w.println("✗ API key appears to be invalid. Please check and try again.")
			w.println("")

			if attempt < maxRetries {
				// Let user re-enter API key
				var keyErr error
				apiKey, keyErr = w.promptAPIKey(provider)
				if keyErr != nil {
					return nil, keyErr
				}
				w.println("")
				w.println("Retrying with new API key...")
				continue
			}

			// Max retries reached
			w.println("Maximum retries reached. You can skip validation and enter a model manually,")
			w.println("or fix your API key and run setup again.")
		} else {
			// Other error - warn but continue
			w.printf("Warning: Could not fetch models: %v\n", fetchErr)
		}
		break
	}

	// If no models fetched, use known models as fallback for the picker
	if len(models) == 0 {
		knownModels := GetKnownModels(provider)
		for _, km := range knownModels {
			models = append(models, km.ID)
		}
		if len(models) > 0 {
			w.println("")
			w.println("Using known models list. You can also enter a custom model name.")
		}
	}

	model, err := w.selectModel(provider, models)
	if err != nil {
		return nil, err
	}

	// Step 6: Configure Claude Code settings
	claudeCodeConfig, err := w.configureClaudeCode()
	if err != nil {
		return nil, err
	}

	// Step 7: Save configuration
	w.println("")
	w.println("Saving configuration...")

	configFile := &ConfigFile{
		Provider:         provider,
		Model:            model,
		APIKey:           apiKey,
		BaseURL:          baseURL,
		AzureEndpoint:    azureEndpoint,
		AzureDeployment:  azureDeployment,
		ClaudeCodeConfig: claudeCodeConfig,
		CreatedAt:        time.Now().Format(time.RFC3339),
		UpdatedAt:        time.Now().Format(time.RFC3339),
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

// configureClaudeCode prompts the user for Claude Code settings.
// By default, it enables --dangerously-skip-permissions for seamless operation.
func (w *Wizard) configureClaudeCode() (*ClaudeCodeConfig, error) {
	w.println("")
	w.println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	w.println("Claude Code Configuration")
	w.println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	w.println("")
	w.println("CLASP launches Claude Code with automatic permissions enabled")
	w.println("(--dangerously-skip-permissions) for seamless tool execution.")
	w.println("")
	w.println("This allows Claude Code to:")
	w.println("  • Edit files without confirmation prompts")
	w.println("  • Run shell commands automatically")
	w.println("  • Use all tools without manual approval")
	w.println("")
	w.println("This is recommended for development workflows, but you can")
	w.println("disable it for stricter permission control.")
	w.println("")

	// Default to skip permissions enabled
	config := &ClaudeCodeConfig{
		SkipPermissions: true,
	}

	w.printf("Enable automatic permissions? [Y/n]: ")
	choice, err := w.reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	choice = strings.TrimSpace(strings.ToLower(choice))
	if choice == "n" || choice == "no" {
		config.SkipPermissions = false
		w.println("")
		w.println("Automatic permissions disabled.")
		w.println("Claude Code will ask for confirmation before using tools.")
	} else {
		w.println("")
		w.println("Automatic permissions enabled.")
	}

	return config, nil
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
	// Use Bubble Tea secure input if terminal is available
	if IsTTY() {
		placeholder := "Paste your key here..."
		if defaultVal != "" {
			placeholder = defaultVal
		}
		return RunSecureInput(prompt, placeholder, true)
	}
	// Fallback for non-TTY environments
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
	return w.selectModelWithTier(provider, models, "")
}

// selectModelWithTier selects a model with optional tier context.
// Returns ErrCancelled if the user cancels the selection.
func (w *Wizard) selectModelWithTier(provider string, models []string, tier string) (string, error) {
	// If TTY available and we have models, use fuzzy picker
	if IsTTY() && len(models) > 0 {
		// Get known model metadata
		knownModels := GetKnownModels(provider)

		// Merge fetched models with known metadata
		modelInfos := MergeModelLists(models, knownModels)

		if len(modelInfos) > 0 {
			selected, err := RunModelPicker(modelInfos, provider, tier)
			if err == ErrCancelled {
				// User cancelled - propagate the cancellation
				w.println("")
				w.println("Setup cancelled. Run 'clasp' to try again.")
				return "", ErrCancelled
			}
			if err != nil {
				// Real error - fall back to manual input
				w.printf("Note: Model picker unavailable (%v). Please enter model name manually.\n", err)
			} else if selected != "" {
				return selected, nil
			}
		}
	}

	// Manual input for non-TTY environments or when picker unavailable
	return w.promptInput("Enter model name", getDefaultModel(provider))
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

// RunProfileCreate runs the interactive profile creation wizard.
func (w *Wizard) RunProfileCreate(profileName string) (*Profile, error) {
	pm := NewProfileManager()

	w.println("")
	w.println("╔═══════════════════════════════════════════════════════════════╗")
	w.println("║             CLASP - Create New Profile                        ║")
	w.println("╚═══════════════════════════════════════════════════════════════╝")
	w.println("")

	// Get profile name if not provided
	if profileName == "" {
		var err error
		profileName, err = w.promptInput("Profile name", "default")
		if err != nil {
			return nil, err
		}
	}

	// Check if profile exists
	if pm.ProfileExists(profileName) {
		w.printf("Warning: Profile '%s' already exists. Overwrite? [y/N]: ", profileName)
		confirm, err := w.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		confirm = strings.TrimSpace(strings.ToLower(confirm))
		if confirm != "y" && confirm != "yes" {
			return nil, fmt.Errorf("profile creation cancelled")
		}
	}

	// Get description
	description, err := w.promptInput("Description (optional)", "")
	if err != nil {
		return nil, err
	}

	// Select provider
	provider, err := w.selectProvider()
	if err != nil {
		return nil, err
	}

	// Get API key
	apiKey, err := w.promptAPIKey(provider)
	if err != nil {
		return nil, err
	}

	// Provider-specific configuration
	var baseURL, azureEndpoint, azureDeployment string

	switch provider {
	case "azure":
		azureEndpoint, err = w.promptInput("Azure OpenAI Endpoint", "https://your-resource.openai.azure.com")
		if err != nil {
			return nil, err
		}
		azureDeployment, err = w.promptInput("Azure Deployment Name", "gpt-4")
		if err != nil {
			return nil, err
		}
	case "custom":
		baseURL, err = w.promptInput("Custom Base URL", "http://localhost:11434/v1")
		if err != nil {
			return nil, err
		}
	}

	// Fetch and select model with API key validation
	w.println("")
	w.println("Fetching available models...")

	var models []string
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		var fetchErr error
		models, fetchErr = w.fetchModels(provider, apiKey, baseURL, azureEndpoint)
		if fetchErr == nil {
			break // Success
		}

		// Check for 401 (invalid API key) errors
		if strings.Contains(fetchErr.Error(), "401") || strings.Contains(fetchErr.Error(), "Unauthorized") || strings.Contains(fetchErr.Error(), "Invalid API key") || strings.Contains(fetchErr.Error(), "Incorrect API key") {
			w.println("")
			w.println("✗ API key appears to be invalid. Please check and try again.")
			w.println("")

			if attempt < maxRetries {
				// Let user re-enter API key
				var keyErr error
				apiKey, keyErr = w.promptAPIKey(provider)
				if keyErr != nil {
					return nil, keyErr
				}
				w.println("")
				w.println("Retrying with new API key...")
				continue
			}

			// Max retries reached
			w.println("Maximum retries reached. You can skip validation and enter a model manually,")
			w.println("or fix your API key and run setup again.")
		} else {
			// Other error - warn but continue
			w.printf("Warning: Could not fetch models: %v\n", fetchErr)
		}
		break
	}

	// If no models fetched, use known models as fallback for the picker
	if len(models) == 0 {
		knownModels := GetKnownModels(provider)
		for _, km := range knownModels {
			models = append(models, km.ID)
		}
		if len(models) > 0 {
			w.println("")
			w.println("Using known models list. You can also enter a custom model name.")
		}
	}

	model, err := w.selectModel(provider, models)
	if err != nil {
		return nil, err
	}

	// Ask about tier mappings
	var tierMappings map[string]TierMapping
	w.println("")
	w.printf("Configure per-tier model mappings? (opus/sonnet/haiku) [y/N]: ")
	tierChoice, _ := w.reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(tierChoice)) == "y" {
		tierMappings, err = w.configureTierMappings(provider, apiKey, baseURL, azureEndpoint, models)
		if err != nil {
			return nil, err
		}
	}

	// Ask about port preference
	var port int
	w.println("")
	portStr, err := w.promptInput("Preferred port (0 for auto)", "0")
	if err != nil {
		return nil, err
	}
	if portStr != "" && portStr != "0" {
		port, _ = strconv.Atoi(portStr)
	}

	// Configure Claude Code settings
	claudeCodeCfg, err := w.configureClaudeCode()
	if err != nil {
		return nil, err
	}

	// Create profile
	profile := &Profile{
		Name:            profileName,
		Description:     description,
		Provider:        provider,
		APIKey:          apiKey,
		BaseURL:         baseURL,
		AzureEndpoint:   azureEndpoint,
		AzureDeployment: azureDeployment,
		DefaultModel:    model,
		TierMappings:    tierMappings,
		Port:            port,
		ClaudeCode:      claudeCodeCfg,
	}

	// Save profile
	if pm.ProfileExists(profileName) {
		if err := pm.UpdateProfile(profile); err != nil {
			return nil, fmt.Errorf("failed to update profile: %w", err)
		}
	} else {
		if err := pm.CreateProfile(profile); err != nil {
			return nil, fmt.Errorf("failed to create profile: %w", err)
		}
	}

	// Ask to set as active
	w.println("")
	w.printf("Set '%s' as the active profile? [Y/n]: ", profileName)
	activeChoice, _ := w.reader.ReadString('\n')
	activeChoice = strings.TrimSpace(strings.ToLower(activeChoice))
	if activeChoice == "" || activeChoice == "y" || activeChoice == "yes" {
		if err := pm.SetActiveProfile(profileName); err != nil {
			w.printf("Warning: Could not set active profile: %v\n", err)
		}
	}

	w.println("")
	w.println("═══════════════════════════════════════════════════════════════")
	w.printf("Profile '%s' created successfully!\n", profileName)
	w.println("")
	w.printf("Use it with: clasp --profile %s\n", profileName)
	w.printf("Or set as default: clasp profile use %s\n", profileName)
	w.println("═══════════════════════════════════════════════════════════════")
	w.println("")

	return profile, nil
}

// configureTierMappings walks through per-tier model configuration.
func (w *Wizard) configureTierMappings(provider, apiKey, baseURL, azureEndpoint string, models []string) (map[string]TierMapping, error) {
	mappings := make(map[string]TierMapping)
	tiers := []string{"opus", "sonnet", "haiku"}
	tierDescriptions := map[string]string{
		"opus":   "Opus tier (most capable, highest cost)",
		"sonnet": "Sonnet tier (balanced)",
		"haiku":  "Haiku tier (fast, low cost)",
	}

	for _, tier := range tiers {
		w.println("")
		w.printf("=== %s ===\n", tierDescriptions[tier])

		// Ask if this tier should use a different provider
		w.printf("Use the same provider (%s)? [Y/n]: ", provider)
		sameProvider, _ := w.reader.ReadString('\n')
		sameProvider = strings.TrimSpace(strings.ToLower(sameProvider))

		tierProvider := provider
		tierAPIKey := apiKey
		tierBaseURL := baseURL
		tierModels := models

		if sameProvider == "n" || sameProvider == "no" {
			var err error
			tierProvider, err = w.selectProvider()
			if err != nil {
				return nil, err
			}

			tierAPIKey, err = w.promptAPIKey(tierProvider)
			if err != nil {
				return nil, err
			}

			if tierProvider == "custom" {
				tierBaseURL, err = w.promptInput("Custom Base URL", "http://localhost:11434/v1")
				if err != nil {
					return nil, err
				}
			}

			// Fetch models for new provider
			tierModels, _ = w.fetchModels(tierProvider, tierAPIKey, tierBaseURL, azureEndpoint)
		}

		// Select model for this tier
		tierModel, err := w.selectModel(tierProvider, tierModels)
		if err != nil {
			return nil, err
		}

		mappings[tier] = TierMapping{
			Provider: tierProvider,
			Model:    tierModel,
			APIKey:   tierAPIKey,
			BaseURL:  tierBaseURL,
		}
	}

	return mappings, nil
}

// RunProfileList displays all profiles.
func (w *Wizard) RunProfileList() error {
	pm := NewProfileManager()

	profiles, err := pm.ListProfiles()
	if err != nil {
		return fmt.Errorf("failed to list profiles: %w", err)
	}

	if len(profiles) == 0 {
		w.println("")
		w.println("No profiles found.")
		w.println("Create one with: clasp profile create <name>")
		w.println("")
		return nil
	}

	// Get active profile
	globalCfg, _ := pm.GetGlobalConfig()
	activeProfile := "default"
	if globalCfg != nil && globalCfg.ActiveProfile != "" {
		activeProfile = globalCfg.ActiveProfile
	}

	w.println("")
	w.println("Available Profiles:")
	w.println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, profile := range profiles {
		isActive := profile.Name == activeProfile
		w.print(FormatProfileInfo(profile, isActive))
	}

	w.println("")
	w.println("* = active profile")
	w.println("")

	return nil
}

// RunProfileShow displays details for a specific or active profile.
func (w *Wizard) RunProfileShow(name string) error {
	pm := NewProfileManager()

	var profile *Profile
	var err error

	if name == "" {
		profile, err = pm.GetActiveProfile()
		if err != nil {
			return fmt.Errorf("no active profile found: %w", err)
		}
	} else {
		profile, err = pm.GetProfile(name)
		if err != nil {
			return fmt.Errorf("profile '%s' not found", name)
		}
	}

	w.println("")
	w.print(FormatProfileDetails(profile))
	w.println("")

	return nil
}

// RunProfileUse switches to a different profile.
func (w *Wizard) RunProfileUse(name string) error {
	pm := NewProfileManager()

	if !pm.ProfileExists(name) {
		return fmt.Errorf("profile '%s' not found", name)
	}

	if err := pm.SetActiveProfile(name); err != nil {
		return fmt.Errorf("failed to switch profile: %w", err)
	}

	profile, _ := pm.GetProfile(name)

	w.println("")
	w.printf("Switched to profile: %s\n", name)
	if profile.Description != "" {
		w.printf("  %s\n", profile.Description)
	}
	w.printf("  Provider: %s, Model: %s\n", profile.Provider, profile.DefaultModel)
	w.println("")

	return nil
}

// RunProfileDelete removes a profile.
func (w *Wizard) RunProfileDelete(name string) error {
	pm := NewProfileManager()

	if !pm.ProfileExists(name) {
		return fmt.Errorf("profile '%s' not found", name)
	}

	// Confirm deletion
	w.printf("Delete profile '%s'? This cannot be undone. [y/N]: ", name)
	confirm, err := w.reader.ReadString('\n')
	if err != nil {
		return err
	}
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm != "y" && confirm != "yes" {
		w.println("Deletion cancelled.")
		return nil
	}

	if err := pm.DeleteProfile(name); err != nil {
		return err
	}

	w.println("")
	w.printf("Profile '%s' deleted.\n", name)
	w.println("")

	return nil
}

// RunProfileExport exports a profile to a file.
func (w *Wizard) RunProfileExport(name, outputPath string) error {
	pm := NewProfileManager()

	data, err := pm.ExportProfile(name)
	if err != nil {
		return err
	}

	if outputPath == "" {
		outputPath = name + "-profile.json"
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	w.println("")
	w.printf("Profile exported to: %s\n", outputPath)
	w.println("Note: API keys are not included in exports for security.")
	w.println("")

	return nil
}

// RunProfileImport imports a profile from a file.
func (w *Wizard) RunProfileImport(inputPath, newName string) error {
	pm := NewProfileManager()

	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := pm.ImportProfile(data, newName); err != nil {
		return err
	}

	profileName := newName
	if profileName == "" {
		var profile Profile
		json.Unmarshal(data, &profile)
		profileName = profile.Name
	}

	w.println("")
	w.printf("Profile '%s' imported successfully.\n", profileName)
	w.println("Note: You'll need to configure the API key using:")
	w.printf("  clasp profile edit %s\n", profileName)
	w.println("")

	return nil
}

// print outputs text without newline.
func (w *Wizard) print(s string) {
	fmt.Fprint(w.writer, s)
}
