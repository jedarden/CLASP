// Package setup provides interactive configuration wizards including Bubble Tea TUI components.
package setup

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
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
	ProfileResultRenamed
	ProfileResultDeleted
)

// ProfileSelectorMode represents the current mode of the selector.
type ProfileSelectorMode int

const (
	ModeSelect ProfileSelectorMode = iota
	ModeRename
	ModeDeleteConfirm
)

// ProfileSelector is a Bubble Tea model for profile selection.
type ProfileSelector struct {
	list          list.Model
	profiles      []*Profile
	selected      *Profile
	result        ProfileSelectorResult
	width         int
	height        int
	activeProfile string

	// Mode handling for rename/delete
	mode        ProfileSelectorMode
	renameInput textinput.Model
	errorMsg    string
	successMsg  string

	// Profile manager for operations
	pm *ProfileManager
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

	profileErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		MarginLeft(2)

	profileSuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("76")).
		MarginLeft(2)

	profileWarningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		MarginLeft(2)

	profileInputStyle = lipgloss.NewStyle().
		MarginLeft(2)
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

	// Create rename input
	ti := textinput.New()
	ti.Placeholder = "Enter new profile name"
	ti.CharLimit = 50
	ti.Width = 40

	return &ProfileSelector{
		list:          l,
		profiles:      profiles,
		activeProfile: activeProfile,
		width:         80,
		height:        20,
		mode:          ModeSelect,
		renameInput:   ti,
		pm:            NewProfileManager(),
	}
}

// Init initializes the profile selector.
func (m *ProfileSelector) Init() tea.Cmd {
	return nil
}

// Update handles input and updates the profile selector state.
func (m *ProfileSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Clear messages on any input
	if _, ok := msg.(tea.KeyMsg); ok {
		m.errorMsg = ""
		m.successMsg = ""
	}

	switch m.mode {
	case ModeRename:
		return m.updateRenameMode(msg)
	case ModeDeleteConfirm:
		return m.updateDeleteMode(msg)
	default:
		return m.updateSelectMode(msg)
	}
}

// updateSelectMode handles input in the main selection mode.
func (m *ProfileSelector) updateSelectMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-10)
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

		case "r":
			// Rename the selected profile
			if item := m.list.SelectedItem(); item != nil {
				if profileItem, ok := item.(ProfileItem); ok {
					if profileItem.Profile.Name == "default" {
						m.errorMsg = "Cannot rename the default profile"
						return m, nil
					}
					m.selected = profileItem.Profile
					m.mode = ModeRename
					m.renameInput.SetValue(profileItem.Profile.Name)
					m.renameInput.Focus()
					return m, textinput.Blink
				}
			}

		case "d", "backspace", "delete":
			// Delete the selected profile
			if item := m.list.SelectedItem(); item != nil {
				if profileItem, ok := item.(ProfileItem); ok {
					if profileItem.Profile.Name == "default" {
						m.errorMsg = "Cannot delete the default profile"
						return m, nil
					}
					m.selected = profileItem.Profile
					m.mode = ModeDeleteConfirm
					return m, nil
				}
			}
		}
	}

	// Update list (for navigation)
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	return m, cmd
}

// updateRenameMode handles input in rename mode.
func (m *ProfileSelector) updateRenameMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			// Cancel rename
			m.mode = ModeSelect
			m.renameInput.SetValue("")
			return m, nil

		case "enter":
			// Perform rename
			newName := strings.TrimSpace(m.renameInput.Value())
			if newName == "" {
				m.errorMsg = "Profile name cannot be empty"
				return m, nil
			}

			oldName := m.selected.Name
			if err := m.pm.RenameProfile(oldName, newName); err != nil {
				m.errorMsg = err.Error()
				return m, nil
			}

			// Update the profile in our list
			m.selected.Name = newName

			// Refresh the list
			m.refreshProfileList()

			// Update active profile reference if needed
			if m.activeProfile == oldName {
				m.activeProfile = newName
			}

			m.successMsg = fmt.Sprintf("Profile renamed to '%s'", newName)
			m.mode = ModeSelect
			m.renameInput.SetValue("")
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

