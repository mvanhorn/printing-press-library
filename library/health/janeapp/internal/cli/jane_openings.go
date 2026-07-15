// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: availability helpers shared by next-opening and watch. Jane's
// /api/v2/openings caps num_days at 7 (422 otherwise), so scanning further out
// means stitching consecutive 7-day windows — the core of next-opening's value.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/janeapp/internal/client"
)

type janeOpening struct {
	StaffMemberID int    `json:"staff_member_id"`
	LocationID    int    `json:"location_id"`
	TreatmentID   int    `json:"treatment_id"`
	Duration      int    `json:"duration"`
	StartAt       string `json:"start_at"`
	EndAt         string `json:"end_at"`
	Status        string `json:"status"`
}

type janeStaffOpenings struct {
	ID        int           `json:"id"`
	FullName  string        `json:"full_name"`
	FirstDate string        `json:"first_date"`
	Openings  []janeOpening `json:"openings"`
}

const janeMaxWindowDays = 7

// fetchOpenings queries one availability window (<=7 days) and returns the flat
// list of openings across all returned staff blocks.
func fetchOpenings(ctx context.Context, c *client.Client, locationID, treatmentID, staffID int, start time.Time, numDays int) ([]janeOpening, error) {
	if numDays < 1 {
		numDays = 1
	}
	if numDays > janeMaxWindowDays {
		numDays = janeMaxWindowDays
	}
	params := map[string]string{
		"location_id":     strconv.Itoa(locationID),
		"treatment_id":    strconv.Itoa(treatmentID),
		"staff_member_id": strconv.Itoa(staffID),
		// Jane's openings endpoint keys the window on `date`, NOT `start_date`.
		// Passing start_date is silently ignored and the API returns only the
		// current near-term window — which is why future availability looked empty.
		"date":     start.Format("2006-01-02"),
		"num_days": strconv.Itoa(numDays),
	}
	data, err := c.Get(ctx, "/api/v2/openings", params)
	if err != nil {
		return nil, err
	}
	var blocks []janeStaffOpenings
	if err := json.Unmarshal(data, &blocks); err != nil {
		return nil, fmt.Errorf("parsing openings: %w", err)
	}
	var out []janeOpening
	for _, b := range blocks {
		out = append(out, b.Openings...)
	}
	return out, nil
}

// parseOpeningTime parses an opening's start_at (RFC3339 with offset).
func parseOpeningTime(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-07:00", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// findNextOpening scans consecutive 7-day windows from `from` up to horizonDays
// and returns the earliest opening at or after `from`. Returns (nil, nil) when
// none is found within the horizon.
func findNextOpening(ctx context.Context, c *client.Client, locationID, treatmentID, staffID int, from time.Time, horizonDays int) (*janeOpening, error) {
	if horizonDays < 1 {
		horizonDays = 1
	}
	scanned := 0
	windowStart := from
	for scanned < horizonDays {
		days := horizonDays - scanned
		if days > janeMaxWindowDays {
			days = janeMaxWindowDays
		}
		ops, err := fetchOpenings(ctx, c, locationID, treatmentID, staffID, windowStart, days)
		if err != nil {
			return nil, err
		}
		var best *janeOpening
		var bestT time.Time
		for i := range ops {
			t, ok := parseOpeningTime(ops[i].StartAt)
			if !ok || t.Before(from) {
				continue
			}
			if best == nil || t.Before(bestT) {
				o := ops[i]
				best = &o
				bestT = t
			}
		}
		if best != nil {
			return best, nil
		}
		scanned += days
		windowStart = windowStart.AddDate(0, 0, days)
	}
	return nil, nil
}
