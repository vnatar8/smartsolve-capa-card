package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed template.xlsx
var templateFS embed.FS

func main() {
	capaNum := flag.String("capa", "", "CAPA number (e.g., CAPA-2025-000054) (required)")
	outputDir := flag.String("output", ".", "Directory to save the CAPA card Excel file")
	token := flag.String("token", "", "JWT token (optional; bypasses reading from Chrome)")

	flag.Parse()

	if *capaNum == "" {
		fmt.Fprintln(os.Stderr, "Error: --capa is required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, `  capa-card --capa CAPA-2025-000054`)
		fmt.Fprintln(os.Stderr, `  capa-card --capa CAPA-2025-000054 --output "Q:\QA\CAPAs\CAPA DM"`)
		os.Exit(1)
	}

	// Validate output directory
	absPath, _ := filepath.Abs(*outputDir)
	*outputDir = absPath
	if info, err := os.Stat(*outputDir); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: output directory does not exist: %s\n", *outputDir)
		os.Exit(1)
	}

	// Step 1: Get JWT token
	jwtToken := *token
	if jwtToken == "" {
		fmt.Println("Reading JWT token from Chrome session...")
		var err error
		jwtToken, err = readJWTFromLocalStorage()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not find SmartSolve token automatically.")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "To get your token, open SmartSolve in Chrome, press F12, go to Console, and run:")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, `  copy(localStorage.getItem("token"))`)
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Then run this tool with --token <paste>")
			os.Exit(1)
		}
		fmt.Printf("Found token (%d chars).\n", len(jwtToken))
	} else {
		fmt.Println("Using provided JWT token.")
	}

	// Step 2: Download CAPA Detail PDF
	fmt.Printf("Downloading CAPA Detail for %s...\n", *capaNum)
	pdfPath, err := downloadCAPADetail(jwtToken, *capaNum)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading CAPA: %v\n", err)
		os.Exit(1)
	}
	// Keep the PDF alongside the card for reference
	// It will be moved to the output directory after card generation

	// Step 3: Extract fields from PDF
	fmt.Println("Extracting CAPA data from PDF...")
	pdfText, err := extractTextFromPDF(pdfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading PDF: %v\n", err)
		os.Exit(1)
	}

	capaData, err := parseCAPADetail(pdfText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing CAPA data: %v\n", err)
		os.Exit(1)
	}

	// Validate that we extracted at least the CAPA number
	if capaData.Number == "" {
		fmt.Fprintln(os.Stderr, "Error: could not extract CAPA data from the PDF.")
		fmt.Fprintln(os.Stderr, "The PDF may be corrupted, or the CAPA number may be incorrect.")
		fmt.Fprintln(os.Stderr, "Verify the CAPA exists in SmartSolve and try again.")
		os.Exit(1)
	}

	// Step 4: Generate the card
	cardName := strings.Replace(*capaNum, "CAPA-", "CAPA Card_", 1) + ".xlsx"
	cardPath := filepath.Join(*outputDir, cardName)

	// Archive existing card if present
	if _, err := os.Stat(cardPath); err == nil {
		archiveDir := filepath.Join(*outputDir, "Archived Cards")
		os.MkdirAll(archiveDir, 0755)
		archivePath := filepath.Join(archiveDir, cardName)
		os.Rename(cardPath, archivePath)
		fmt.Printf("Archived existing card to: %s\n", archivePath)
	}

	fmt.Println("Generating CAPA card...")
	if err := generateCard(capaData, cardPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating card: %v\n", err)
		os.Exit(1)
	}

	// Step 5: Summary
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("CAPA Card Generated")
	fmt.Println("========================================")
	fmt.Printf("CAPA:    %s\n", capaData.Number)
	fmt.Printf("Title:   %s\n", capaData.Title)
	fmt.Printf("Owner:   %s\n", capaData.Owner)
	fmt.Printf("Date:    %s\n", capaData.InitiationDate.Format("02-Jan-2006"))

	if capaData.HasActions {
		fmt.Printf("Actions: %d corrective/preventive\n", len(capaData.Actions))
		for i, a := range capaData.Actions {
			fmt.Printf("  %d. %s (due: %s, owner: %s)\n", i+1, a.Title, a.DueDate.Format("02-Jan-2006"), a.Owner)
		}
		if len(capaData.VOEActions) > 0 {
			fmt.Printf("VOE:     %d effectiveness checks\n", len(capaData.VOEActions))
		}
		fmt.Printf("Root Cause: %s\n", capaData.RootCause)
	} else {
		fmt.Println("Status:  Investigation phase (front side only)")
	}

	// Move the PDF to the output directory for reference
	pdfName := *capaNum + ".pdf"
	pdfDest := filepath.Join(*outputDir, pdfName)
	os.Rename(pdfPath, pdfDest)

	fmt.Printf("\nSaved to: %s\n", cardPath)
	fmt.Printf("PDF kept at: %s\n", pdfDest)
}