// updateDeleteMode handles input in delete confirmation mode.
func (m *ProfileSelector) updateDeleteMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "n", "N":
			// Cancel delete
			m.mode = ModeSelect
			return m, nil

		case "y", "Y", "enter":
			// Perform delete
			profileName := m.selected.Name
			if err := m.pm.DeleteProfile(profileName); err != nil {
				m.errorMsg = err.Error()
				m.mode = ModeSelect
				return m, nil
			}

			// Refresh the list
			m.refreshProfileList()

			// Update active profile if deleted
			if m.activeProfile == profileName {
				m.activeProfile = "default"
			}

			m.successMsg = fmt.Sprintf("Profile '%s' deleted", profileName)
			m.mode = ModeSelect
			m.selected = nil
			return m, nil
		}
	}

	return m, nil
}

// refreshProfileList reloads profiles from disk and updates the list.
func (m *ProfileSelector) refreshProfileList() {
	profiles, err := m.pm.ListProfiles()
	if err != nil {
		return
	}
	m.profiles = profiles

	// Rebuild list items
	items := make([]list.Item, 0, len(profiles)+1)
	for _, p := range profiles {
		items = append(items, ProfileItem{
			Profile:  p,
			IsActive: p.Name == m.activeProfile,
		})
	}
	items = append(items, createNewItem{})

	m.list.SetItems(items)
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

	// Show error or success message
	if m.errorMsg != "" {
		b.WriteString(profileErrorStyle.Render("✗ " + m.errorMsg))
		b.WriteString("\n\n")
	}
	if m.successMsg != "" {
		b.WriteString(profileSuccessStyle.Render("✓ " + m.successMsg))
		b.WriteString("\n\n")
	}

	// Mode-specific views
	switch m.mode {
	case ModeRename:
		b.WriteString(m.viewRenameMode())
	case ModeDeleteConfirm:
		b.WriteString(m.viewDeleteMode())
	default:
		b.WriteString(m.viewSelectMode())
	}

	return b.String()
}

// viewSelectMode renders the main selection view.
func (m *ProfileSelector) viewSelectMode() string {
	var b strings.Builder

	// Profile list
	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	// Help text
	b.WriteString(profileHelpStyle.Render("↑/↓ Navigate • Enter Select • n New • r Rename • d Delete • q Quit"))
	b.WriteString("\n")

	return b.String()
}

// viewRenameMode renders the rename input view.
func (m *ProfileSelector) viewRenameMode() string {
	var b strings.Builder

	b.WriteString(profileWarningStyle.Render(fmt.Sprintf("Renaming profile: %s", m.selected.Name)))
	b.WriteString("\n\n")

	b.WriteString(profileInputStyle.Render("New name: "))
	b.WriteString(m.renameInput.View())
	b.WriteString("\n\n")

	b.WriteString(profileHelpStyle.Render("Enter Confirm • Esc Cancel"))
	b.WriteString("\n")

	return b.String()
}

// viewDeleteMode renders the delete confirmation view.
func (m *ProfileSelector) viewDeleteMode() string {
	var b strings.Builder

	b.WriteString(profileErrorStyle.Render(fmt.Sprintf("Delete profile '%s'?", m.selected.Name)))
	b.WriteString("\n\n")

	b.WriteString(profileHelpStyle.Render(fmt.Sprintf("This will permanently remove the profile '%s'.", m.selected.Name)))
	b.WriteString("\n")
	b.WriteString(profileHelpStyle.Render("This action cannot be undone."))
	b.WriteString("\n\n")

	b.WriteString(profileWarningStyle.Render("Press Y to confirm, N or Esc to cancel"))
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

	// Show selector for 1+ profiles (always includes "Create New" option)
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
