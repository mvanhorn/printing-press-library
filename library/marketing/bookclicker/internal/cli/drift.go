// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: list engagement decay across syncs.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/marketing/bookclicker/internal/store"

	"github.com/spf13/cobra"
)

type driftRow struct {
	ListID       int64   `json:"list_id"`
	Name         string  `json:"name"`
	FirstOpen    float64 `json:"first_open_rate"`
	LatestOpen   float64 `json:"latest_open_rate"`
	OpenDrop     float64 `json:"open_rate_drop"`
	FirstClick   float64 `json:"first_click_rate"`
	LatestClick  float64 `json:"latest_click_rate"`
	ClickDrop    float64 `json:"click_rate_drop"`
	MemberChange int64   `json:"member_change"`
	Samples      int     `json:"samples"`
}

type driftResult struct {
	Lists   []driftRow `json:"lists"`
	Count   int        `json:"count"`
	MinDrop float64    `json:"min_drop"`
	Note    string     `json:"note,omitempty"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		flagMinDrop float64
		flagLimit   int
	)

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Show newsletters whose open or click rate has decayed since earlier syncs.",
		Long: "Compare each newsletter's earliest and latest observed engagement rates.\n\n" +
			"Requires at least two snapshots: every 'sync' records one. A single sync\n" +
			"produces no drift, which is expected rather than an error.",
		Example: "  bookclicker-pp-cli drift --min-drop 0.05 --agent",
		// read-only against Bookclicker; the snapshot it records lands only in
		// the local store, which the local-write annotation declares.
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "drift")
			}
			if flagMinDrop < 0 {
				return usageErr(fmt.Errorf("--min-drop must be zero or positive"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			empty := driftResult{Lists: make([]driftRow, 0), MinDrop: flagMinDrop,
				Note: "no rate history yet; run 'sync' at least twice to compare snapshots"}
			db, handled, err := openMirror(ctx, dbPath, cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, empty)
			if err != nil || handled {
				return err
			}
			defer db.Close()

			if err := store.EnsureBookclickerTables(ctx, db); err != nil {
				return err
			}
			// Snapshot before comparing, so a sync followed by drift records the
			// fresh rates. Gated on sync freshness, so repeated drift runs cannot
			// fabricate data points.
			if _, err := store.SnapshotListRatesIfStale(ctx, db); err != nil {
				return err
			}

			rows, err := db.DB().QueryContext(ctx, `
				SELECT h.list_id,
				       COUNT(*)                                   AS samples,
				       MIN(h.observed_at)                         AS first_at,
				       MAX(h.observed_at)                         AS last_at
				FROM list_rate_history h
				GROUP BY h.list_id
				HAVING COUNT(*) > 1`)
			if err != nil {
				return fmt.Errorf("querying rate history: %w", err)
			}
			type span struct {
				listID          int64
				samples         int
				firstAt, lastAt string
			}
			spans := make([]span, 0)
			for rows.Next() {
				var s span
				var f, l sql.NullString
				if err := rows.Scan(&s.listID, &s.samples, &f, &l); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning rate history: %w", err)
				}
				s.firstAt, s.lastAt = f.String, l.String
				spans = append(spans, s)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating rate history: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing rows: %w", err)
			}

			out := make([]driftRow, 0, len(spans))
			for _, s := range spans {
				var fo, fc, lo, lc sql.NullFloat64
				var fm, lm sql.NullInt64
				_ = db.DB().QueryRowContext(ctx,
					`SELECT open_rate, click_rate, member_count FROM list_rate_history WHERE list_id = ? AND observed_at = ?`,
					s.listID, s.firstAt).Scan(&fo, &fc, &fm)
				_ = db.DB().QueryRowContext(ctx,
					`SELECT open_rate, click_rate, member_count FROM list_rate_history WHERE list_id = ? AND observed_at = ?`,
					s.listID, s.lastAt).Scan(&lo, &lc, &lm)

				openDrop := fo.Float64 - lo.Float64
				clickDrop := fc.Float64 - lc.Float64
				if openDrop < flagMinDrop && clickDrop < flagMinDrop {
					continue
				}
				var name sql.NullString
				_ = db.DB().QueryRowContext(ctx,
					`SELECT name FROM lists WHERE CAST(id AS INTEGER) = ?`, s.listID).Scan(&name)

				out = append(out, driftRow{
					ListID: s.listID, Name: name.String, Samples: s.samples,
					FirstOpen: fo.Float64, LatestOpen: lo.Float64, OpenDrop: openDrop,
					FirstClick: fc.Float64, LatestClick: lc.Float64, ClickDrop: clickDrop,
					MemberChange: lm.Int64 - fm.Int64,
				})
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].OpenDrop > out[j].OpenDrop })
			if flagLimit > 0 && len(out) > flagLimit {
				out = out[:flagLimit]
			}

			res := driftResult{Lists: out, Count: len(out), MinDrop: flagMinDrop}
			if len(spans) == 0 {
				res.Note = "no newsletter has two or more snapshots yet; run 'sync' again later to build history"
			} else if len(out) == 0 {
				res.Note = fmt.Sprintf("no newsletter dropped by %.3f or more across %d tracked lists", flagMinDrop, len(spans))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-38s %8s %8s %8s %8s\n", "NEWSLETTER", "OPEN→", "DROP", "CLICK→", "MEMBERS")
			for _, d := range out {
				fmt.Fprintf(w, "%-38s %7.1f%% %7.1f%% %7.1f%% %+8d\n",
					truncPad(d.Name, 38), d.LatestOpen*100, d.OpenDrop*100, d.LatestClick*100, d.MemberChange)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().Float64Var(&flagMinDrop, "min-drop", 0.02, "Minimum absolute rate drop to report (0.05 = 5 points)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum newsletters to return")
	return cmd
}
