// Package setup provides profile management for CLASP configuration.
package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jedarden/clasp/internal/secrets"
)

// Profile represents a saved CLASP configuration profile.
type Profile struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Provider configuration
	Provider   string `json:"provider"`
	APIKey     string `json:"api_key,omitempty"`
	APIKeyEnv  string `json:"api_key_env,omitempty"` // Reference to env var instead of storing key
	BaseURL    string `json:"base_url,omitempty"`

	// Azure-specific
	AzureEndpoint   string `json:"azure_endpoint,omitempty"`
	AzureDeployment string `json:"azure_deployment,omitempty"`

	// Model configuration
	DefaultModel string `json:"default_model,omitempty"`

	// Per-tier model mappings
	TierMappings map[string]TierMapping `json:"tier_mappings,omitempty"`

	// Default parameters
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`

	// Server preferences
	Port int `json:"port,omitempty"`

	// Feature flags
	RateLimitEnabled      bool `json:"rate_limit_enabled,omitempty"`
	CacheEnabled          bool `json:"cache_enabled,omitempty"`
	CircuitBreakerEnabled bool `json:"circuit_breaker_enabled,omitempty"`

	// Claude Code settings
	ClaudeCode *ClaudeCodeConfig `json:"claude_code,omitempty"`

	// Timestamps
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// TierMapping represents model mapping for a specific tier.
type TierMapping struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key,omitempty"`
	APIKeyEnv string `json:"api_key_env,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
}

// GlobalConfig holds global settings and active profile reference.
type GlobalConfig struct {
	ActiveProfile string `json:"active_profile"`
	LastUsed      string `json:"last_used,omitempty"`
}

// ProfileManager handles profile CRUD operations.
type ProfileManager struct {
	baseDir string
}

// NewProfileManager creates a new profile manager.
func NewProfileManager() *ProfileManager {
	home, _ := os.UserHomeDir()
	return &ProfileManager{
		baseDir: filepath.Join(home, ".clasp"),
	}
}

// GetProfilesDir returns the profiles directory path.
func (pm *ProfileManager) GetProfilesDir() string {
	return filepath.Join(pm.baseDir, "profiles")
}

// GetGlobalConfigPath returns the global config file path.
func (pm *ProfileManager) GetGlobalConfigPath() string {
	return filepath.Join(pm.baseDir, "config.json")
}

// EnsureDirectories creates necessary directories if they don't exist.
func (pm *ProfileManager) EnsureDirectories() error {
	profilesDir := pm.GetProfilesDir()
	return os.MkdirAll(profilesDir, 0700)
}

// CreateProfile creates a new profile.
func (pm *ProfileManager) CreateProfile(profile *Profile) error {
	if err := pm.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
	}

	// Check if profile already exists
	if pm.ProfileExists(profile.Name) {
		return fmt.Errorf("profile '%s' already exists", profile.Name)
	}

	// Set timestamps
	now := time.Now().Format(time.RFC3339)
	profile.CreatedAt = now
	profile.UpdatedAt = now

	// Save profile
	return pm.saveProfile(profile)
}

// UpdateProfile updates an existing profile.
func (pm *ProfileManager) UpdateProfile(profile *Profile) error {
	if !pm.ProfileExists(profile.Name) {
		return fmt.Errorf("profile '%s' does not exist", profile.Name)
	}

	// Update timestamp
	profile.UpdatedAt = time.Now().Format(time.RFC3339)

	return pm.saveProfile(profile)
}

// GetProfile retrieves a profile by name.
func (pm *ProfileManager) GetProfile(name string) (*Profile, error) {
	profilePath := pm.getProfilePath(name)

	data, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("profile '%s' not found", name)
		}
		return nil, err
	}

	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}

	return &profile, nil
}

// DeleteProfile removes a profile.
func (pm *ProfileManager) DeleteProfile(name string) error {
	if name == "default" {
		return fmt.Errorf("cannot delete the default profile")
	}

	profilePath := pm.getProfilePath(name)
	if err := os.Remove(profilePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("profile '%s' not found", name)
		}
		return err
	}

	// If this was the active profile, switch to default
	globalCfg, err := pm.GetGlobalConfig()
	if err == nil && globalCfg.ActiveProfile == name {
		globalCfg.ActiveProfile = "default"
		pm.SaveGlobalConfig(globalCfg)
	}

	return nil
}

// ListProfiles returns all available profiles.
func (pm *ProfileManager) ListProfiles() ([]*Profile, error) {
	profilesDir := pm.GetProfilesDir()

	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Profile{}, nil
		}
		return nil, err
	}

	var profiles []*Profile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".json")
		profile, err := pm.GetProfile(name)
		if err != nil {
			continue // Skip invalid profiles
		}
		profiles = append(profiles, profile)
	}

	// Sort by name
	sort.Slice(profiles, func(i, j int) bool {
		// Default profile always first
		if profiles[i].Name == "default" {
			return true
		}
		if profiles[j].Name == "default" {
			return false
		}
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, nil
}

// ProfileExists checks if a profile exists.
func (pm *ProfileManager) ProfileExists(name string) bool {
	profilePath := pm.getProfilePath(name)
	_, err := os.Stat(profilePath)
	return err == nil
}

// GetActiveProfile returns the currently active profile.
func (pm *ProfileManager) GetActiveProfile() (*Profile, error) {
	globalCfg, err := pm.GetGlobalConfig()
	if err != nil {
		// Default to "default" profile
		return pm.GetProfile("default")
	}

	if globalCfg.ActiveProfile == "" {
		globalCfg.ActiveProfile = "default"
	}

	return pm.GetProfile(globalCfg.ActiveProfile)
}

// SetActiveProfile sets the active profile.
func (pm *ProfileManager) SetActiveProfile(name string) error {
	if !pm.ProfileExists(name) {
		return fmt.Errorf("profile '%s' not found", name)
	}

	globalCfg, err := pm.GetGlobalConfig()
	if err != nil {
		globalCfg = &GlobalConfig{}
	}

	globalCfg.ActiveProfile = name
	globalCfg.LastUsed = time.Now().Format(time.RFC3339)

	return pm.SaveGlobalConfig(globalCfg)
}

// GetGlobalConfig retrieves the global configuration.
func (pm *ProfileManager) GetGlobalConfig() (*GlobalConfig, error) {
	configPath := pm.GetGlobalConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config GlobalConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveGlobalConfig saves the global configuration.
func (pm *ProfileManager) SaveGlobalConfig(config *GlobalConfig) error {
	if err := pm.EnsureDirectories(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	configPath := pm.GetGlobalConfigPath()
	return os.WriteFile(configPath, data, 0600)
}

// ApplyProfileToEnv applies profile settings to environment variables.
func (pm *ProfileManager) ApplyProfileToEnv(profile *Profile) error {
	os.Setenv("PROVIDER", profile.Provider)

	// Set API key - prefer env var reference, then direct key
	apiKey := profile.APIKey
	if profile.APIKeyEnv != "" {
		if envKey := os.Getenv(profile.APIKeyEnv); envKey != "" {
			apiKey = envKey
		}
	}

	switch profile.Provider {
	case "openai":
		os.Setenv("OPENAI_API_KEY", apiKey)
		if profile.BaseURL != "" {
			os.Setenv("OPENAI_BASE_URL", profile.BaseURL)
		}
	case "azure":
		os.Setenv("AZURE_API_KEY", apiKey)
		os.Setenv("AZURE_OPENAI_ENDPOINT", profile.AzureEndpoint)
		os.Setenv("AZURE_DEPLOYMENT_NAME", profile.AzureDeployment)
	case "openrouter":
		os.Setenv("OPENROUTER_API_KEY", apiKey)
	case "anthropic":
		os.Setenv("ANTHROPIC_API_KEY", apiKey)
	case "custom":
		if apiKey != "" {
			os.Setenv("CUSTOM_API_KEY", apiKey)
		}
		os.Setenv("CUSTOM_BASE_URL", profile.BaseURL)
	}

	if profile.DefaultModel != "" {
		os.Setenv("CLASP_MODEL", profile.DefaultModel)
	}

	if profile.Port > 0 {
		os.Setenv("CLASP_PORT", fmt.Sprintf("%d", profile.Port))
	}

	// Apply tier mappings
	if len(profile.TierMappings) > 0 {
		os.Setenv("CLASP_MULTI_PROVIDER", "true")

		for tier, mapping := range profile.TierMappings {
			tierUpper := strings.ToUpper(tier)
			if mapping.Provider != "" {
				os.Setenv(fmt.Sprintf("CLASP_%s_PROVIDER", tierUpper), mapping.Provider)
			}
			if mapping.Model != "" {
				os.Setenv(fmt.Sprintf("CLASP_%s_MODEL", tierUpper), mapping.Model)
			}
			// Apply tier-specific API key
			tierAPIKey := mapping.APIKey
			if mapping.APIKeyEnv != "" {
				if envKey := os.Getenv(mapping.APIKeyEnv); envKey != "" {
					tierAPIKey = envKey
				}
			}
			if tierAPIKey != "" {
				os.Setenv(fmt.Sprintf("CLASP_%s_API_KEY", tierUpper), tierAPIKey)
			}
			if mapping.BaseURL != "" {
				os.Setenv(fmt.Sprintf("CLASP_%s_BASE_URL", tierUpper), mapping.BaseURL)
			}
		}
	}

	// Apply feature flags
	if profile.RateLimitEnabled {
		os.Setenv("CLASP_RATE_LIMIT", "true")
	}
	if profile.CacheEnabled {
		os.Setenv("CLASP_CACHE", "true")
	}
	if profile.CircuitBreakerEnabled {
		os.Setenv("CLASP_CIRCUIT_BREAKER", "true")
	}

	return nil
}

// ExportProfile exports a profile to JSON.
// API keys are never included in exports for security.
// Instead, use api_key_env to reference environment variables.
func (pm *ProfileManager) ExportProfile(name string) ([]byte, error) {
	profile, err := pm.GetProfile(name)
	if err != nil {
		return nil, err
	}

	// Create a copy without sensitive data for export
	exportProfile := *profile
	exportProfile.APIKey = "" // Never export raw API keys

	// Clear tier mapping API keys too
	if exportProfile.TierMappings != nil {
		for tier, mapping := range exportProfile.TierMappings {
			mapping.APIKey = "" // Never export raw API keys
			exportProfile.TierMappings[tier] = mapping
		}
	}

	// If no api_key_env was set, suggest a reasonable default
	if exportProfile.APIKeyEnv == "" && profile.APIKey != "" {
		switch profile.Provider {
		case "openai":
			exportProfile.APIKeyEnv = "OPENAI_API_KEY"
		case "azure":
			exportProfile.APIKeyEnv = "AZURE_API_KEY"
		case "openrouter":
			exportProfile.APIKeyEnv = "OPENROUTER_API_KEY"
		case "anthropic":
			exportProfile.APIKeyEnv = "ANTHROPIC_API_KEY"
		case "custom":
			exportProfile.APIKeyEnv = "CUSTOM_API_KEY"
		}
	}

	return json.MarshalIndent(exportProfile, "", "  ")
}

// ImportProfile imports a profile from JSON.
func (pm *ProfileManager) ImportProfile(data []byte, newName string) error {
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return fmt.Errorf("invalid profile data: %w", err)
	}

	if newName != "" {
		profile.Name = newName
	}

	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
	}

	return pm.CreateProfile(&profile)
}

// MigrateOldConfig migrates from old config.json format to new profile system.
func (pm *ProfileManager) MigrateOldConfig() error {
	oldConfigPath := GetConfigPath()

	data, err := os.ReadFile(oldConfigPath)
	if err != nil {
		return nil // No old config to migrate
	}

	var oldConfig ConfigFile
	if err := json.Unmarshal(data, &oldConfig); err != nil {
		return nil // Invalid old config
	}

	// Create default profile from old config
	profile := &Profile{
		Name:            "default",
		Description:     "Migrated from legacy config",
		Provider:        oldConfig.Provider,
		APIKey:          oldConfig.APIKey,
		BaseURL:         oldConfig.BaseURL,
		AzureEndpoint:   oldConfig.AzureEndpoint,
		AzureDeployment: oldConfig.AzureDeployment,
		DefaultModel:    oldConfig.Model,
	}

	// Create profile if it doesn't exist
	if !pm.ProfileExists("default") {
		if err := pm.CreateProfile(profile); err != nil {
			return err
		}
	}

	// Set as active profile
	return pm.SetActiveProfile("default")
}

// saveProfile writes a profile to disk.
func (pm *ProfileManager) saveProfile(profile *Profile) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}

	profilePath := pm.getProfilePath(profile.Name)
	return os.WriteFile(profilePath, data, 0600)
}

// getProfilePath returns the file path for a profile.
func (pm *ProfileManager) getProfilePath(name string) string {
	return filepath.Join(pm.GetProfilesDir(), name+".json")
}

// FormatProfileInfo returns a formatted string with profile information.
func FormatProfileInfo(profile *Profile, isActive bool) string {
	var sb strings.Builder

	activeMarker := "  "
	if isActive {
		activeMarker = "* "
	}

	sb.WriteString(fmt.Sprintf("%s%s\n", activeMarker, profile.Name))
	sb.WriteString(fmt.Sprintf("    Provider: %s\n", profile.Provider))
	if profile.DefaultModel != "" {
		sb.WriteString(fmt.Sprintf("    Model: %s\n", profile.DefaultModel))
	}
	if profile.Description != "" {
		sb.WriteString(fmt.Sprintf("    Description: %s\n", profile.Description))
	}

	return sb.String()
}

// FormatProfileDetails returns detailed profile information.
func FormatProfileDetails(profile *Profile) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Profile: %s\n", profile.Name))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if profile.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", profile.Description))
	}
	sb.WriteString(fmt.Sprintf("Provider:    %s\n", profile.Provider))

	if profile.DefaultModel != "" {
		sb.WriteString(fmt.Sprintf("Model:       %s\n", profile.DefaultModel))
	}

	if profile.APIKeyEnv != "" {
		sb.WriteString(fmt.Sprintf("API Key:     ${%s}\n", profile.APIKeyEnv))
	} else if profile.APIKey != "" {
		// Mask the API key using centralized secrets package
		masked := secrets.MaskAPIKey(profile.APIKey)
		sb.WriteString(fmt.Sprintf("API Key:     %s\n", masked))
	}

	if profile.BaseURL != "" {
		sb.WriteString(fmt.Sprintf("Base URL:    %s\n", profile.BaseURL))
	}

	if profile.Provider == "azure" {
		sb.WriteString(fmt.Sprintf("Azure Endpoint:   %s\n", profile.AzureEndpoint))
		sb.WriteString(fmt.Sprintf("Azure Deployment: %s\n", profile.AzureDeployment))
	}

	if profile.Port > 0 {
		sb.WriteString(fmt.Sprintf("Port:        %d\n", profile.Port))
	}

	// Tier mappings
	if len(profile.TierMappings) > 0 {
		sb.WriteString("\nModel Routing:\n")
		for tier, mapping := range profile.TierMappings {
			sb.WriteString(fmt.Sprintf("  %s → %s", tier, mapping.Model))
			if mapping.Provider != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", mapping.Provider))
			}
			sb.WriteString("\n")
		}
	}

	// Feature flags
	var features []string
	if profile.RateLimitEnabled {
		features = append(features, "rate-limit")
	}
	if profile.CacheEnabled {
		features = append(features, "cache")
	}
	if profile.CircuitBreakerEnabled {
		features = append(features, "circuit-breaker")
	}
	if len(features) > 0 {
		sb.WriteString(fmt.Sprintf("\nFeatures:    %s\n", strings.Join(features, ", ")))
	}

	sb.WriteString(fmt.Sprintf("\nCreated:     %s\n", profile.CreatedAt))
	sb.WriteString(fmt.Sprintf("Updated:     %s\n", profile.UpdatedAt))

	return sb.String()
}

// maskAPIKey masks an API key for display.
// Deprecated: Use secrets.MaskAPIKey instead. This function is kept for backwards compatibility.
func maskAPIKey(key string) string {
	return secrets.MaskAPIKey(key)
}
