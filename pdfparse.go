package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	gopdf "github.com/ledongthuc/pdf"
)

var datePattern = regexp.MustCompile(`\b(\d{2}-[A-Z][a-z]{2}-\d{4})\b`)
var capaNumberPattern = regexp.MustCompile(`\bCAPA-\d{4}-\d{6}\b`)
var usernameFullNamePattern = regexp.MustCompile(`\b([A-Z][A-Z0-9]+\.[A-Z][A-Z0-9]+)\s+([A-Z][a-z]+(?:\s+[A-Z][a-z]+)+)`)

func parseDateString(s string) (time.Time, error) {
	return time.Parse("02-Jan-2006", strings.TrimSpace(s))
}

func isEffectivenessAction(actionType string) bool {
	return strings.Contains(actionType, "Effectiveness")
}

// extractTextFromPDF extracts text from a PDF using PyMuPDF (preferred), pdftotext, or Go PDF library.
// PyMuPDF produces the most consistent output across different SmartSolve CAPA PDF structures.
func extractTextFromPDF(pdfPath string) (string, error) {
	// Try PyMuPDF via Python first (most reliable for SmartSolve PDFs)
	if text, err := runPyMuPDF(pdfPath); err == nil && len(text) > 100 {
		return text, nil
	}
	// Try pdftotext as fallback
	if text, err := runPdftotext(pdfPath, false); err == nil && len(text) > 100 {
		return text, nil
	}
	// Last resort: Go PDF library
	return extractWithGoLibrary(pdfPath)
}

// extractLayoutTextFromPDF extracts text with layout preservation (for owner detection).
func extractLayoutTextFromPDF(pdfPath string) (string, error) {
	return runPdftotext(pdfPath, true)
}

// runPyMuPDF shells out to Python with PyMuPDF (fitz) for text extraction.
// This produces clean line-by-line output that is consistent across all CAPA PDF structures.
func runPyMuPDF(pdfPath string) (string, error) {
	script := `import sys, os, fitz
sys.stdout.reconfigure(encoding='utf-8', errors='replace')
doc = fitz.open(sys.argv[1])
for page in doc:
    sys.stdout.write(page.get_text())
doc.close()`
	cmd := exec.Command("python", "-c", script, pdfPath)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("PyMuPDF extraction failed: %w", err)
	}
	return string(out), nil
}

func runPdftotext(pdfPath string, layout bool) (string, error) {
	args := []string{}
	if layout {
		args = append(args, "-layout")
	}
	args = append(args, pdfPath, "-")
	cmd := exec.Command("pdftotext", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func extractWithGoLibrary(pdfPath string) (string, error) {
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return "", fmt.Errorf("cannot read PDF file: %w", err)
	}
	data = bytes.ReplaceAll(data, []byte(" \n"), []byte("\r\n"))
	reader := bytes.NewReader(data)
	r, err := gopdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("cannot parse PDF: %w", err)
	}
	var text strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		content, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		text.WriteString(content)
	}
	return text.String(), nil
}

