// Package setup provides interactive configuration wizards including Bubble Tea TUI components.
package setup

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

// ModelInfo contains metadata about a model.
type ModelInfo struct {
	ID           string  // Model ID (e.g., "gpt-4o")
	Name         string  // Display name (e.g., "GPT-4o")
	Desc         string  // Short description
	Provider     string  // Provider name
	InputPrice   float64 // Price per 1M input tokens in USD
	OutputPrice  float64 // Price per 1M output tokens in USD
	ContextSize  int     // Context window size in tokens
	IsRecommended bool   // Whether this is a recommended model
}

// Implement list.Item interface
func (m ModelInfo) FilterValue() string { return m.ID + " " + m.Name }
func (m ModelInfo) Title() string       { return m.ID }
func (m ModelInfo) Description() string {
	var parts []string
	if m.Desc != "" {
		parts = append(parts, m.Desc)
	}
	if m.ContextSize > 0 {
		parts = append(parts, fmt.Sprintf("%dk ctx", m.ContextSize/1000))
	}
	if m.InputPrice > 0 || m.OutputPrice > 0 {
		parts = append(parts, fmt.Sprintf("$%.2f/$%.2f per 1M", m.InputPrice, m.OutputPrice))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

// ModelPicker is a Bubble Tea model for fuzzy model selection.
type ModelPicker struct {
	list           list.Model
	filterInput    textinput.Model
	models         []ModelInfo
	filtered       []ModelInfo
	selected       *ModelInfo
	canceled       bool
	width          int
	height         int
	provider       string
	tier           string // Optional: which tier we're selecting for (opus/sonnet/haiku)
	showHelp       bool
	err            error
	cursorBlink    bool   // For cursor animation
	lastFilterText string // Track previous filter for change detection
}

// Styles for the model picker
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginLeft(2)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("170"))

	paginationStyle = lipgloss.NewStyle().
			PaddingLeft(4)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(4)

	filterPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39"))

	recommendedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("76")).
				SetString("★ ")

	filterLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")).
				PaddingLeft(2)

	filterInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	separatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				PaddingLeft(2)

	countStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(2)
)

// NewModelPicker creates a new fuzzy model picker.
func NewModelPicker(models []ModelInfo, provider, tier string) *ModelPicker {
	// Create text input for filtering
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()
	ti.PromptStyle = filterPromptStyle
	ti.CharLimit = 50
	ti.Width = 40

	// Create list with custom delegate
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetSpacing(0)

	// Convert to list items
	items := make([]list.Item, len(models))
	for i, m := range models {
		items[i] = m
	}

	l := list.New(items, delegate, 0, 0)
	// Don't show the list's title - we render our own
	l.SetShowTitle(false)
	// Don't show the list's filter - we have our own visible filter input
	l.SetFilteringEnabled(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(true)
	l.SetShowHelp(false) // We show our own help
	l.DisableQuitKeybindings()

	// Set styles
	l.Styles.PaginationStyle = paginationStyle

	return &ModelPicker{
		list:        l,
		filterInput: ti,
		models:      models,
		filtered:    models,
		provider:    provider,
		tier:        tier,
		width:       80,
		height:      20,
	}
}

// Init initializes the model picker.
func (m *ModelPicker) Init() tea.Cmd {
	return nil
}

// Update handles input and updates the model picker state.
func (m *ModelPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4) // Leave room for filter input
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.canceled = true
			return m, tea.Quit

		case "enter":
			if item, ok := m.list.SelectedItem().(ModelInfo); ok {
				m.selected = &item
				return m, tea.Quit
			}

		case "?":
			m.showHelp = !m.showHelp
			return m, nil

		case "esc":
			if m.filterInput.Value() != "" {
				m.filterInput.SetValue("")
				m.applyFilter("")
				return m, nil
			}
			m.canceled = true
			return m, tea.Quit
		}
	}

	// Update filter input
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)

	// Apply filter if changed
	currentFilter := m.filterInput.Value()
	if currentFilter != m.lastFilterText {
		m.applyFilter(currentFilter)
		m.lastFilterText = currentFilter
	}

	// Update list (for navigation)
	var listCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)

	return m, tea.Batch(cmd, listCmd)
}

