// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/parks"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/store"
)

// collectCacheReport summarizes the local snapshot history, which is the only
// local data that 'typical' and 'waits' read. It opens the database read-only,
// never creates it and never fetches. staleAfter is --max-age; zero turns the
// fresh/stale verdict off. Snapshots are recorded only when the user runs
// 'snapshot' (from their own scheduler), so a stale verdict is a hint about
// that schedule, not something this CLI repairs by itself.
func collectCacheReport(ctx context.Context, dbPath string, staleAfter time.Duration) map[string]any {
	report := map[string]any{"db_path": dbPath, "store_schema_version": store.StoreSchemaVersion}
	if staleAfter > 0 {
		report["stale_after"] = staleAfter.String()
	} else {
		report["stale_after"] = "disabled (--max-age 0)"
	}

	fi, err := os.Stat(dbPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			report["status"] = "unknown"
			report["hint"] = "No snapshot history yet; run 'japan-theme-parks-pp-cli snapshot' to record waits that 'typical' and 'waits' can use."
			return report
		}
		report["status"] = "error"
		report["error"] = err.Error()
		return report
	}
	report["db_bytes"] = fi.Size()

	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		report["status"] = "error"
		report["error"] = err.Error()
		return report
	}
	defer db.Close()

	if v, verr := db.SchemaVersion(); verr == nil {
		report["schema_version"] = v
		report["migration_pending"] = v < store.StoreSchemaVersion
	}
	if err := db.RejectNewerSchema(); err != nil {
		report["status"] = "error"
		report["error"] = err.Error()
		return report
	}

	byPark, err := db.WaitHistoryByPark(ctx)
	if err != nil {
		report["status"] = "error"
		report["error"] = err.Error()
		return report
	}
	now := time.Now()
	total := 0
	var first, last time.Time
	parkRows := make([]map[string]any, 0, len(byPark))
	for _, ph := range byPark {
		total += ph.Rows
		if !ph.First.IsZero() && (first.IsZero() || ph.First.Before(first)) {
			first = ph.First
		}
		if ph.Last.After(last) {
			last = ph.Last
		}
		row := map[string]any{"queue_times_id": ph.ParkID, "rows": ph.Rows,
			"first_snapshot": jst(ph.First), "last_snapshot": jst(ph.Last)}
		if p, ok := parks.Lookup(strconv.Itoa(ph.ParkID)); ok {
			row["park"] = p.Key
		}
		if !ph.Last.IsZero() {
			row["staleness"] = now.Sub(ph.Last).Round(time.Minute).String()
		}
		parkRows = append(parkRows, row)
	}
	report["rows"] = total
	report["parks"] = parkRows
	if total == 0 {
		report["status"] = "empty"
		report["hint"] = "The history database has no snapshots yet; run 'japan-theme-parks-pp-cli snapshot'."
		return report
	}
	report["first_snapshot"] = jst(first)
	report["last_snapshot"] = jst(last)
	if last.IsZero() {
		report["status"] = "unknown"
		report["hint"] = "Stored snapshot times could not be read."
		return report
	}
	age := now.Sub(last)
	report["staleness"] = age.Round(time.Minute).String()
	switch {
	case staleAfter <= 0:
		report["status"] = "ok"
	case age <= staleAfter:
		report["status"] = "fresh"
	default:
		report["status"] = "stale"
		report["hint"] = "The newest snapshot is older than --max-age. This CLI does not schedule snapshots; run 'japan-theme-parks-pp-cli snapshot' or check your own scheduler."
	}
	return report
}

// renderCacheReport prints the history section of the human doctor output.
func renderCacheReport(w io.Writer, rep map[string]any) {
	status, _ := rep["status"].(string)
	indicator := green("OK")
	switch status {
	case "stale":
		indicator = yellow("WARN")
	case "unknown", "empty":
		indicator = yellow("INFO")
	case "error":
		indicator = red("FAIL")
	}
	fmt.Fprintf(w, "  %s Snapshot History: %s\n", indicator, status)
	for _, key := range []string{"error", "db_path", "schema_version", "store_schema_version", "migration_pending", "db_bytes", "rows", "first_snapshot", "last_snapshot", "staleness", "stale_after"} {
		if v, ok := rep[key]; ok {
			fmt.Fprintf(w, "    %s: %v\n", key, v)
		}
	}
	if rows, ok := rep["parks"].([]map[string]any); ok && len(rows) > 0 {
		fmt.Fprintln(w, "    parks:")
		for _, r := range rows {
			name, _ := r["park"].(string)
			if name == "" {
				name = fmt.Sprintf("queue-times %v", r["queue_times_id"])
			}
			fmt.Fprintf(w, "      - %s: %v rows, %v to %v, newest %v old\n", name, r["rows"], r["first_snapshot"], r["last_snapshot"], r["staleness"])
		}
	}
	if hint, ok := rep["hint"]; ok {
		fmt.Fprintf(w, "    hint: %v\n", hint)
	}
}