// parseCAPADetail parses CAPA header fields from pdftotext output (non-layout mode).
// It handles both the "label on one line, value on next" format (spec) and the
// SmartSolve pdftotext format where values may be on the same line.
func parseCAPADetail(text string) (*CAPAData, error) {
	lines := strings.Split(text, "\n")
	data := &CAPAData{}

	// === CAPA Number ===
	if m := capaNumberPattern.FindString(text); m != "" {
		data.Number = m
	}

	// === Initiation Date ===
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Pattern 1: label on previous line, date on next line (spec format)
		if trimmed == "Initiation Date" && i+1 < len(lines) {
			if d, err := parseDateString(strings.TrimSpace(lines[i+1])); err == nil {
				data.InitiationDate = d
				break
			}
		}

		// Pattern 2: date on same line as status (pdftotext format)
		if data.InitiationDate.IsZero() && (strings.Contains(line, "INWORKS") || strings.Contains(line, "CLOSED")) {
			if m := datePattern.FindString(line); m != "" {
				if d, err := parseDateString(m); err == nil {
					data.InitiationDate = d
				}
			}
		}
	}

	// === CAPA Owner ===
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "CAPA Owner" && i+1 < len(lines) {
			ownerLine := strings.TrimSpace(lines[i+1])
			if m := usernameFullNamePattern.FindStringSubmatch(ownerLine); len(m) >= 3 {
				data.Owner = m[2]
			} else {
				parts := strings.SplitN(ownerLine, " ", 2)
				if len(parts) >= 2 {
					data.Owner = parts[1]
				}
			}
			break
		}
	}
	if data.Owner == "" {
		for _, line := range lines {
			if strings.Contains(line, "INWORKS") || strings.Contains(line, "CLOSED") {
				if m := usernameFullNamePattern.FindStringSubmatch(line); len(m) >= 3 {
					data.Owner = m[2]
					break
				}
			}
		}
	}

	// === Title ===
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Pattern 1: "Title" on its own line, next non-empty line is the title
		if trimmed == "Title" && i+1 < len(lines) {
			nextLine := strings.TrimSpace(lines[i+1])
			if nextLine != "" && nextLine != "Description" {
				data.Title = nextLine
				break
			}
		}
	}
	// Pattern 2: title appears after "Priority Title" label block (pdftotext)
	if data.Title == "" {
		for i, line := range lines {
			if strings.HasSuffix(strings.TrimSpace(line), "Title") {
				for j := i + 1; j < min(i+10, len(lines)); j++ {
					val := strings.TrimSpace(lines[j])
					if val == "" || val == "Description" || val == "CAPA Detail" {
						continue
					}
					if strings.Contains(val, "Audit") && strings.Contains(val, "Product") {
						continue
					}
					if !strings.HasPrefix(val, "Status") && !strings.HasPrefix(val, "All Signature") &&
						!strings.HasPrefix(val, "Master") && len(val) > 10 {
						data.Title = val
						break
					}
				}
				if data.Title != "" {
					break
				}
			}
		}
	}

	// === Effectiveness Review Interval ===
	effLinePattern := regexp.MustCompile(`Effectiveness Review Interval\s+(\d+)`)
	if m := effLinePattern.FindStringSubmatch(text); len(m) >= 2 {
		data.EffInterval = parseInt(m[1])
	}
	if data.EffInterval == 0 {
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "Effectiveness Review Interval" {
				for j := i + 1; j < min(i+5, len(lines)); j++ {
					val := strings.TrimSpace(lines[j])
					if n := parseInt(val); n > 0 {
						data.EffInterval = n
						break
					}
				}
				if data.EffInterval > 0 {
					break
				}
			}
		}
	}
	// Fallback: "NNN <SITE>" pattern
	if data.EffInterval == 0 {
		effFallback := regexp.MustCompile(`^(\d+)\s+[A-Z]{3}\b`)
		for _, line := range lines {
			if m := effFallback.FindStringSubmatch(strings.TrimSpace(line)); len(m) >= 2 {
				data.EffInterval = parseInt(m[1])
				break
			}
		}
	}

	if data.EffInterval > 0 {
		data.EffUnit = "Days"
	}

	// === Problem Statement (Description) ===
	// In pdftotext output, the description appears as a standalone paragraph
	// near the "Description" label, but with several intervening label lines.
	// Look for the first long paragraph (>50 chars) within 15 lines of "Description".
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Description" && data.Description == "" {
			for j := i + 1; j < min(i+15, len(lines)); j++ {
				val := strings.TrimSpace(lines[j])
				// Look for a substantial paragraph (not a label)
				if len(val) > 50 && !strings.HasPrefix(val, "All Signature") &&
					!strings.HasPrefix(val, "Report is") &&
					!strings.HasPrefix(val, "Data is") &&
					!strings.HasPrefix(val, "Page ") {
					data.Description = val
					break
				}
			}
			if data.Description != "" {
				break
			}
		}
	}

	// === Failure Mode and Caused By ===
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// "Manufacturing Manufacturing Operator Error Internal Associate did not follow defined process"
		if strings.Contains(trimmed, "Operator Error") && len(trimmed) > 20 {
			data.FailureMode = "Operator Error"
			idx := strings.Index(trimmed, "Operator Error ")
			if idx >= 0 {
				remainder := strings.TrimSpace(trimmed[idx+len("Operator Error "):])
				if len(remainder) > 10 {
					data.FailureModeDesc = remainder
				}
			}
		}
		// Extract "Caused By" organizational unit using configured site codes
		hasSiteCode := false
		for _, sc := range config.SiteCodes {
			if strings.Contains(trimmed, sc) {
				hasSiteCode = true
				break
			}
		}
		if hasSiteCode || strings.Contains(trimmed, "Caused By") {
			for _, marker := range config.SiteCodes {
				if idx := strings.Index(trimmed, marker); idx >= 0 {
					data.CausedBy = strings.TrimSpace(trimmed[idx:])
					// Remove trailing "Department" or "Organization"
					data.CausedBy = strings.TrimSuffix(data.CausedBy, " Department")
					data.CausedBy = strings.TrimSuffix(data.CausedBy, " Organization Unit Type")
					break
				}
			}
		}
	}

	// === Root Cause ===
	// Strategy: look for the detailed root cause paragraph that appears after
	// "Failure Mode Description" label line in pdftotext output. This is usually
	// a multi-sentence explanation. If too long for the card, we truncate.
	// Fallback 1: "Failure Mode Description" as a standalone label (PyMuPDF format)
	// Fallback 2: generic "Root Cause Description" value

	// pdftotext format: line contains "Failure Mode Description", next non-empty
	// line is the detailed root cause paragraph
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "Failure Mode Description") {
			for j := i + 1; j < min(i+5, len(lines)); j++ {
				val := strings.TrimSpace(lines[j])
				if val != "" && len(val) > 20 &&
					!strings.HasPrefix(val, "Page ") &&
					!strings.HasPrefix(val, "Data is") &&
					!strings.HasPrefix(val, "All Signature") &&
					!strings.HasPrefix(val, "Report is") {
					data.RootCause = val
					break
				}
			}
			if data.RootCause != "" {
				break
			}
		}
	}

	// Also look for "Internal Associate" or similar specific failure mode descriptions
	// embedded in combined pdftotext lines like "Manufacturing Operator Error Internal Associate..."
	if data.RootCause == "" {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, "Operator Error") || strings.Contains(trimmed, "Failure Mode") {
				// Extract the part after known labels
				for _, marker := range []string{"Operator Error ", "Failure Mode "} {
					idx := strings.LastIndex(trimmed, marker)
					if idx >= 0 {
						remainder := strings.TrimSpace(trimmed[idx+len(marker):])
						if len(remainder) > 15 && !strings.HasPrefix(remainder, "Description") {
							data.RootCause = remainder
							break
						}
					}
				}
				if data.RootCause != "" {
					break
				}
			}
		}
	}

	// Fallback: use Root Cause Description (generic)
	if data.RootCause == "" {
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "Root Cause Description" {
				for j := i + 1; j < min(i+5, len(lines)); j++ {
					rc := strings.TrimSpace(lines[j])
					if isRootCauseText(rc) {
						data.RootCause = rc
						break
					}
				}
				if data.RootCause == "" {
					for j := i - 1; j >= max(0, i-5); j-- {
						rc := strings.TrimSpace(lines[j])
						if isRootCauseText(rc) {
							data.RootCause = rc
							break
						}
					}
				}
				break
			}
		}
	}

	// Save the full root cause for the reference tab
	data.FullRootCause = data.RootCause

	// Truncate for the card (root cause cell is ~2-3 lines at 11pt)
	if len(data.RootCause) > 300 {
		data.RootCause = data.RootCause[:297] + "..."
	}

	// === Parse Actions ===
	data.Actions, data.VOEActions = parseActions(lines)
	data.HasActions = len(data.Actions) > 0

	return data, nil
}

