package main

import "time"

type CAPAAction struct {
	Title       string
	Description string // full action plan description (verbatim from PDF)
	DueDate     time.Time
	Owner       string
	ActionType  string
}

type CAPAData struct {
	Number               string
	Title                string
	Description          string // full problem statement from PDF
	InitiationDate       time.Time
	Owner                string
	RootCause            string // truncated for the card
	FullRootCause        string // complete root cause paragraph (verbatim)
	FailureMode          string // e.g., "Operator Error"
	FailureModeDesc      string // e.g., "Internal Associate did not follow defined process"
	CausedBy             string // organizational unit that caused the issue
	Actions              []CAPAAction
	VOEActions           []CAPAAction
	EffInterval          int
	EffUnit              string
	HasActions           bool
}
