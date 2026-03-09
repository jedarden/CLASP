// CLASP - Claude Language Agent Super Proxy
// Profile management commands and functions
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jedarden/clasp/internal/setup"
)

// handleProfileCommand handles all profile subcommands.
func handleProfileCommand(args []string) {
	if len(args) == 0 {
		printProfileHelp()
		return
	}

	wizard := setup.NewWizard()

	switch args[0] {
	case "create":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		if _, err := wizard.RunProfileCreate(name); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "list":
		if err := wizard.RunProfileList(); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "show":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		if err := wizard.RunProfileShow(name); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "use":
		if len(args) < 2 {
			fmt.Println("Usage: clasp profile use <name>")
			os.Exit(1)
		}
		if err := wizard.RunProfileUse(args[1]); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "delete":
		if len(args) < 2 {
			fmt.Println("Usage: clasp profile delete <name>")
			os.Exit(1)
		}
		if err := wizard.RunProfileDelete(args[1]); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "edit":
		// Edit is essentially create with overwrite
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		if _, err := wizard.RunProfileCreate(name); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "export":
		if len(args) < 2 {
			fmt.Println("Usage: clasp profile export <name> [output-file]")
			os.Exit(1)
		}
		outputPath := ""
		if len(args) > 2 {
			outputPath = args[2]
		}
		if err := wizard.RunProfileExport(args[1], outputPath); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "import":
		if len(args) < 2 {
			fmt.Println("Usage: clasp profile import <file> [new-name]")
			os.Exit(1)
		}
		newName := ""
		if len(args) > 2 {
			newName = args[2]
		}
		if err := wizard.RunProfileImport(args[1], newName); err != nil {
			log.Fatalf("[CLASP] %v", err)
		}

	case "help", "-h", "--help":
		printProfileHelp()

	default:
		fmt.Printf("Unknown profile command: %s\n", args[0])
		printProfileHelp()
		os.Exit(1)
	}
}

// printProfileHelp shows profile command help.
func printProfileHelp() {
	fmt.Print(`
CLASP Profile Management

Usage: clasp profile <command> [arguments]

Commands:
  create [name]        Create a new profile interactively
  list                 List all available profiles
  show [name]          Show profile details (current profile if name omitted)
  use <name>           Switch to a different profile
  edit <name>          Edit an existing profile
  delete <name>        Delete a profile
  export <name> [file] Export profile to JSON file
  import <file> [name] Import profile from JSON file

Quick Commands:
  clasp use <name>     Quick alias for 'clasp profile use'

Examples:
  clasp profile create work
  clasp profile list
  clasp profile use personal
  clasp profile export work ./work-profile.json
  clasp profile import ./shared.json team

Profiles are stored in ~/.clasp/profiles/
`)
}

// selectProfile handles profile selection logic.
// Returns the selected profile name or empty string if no selection.
func selectProfile(profileFlag string, proxyOnly bool) string {
	selectedProfileName := profileFlag

	if selectedProfileName == "" && !proxyOnly {
		pm := setup.NewProfileManager()
		profiles, _ := pm.ListProfiles()

		if setup.IsTTY() {
			// Interactive mode - show selector (works with 0+ profiles)
			profileName, createNew, canceled := setup.SelectOrCreateProfile()
			if canceled {
				fmt.Println("\n[CLASP] Canceled.")
				os.Exit(0)
			}
			if createNew {
				// User wants to create a new profile
				wizard := setup.NewWizard()
				newProfile, err := wizard.RunProfileCreate("")
				if err != nil {
					if err == setup.ErrCanceled {
						fmt.Println("\n[CLASP] Setup canceled.")
						os.Exit(0)
					}
					log.Fatalf("[CLASP] Failed to create profile: %v", err)
				}
				selectedProfileName = newProfile.Name
			} else {
				selectedProfileName = profileName
			}
		} else if len(profiles) > 0 {
			// Non-TTY: use active profile or first available
			if globalCfg, err := pm.GetGlobalConfig(); err == nil && globalCfg.ActiveProfile != "" {
				selectedProfileName = globalCfg.ActiveProfile
			} else {
				selectedProfileName = profiles[0].Name
			}
		}
	}

	return selectedProfileName
}

// applyProfile applies the selected profile to the environment.
func applyProfile(profileName string) {
	if profileName == "" {
		return
	}

	pm := setup.NewProfileManager()
	profile, err := pm.GetProfile(profileName)
	if err != nil {
		log.Fatalf("[CLASP] Profile '%s' not found. Run 'clasp profile list' to see available profiles.", profileName)
	}
	if err := pm.ApplyProfileToEnv(profile); err != nil {
		log.Fatalf("[CLASP] Failed to apply profile: %v", err)
	}
	log.Printf("[CLASP] Using profile: %s", profileName)
}