// parseCAPADetailFromLayout parses action items from the pdftotext -layout output.
// This is more reliable for the tabular action sections where columns are preserved.
func parseCAPADetailFromLayout(layoutText string, data *CAPAData) {
	lines := strings.Split(layoutText, "\n")
	var actions []CAPAAction
	var voeActions []CAPAAction

	// Pattern for action lines in layout mode:
	// "Corrective     Action Plan     <title>     <date>"
	// "Effectiveness Review Plan      <title>"
	actionLinePattern := regexp.MustCompile(`^\s*(Corrective|Preventive|Effectiveness Review Plan)\s+Action Plan\s+(.+?)(?:\s{2,}(\d{2}-[A-Z][a-z]{2}-\d{4}))?\s*$`)
	effLinePattern := regexp.MustCompile(`^\s*(Effectiveness Review Plan)\s+(.+?)(?:\s{2,}(\d{2}-[A-Z][a-z]{2}-\d{4}))?\s*$`)

	// Also match lines where the action type appears at the start but title is on continuation lines
	actionStartPattern := regexp.MustCompile(`^\s*(Corrective|Preventive)\s+Action Plan\s*$`)

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Try full action line with title and date
		if m := actionLinePattern.FindStringSubmatch(line); len(m) >= 3 {
			action := CAPAAction{
				ActionType: m[1],
				Title:      strings.TrimSpace(m[2]),
			}
			if len(m) >= 4 && m[3] != "" {
				if d, err := parseDateString(m[3]); err == nil {
					action.DueDate = d
				}
			}
			// Look for title continuation on next lines.
			// In layout mode, continuation lines are heavily indented and may share
			// a line with column labels like "Action Type". Extract just the title
			// portion by looking at the column position of the original title.
			titleCol := strings.Index(line, action.Title)
			for j := i + 1; j < min(i+4, len(lines)); j++ {
				nextRaw := lines[j]
				next := strings.TrimSpace(nextRaw)
				if next == "" || strings.HasPrefix(next, "Item No.") ||
					strings.HasPrefix(next, "Action Type") && titleCol < 0 {
					break
				}
				// If we know the title column, extract text from that position
				if titleCol >= 0 && len(nextRaw) > titleCol {
					portion := strings.TrimSpace(nextRaw[titleCol:])
					if portion != "" && !strings.HasPrefix(portion, "Action Type") &&
						!strings.HasPrefix(portion, "Item No.") {
						action.Title += " " + portion
						continue
					}
				}
				// Fallback: if the line is purely continuation text (heavily indented)
				if len(nextRaw) > 30 && nextRaw[0] == ' ' && next != "" &&
					!strings.HasPrefix(next, "Action Type") &&
					!strings.HasPrefix(next, "Item No.") {
					action.Title += " " + next
					continue
				}
				break
			}
			// Find owner from "Assigned to User" in following lines
			action.Owner = findOwnerInLayout(lines, i, i+80)

			action.Title = cleanActionTitle(action.Title)
			if isEffectivenessAction(action.ActionType) {
				voeActions = append(voeActions, action)
			} else {
				actions = append(actions, action)
			}
			continue
		}

		// Try effectiveness line without "Action Plan" prefix
		if m := effLinePattern.FindStringSubmatch(line); len(m) >= 3 {
			// Only match if this is in the action plan section (not header)
			if strings.Contains(line, "Action Plan") || i > 100 {
				action := CAPAAction{
					ActionType: m[1],
					Title:      strings.TrimSpace(m[2]),
				}
				if len(m) >= 4 && m[3] != "" {
					if d, err := parseDateString(m[3]); err == nil {
						action.DueDate = d
					}
				}
				action.Owner = findOwnerInLayout(lines, i, i+80)
				voeActions = append(voeActions, action)
				continue
			}
		}

		// Handle action type on its own line, title on next lines
		if m := actionStartPattern.FindStringSubmatch(line); len(m) >= 2 {
			var titleParts []string
			var dueDate time.Time
			foundDate := false
			// Find the title column position from the next line that has title text
			titleCol := -1
			for j := i + 1; j < min(i+10, len(lines)); j++ {
				next := strings.TrimSpace(lines[j])
				if next == "" || strings.HasPrefix(next, "Action Type") && titleCol < 0 ||
					strings.HasPrefix(next, "Item No.") {
					if foundDate {
						break
					}
					continue
				}
				if dm := datePattern.FindString(next); dm != "" && !foundDate {
					if d, err := parseDateString(dm); err == nil {
						dueDate = d
						foundDate = true
						beforeDate := strings.TrimSpace(strings.Split(lines[j], dm)[0])
						if beforeDate != "" {
							titleParts = append(titleParts, strings.TrimSpace(beforeDate))
							if titleCol < 0 {
								titleCol = strings.Index(lines[j], strings.TrimSpace(beforeDate))
							}
						}
					}
					continue
				}
				// Continuation line: extract text at titleCol position if known
				raw := lines[j]
				if titleCol >= 0 && len(raw) > titleCol {
					portion := strings.TrimSpace(raw[titleCol:])
					if portion != "" && !strings.HasPrefix(portion, "Action Type") &&
						!strings.HasPrefix(portion, "Item No.") &&
						!strings.HasPrefix(portion, "Action Plan") {
						titleParts = append(titleParts, portion)
						continue
					}
				}
				// Otherwise, use trimmed line
				if !strings.HasPrefix(next, "Action Type") && !strings.HasPrefix(next, "Item No.") &&
					!strings.HasPrefix(next, "Action Plan") {
					titleParts = append(titleParts, next)
					if titleCol < 0 {
						titleCol = strings.Index(raw, next)
					}
				} else if foundDate {
					break
				}
			}
			if len(titleParts) > 0 {
				action := CAPAAction{
					ActionType: m[1],
					Title:      cleanActionTitle(strings.Join(titleParts, " ")),
					DueDate:    dueDate,
				}
				action.Owner = findOwnerInLayout(lines, i, i+80)
				actions = append(actions, action)
			}
		}
	}

	if len(actions) > 0 || len(voeActions) > 0 {
		data.Actions = actions
		data.VOEActions = voeActions
		data.HasActions = len(actions) > 0
	}
}

