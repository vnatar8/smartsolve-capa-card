package main

import (
	"testing"
	"time"
)

func TestAddBusinessDays(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
		days  int
		want  time.Time
	}{
		{
			name:  "simple weekdays",
			start: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), // Monday
			days:  5,
			want:  time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC), // next Monday
		},
		{
			name:  "skip weekend",
			start: time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC), // Friday
			days:  1,
			want:  time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC), // Monday
		},
		{
			name:  "10 business days from Wednesday",
			start: time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC), // Wednesday
			days:  10,
			want:  time.Date(2026, 1, 21, 0, 0, 0, 0, time.UTC), // Wednesday 2 weeks later
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addBusinessDays(tt.start, tt.days)
			if !got.Equal(tt.want) {
				t.Errorf("addBusinessDays(%v, %d) = %v, want %v", tt.start.Format("Mon 2006-01-02"), tt.days, got.Format("Mon 2006-01-02"), tt.want.Format("Mon 2006-01-02"))
			}
		})
	}
}

func TestCalculateVOEDates(t *testing.T) {
	actions := []CAPAAction{
		{DueDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)},
		{DueDate: time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC)},
		{DueDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)},
	}

	voe1, voe2 := calculateVOEDates(actions, 90, 0)
	if voe1.IsZero() {
		t.Fatal("VOE 1 should not be zero")
	}
	if !voe2.IsZero() {
		t.Fatal("VOE 2 should be zero when second interval is 0")
	}

	expected := addBusinessDays(time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC), 90)
	if !voe1.Equal(expected) {
		t.Errorf("VOE 1 = %v, want %v", voe1, expected)
	}
}

func TestCalculateVOEDates_NoActions(t *testing.T) {
	voe1, voe2 := calculateVOEDates(nil, 90, 0)
	if !voe1.IsZero() || !voe2.IsZero() {
		t.Error("expected zero dates for empty actions")
	}
}
