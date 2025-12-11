// Package setup provides interactive configuration wizards including Bubble Tea TUI components.
package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProfileItem represents a profile in the selector list.
type ProfileItem struct {
	Profile  *Profile
	IsActive bool
}

// Implement list.Item interface
func (p ProfileItem) FilterValue() string { return p.Profile.Name + " " + p.Profile.Description }
func (p ProfileItem) Title() string {
	if p.IsActive {
		return "★ " + p.Profile.Name
	}
	return "  " + p.Profile.Name
}
func (p ProfileItem) Description() string {
	var parts []string
	parts = append(parts, p.Profile.Provider)
	if p.Profile.DefaultModel != "" {
		parts = append(parts, p.Profile.DefaultModel)
	}
	if p.Profile.Description != "" {
		parts = append(parts, p.Profile.Description)
	}
	return strings.Join(parts, " · ")
}

// ProfileSelectorResult represents the result of the profile selector.
type ProfileSelectorResult int

const (
	ProfileResultSelected ProfileSelectorResult = iota
	ProfileResultCreateNew
	ProfileResultCancelled
)

// ProfileSelector is a Bubble Tea model for profile selection.
type ProfileSelector struct {
	list           list.Model
	profiles       []*Profile
	selected       *Profile
	result         ProfileSelectorResult
	width          int
	height         int
	activeProfile  string
}

// Styles for the profile selector
var (
	profileTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		MarginLeft(2).
		MarginBottom(1)

	profileHelpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginLeft(2)

	profileActiveStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("76"))

	profileBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1)
)

// createNewItem is a special item for creating a new profile.
type createNewItem struct{}

func (c createNewItem) FilterValue() string { return "create new profile" }
func (c createNewItem) Title() string       { return "  ➕ Create New Profile" }
func (c createNewItem) Description() string { return "Set up a new provider configuration" }

// NewProfileSelector creates a new profile selector.
func NewProfileSelector(profiles []*Profile, activeProfile string) *ProfileSelector {
	// Create list with custom delegate
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetSpacing(0)

	// Convert to list items - add "Create New" option first
	items := make([]list.Item, 0, len(profiles)+1)

	// Add existing profiles
	for _, p := range profiles {
		items = append(items, ProfileItem{
			Profile:  p,
			IsActive: p.Name == activeProfile,
		})
	}

	// Add "Create New" option at the end
	items = append(items, createNewItem{})

	l := list.New(items, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(false)
	l.SetShowPagination(true)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	return &ProfileSelector{
		list:          l,
		profiles:      profiles,
		activeProfile: activeProfile,
		width:         80,
		height:        20,
	}
}

// Init initializes the profile selector.
func (m *ProfileSelector) Init() tea.Cmd {
	return nil
}

// Update handles input and updates the profile selector state.
func (m *ProfileSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-8)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.result = ProfileResultCancelled
			return m, tea.Quit

		case "enter":
			if item := m.list.SelectedItem(); item != nil {
				switch v := item.(type) {
				case ProfileItem:
					m.selected = v.Profile
					m.result = ProfileResultSelected
				case createNewItem:
					m.result = ProfileResultCreateNew
				}
				return m, tea.Quit
			}

		case "esc":
			m.result = ProfileResultCancelled
			return m, tea.Quit

		case "n":
			// Quick key for "New Profile"
			m.result = ProfileResultCreateNew
			return m, tea.Quit
		}
	}

	// Update list (for navigation)
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	return m, cmd
}

// View renders the profile selector.
func (m *ProfileSelector) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(profileTitleStyle.Render("╔═══════════════════════════════════════════════════════════════╗"))
	b.WriteString("\n")
	b.WriteString(profileTitleStyle.Render("║        CLASP - Select Profile                                 ║"))
	b.WriteString("\n")
	b.WriteString(profileTitleStyle.Render("╚═══════════════════════════════════════════════════════════════╝"))
	b.WriteString("\n\n")

	// Show active profile info
	if m.activeProfile != "" {
		b.WriteString(profileHelpStyle.Render(fmt.Sprintf("Current: %s", profileActiveStyle.Render(m.activeProfile))))
		b.WriteString("\n\n")
	}

	// Profile list
	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	// Help text
	b.WriteString(profileHelpStyle.Render("↑/↓ Navigate • Enter Select • n New Profile • q Quit"))
	b.WriteString("\n")

	return b.String()
}

// Selected returns the selected profile, or nil if cancelled or creating new.
func (m *ProfileSelector) Selected() *Profile {
	return m.selected
}

// Result returns the result of the selection.
func (m *ProfileSelector) Result() ProfileSelectorResult {
	return m.result
}

// RunProfileSelector runs the Bubble Tea profile selector.
// Returns the selected profile name, or empty string for create new / cancelled.
// The second return value indicates whether to create a new profile.
func RunProfileSelector(profiles []*Profile, activeProfile string) (selectedProfile string, createNew bool, cancelled bool, err error) {
	if len(profiles) == 0 {
		// No profiles exist, go straight to create
		return "", true, false, nil
	}

	// If only one profile exists, use it automatically (no need for selector)
	if len(profiles) == 1 {
		return profiles[0].Name, false, false, nil
	}

	selector := NewProfileSelector(profiles, activeProfile)

	p := tea.NewProgram(selector, tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return "", false, false, fmt.Errorf("error running profile selector: %w", err)
	}

	result := m.(*ProfileSelector)
	switch result.Result() {
	case ProfileResultSelected:
		if result.Selected() != nil {
			return result.Selected().Name, false, false, nil
		}
		return "", false, true, nil
	case ProfileResultCreateNew:
		return "", true, false, nil
	case ProfileResultCancelled:
		return "", false, true, nil
	}

	return "", false, true, nil
}

// HasProfiles checks if any profiles exist.
func HasProfiles() bool {
	pm := NewProfileManager()
	profiles, err := pm.ListProfiles()
	if err != nil {
		return false
	}
	return len(profiles) > 0
}

// SelectOrCreateProfile shows the profile selector TUI if profiles exist,
// otherwise returns that a new profile should be created.
// Returns: profileName (if selected), createNew (if should create), cancelled (if user quit)
func SelectOrCreateProfile() (profileName string, createNew bool, cancelled bool) {
	pm := NewProfileManager()

	profiles, err := pm.ListProfiles()
	if err != nil || len(profiles) == 0 {
		// No profiles, need to create one
		return "", true, false
	}

	// Get active profile
	activeProfile := ""
	if globalCfg, err := pm.GetGlobalConfig(); err == nil && globalCfg != nil {
		activeProfile = globalCfg.ActiveProfile
	}

	// Only show selector if TTY is available
	if !IsTTY() {
		// Non-interactive mode - use active profile or first profile
		if activeProfile != "" {
			return activeProfile, false, false
		}
		if len(profiles) > 0 {
			return profiles[0].Name, false, false
		}
		return "", true, false
	}

	// Run the selector TUI
	selectedProfile, createNew, cancelled, err := RunProfileSelector(profiles, activeProfile)
	if err != nil {
		// On error, fall back to active profile
		if activeProfile != "" {
			return activeProfile, false, false
		}
		return "", true, false
	}

	return selectedProfile, createNew, cancelled
}