// findOwnerInLayout searches for owner name near the "Assigned to User" line in layout text.
func findOwnerInLayout(lines []string, startLine, endLine int) string {
	endLine = min(endLine, len(lines))
	for j := startLine; j < endLine; j++ {
		line := lines[j]
		if strings.Contains(line, "Assigned to User") {
			// In layout mode, the line often contains: "... Assigned to User    USERNAME    Full Name"
			if m := usernameFullNamePattern.FindStringSubmatch(line); len(m) >= 3 {
				return m[2]
			}

			// USERNAME might be on same line without full name after it.
			// Look for USERNAME pattern, then search nearby lines for the full name.
			usernameOnlyPattern := regexp.MustCompile(`\b([A-Z][A-Z0-9]+\.[A-Z][A-Z0-9]+)\b`)
			if um := usernameOnlyPattern.FindString(line); um != "" {
				// Search lines before and after for a person name
				for k := j - 3; k <= min(j+5, endLine-1); k++ {
					if k < 0 || k == j {
						continue
					}
					candidate := strings.TrimSpace(lines[k])
					// Check for "USERNAME Full Name" pattern
					if mp := usernameFullNamePattern.FindStringSubmatch(lines[k]); len(mp) >= 3 {
						return mp[2]
					}
					// Check for standalone person name
					if looksLikePersonName(candidate) {
						return candidate
					}
					// Check for person name embedded in a longer line
					// e.g., "... Robert Cox"
					fields := strings.Fields(candidate)
					for fi := 0; fi < len(fields)-1; fi++ {
						if len(fields[fi]) > 1 && fields[fi][0] >= 'A' && fields[fi][0] <= 'Z' &&
							len(fields[fi+1]) > 1 && fields[fi+1][0] >= 'A' && fields[fi+1][0] <= 'Z' {
							name := fields[fi] + " " + fields[fi+1]
							if looksLikePersonName(name) {
								return name
							}
						}
					}
				}
			}

			// Check next few lines for USERNAME Full Name pattern
			for k := j + 1; k < min(j+5, endLine); k++ {
				if m := usernameFullNamePattern.FindStringSubmatch(lines[k]); len(m) >= 3 {
					return m[2]
				}
			}
			break
		}
	}
	return ""
}

