# SmartSolve CAPA Card Generator

A CLI tool that downloads a CAPA Detail PDF from [IQVIA SmartSolve](https://www.iqvia.com/solutions/compliance/smartsolve) eQMS, extracts key fields, and generates a formatted Excel CAPA card for your CAPA DM (Daily Management) board.

The card is a two-sided Excel file (Front and Back) designed to be printed, folded, and placed in a transparent card holder for visual management.

## Features

- Downloads CAPA Detail PDF automatically from SmartSolve
- Extracts all key fields: CAPA number, title, owner, initiation date, root cause, failure mode, actions, VOE dates
- Generates a formatted Excel card from an embedded template (no external template file needed)
- Calculates VOE (Verification of Effectiveness) due dates using business day arithmetic
- Creates a Reference tab with complete CAPA details (full root cause, action descriptions, failure modes)
- Reads JWT auth token from Chrome automatically (no manual copy-paste in most cases)
- Single binary with the Excel template embedded inside

## Prerequisites

- **Windows** (uses Windows APIs for reading Chrome's locked database files)
- **Google Chrome** with an active SmartSolve session
- **pdftotext** from [Poppler](https://poppler.freedesktop.org/) (for PDF text extraction)
- **Go 1.21+** (only needed if building from source)

### Installing pdftotext

pdftotext is part of Poppler utilities. On Windows:

1. Download Poppler for Windows from [this release page](https://github.com/osudrl/poppler-windows/releases)
2. Extract and add the `bin` folder to your PATH
3. Verify: `pdftotext -v` should print the version

## Setup

### 1. Clone and configure

```
git clone https://github.com/<your-username>/smartsolve-capa-card.git
cd smartsolve-capa-card
```

Edit `config.go` with your SmartSolve details:

```go
var config = struct {
    APIURL    string
    SiteCodes []string
}{
    APIURL:    "https://yourcompany.wopi.pilgrimasp.com/prod/smartsolve/",
    SiteCodes: []string{"NYC ", "LON ", "MEL "},
}
```

**How to find your APIURL:** Log into SmartSolve, open DevTools (F12), go to the Network tab, and look at API request URLs when navigating to a CAPA list.

**SiteCodes** are used for extracting the "Caused By" organizational unit from the PDF. Use the 3-letter site codes from your SmartSolve instance followed by a space.

### 2. Customize the template

The file `template.xlsx` is the Excel card template that gets embedded into the binary. Customize it to match your card holder dimensions, fonts, and layout before building. The tool populates specific cells; see the "Card Layout" section below.

### 3. Build

```
go build -o capa-card.exe .
```

## Usage

```
capa-card.exe --capa <CAPA-number> [--output <directory>] [--token <jwt>]
```

### Required

- `--capa <number>` : CAPA number (e.g., `CAPA-2025-000054`)

### Optional

- `--output <path>` : Directory to save the Excel card (default: current directory)
- `--token <jwt>` : JWT token (bypasses Chrome auto-detection)

### Examples

```
capa-card.exe --capa CAPA-2025-000054
capa-card.exe --capa CAPA-2025-000054 --output "Q:\QA\CAPAs\CAPA DM"
```

### Output

The tool saves: `CAPA Card_YYYY-NNNNNN.xlsx` (e.g., `CAPA Card_2025-000054.xlsx`)

If a card with the same name already exists, the old one is moved to an `Archived Cards` subfolder.

## How It Works

1. **Reads JWT token** from Chrome's Local Storage (same as [smartsolve-capa-downloader](https://github.com/vnatar8/smartsolve-capa-downloader))
2. **Downloads the CAPA Detail PDF** from SmartSolve using the JWT token
3. **Extracts text** from the PDF using `pdftotext` (with and without layout mode for best results)
4. **Parses fields** from the extracted text: header info, root cause analysis, failure modes, action items with titles/descriptions/owners/dates, effectiveness review plans
5. **Populates the Excel template** with extracted data across three sheets
6. **Saves** the card to the output directory

## Card Layout

### Sheet (Front)
The front side shows CAPA summary info:
- CAPA number, initiation date, owner
- CAPA title (in the problem statement area)
- Due dates: Investigation, Planning, VOE 1, VOE 2

### Sheet (Back)
The back side shows root cause and actions:
- Root cause summary (truncated to fit)
- Action items sorted by due date: number, title, due date, owner

### Reference Tab
Contains all CAPA information verbatim (no truncation):
- Full header info (number, title, date, owner, complete description)
- Full root cause paragraph, failure mode, failure mode description, caused by, effectiveness interval
- All corrective/preventive actions with full descriptions
- All effectiveness (VOE) actions with full descriptions

## VOE Date Calculation

VOE due dates are calculated automatically:
1. Find the latest due date among all corrective/preventive actions
2. Add the effectiveness review interval (in business days, excluding weekends)
3. Business days = weekdays only (Monday through Friday)

## If Auto-Detection Fails

If the tool can't find your SmartSolve token in Chrome automatically:

```
To get your token, open SmartSolve in Chrome, press F12, go to Console, and run:

  copy(localStorage.getItem("token"))

Then run this tool with --token <paste>
```

The token is valid for 24 hours.

## Project Structure

```
├── config.go              # SmartSolve instance configuration
├── main.go                # CLI entry point, orchestration
├── auth.go                # JWT token reading from Chrome's Local Storage
├── download.go            # CAPA Detail PDF download from SmartSolve
├── pdfparse.go            # PDF text extraction and field parsing
├── types.go               # Data structures (CAPAData, CAPAAction)
├── excel.go               # Excel template population (Front, Back, Reference)
├── voe.go                 # VOE due date calculation (business days)
├── template.xlsx          # Embedded Excel card template
├── *_test.go              # Tests
├── go.mod / go.sum        # Go module dependencies
└── README.md
```

## Limitations

- **Windows only.** Uses Windows-specific APIs for reading Chrome's locked Local Storage files.
- **Chrome only.** Other browsers store localStorage differently.
- **Requires pdftotext.** The PDF text extraction depends on Poppler's pdftotext utility.
- **PDF layout sensitivity.** SmartSolve PDFs have a specific layout. If your SmartSolve instance produces differently formatted PDFs, the parser may need adjustment.

## License

MIT
