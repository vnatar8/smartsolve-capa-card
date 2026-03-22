package main

// SmartSolve instance configuration.
// Update these values to match your organization's SmartSolve deployment.
var config = struct {
	// APIURL is the SmartSolve WOPI API base URL.
	// Example: "https://mycompany.wopi.pilgrimasp.com/prod/smartsolve/"
	APIURL string

	// SiteCodes are your organization's site codes used in "Caused By" extraction.
	// These are matched against pdftotext output to find organizational unit info.
	// Example: []string{"NYC ", "LON ", "MEL "}
	SiteCodes []string
}{
	APIURL:    "https://CHANGEME.wopi.pilgrimasp.com/prod/smartsolve/",
	SiteCodes: []string{"CHANGEME "},
}
