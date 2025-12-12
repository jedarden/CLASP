// Package setup provides secure input handling for sensitive data like API keys.
package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SecureInput provides password-style masked input for API keys.
type SecureInput struct {
	textInput   textinput.Model
	prompt      string
	placeholder string
	value       string
	submitted   bool
	canceled    bool
	showLast4   bool // Show last 4 characters
}

// Styles for secure input
var (
	securePromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39"))

	secureHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	validKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("76"))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))
)

// NewSecureInput creates a new password-style input for API keys.
func NewSecureInput(prompt, placeholder string, showLast4 bool) *SecureInput {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.Focus()
	ti.CharLimit = 200 // API keys can be long
	ti.Width = 50

	return &SecureInput{
		textInput:   ti,
		prompt:      prompt,
		placeholder: placeholder,
		showLast4:   showLast4,
	}
}

// Init initializes the secure input.
func (s *SecureInput) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles input and updates the secure input state.
func (s *SecureInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c":
			s.canceled = true
			return s, tea.Quit

		case "enter":
			s.value = s.textInput.Value()
			s.submitted = true
			return s, tea.Quit

		case "esc":
			s.canceled = true
			return s, tea.Quit
		}
	}

	var cmd tea.Cmd
	s.textInput, cmd = s.textInput.Update(msg)
	return s, cmd
}

// View renders the secure input.
func (s *SecureInput) View() string {
	var b strings.Builder

	// Prompt
	_, _ = b.WriteString("\n")
	_, _ = b.WriteString(securePromptStyle.Render(s.prompt))
	_, _ = b.WriteString("\n\n")

	// Text input with masked display
	_, _ = b.WriteString("  ")
	_, _ = b.WriteString(s.textInput.View())
	_, _ = b.WriteString("\n")

	// Show preview with last 4 chars if enabled and has value
	if s.showLast4 && len(s.textInput.Value()) > 4 {
		value := s.textInput.Value()
		last4 := value[len(value)-4:]
		masked := strings.Repeat("•", len(value)-4) + last4
		_, _ = b.WriteString("\n")
		_, _ = b.WriteString(secureHintStyle.Render(fmt.Sprintf("  Preview: %s", masked)))
		_, _ = b.WriteString("\n")
	}

	// Validation hint
	_, _ = b.WriteString("\n")
	switch {
	case s.validateAPIKey():
		_, _ = b.WriteString(validKeyStyle.Render("  ✓ Key format looks valid"))
	case s.textInput.Value() != "":
		_, _ = b.WriteString(secureHintStyle.Render("  Enter your API key (press Enter when done)"))
	default:
		_, _ = b.WriteString(secureHintStyle.Render("  Paste or type your API key"))
	}
	_, _ = b.WriteString("\n")

	// Help
	_, _ = b.WriteString("\n")
	_, _ = b.WriteString(secureHintStyle.Render("  Press Enter to confirm • Esc to cancel"))
	_, _ = b.WriteString("\n")

	return b.String()
}

// validateAPIKey performs basic validation on the API key format.
func (s *SecureInput) validateAPIKey() bool {
	value := s.textInput.Value()
	if len(value) < 10 {
		return false
	}

	// Check common API key prefixes
	prefixes := []string{
		"sk-",      // OpenAI, Anthropic
		"sk-or-",   // OpenRouter
		"sk-ant-",  // Anthropic
		"sk-proj-", // OpenAI project keys
		"xai-",     // xAI
		"AIza",     // Google
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}

	// For Azure and custom, just check minimum length
	return len(value) >= 20
}

// Value returns the entered value.
func (s *SecureInput) Value() string {
	return s.value
}

// Submitted returns true if the user pressed Enter.
func (s *SecureInput) Submitted() bool {
	return s.submitted
}

// Canceled returns true if the user pressed Esc or Ctrl+C.
func (s *SecureInput) Canceled() bool {
	return s.canceled
}

// RunSecureInput runs the secure input and returns the entered value.
// Returns empty string if canceled.
func RunSecureInput(prompt, placeholder string, showLast4 bool) (string, error) {
	input := NewSecureInput(prompt, placeholder, showLast4)

	p := tea.NewProgram(input)
	m, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running secure input: %w", err)
	}

	result, ok := m.(*SecureInput)
	if !ok {
		return "", fmt.Errorf("unexpected model type")
	}
	if result.Canceled() {
		return "", nil
	}

	return result.Value(), nil
}

// WarnCLIAPIKey prints a warning if API key is passed via command line.
func WarnCLIAPIKey() string {
	return warningStyle.Render(`
⚠️  Warning: API key passed via command line may be visible in shell history.
   Consider using environment variables or 'clasp setup' instead.

   Secure alternatives:
   • Set environment variable: export OPENAI_API_KEY=sk-...
   • Run setup wizard: clasp setup
   • Use profile: clasp profile create myprofile

`)
}

// MaskAPIKeyForDisplay masks an API key for safe display.
func MaskAPIKeyForDisplay(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("•", len(key))
	}
	return key[:4] + strings.Repeat("•", len(key)-8) + key[len(key)-4:]
}
