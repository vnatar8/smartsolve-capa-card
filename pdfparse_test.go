package main

import (
	"testing"
	"time"
)

func TestParseDateString(t *testing.T) {
	tests := []struct {
		input string
		want  time.Time
	}{
		{"15-Dec-2025", time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)},
		{"29-May-2026", time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC)},
		{"01-Jan-2024", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		got, err := parseDateString(tt.input)
		if err != nil {
			t.Errorf("parseDateString(%q) error: %v", tt.input, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("parseDateString(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsEffectivenessAction(t *testing.T) {
	if !isEffectivenessAction("Effectiveness Review Plan") {
		t.Error("should be true for Effectiveness Review Plan")
	}
	if isEffectivenessAction("Corrective") {
		t.Error("should be false for Corrective")
	}
	if isEffectivenessAction("Preventive") {
		t.Error("should be false for Preventive")
	}
}

func TestParseCAPADetail(t *testing.T) {
	// Simulated PDF text matching real structure
	text := `CAPA Detail
All Signature (Date and Time) shown in this report are in GMT.
All blank fields are considered as data not applicable or not yet available
CAPA-2025-000054
CAPA Number
Status
INWORKS
Initiation Date
15-Dec-2025
Site
RCH Richmond
CAPA Against
Process
CAPA Type
Internal
CAPA Owner
VIGNESH.NATARAJAN Vignesh Natarajan
CAPA Information
Master CAPA Number
Department

CAPA Source
Audit
Source Number
RCH-AUD-INT-2025-0024
Product Class
Product
Product
3801450 FORMULA R PARAFFIN
Process
Manufacturing Manufacturing
Title
2025 MDSAP Chapter 6 Internal Audit
Description
DHR of lot 2511051 was reviewed and fields were left blank.
Effectiveness Review Interval
Effectiveness Review Interval Unit
180
Days
`
	data, err := parseCAPADetail(text)
	if err != nil {
		t.Fatalf("parseCAPADetail failed: %v", err)
	}

	if data.Number != "CAPA-2025-000054" {
		t.Errorf("Number = %q, want CAPA-2025-000054", data.Number)
	}
	if data.Title != "2025 MDSAP Chapter 6 Internal Audit" {
		t.Errorf("Title = %q, want '2025 MDSAP Chapter 6 Internal Audit'", data.Title)
	}
	if data.Owner != "Vignesh Natarajan" {
		t.Errorf("Owner = %q, want 'Vignesh Natarajan'", data.Owner)
	}
	if !data.InitiationDate.Equal(time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("InitiationDate = %v, want 2025-12-15", data.InitiationDate)
	}
	if data.EffInterval != 180 {
		t.Errorf("EffInterval = %d, want 180", data.EffInterval)
	}
}

func TestParseCAPADetail_RootCause(t *testing.T) {
	text := `CAPA-2025-000054
Root Cause Against
Process
Root Cause
Lack of process, ineffective process, or incorrectly followed process
Root Cause Description
100
Root Cause Percentage
`
	data, err := parseCAPADetail(text)
	if err != nil {
		t.Fatalf("parseCAPADetail failed: %v", err)
	}
	want := "Lack of process, ineffective process, or incorrectly followed process"
	if data.RootCause != want {
		t.Errorf("RootCause = %q, want %q", data.RootCause, want)
	}
}

func TestParseCAPADetail_Actions(t *testing.T) {
	text := `CAPA-2025-000054
Action Plan Items (2)
Action Type
Due Date
Completed By
Completed Date
Completed In
Action Item Title
Action Type
Corrective
29-May-2026
Install Signage in Surgi Path
Action Plan
Action Type
Corrective
Action Type
Action Plan
4
Item No.
Design and install signage
Action Plan Description
VIGNESH.NATARAJAN
Vignesh Natarajan
Assigned to User
Checklist
No
Is Action Item Complete?
Action Item Title
Action Type
Effectiveness Review Plan
15-Aug-2026
Verify signage effectiveness
Action Plan
Action Type
Effectiveness Review Plan
Action Type
Action Plan
5
Item No.
Verify
Action Plan Description
JOHN.DOE
John Doe
Assigned to User
Checklist
No
Is Action Item Complete?
`
	data, err := parseCAPADetail(text)
	if err != nil {
		t.Fatalf("parseCAPADetail failed: %v", err)
	}

	if len(data.Actions) != 1 {
		t.Fatalf("expected 1 corrective action, got %d", len(data.Actions))
	}
	if len(data.VOEActions) != 1 {
		t.Fatalf("expected 1 VOE action, got %d", len(data.VOEActions))
	}
	if !data.HasActions {
		t.Error("HasActions should be true")
	}

	a := data.Actions[0]
	if a.ActionType != "Corrective" {
		t.Errorf("Action type = %q, want Corrective", a.ActionType)
	}
	if !a.DueDate.Equal(time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Action due date = %v, want 2026-05-29", a.DueDate)
	}
	if a.Title != "Install Signage in Surgi Path" {
		t.Errorf("Action title = %q, want 'Install Signage in Surgi Path'", a.Title)
	}
	if a.Owner != "Vignesh Natarajan" {
		t.Errorf("Action owner = %q, want 'Vignesh Natarajan'", a.Owner)
	}

	v := data.VOEActions[0]
	if v.ActionType != "Effectiveness Review Plan" {
		t.Errorf("VOE action type = %q, want 'Effectiveness Review Plan'", v.ActionType)
	}
}