func isRootCauseText(s string) bool {
	if s == "" {
		return false
	}
	skipLabels := []string{
		"Process", "Root Cause", "Product / Process", "Manufacturing",
		"Operator Error", "Product", "Root Cause Against",
	}
	for _, label := range skipLabels {
		if s == label {
			return false
		}
	}
	if strings.HasPrefix(s, "Root Cause") {
		return false
	}
	return len(s) > 20
}

// parseActions parses actions from non-layout pdftotext output.
// This works for both the spec format and basic pdftotext output.
// parseActions parses actions from PyMuPDF text output.
// PyMuPDF produces consistent line-by-line output where each action block follows:
//
//	Action Item Title          <- label
//	Action Type                <- label
//	<type>                     <- "Corrective"/"Preventive"/"Effectiveness..." (may have page noise before)
//	<date>                     <- DD-Mon-YYYY (due date)
//	<USERNAME>                 <- optional (completed by, for closed actions)
//	<completed date>           <- optional
//	<completed phase>          <- optional ("Implementation", "Planning")
//	<Full Name>                <- optional (completed by full name)
//	<title lines...>           <- action title (one or more lines)
//	Action Plan                <- end of title section (or "Effectiveness Review Plan" for VOE)
//	...
//	<description lines>        <- between "Item No." and "Action Plan Description"
//	Action Plan Description    <- end of description
//	...
//	<USERNAME>                 <- owner username
//	<Full Name>                <- owner full name
//	Assigned to User           <- label
func parseActions(lines []string) (actions []CAPAAction, voeActions []CAPAAction) {
	// Noise lines to skip (page breaks, headers, footers)
	isNoise := func(s string) bool {
		return s == "" ||
			strings.HasPrefix(s, "Page ") ||
			strings.HasPrefix(s, "Data is effective") ||
			strings.HasPrefix(s, "Report is Generated") ||
			strings.HasPrefix(s, "All Signature") ||
			strings.HasPrefix(s, "All blank fields")
	}

	// Check if a line looks like a USERNAME (ALL.CAPS.WITH.DOT)
	isUsername := func(s string) bool {
		return len(s) > 3 && strings.Contains(s, ".") && s == strings.ToUpper(s)
	}

	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "Action Item Title" {
			continue
		}

		action := CAPAAction{}

		// Find boundary: next "Action Item Title" or +100 lines
		searchEnd := min(i+100, len(lines))
		for j := i + 1; j < searchEnd; j++ {
			if strings.TrimSpace(lines[j]) == "Action Item Title" {
				searchEnd = j
				break
			}
		}

		// === Scan forward from "Action Item Title", skipping noise ===
		// Phase 1: Find action type and due date
		// Phase 2: Skip completed-by info (USERNAME, date, phase, full name)
		// Phase 3: Collect title lines until "Action Plan" or "Effectiveness Review Plan"

		phase := 1 // 1=looking for type/date, 2=skipping completed-by, 3=collecting title
		var titleParts []string
		foundType := false
		foundDate := false
		titleDone := false

		for j := i + 1; j < min(i+30, searchEnd); j++ {
			if titleDone {
				break
			}

			val := strings.TrimSpace(lines[j])

			if isNoise(val) {
				continue
			}

			if val == "Action Type" && !foundType {
				continue // skip the label
			}

			if phase == 1 { // Looking for type and date
				if !foundType {
					if val == "Corrective" || val == "Preventive" || strings.Contains(val, "Effectiveness") {
						action.ActionType = val
						foundType = true
						continue
					}
				}
				if foundType && !foundDate {
					if d, err := parseDateString(val); err == nil {
						action.DueDate = d
						foundDate = true
						phase = 2
						continue
					}
					// For effectiveness actions, there may be no date before the title
					if strings.Contains(action.ActionType, "Effectiveness") {
						phase = 3
						titleParts = append(titleParts, val)
						continue
					}
				}
				if !foundType {
					if d, err := parseDateString(val); err == nil {
						action.DueDate = d
						foundDate = true
						continue
					}
				}
			} else if phase == 2 { // Skipping optional completed-by info
				if isUsername(val) {
					continue
				}
				if _, err := parseDateString(val); err == nil {
					continue
				}
				if val == "Implementation" || val == "Planning" || val == "Investigation" ||
					val == "Verification" {
					continue
				}
				if looksLikePersonName(val) {
					// Check if next non-noise line starts the title section
					nextVal := ""
					for k := j + 1; k < min(j+5, searchEnd); k++ {
						nv := strings.TrimSpace(lines[k])
						if !isNoise(nv) {
							nextVal = nv
							break
						}
					}
					if nextVal == "Action Plan" || strings.Contains(nextVal, "Effectiveness") {
						titleParts = append(titleParts, val)
						phase = 3
					}
					// Otherwise it's the completed-by name, skip it
					continue
				}
				phase = 3
				titleParts = append(titleParts, val)
			} else if phase == 3 { // Collecting title lines
				if val == "Action Plan" || val == "Effectiveness Review Plan" {
					titleDone = true
					continue
				}
				titleParts = append(titleParts, val)
			}
		}

		if len(titleParts) > 0 {
			action.Title = cleanActionTitle(strings.Join(titleParts, " "))
		}

		// === Description ===
		// Lines between "Item No." and "Action Plan Description"
		inDesc := false
		var descParts []string
		for j := i; j < searchEnd; j++ {
			val := strings.TrimSpace(lines[j])
			if val == "Item No." {
				inDesc = true
				continue
			}
			if inDesc {
				if val == "Action Plan Description" {
					break
				}
				if isNoise(val) || val == "" {
					continue
				}
				descParts = append(descParts, val)
			}
		}
		if len(descParts) > 0 {
			action.Description = strings.Join(descParts, " ")
		}

		// === Owner ===
		// Find USERNAME followed by Full Name near "Assigned to User" or "Assigned to Role"
		for j := i; j < searchEnd; j++ {
			val := strings.TrimSpace(lines[j])
			if val == "Assigned to Role" || val == "Assigned to User" {
				// Search backwards for USERNAME + Full Name pair
				for k := j - 1; k >= max(j-8, i); k-- {
					candidate := strings.TrimSpace(lines[k])
					if isNoise(candidate) {
						continue
					}
					if looksLikePersonName(candidate) {
						action.Owner = candidate
						break
					}
				}
				if action.Owner != "" {
					break
				}
				// Search forward as fallback
				for k := j + 1; k < min(j+10, searchEnd); k++ {
					candidate := strings.TrimSpace(lines[k])
					if isNoise(candidate) {
						continue
					}
					if isUsername(candidate) {
						// Next non-noise line should be the full name
						for m := k + 1; m < min(k+5, searchEnd); m++ {
							name := strings.TrimSpace(lines[m])
							if isNoise(name) {
								continue
							}
							if looksLikePersonName(name) {
								action.Owner = name
							}
							break
						}
						break
					}
				}
				break
			}
		}

		if isEffectivenessAction(action.ActionType) {
			voeActions = append(voeActions, action)
		} else {
			actions = append(actions, action)
		}
	}
	return
}

