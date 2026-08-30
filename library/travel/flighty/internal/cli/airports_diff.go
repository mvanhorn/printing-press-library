// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// airports diff — what changed since the last sync: status transitions,
// added/cleared warnings, and cumulative-delay deltas. Flighty's site keeps
// no history (every page load is a live snapshot), so the delta only exists
// in the local SQLite snapshot history recorded at sync time.
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/flighty/internal/store"
)

// flightyRecordSnapshotFromStore copies the current catalog rows into the
// snapshot-history table. Called from the sync command after a successful
// sync. Best-effort: errors surface on stderr without failing the sync.
func flightyRecordSnapshotFromStore(ctx context.Context, db *store.Store) {
	if err := db.EnsureFlightySnapshotTable(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: snapshot history unavailable: %v\n", err)
		return
	}
	rows, err := db.DB().QueryContext(ctx, `
		SELECT data FROM resources WHERE resource_type = 'airports'`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not read catalog for snapshot: %v\n", err)
		return
	}
	type snapRow struct {
		IATA string
		Data json.RawMessage
	}
	// Drain first: no nested queries while rows is open.
	scanned := make([]snapRow, 0, 160)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var ap flightyCatalogAirport
		if json.Unmarshal([]byte(data), &ap) != nil || ap.IATA == "" {
			continue
		}
		scanned = append(scanned, snapRow{IATA: ap.IATA, Data: json.RawMessage(data)})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		fmt.Fprintf(os.Stderr, "warning: snapshot read failed: %v\n", err)
		return
	}
	_ = rows.Close()
	now := time.Now()
	for _, r := range scanned {
		if err := db.InsertFlightySnapshot(ctx, now, r.IATA, r.Data); err != nil {
			fmt.Fprintf(os.Stderr, "warning: snapshot insert failed for %s: %v\n", r.IATA, err)
			return
		}
	}
}

// flightySnapshotAirport is the subset of a catalog row diff cares about.
type flightySnapshotAirport struct {
	IATA            string   `json:"iata"`
	Name            string   `json:"name"`
	City            string   `json:"city"`
	Status          string   `json:"status,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	CumulativeDelay int      `json:"cumulativeDelay,omitempty"`
}

// flightyStatusChange is one airport's transition between snapshots.
type flightyStatusChange struct {
	IATA       string   `json:"iata"`
	Name       string   `json:"name,omitempty"`
	From       string   `json:"from,omitempty"`
	To         string   `json:"to,omitempty"`
	Added      []string `json:"addedWarnings,omitempty"`
	Cleared    []string `json:"clearedWarnings,omitempty"`
	DelayDelta *int     `json:"cumulativeDelayDelta,omitempty"`
}

func newNovelAirportsDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "What changed since the last sync: status transitions, new/cleared warnings, delay deltas.",
		Long:        "Use this command to see what changed since the last sync. Do NOT use it for current live status; use 'airports list' instead (sync first). Requires at least two synced snapshots.",
		Example:     "  flighty-pp-cli airports diff --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "airports diff")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("flighty-pp-cli")
			}
			db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			newest, previous, ok := db.LatestFlightySnapshotTimes(cmd.Context())
			if !ok {
				fmt.Fprintln(cmd.ErrOrStderr(), "not enough snapshot history: run 'flighty-pp-cli sync --resources airports --full' at least twice, then diff again")
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"changes": []flightyStatusChange{}, "note": "need two syncs to diff"}, flags)
				}
				return nil
			}
			newestRows, err := db.FlightySnapshotsAt(cmd.Context(), newest)
			if err != nil {
				return fmt.Errorf("reading newest snapshot: %w", err)
			}
			prevRows, err := db.FlightySnapshotsAt(cmd.Context(), previous)
			if err != nil {
				return fmt.Errorf("reading previous snapshot: %w", err)
			}

			changes := flightyDiffSnapshots(prevRows, newestRows)
			view := map[string]any{
				"compared": map[string]string{
					"previous": previous.UTC().Format(time.RFC3339),
					"current":  newest.UTC().Format(time.RFC3339),
				},
				"changes": changes,
			}
			if len(changes) == 0 {
				view["note"] = "no changes between the last two syncs"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(changes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No changes between the last two syncs.")
				return nil
			}
			for _, ch := range changes {
				line := fmt.Sprintf("%s %-24s %s -> %s", ch.IATA, ch.Name, ch.From, ch.To)
				if len(ch.Added) > 0 {
					line += fmt.Sprintf(" +warnings:%v", ch.Added)
				}
				if len(ch.Cleared) > 0 {
					line += fmt.Sprintf(" -warnings:%v", ch.Cleared)
				}
				if ch.DelayDelta != nil {
					line += fmt.Sprintf(" delayΔ:%+dm", *ch.DelayDelta)
				}
				fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// flightyDiffSnapshots compares two snapshot maps and returns sorted changes.
func flightyDiffSnapshots(previous, current map[string]json.RawMessage) []flightyStatusChange {
	decode := func(raw json.RawMessage) (flightySnapshotAirport, bool) {
		var ap flightySnapshotAirport
		if err := json.Unmarshal(raw, &ap); err != nil || ap.IATA == "" {
			return ap, false
		}
		return ap, true
	}
	changes := []flightyStatusChange{}
	iatas := map[string]bool{}
	for iata := range previous {
		iatas[iata] = true
	}
	for iata := range current {
		iatas[iata] = true
	}
	for iata := range iatas {
		var prev, cur flightySnapshotAirport
		var hasPrev, hasCur bool
		if raw, ok := previous[iata]; ok {
			prev, hasPrev = decode(raw)
		}
		if raw, ok := current[iata]; ok {
			cur, hasCur = decode(raw)
		}
		if !hasPrev || !hasCur {
			continue // airport added/removed from tracking; not a status change
		}
		ch := flightyStatusChange{IATA: iata, Name: cur.Name}
		if prev.Status != cur.Status {
			ch.From = prev.Status
			ch.To = cur.Status
		}
		prevW := warningSet(prev.Warnings)
		curW := warningSet(cur.Warnings)
		for w := range curW {
			if !prevW[w] {
				ch.Added = append(ch.Added, w)
			}
		}
		for w := range prevW {
			if !curW[w] {
				ch.Cleared = append(ch.Cleared, w)
			}
		}
		if cur.CumulativeDelay != prev.CumulativeDelay {
			d := cur.CumulativeDelay - prev.CumulativeDelay
			ch.DelayDelta = &d
		}
		if ch.From != "" || ch.To != "" || len(ch.Added) > 0 || len(ch.Cleared) > 0 || ch.DelayDelta != nil {
			sort.Strings(ch.Added)
			sort.Strings(ch.Cleared)
			changes = append(changes, ch)
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].IATA < changes[j].IATA })
	return changes
}

func warningSet(warnings []string) map[string]bool {
	out := map[string]bool{}
	for _, w := range warnings {
		out[w] = true
	}
	return out
}
