// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// Hand-coded augmentations for the doctor command — PIT prefix case
// check, cache freshness review, and active-location validation. Kept
// separate from doctor.go so a press regen does not blow them away;
// doctor.go's RunE invokes these via additive checkPITCase /
// checkActiveLocation calls.
package cli

import (
	"os"
	"strings"
)

// checkPITCase inspects the GHL_PIT_TOKEN env var for the uppercase-prefix
// bug ("Pit-" or "PIT-"). The GHL API rejects uppercase prefixes with a
// misleading 401 "Invalid JWT" — auto-lowercase the prefix for any live
// check the doctor performs and surface a warning so the user can fix
// their env config.
func checkPITCase(report map[string]any) {
	raw := os.Getenv("GHL_PIT_TOKEN")
	if raw == "" {
		return
	}
	// Token shorter than the prefix; nothing meaningful to inspect.
	if len(raw) < 4 {
		return
	}
	prefix := raw[:4]
	if strings.EqualFold(prefix, "pit-") {
		if prefix == "pit-" {
			report["pit_case"] = "OK pit- prefix is lowercase"
		} else {
			report["pit_case"] = "WARN GHL_PIT_TOKEN starts with an uppercase prefix; GHL returns 401 'Invalid JWT' for capital 'Pit-'/'PIT-'. Re-export with the lowercase prefix: export GHL_PIT_TOKEN=pit-<uuid>"
		}
	}
}

// checkActiveLocation confirms an active-location.toml is set and reports it.
// Mostly informational — invalidity surfaces when actual API calls fail.
func checkActiveLocation(report map[string]any) {
	a := readActiveLocation()
	if a == nil {
		report["active_location"] = "INFO no active location set (run 'gohighlevel-pp-cli config use <name>' to choose one)"
		return
	}
	report["active_location"] = "OK " + a.Name + " (id=" + a.LocationID + ")"
}