// applyFilter filters the model list using fuzzy matching.
func (m *ModelPicker) applyFilter(filter string) {
	if filter == "" {
		// Reset to all models
		items := make([]list.Item, len(m.models))
		for i, model := range m.models {
			items[i] = model
		}
		m.list.SetItems(items)
		m.filtered = m.models
		return
	}

	// Fuzzy match against model IDs and names
	var sources []string
	for _, model := range m.models {
		sources = append(sources, model.ID+" "+model.Name+" "+model.Desc)
	}

	matches := fuzzy.Find(filter, sources)

	// Sort by score
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	// Build filtered list
	filtered := make([]ModelInfo, 0, len(matches))
	for _, match := range matches {
		filtered = append(filtered, m.models[match.Index])
	}

	items := make([]list.Item, len(filtered))
	for i, model := range filtered {
		items[i] = model
	}
	m.list.SetItems(items)
	m.filtered = filtered
}

// View renders the model picker with visible filter input.
func (m *ModelPicker) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	var b strings.Builder

	// Title
	b.WriteString("\n")
	title := fmt.Sprintf("Select model for %s", m.provider)
	if m.tier != "" {
		title = fmt.Sprintf("Select %s tier model", m.tier)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Visible filter input line with cursor
	filterText := m.filterInput.Value()
	cursor := "▌" // Block cursor character
	if filterText == "" {
		// Show placeholder when empty
		b.WriteString(filterLabelStyle.Render("Filter: "))
		b.WriteString(cursorStyle.Render(cursor))
		b.WriteString(helpStyle.Render(" Type to filter..."))
	} else {
		// Show filter text with cursor at end
		b.WriteString(filterLabelStyle.Render("Filter: "))
		b.WriteString(filterInputStyle.Render(filterText))
		b.WriteString(cursorStyle.Render(cursor))
	}
	b.WriteString("\n")

	// Separator line
	separatorWidth := 50
	if m.width > 0 && m.width < 80 {
		separatorWidth = m.width - 4
	}
	separator := strings.Repeat("─", separatorWidth)
	b.WriteString(separatorStyle.Render(separator))
	b.WriteString("\n")

	// Model list (without the list's built-in filter UI)
	b.WriteString(m.list.View())

	// Result count
	b.WriteString("\n")
	showingCount := len(m.filtered)
	totalCount := len(m.models)
	if filterText != "" {
		if showingCount == 0 {
			b.WriteString(countStyle.Render(fmt.Sprintf("No models match '%s'  •  Press Esc to clear filter", filterText)))
		} else {
			b.WriteString(countStyle.Render(fmt.Sprintf("Showing %d of %d models", showingCount, totalCount)))
		}
	} else {
		b.WriteString(countStyle.Render(fmt.Sprintf("Showing %d models  •  Type to filter", totalCount)))
	}
	b.WriteString("\n")

	// Help section
	if m.showHelp {
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"))
		b.WriteString(helpStyle.Render("Keyboard shortcuts:\n"))
		b.WriteString(helpStyle.Render("  ↑/↓ or j/k    Navigate list\n"))
		b.WriteString(helpStyle.Render("  Type          Filter models\n"))
		b.WriteString(helpStyle.Render("  Backspace     Delete character\n"))
		b.WriteString(helpStyle.Render("  Enter         Select model\n"))
		b.WriteString(helpStyle.Render("  Esc           Clear filter / Cancel\n"))
		b.WriteString(helpStyle.Render("  ?             Toggle help\n"))
		b.WriteString(helpStyle.Render("  q             Quit\n"))
	}

	return b.String()
}

// Selected returns the selected model, or nil if canceled.
func (m *ModelPicker) Selected() *ModelInfo {
	return m.selected
}