func isActionTitleCandidate(val string) bool {
	if val == "" || len(val) < 5 {
		return false
	}
	skipValues := []string{
		"Corrective", "Preventive", "Action Type", "Due Date",
		"Completed By", "Completed Date", "Completed In",
		"Action Item Title", "All Signature", "All blank",
	}
	for _, s := range skipValues {
		if val == s || strings.HasPrefix(val, s) {
			return false
		}
	}
	if strings.Contains(val, "Effectiveness") {
		return false
	}
	if strings.Contains(val, ".") && val == strings.ToUpper(val) {
		return false
	}
	if datePattern.MatchString(val) {
		return false
	}
	return true
}

func looksLikePersonName(s string) bool {
	if s == "" {
		return false
	}
	words := strings.Fields(s)
	if len(words) < 2 || len(words) > 4 {
		return false
	}
	for _, w := range words {
		if len(w) == 0 || w[0] < 'A' || w[0] > 'Z' {
			return false
		}
	}
	lower := strings.ToLower(s)
	rejectPhrases := []string{
		"action plan", "assigned to", "richmond", "rch ", "all ",
		"data ", "page ", "report ", "due date", "item no",
		"root cause", "verification", "checklist", "completed",
	}
	for _, r := range rejectPhrases {
		if strings.Contains(lower, r) {
			return false
		}
	}
	return true
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

// cleanActionTitle fixes common pdftotext artifacts in action titles:
// - Joins broken words ("Devic e" -> "Device", "Overlay S" -> keeps as-is since we can't know)
// - Removes trailing prepositions that indicate truncation
func cleanActionTitle(title string) string {
	// Fix common mid-word breaks from pdftotext column width
	replacements := map[string]string{
		"Devic e ":      "Device ",
		"Devic e\n":     "Device\n",
		"Overlay S":     "Overlay System",
		"Containme nt":  "Containment",
		"Dist ribution": "Distribution",
		"Pla nt":        "Plant",
		"ribution Center)": "ribution Center)",
	}
	for old, new := range replacements {
		title = strings.ReplaceAll(title, old, new)
	}

	// Remove trailing " to" or " to " that indicates truncation
	title = strings.TrimRight(title, " ")
	if strings.HasSuffix(title, " to") {
		title = strings.TrimSuffix(title, " to")
	}

	return strings.TrimSpace(title)
}

