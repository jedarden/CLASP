package tests

import (
	"testing"

	"github.com/jedarden/clasp/internal/setup"
)

func TestModelPickerCreation(t *testing.T) {
	models := []setup.ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Desc: "Most capable multimodal"},
		{ID: "gpt-4o-mini", Name: "GPT-4o-mini", Desc: "Fast and affordable"},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Desc: "Fast and efficient"},
	}

	picker := setup.NewModelPicker(models, "openai", "")
	if picker == nil {
		t.Fatal("Expected picker to be created")
	}

	// Test that the picker initializes correctly
	if picker.Canceled() {
		t.Error("Picker should not be canceled initially")
	}

	if picker.Selected() != nil {
		t.Error("Picker should not have a selection initially")
	}
}

func TestGetKnownModels(t *testing.T) {
	tests := []struct {
		provider    string
		expectEmpty bool
	}{
		{"openai", false},
		{"openrouter", false},
		{"anthropic", false},
		{"azure", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			models := setup.GetKnownModels(tt.provider)
			if tt.expectEmpty && len(models) != 0 {
				t.Errorf("Expected empty models for %s, got %d", tt.provider, len(models))
			}
			if !tt.expectEmpty && len(models) == 0 {
				t.Errorf("Expected models for %s, got empty", tt.provider)
			}
		})
	}
}

func TestMergeModelLists(t *testing.T) {
	fetched := []string{"gpt-4o", "gpt-4o-mini", "custom-model"}
	known := []setup.ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Desc: "Most capable", IsRecommended: true},
		{ID: "gpt-4o-mini", Name: "GPT-4o-mini", Desc: "Fast"},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Desc: "Previous flagship"},
	}

	result := setup.MergeModelLists(fetched, known)

	if len(result) != 3 {
		t.Errorf("Expected 3 merged models, got %d", len(result))
	}

	// First should be recommended
	if !result[0].IsRecommended {
		t.Error("First model should be recommended")
	}

	// Custom model should be included with just ID
	found := false
	for _, m := range result {
		if m.ID == "custom-model" {
			found = true
			if m.Name != "custom-model" {
				t.Errorf("Custom model name should be ID, got %s", m.Name)
			}
		}
	}
	if !found {
		t.Error("Custom model should be in result")
	}
}