// Canceled returns true if the user canceled the selection.
func (m *ModelPicker) Canceled() bool {
	return m.canceled
}

// GetKnownModels returns model metadata for known providers.
func GetKnownModels(provider string) []ModelInfo {
	switch provider {
	case "openai":
		return getOpenAIModels()
	case "openrouter":
		return getOpenRouterModels()
	case "anthropic":
		return getAnthropicModels()
	case "azure":
		return getAzureModels()
	case "gemini":
		return getGeminiModels()
	case "deepseek":
		return getDeepSeekModels()
	case "ollama":
		return getOllamaModels()
	default:
		return nil
	}
}

func getOllamaModels() []ModelInfo {
	return []ModelInfo{
		// Recommended for coding
		{ID: "qwen2.5-coder:7b", Name: "Qwen 2.5 Coder 7B", Desc: "Excellent code model - 4.7GB", ContextSize: 32768, IsRecommended: true},
		{ID: "qwen2.5-coder:14b", Name: "Qwen 2.5 Coder 14B", Desc: "Larger code model - 9.0GB", ContextSize: 32768},
		{ID: "qwen2.5-coder:32b", Name: "Qwen 2.5 Coder 32B", Desc: "Most capable Qwen coder - 20GB", ContextSize: 32768},
		{ID: "deepseek-coder-v2", Name: "DeepSeek Coder V2", Desc: "Strong code model - 8.9GB", ContextSize: 128000, IsRecommended: true},
		{ID: "codellama:7b", Name: "CodeLlama 7B", Desc: "Meta's code model - 3.8GB", ContextSize: 16384},
		{ID: "codellama:13b", Name: "CodeLlama 13B", Desc: "Larger CodeLlama - 7.4GB", ContextSize: 16384},
		{ID: "starcoder2:7b", Name: "StarCoder2 7B", Desc: "Fast code completion - 4.0GB", ContextSize: 16384},
		// General purpose
		{ID: "llama3.2:3b", Name: "Llama 3.2 3B", Desc: "Fast, capable - 2.0GB", ContextSize: 131072, IsRecommended: true},
		{ID: "llama3.1:8b", Name: "Llama 3.1 8B", Desc: "Good balance - 4.7GB", ContextSize: 131072, IsRecommended: true},
		{ID: "llama3.1:70b", Name: "Llama 3.1 70B", Desc: "Most capable - 40GB", ContextSize: 131072},
		{ID: "mistral:7b", Name: "Mistral 7B", Desc: "Fast and efficient - 4.1GB", ContextSize: 32768},
		{ID: "mixtral:8x7b", Name: "Mixtral 8x7B", Desc: "MoE model - 26GB", ContextSize: 32768},
		{ID: "gemma2:9b", Name: "Gemma 2 9B", Desc: "Google's model - 5.4GB", ContextSize: 8192},
		{ID: "phi3:mini", Name: "Phi-3 Mini", Desc: "Microsoft's small model - 2.4GB", ContextSize: 128000},
	}
}

