// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"fmt"
	"time"
)

// solaredge_call_log backs the budget tracker. Its CREATE TABLE IF NOT
// EXISTS lives in extras.go's migrateExtras, which runs inside the same
// locked, version-stamped migration transaction every other table in the
// store goes through — so by the time any function below runs, Open /
// OpenWithContext has already guaranteed the table exists.
//
// The SolarEdge Monitoring API does not expose the remaining daily quota
// (the 300-requests-per-account-token-per-site limit documented by the
// vendor has no corresponding response header), so this table is the only
// source of truth this CLI can offer for "how much of today's budget have
// I used" — and it only sees calls routed through the commands that call
// RecordSolarEdgeAPICalls below.

// RecordSolarEdgeAPICalls increments today's tracked call count for siteID
// by n. Call this once per command invocation, after the live calls for
// that invocation have completed (success or failure both count against
// the vendor's quota, so record regardless of the result).
func RecordSolarEdgeAPICalls(db *sql.DB, siteID string, n int) error {
	if n <= 0 {
		return nil
	}
	day := time.Now().UTC().Format("2006-01-02")
	_, err := db.Exec(`INSERT INTO solaredge_call_log (day, site_id, calls) VALUES (?, ?, ?)
		ON CONFLICT(day, site_id) DO UPDATE SET calls = calls + excluded.calls`, day, siteID, n)
	if err != nil {
		return fmt.Errorf("recording solaredge api calls: %w", err)
	}
	return nil
}

// SolarEdgeCallsToday returns today's tracked call count for siteID.
func SolarEdgeCallsToday(db *sql.DB, siteID string) (int, error) {
	day := time.Now().UTC().Format("2006-01-02")
	var n int
	err := db.QueryRow(`SELECT calls FROM solaredge_call_log WHERE day = ? AND site_id = ?`, day, siteID).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading solaredge call count: %w", err)
	}
	return n, nil
}

// SolarEdgeCallsTodayAllSites returns today's tracked call counts for every
// site that has at least one recorded call, keyed by site ID.
func SolarEdgeCallsTodayAllSites(db *sql.DB) (map[string]int, error) {
	day := time.Now().UTC().Format("2006-01-02")
	rows, err := db.Query(`SELECT site_id, calls FROM solaredge_call_log WHERE day = ?`, day)
	if err != nil {
		return nil, fmt.Errorf("reading solaredge call counts: %w", err)
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var siteID string
		var n int
		if err := rows.Scan(&siteID, &n); err != nil {
			return nil, fmt.Errorf("scanning solaredge call count: %w", err)
		}
		result[siteID] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating solaredge call counts: %w", err)
	}
	return result, nil
}
