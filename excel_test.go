package main

import (
	"os"
	"testing"
	"time"
)

func TestGenerateCard(t *testing.T) {
	data := &CAPAData{
		Number:         "CAPA-2025-000054",
		Title:          "2025 MDSAP Chapter 6 Internal Audit",
		InitiationDate: time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC),
		Owner:          "Vignesh Natarajan",
		RootCause:      "Lack of process, ineffective process, or incorrectly followed process",
		HasActions:     true,
		EffInterval:    180,
		Actions: []CAPAAction{
			{Title: "Install Signage in Surgi Production Area", DueDate: time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC), Owner: "Robert Cox", ActionType: "Corrective"},
			{Title: "Update RCH-WI-000362", DueDate: time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC), Owner: "Jeff Stanley", ActionType: "Corrective"},
			{Title: "Update RCH-WI-000289", DueDate: time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC), Owner: "Jeff Stanley", ActionType: "Corrective"},
		},
	}

	outputPath := t.TempDir() + "/test_card.xlsx"
	err := generateCard(data, outputPath)
	if err != nil {
		t.Fatalf("generateCard failed: %v", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if info.Size() < 5000 {
		t.Fatalf("output file too small: %d bytes", info.Size())
	}
	t.Logf("Generated card: %d bytes", info.Size())
}

func TestGenerateCard_Scenario1(t *testing.T) {
	data := &CAPAData{
		Number:         "CAPA-2026-000012",
		Title:          "Form269 issues",
		InitiationDate: time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		Owner:          "Sandra Parker",
		HasActions:     false,
	}

	outputPath := t.TempDir() + "/test_card_s1.xlsx"
	err := generateCard(data, outputPath)
	if err != nil {
		t.Fatalf("generateCard (scenario 1) failed: %v", err)
	}

	info, _ := os.Stat(outputPath)
	t.Logf("Generated scenario 1 card: %d bytes", info.Size())
}

func TestShortenName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Robert Cox", "Robert Cox"},       // short enough
		{"Jeff Stanley", "Jeff Stanley"},    // short enough
		{"Vignesh Natarajan", "Vignesh N."}, // > 15 chars
		{"X", "X"},                          // single char
	}
	for _, tt := range tests {
		got := shortenName(tt.input)
		if got != tt.want {
			t.Errorf("shortenName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