func getOpenAIModels() []ModelInfo {
	return []ModelInfo{
		// GPT-5 series (Responses API required)
		{ID: "gpt-5", Name: "GPT-5", Desc: "Next-gen flagship (Responses API)", InputPrice: 5.0, OutputPrice: 15.0, ContextSize: 256000, IsRecommended: true},
		{ID: "gpt-5-mini", Name: "GPT-5 Mini", Desc: "Smaller, faster GPT-5 (Responses API)", InputPrice: 1.0, OutputPrice: 4.0, ContextSize: 256000},
		{ID: "gpt-5-turbo", Name: "GPT-5 Turbo", Desc: "Optimized GPT-5 (Responses API)", InputPrice: 3.0, OutputPrice: 12.0, ContextSize: 256000},
		// GPT-5.1 Codex series (Responses API required)
		{ID: "gpt-5.1-codex", Name: "GPT-5.1 Codex", Desc: "Code-optimized (Responses API)", InputPrice: 5.0, OutputPrice: 15.0, ContextSize: 256000, IsRecommended: true},
		{ID: "gpt-5.1-codex-mini", Name: "GPT-5.1 Codex Mini", Desc: "Smaller codex (Responses API)", InputPrice: 1.0, OutputPrice: 4.0, ContextSize: 256000},
		// Codex models (Responses API required)
		{ID: "codex", Name: "Codex", Desc: "Code completion (Responses API)", InputPrice: 2.0, OutputPrice: 8.0, ContextSize: 128000},
		{ID: "codex-mini", Name: "Codex Mini", Desc: "Smaller codex (Responses API)", InputPrice: 0.5, OutputPrice: 2.0, ContextSize: 128000},
		// GPT-4.5 (newest)
		{ID: "gpt-4.5-preview", Name: "GPT-4.5 Preview", Desc: "Most capable preview model", InputPrice: 75.0, OutputPrice: 150.0, ContextSize: 128000, IsRecommended: true},
		// o3 series
		{ID: "o3-mini", Name: "o3-mini", Desc: "Smallest reasoning model", InputPrice: 1.10, OutputPrice: 4.40, ContextSize: 200000, IsRecommended: true},
		{ID: "o3-mini-2025-01-31", Name: "o3-mini (Jan 2025)", Desc: "Dated version of o3-mini", InputPrice: 1.10, OutputPrice: 4.40, ContextSize: 200000},
		// o1 series
		{ID: "o1", Name: "o1", Desc: "Full reasoning model", InputPrice: 15.0, OutputPrice: 60.0, ContextSize: 200000},
		{ID: "o1-2024-12-17", Name: "o1 (Dec 2024)", Desc: "Dated version of o1", InputPrice: 15.0, OutputPrice: 60.0, ContextSize: 200000},
		{ID: "o1-preview", Name: "o1 Preview", Desc: "Preview reasoning model", InputPrice: 15.0, OutputPrice: 60.0, ContextSize: 128000},
		{ID: "o1-mini", Name: "o1-mini", Desc: "Small reasoning model", InputPrice: 3.0, OutputPrice: 12.0, ContextSize: 128000},
		// GPT-4o series
		{ID: "gpt-4o", Name: "GPT-4o", Desc: "Flagship multimodal model", InputPrice: 2.50, OutputPrice: 10.0, ContextSize: 128000, IsRecommended: true},
		{ID: "gpt-4o-2024-11-20", Name: "GPT-4o (Nov 2024)", Desc: "Latest GPT-4o snapshot", InputPrice: 2.50, OutputPrice: 10.0, ContextSize: 128000},
		{ID: "gpt-4o-2024-08-06", Name: "GPT-4o (Aug 2024)", Desc: "Structured outputs support", InputPrice: 2.50, OutputPrice: 10.0, ContextSize: 128000},
		{ID: "chatgpt-4o-latest", Name: "ChatGPT-4o Latest", Desc: "Dynamic ChatGPT version", InputPrice: 5.0, OutputPrice: 15.0, ContextSize: 128000},
		// GPT-4o-mini series
		{ID: "gpt-4o-mini", Name: "GPT-4o-mini", Desc: "Fast, affordable GPT-4 class", InputPrice: 0.15, OutputPrice: 0.60, ContextSize: 128000, IsRecommended: true},
		{ID: "gpt-4o-mini-2024-07-18", Name: "GPT-4o-mini (Jul 2024)", Desc: "Initial mini release", InputPrice: 0.15, OutputPrice: 0.60, ContextSize: 128000},
		// GPT-4 Turbo
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Desc: "Previous flagship with vision", InputPrice: 10.0, OutputPrice: 30.0, ContextSize: 128000},
		{ID: "gpt-4-turbo-preview", Name: "GPT-4 Turbo Preview", Desc: "Preview with vision", InputPrice: 10.0, OutputPrice: 30.0, ContextSize: 128000},
		// GPT-4 base
		{ID: "gpt-4", Name: "GPT-4", Desc: "Original GPT-4", InputPrice: 30.0, OutputPrice: 60.0, ContextSize: 8192},
		{ID: "gpt-4-0613", Name: "GPT-4 (Jun 2023)", Desc: "Dated GPT-4 snapshot", InputPrice: 30.0, OutputPrice: 60.0, ContextSize: 8192},
		// GPT-3.5
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Desc: "Fast and efficient", InputPrice: 0.50, OutputPrice: 1.50, ContextSize: 16385},
		{ID: "gpt-3.5-turbo-0125", Name: "GPT-3.5 Turbo (Jan 2024)", Desc: "Latest 3.5 version", InputPrice: 0.50, OutputPrice: 1.50, ContextSize: 16385},
	}
}

