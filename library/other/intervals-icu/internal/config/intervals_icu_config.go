// Copyright 2026 Milos Mladenovic and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import "os"

// AthleteID resolves the intervals.icu athlete to target for athlete-scoped
// endpoints (/api/v1/athlete/{id}/...). intervals.icu accepts "0" as an alias
// for the athlete that owns the API key, so commands work without the user
// looking up their numeric/"i"-prefixed id first. INTERVALS_ICU_ATHLETE_ID
// overrides it (needed by coaches acting on another athlete).
func (c *Config) AthleteID() string {
	if v := os.Getenv("INTERVALS_ICU_ATHLETE_ID"); v != "" {
		return v
	}
	return "0"
}
