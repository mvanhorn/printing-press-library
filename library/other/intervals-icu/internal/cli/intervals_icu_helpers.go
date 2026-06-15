// Copyright 2026 Milos Mladenovic and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the hand-authored intervals.icu transcendence commands
// (form, curve compare, wellness trends, since, gear status). These commands
// read live API data scoped to the configured athlete; they do not reimplement
// any endpoint locally.

package cli

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/intervals-icu/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/intervals-icu/internal/config"
)

// jsonStr extracts a string field from a decoded JSON object, tolerating
// missing keys and non-string values (returns "").
func jsonStr(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return strings.Trim(string(raw), `"`)
	}
	return s
}

// round1 rounds to one decimal place for display stability.
func round1(f float64) float64 { return math.Round(f*10) / 10 }

// athleteID loads config and returns the athlete id to target (default "0",
// which intervals.icu resolves to the key owner).
func athleteID(flags *rootFlags) string {
	cfg, err := config.Load(flags.configPath)
	if err != nil || cfg == nil {
		return "0"
	}
	return cfg.AthleteID()
}

// parseWindowDays converts a duration-ish flag ("90d", "6w", "1440h", or a bare
// integer treated as days) into a whole number of days, clamped to >= 1.
func parseWindowDays(s string, def int) int {
	if s == "" {
		return def
	}
	// Bare integer → days.
	if d, err := cliutil.ParseDurationLoose(s + "d"); err == nil && onlyDigits(s) {
		days := int(d.Hours() / 24)
		if days < 1 {
			days = 1
		}
		return days
	}
	if d, err := cliutil.ParseDurationLoose(s); err == nil {
		days := int(d.Hours() / 24)
		if days < 1 {
			days = 1
		}
		return days
	}
	return def
}

// validWindow reports whether s is a usable window: a bare integer (days) or a
// duration cliutil.ParseDurationLoose accepts (7d, 2w, 48h, ...).
func validWindow(s string) bool {
	if onlyDigits(s) {
		return true
	}
	if _, err := cliutil.ParseDurationLoose(s); err == nil {
		return true
	}
	return false
}

func onlyDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// localDate returns a YYYY-MM-DD string for an offset (negative = past) of days
// from now, in the machine's local time zone. intervals.icu's oldest/newest
// params are athlete-local ISO days, so anchoring on local (not UTC) avoids an
// off-by-one near midnight for users west of UTC.
func localDate(daysFromNow int) string {
	return time.Now().AddDate(0, 0, daysFromNow).Format("2006-01-02")
}