func getAnthropicModels() []ModelInfo {
	return []ModelInfo{
		{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Desc: "Best overall performance", InputPrice: 3.0, OutputPrice: 15.0, ContextSize: 200000, IsRecommended: true},
		{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Desc: "Fast and affordable", InputPrice: 0.25, OutputPrice: 1.25, ContextSize: 200000, IsRecommended: true},
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Desc: "Most capable", InputPrice: 15.0, OutputPrice: 75.0, ContextSize: 200000},
		{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Desc: "Balanced performance", InputPrice: 3.0, OutputPrice: 15.0, ContextSize: 200000},
		{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Desc: "Fast and efficient", InputPrice: 0.25, OutputPrice: 1.25, ContextSize: 200000},
	}
}

func getAzureModels() []ModelInfo {
	return []ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Desc: "Flagship multimodal (deploy name varies)", ContextSize: 128000, IsRecommended: true},
		{ID: "gpt-4o-mini", Name: "GPT-4o-mini", Desc: "Fast, affordable GPT-4 class", ContextSize: 128000, IsRecommended: true},
		{ID: "gpt-4", Name: "GPT-4", Desc: "Original GPT-4", ContextSize: 8192},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Desc: "Extended context GPT-4", ContextSize: 128000},
		{ID: "gpt-35-turbo", Name: "GPT-3.5 Turbo", Desc: "Fast and efficient", ContextSize: 16385},
	}
}

func getGeminiModels() []ModelInfo {
	return []ModelInfo{
		{ID: "gemini-2.0-flash-exp", Name: "Gemini 2.0 Flash", Desc: "Latest experimental - fast & capable", ContextSize: 1048576, IsRecommended: true},
		{ID: "gemini-2.0-flash-thinking-exp", Name: "Gemini 2.0 Flash Thinking", Desc: "Enhanced reasoning capabilities", ContextSize: 1048576, IsRecommended: true},
		{ID: "gemini-1.5-pro", Name: "Gemini 1.5 Pro", Desc: "Production-ready pro model", ContextSize: 2097152},
		{ID: "gemini-1.5-flash", Name: "Gemini 1.5 Flash", Desc: "Fast and efficient", ContextSize: 1048576},
		{ID: "gemini-1.5-flash-8b", Name: "Gemini 1.5 Flash 8B", Desc: "Lightweight 8B model", ContextSize: 1048576},
		{ID: "gemini-exp-1206", Name: "Gemini Experimental 1206", Desc: "Latest experimental features", ContextSize: 2097152},
	}
}

func getDeepSeekModels() []ModelInfo {
	return []ModelInfo{
		{ID: "deepseek-chat", Name: "DeepSeek Chat", Desc: "General purpose chat model - fast and capable", InputPrice: 0.14, OutputPrice: 0.28, ContextSize: 64000, IsRecommended: true},
		{ID: "deepseek-coder", Name: "DeepSeek Coder", Desc: "Optimized for code generation and understanding", InputPrice: 0.14, OutputPrice: 0.28, ContextSize: 64000, IsRecommended: true},
		{ID: "deepseek-reasoner", Name: "DeepSeek Reasoner", Desc: "Enhanced reasoning capabilities for complex tasks", InputPrice: 0.55, OutputPrice: 2.19, ContextSize: 64000},
	}
}

