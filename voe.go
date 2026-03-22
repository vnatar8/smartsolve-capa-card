package main

import "time"

func addBusinessDays(start time.Time, businessDays int) time.Time {
	current := start
	added := 0
	for added < businessDays {
		current = current.AddDate(0, 0, 1)
		if current.Weekday() != time.Saturday && current.Weekday() != time.Sunday {
			added++
		}
	}
	return current
}

func calculateVOEDates(actions []CAPAAction, interval1 int, interval2 int) (time.Time, time.Time) {
	if len(actions) == 0 || interval1 == 0 {
		return time.Time{}, time.Time{}
	}

	var latest time.Time
	for _, a := range actions {
		if a.DueDate.After(latest) {
			latest = a.DueDate
		}
	}

	if latest.IsZero() {
		return time.Time{}, time.Time{}
	}

	voe1 := addBusinessDays(latest, interval1)
	var voe2 time.Time
	if interval2 > 0 {
		voe2 = addBusinessDays(latest, interval2)
	}

	return voe1, voe2
}
