package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
)

func generateCard(data *CAPAData, outputPath string) error {
	templateData, err := templateFS.ReadFile("template.xlsx")
	if err != nil {
		return fmt.Errorf("cannot read embedded template: %w", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(templateData))
	if err != nil {
		return fmt.Errorf("cannot open template: %w", err)
	}
	defer f.Close()

	front := "Sheet (Front)"
	back := "Sheet (Back)"

	// === FRONT SIDE ===

	// A1: CAPA number
	f.SetCellValue(front, "A1", data.Number)

	// E1: Initiation Date (as time.Time for proper Excel date formatting)
	f.SetCellValue(front, "E1", data.InitiationDate)

	// E2: Owner
	f.SetCellValue(front, "E2", data.Owner)

	// A3: CAPA Title (in the merged problem statement area)
	f.SetCellValue(front, "A3", data.Title)

	// E4, E5: Investigation/Planning due dates - clear template values
	// These will show the actual CAPA's dates, not the template's sample data
	f.SetCellValue(front, "E4", "")
	f.SetCellValue(front, "E5", "")

	// VOE dates
	if data.HasActions && data.EffInterval > 0 {
		voe1, voe2 := calculateVOEDates(data.Actions, data.EffInterval, 0)
		if !voe1.IsZero() {
			f.SetCellValue(front, "E6", voe1)
		} else {
			f.SetCellValue(front, "E6", "TBD")
		}
		if !voe2.IsZero() {
			f.SetCellValue(front, "E7", voe2)
		} else {
			f.SetCellValue(front, "E7", "")
		}
	} else {
		f.SetCellValue(front, "E6", "TBD")
		f.SetCellValue(front, "E7", "")
	}

	// === BACK SIDE ===

	if data.HasActions {
		// B1: Root cause text
		f.SetCellValue(back, "B1", data.RootCause)

		// Sort actions by due date ascending
		sorted := make([]CAPAAction, len(data.Actions))
		copy(sorted, data.Actions)
		sort.SliceStable(sorted, func(i, j int) bool {
			return sorted[i].DueDate.Before(sorted[j].DueDate)
		})

		// Clear template action rows 3-6
		for r := 3; r <= 6; r++ {
			f.SetCellValue(back, fmt.Sprintf("A%d", r), "")
			f.SetCellValue(back, fmt.Sprintf("B%d", r), "")
			f.SetCellValue(back, fmt.Sprintf("E%d", r), "")
			f.SetCellValue(back, fmt.Sprintf("F%d", r), "")
		}

		// Copy style from row 3 (first template action row) for formatting
		row3StyleA, _ := f.GetCellStyle(back, "A3")
		row3StyleB, _ := f.GetCellStyle(back, "B3")
		row3StyleE, _ := f.GetCellStyle(back, "E3")
		row3StyleF, _ := f.GetCellStyle(back, "F3")

		// Populate action rows starting at row 3
		for idx, action := range sorted {
			row := 3 + idx

			// Merge B:D for this row
			f.MergeCell(back, fmt.Sprintf("B%d", row), fmt.Sprintf("D%d", row))

			f.SetCellValue(back, fmt.Sprintf("A%d", row), idx+1)
			f.SetCellValue(back, fmt.Sprintf("B%d", row), action.Title)
			if !action.DueDate.IsZero() {
				f.SetCellValue(back, fmt.Sprintf("E%d", row), action.DueDate)
			}
			f.SetCellValue(back, fmt.Sprintf("F%d", row), shortenName(action.Owner))

			// Apply formatting from template row 3 to rows beyond the template
			if row > 6 {
				f.SetCellStyle(back, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), row3StyleA)
				f.SetCellStyle(back, fmt.Sprintf("B%d", row), fmt.Sprintf("D%d", row), row3StyleB)
				f.SetCellStyle(back, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), row3StyleE)
				f.SetCellStyle(back, fmt.Sprintf("F%d", row), fmt.Sprintf("F%d", row), row3StyleF)
				f.SetRowHeight(back, row, 40)
			}
		}
	} else {
		// Scenario 1: no actions - clear back side data, keep headers
		f.SetCellValue(back, "B1", "")
		for r := 3; r <= 6; r++ {
			f.SetCellValue(back, fmt.Sprintf("A%d", r), "")
			f.SetCellValue(back, fmt.Sprintf("B%d", r), "")
			f.SetCellValue(back, fmt.Sprintf("E%d", r), "")
			f.SetCellValue(back, fmt.Sprintf("F%d", r), "")
		}
	}

	// === REFERENCE TAB ===
	// Contains all CAPA information verbatim (no truncation)
	ref := "Reference"
	f.NewSheet(ref)

	// Header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Family: "Aptos Narrow"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"BFBFBF"}},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
	})
	valueStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 11, Family: "Aptos Narrow"},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
	})

	// Column widths
	f.SetColWidth(ref, "A", "A", 22)
	f.SetColWidth(ref, "B", "B", 80)

	row := 1

	// CAPA Header Info
	writeRefRow := func(label, value string) {
		f.SetCellValue(ref, fmt.Sprintf("A%d", row), label)
		f.SetCellValue(ref, fmt.Sprintf("B%d", row), value)
		f.SetCellStyle(ref, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), headerStyle)
		f.SetCellStyle(ref, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), valueStyle)
		row++
	}

	writeRefRow("CAPA Number", data.Number)
	writeRefRow("Title", data.Title)
	writeRefRow("Initiation Date", data.InitiationDate.Format("02-Jan-2006"))
	writeRefRow("Owner", data.Owner)
	writeRefRow("Description", data.Description)
	row++ // blank row

	// Root Cause Section
	writeRefRow("Root Cause (Full)", data.FullRootCause)
	writeRefRow("Failure Mode", data.FailureMode)
	writeRefRow("Failure Mode Description", data.FailureModeDesc)
	writeRefRow("Caused By", data.CausedBy)
	writeRefRow("Effectiveness Interval", fmt.Sprintf("%d %s", data.EffInterval, data.EffUnit))
	row++ // blank row

	// Actions Section Header
	f.SetCellValue(ref, fmt.Sprintf("A%d", row), "CORRECTIVE/PREVENTIVE ACTIONS")
	f.SetCellStyle(ref, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), headerStyle)
	row++

	// Action column headers
	f.SetColWidth(ref, "C", "C", 15)
	f.SetColWidth(ref, "D", "D", 15)
	f.SetColWidth(ref, "E", "E", 80)
	for _, col := range []string{"A", "B", "C", "D", "E"} {
		f.SetCellStyle(ref, fmt.Sprintf("%s%d", col, row), fmt.Sprintf("%s%d", col, row), headerStyle)
	}
	f.SetCellValue(ref, fmt.Sprintf("A%d", row), "#")
	f.SetCellValue(ref, fmt.Sprintf("B%d", row), "Action Title")
	f.SetCellValue(ref, fmt.Sprintf("C%d", row), "Due Date")
	f.SetCellValue(ref, fmt.Sprintf("D%d", row), "Owner")
	f.SetCellValue(ref, fmt.Sprintf("E%d", row), "Full Description")
	row++

	// Action rows
	for idx, action := range data.Actions {
		for _, col := range []string{"A", "B", "C", "D", "E"} {
			f.SetCellStyle(ref, fmt.Sprintf("%s%d", col, row), fmt.Sprintf("%s%d", col, row), valueStyle)
		}
		f.SetCellValue(ref, fmt.Sprintf("A%d", row), idx+1)
		f.SetCellValue(ref, fmt.Sprintf("B%d", row), action.Title)
		if !action.DueDate.IsZero() {
			f.SetCellValue(ref, fmt.Sprintf("C%d", row), action.DueDate.Format("02-Jan-2006"))
		}
		f.SetCellValue(ref, fmt.Sprintf("D%d", row), action.Owner)
		f.SetCellValue(ref, fmt.Sprintf("E%d", row), action.Description)
		row++
	}
	row++ // blank row

	// VOE Actions
	if len(data.VOEActions) > 0 {
		f.SetCellValue(ref, fmt.Sprintf("A%d", row), "EFFECTIVENESS (VOE) ACTIONS")
		f.SetCellStyle(ref, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), headerStyle)
		row++

		for _, col := range []string{"A", "B", "C", "D", "E"} {
			f.SetCellStyle(ref, fmt.Sprintf("%s%d", col, row), fmt.Sprintf("%s%d", col, row), headerStyle)
		}
		f.SetCellValue(ref, fmt.Sprintf("A%d", row), "#")
		f.SetCellValue(ref, fmt.Sprintf("B%d", row), "Action Title")
		f.SetCellValue(ref, fmt.Sprintf("C%d", row), "Due Date")
		f.SetCellValue(ref, fmt.Sprintf("D%d", row), "Owner")
		f.SetCellValue(ref, fmt.Sprintf("E%d", row), "Full Description")
		row++

		for idx, action := range data.VOEActions {
			for _, col := range []string{"A", "B", "C", "D", "E"} {
				f.SetCellStyle(ref, fmt.Sprintf("%s%d", col, row), fmt.Sprintf("%s%d", col, row), valueStyle)
			}
			f.SetCellValue(ref, fmt.Sprintf("A%d", row), idx+1)
			f.SetCellValue(ref, fmt.Sprintf("B%d", row), action.Title)
			if !action.DueDate.IsZero() {
				f.SetCellValue(ref, fmt.Sprintf("C%d", row), action.DueDate.Format("02-Jan-2006"))
			}
			f.SetCellValue(ref, fmt.Sprintf("D%d", row), action.Owner)
			f.SetCellValue(ref, fmt.Sprintf("E%d", row), action.Description)
			row++
		}
	}

	return f.SaveAs(outputPath)
}

// shortenName truncates owner names longer than 15 characters to
// "FirstName L." format for fitting in the Excel action row cells.
func shortenName(name string) string {
	if len(name) <= 15 {
		return name
	}
	parts := strings.Split(name, " ")
	if len(parts) >= 2 {
		return parts[0] + " " + string(parts[len(parts)-1][0]) + "."
	}
	return name
}