func getOpenRouterModels() []ModelInfo {
	return []ModelInfo{
		// Anthropic via OpenRouter
		{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Desc: "Best overall via OpenRouter", InputPrice: 3.0, OutputPrice: 15.0, ContextSize: 200000, IsRecommended: true, Provider: "Anthropic"},
		{ID: "anthropic/claude-3.5-haiku", Name: "Claude 3.5 Haiku", Desc: "Fast Claude via OpenRouter", InputPrice: 0.25, OutputPrice: 1.25, ContextSize: 200000, Provider: "Anthropic"},
		{ID: "anthropic/claude-3-opus", Name: "Claude 3 Opus", Desc: "Most capable Claude", InputPrice: 15.0, OutputPrice: 75.0, ContextSize: 200000, Provider: "Anthropic"},
		// OpenAI via OpenRouter
		{ID: "openai/gpt-4o", Name: "GPT-4o", Desc: "OpenAI flagship via OpenRouter", InputPrice: 2.50, OutputPrice: 10.0, ContextSize: 128000, IsRecommended: true, Provider: "OpenAI"},
		{ID: "openai/gpt-4o-mini", Name: "GPT-4o-mini", Desc: "Affordable GPT-4 class", InputPrice: 0.15, OutputPrice: 0.60, ContextSize: 128000, Provider: "OpenAI"},
		{ID: "openai/o1-preview", Name: "o1 Preview", Desc: "Reasoning model", InputPrice: 15.0, OutputPrice: 60.0, ContextSize: 128000, Provider: "OpenAI"},
		{ID: "openai/o1-mini", Name: "o1-mini", Desc: "Small reasoning model", InputPrice: 3.0, OutputPrice: 12.0, ContextSize: 128000, Provider: "OpenAI"},
		// Google
		{ID: "google/gemini-2.0-flash-exp", Name: "Gemini 2.0 Flash", Desc: "Latest Gemini experimental", InputPrice: 0.075, OutputPrice: 0.30, ContextSize: 1048576, IsRecommended: true, Provider: "Google"},
		{ID: "google/gemini-pro-1.5", Name: "Gemini Pro 1.5", Desc: "Advanced Gemini", InputPrice: 1.25, OutputPrice: 5.0, ContextSize: 2097152, Provider: "Google"},
		{ID: "google/gemini-flash-1.5", Name: "Gemini Flash 1.5", Desc: "Fast Gemini", InputPrice: 0.075, OutputPrice: 0.30, ContextSize: 1048576, Provider: "Google"},
		// Meta Llama
		{ID: "meta-llama/llama-3.1-405b-instruct", Name: "Llama 3.1 405B", Desc: "Largest open-weight model", InputPrice: 3.0, OutputPrice: 3.0, ContextSize: 131072, IsRecommended: true, Provider: "Meta"},
		{ID: "meta-llama/llama-3.1-70b-instruct", Name: "Llama 3.1 70B", Desc: "Large Llama model", InputPrice: 0.35, OutputPrice: 0.40, ContextSize: 131072, Provider: "Meta"},
		{ID: "meta-llama/llama-3.1-8b-instruct", Name: "Llama 3.1 8B", Desc: "Small Llama model", InputPrice: 0.055, OutputPrice: 0.055, ContextSize: 131072, Provider: "Meta"},
		// DeepSeek
		{ID: "deepseek/deepseek-chat", Name: "DeepSeek Chat", Desc: "Latest DeepSeek model", InputPrice: 0.14, OutputPrice: 0.28, ContextSize: 64000, Provider: "DeepSeek"},
		{ID: "deepseek/deepseek-r1", Name: "DeepSeek R1", Desc: "Reasoning-focused model", InputPrice: 0.55, OutputPrice: 2.19, ContextSize: 64000, IsRecommended: true, Provider: "DeepSeek"},
		// Mistral
		{ID: "mistralai/mistral-large", Name: "Mistral Large", Desc: "Flagship Mistral", InputPrice: 2.0, OutputPrice: 6.0, ContextSize: 128000, Provider: "Mistral"},
		{ID: "mistralai/mistral-medium", Name: "Mistral Medium", Desc: "Balanced Mistral", InputPrice: 2.7, OutputPrice: 8.1, ContextSize: 32000, Provider: "Mistral"},
		{ID: "mistralai/mistral-small", Name: "Mistral Small", Desc: "Fast Mistral", InputPrice: 0.2, OutputPrice: 0.6, ContextSize: 32000, Provider: "Mistral"},
		// xAI Grok
		{ID: "x-ai/grok-2", Name: "Grok 2", Desc: "xAI's latest model", InputPrice: 2.0, OutputPrice: 10.0, ContextSize: 131072, Provider: "xAI"},
		{ID: "x-ai/grok-beta", Name: "Grok Beta", Desc: "xAI beta model", InputPrice: 5.0, OutputPrice: 15.0, ContextSize: 131072, Provider: "xAI"},
		// Qwen
		{ID: "qwen/qwen-2.5-72b-instruct", Name: "Qwen 2.5 72B", Desc: "Large Qwen model", InputPrice: 0.35, OutputPrice: 0.40, ContextSize: 131072, Provider: "Alibaba"},
		{ID: "qwen/qwen-2.5-coder-32b-instruct", Name: "Qwen 2.5 Coder 32B", Desc: "Coding-focused Qwen", InputPrice: 0.07, OutputPrice: 0.16, ContextSize: 131072, Provider: "Alibaba"},
	}
}

