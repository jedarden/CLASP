package tests

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/jedarden/clasp/internal/setup"
)

func TestRenameProfile(t *testing.T) {
    // Set up a temporary home directory
    tmpDir := t.TempDir()
    originalHome := os.Getenv("HOME")
    os.Setenv("HOME", tmpDir)
    defer os.Setenv("HOME", originalHome)

    pm := setup.NewProfileManager()

    // Create test profiles
    testProfile := &setup.Profile{
        Name:         "test-profile",
        Provider:     "anthropic",
        DefaultModel: "claude-3-sonnet",
        CreatedAt:    time.Now().Format(time.RFC3339),
        UpdatedAt:    time.Now().Format(time.RFC3339),
    }

    if err := pm.CreateProfile(testProfile); err != nil {
        t.Fatalf("Failed to create test profile: %v", err)
    }

    // Test rename
    err := pm.RenameProfile("test-profile", "renamed-profile")
    if err != nil {
        t.Fatalf("RenameProfile failed: %v", err)
    }

    // Verify old profile doesn't exist
    if pm.ProfileExists("test-profile") {
        t.Error("Old profile still exists after rename")
    }

    // Verify new profile exists
    if !pm.ProfileExists("renamed-profile") {
        t.Error("Renamed profile doesn't exist")
    }

    // Verify profile data is preserved
    renamedProfile, err := pm.GetProfile("renamed-profile")
    if err != nil {
        t.Fatalf("Failed to get renamed profile: %v", err)
    }
    if renamedProfile.Provider != "anthropic" {
        t.Errorf("Provider not preserved: got %s, want anthropic", renamedProfile.Provider)
    }
    if renamedProfile.DefaultModel != "claude-3-sonnet" {
        t.Errorf("Model not preserved: got %s, want claude-3-sonnet", renamedProfile.DefaultModel)
    }
}

func TestRenameDefaultProfileFails(t *testing.T) {
    tmpDir := t.TempDir()
    originalHome := os.Getenv("HOME")
    os.Setenv("HOME", tmpDir)
    defer os.Setenv("HOME", originalHome)

    pm := setup.NewProfileManager()

    // Create default profile
    defaultProfile := &setup.Profile{
        Name:      "default",
        Provider:  "openai",
        CreatedAt: time.Now().Format(time.RFC3339),
        UpdatedAt: time.Now().Format(time.RFC3339),
    }
    pm.CreateProfile(defaultProfile)

    // Try to rename default - should fail
    err := pm.RenameProfile("default", "new-name")
    if err == nil {
        t.Error("Expected error when renaming default profile, got nil")
    }
}

func TestRenameToExistingProfileFails(t *testing.T) {
    tmpDir := t.TempDir()
    originalHome := os.Getenv("HOME")
    os.Setenv("HOME", tmpDir)
    defer os.Setenv("HOME", originalHome)

    pm := setup.NewProfileManager()

    // Create two profiles
    profile1 := &setup.Profile{
        Name:      "profile1",
        Provider:  "openai",
        CreatedAt: time.Now().Format(time.RFC3339),
        UpdatedAt: time.Now().Format(time.RFC3339),
    }
    profile2 := &setup.Profile{
        Name:      "profile2",
        Provider:  "anthropic",
        CreatedAt: time.Now().Format(time.RFC3339),
        UpdatedAt: time.Now().Format(time.RFC3339),
    }
    pm.CreateProfile(profile1)
    pm.CreateProfile(profile2)

    // Try to rename profile1 to profile2 - should fail
    err := pm.RenameProfile("profile1", "profile2")
    if err == nil {
        t.Error("Expected error when renaming to existing profile name, got nil")
    }
}

func TestRenameUpdatesActiveProfile(t *testing.T) {
    tmpDir := t.TempDir()
    originalHome := os.Getenv("HOME")
    os.Setenv("HOME", tmpDir)
    defer os.Setenv("HOME", originalHome)

    pm := setup.NewProfileManager()

    // Create and set active profile
    profile := &setup.Profile{
        Name:      "myprofile",
        Provider:  "openai",
        CreatedAt: time.Now().Format(time.RFC3339),
        UpdatedAt: time.Now().Format(time.RFC3339),
    }
    pm.CreateProfile(profile)
    pm.SetActiveProfile("myprofile")

    // Rename
    err := pm.RenameProfile("myprofile", "newprofile")
    if err != nil {
        t.Fatalf("RenameProfile failed: %v", err)
    }

    // Verify active profile was updated
    globalCfg, err := pm.GetGlobalConfig()
    if err != nil {
        t.Fatalf("Failed to get global config: %v", err)
    }
    if globalCfg.ActiveProfile != "newprofile" {
        t.Errorf("Active profile not updated: got %s, want newprofile", globalCfg.ActiveProfile)
    }
}

func TestDeleteProfile(t *testing.T) {
    tmpDir := t.TempDir()
    originalHome := os.Getenv("HOME")
    os.Setenv("HOME", tmpDir)
    defer os.Setenv("HOME", originalHome)

    pm := setup.NewProfileManager()

    // Create test profile
    profile := &setup.Profile{
        Name:      "todelete",
        Provider:  "openai",
        CreatedAt: time.Now().Format(time.RFC3339),
        UpdatedAt: time.Now().Format(time.RFC3339),
    }
    pm.CreateProfile(profile)

    // Verify it exists
    if !pm.ProfileExists("todelete") {
        t.Fatal("Profile should exist before delete")
    }

    // Delete
    err := pm.DeleteProfile("todelete")
    if err != nil {
        t.Fatalf("DeleteProfile failed: %v", err)
    }

    // Verify it's gone
    if pm.ProfileExists("todelete") {
        t.Error("Profile still exists after delete")
    }

    // Verify file is deleted
    profilesDir := filepath.Join(tmpDir, ".clasp", "profiles")
    profilePath := filepath.Join(profilesDir, "todelete.json")
    if _, err := os.Stat(profilePath); !os.IsNotExist(err) {
        t.Error("Profile file still exists after delete")
    }
}

func TestDeleteDefaultProfileFails(t *testing.T) {
    tmpDir := t.TempDir()
    originalHome := os.Getenv("HOME")
    os.Setenv("HOME", tmpDir)
    defer os.Setenv("HOME", originalHome)

    pm := setup.NewProfileManager()

    // Create default profile
    defaultProfile := &setup.Profile{
        Name:      "default",
        Provider:  "openai",
        CreatedAt: time.Now().Format(time.RFC3339),
        UpdatedAt: time.Now().Format(time.RFC3339),
    }
    pm.CreateProfile(defaultProfile)

    // Try to delete default - should fail
    err := pm.DeleteProfile("default")
    if err == nil {
        t.Error("Expected error when deleting default profile, got nil")
    }

    // Verify default still exists
    if !pm.ProfileExists("default") {
        t.Error("Default profile should still exist after failed delete")
    }
}