// MergeModelLists merges fetched models with known model metadata.
func MergeModelLists(fetched []string, known []ModelInfo) []ModelInfo {
	// Create map of known models by ID
	knownMap := make(map[string]ModelInfo)
	for _, m := range known {
		knownMap[m.ID] = m
	}

	// Build result list
	result := make([]ModelInfo, 0, len(fetched))
	seen := make(map[string]bool)

	// First add recommended models if they exist in fetched
	for _, m := range known {
		if m.IsRecommended {
			for _, f := range fetched {
				if strings.EqualFold(f, m.ID) && !seen[f] {
					result = append(result, m)
					seen[f] = true
					break
				}
			}
		}
	}

	// Then add remaining fetched models
	for _, f := range fetched {
		if seen[f] {
			continue
		}
		if m, ok := knownMap[f]; ok {
			result = append(result, m)
		} else {
			// Unknown model - add with just ID
			result = append(result, ModelInfo{ID: f, Name: f})
		}
		seen[f] = true
	}

	return result
}

// ErrCanceled is returned when the user cancels the model picker (Ctrl+C or Esc).
var ErrCanceled = fmt.Errorf("setup canceled")

// RunModelPicker runs the Bubble Tea model picker and returns the selected model.
// Returns ErrCanceled if the user presses Ctrl+C or Esc to cancel.
// Returns empty string with nil error only if no models were selected but not canceled.
func RunModelPicker(models []ModelInfo, provider, tier string) (string, error) {
	if len(models) == 0 {
		return "", fmt.Errorf("no models available")
	}

	picker := NewModelPicker(models, provider, tier)

	p := tea.NewProgram(picker, tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running model picker: %w", err)
	}

	result := m.(*ModelPicker)
	if result.Canceled() {
		return "", ErrCanceled
	}
	if result.Selected() == nil {
		return "", nil
	}

	return result.Selected().ID, nil
}

// IsTTY checks if stdin is a terminal.
func IsTTY() bool {
	// Check if stdin is a terminal using os.Stdin.Stat()
	if fileInfo, err := os.Stdin.Stat(); err == nil {
		// If mode has ModeCharDevice, it's a terminal
		return (fileInfo.Mode() & os.ModeCharDevice) != 0
	}
	return false
}
