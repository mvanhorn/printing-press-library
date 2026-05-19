// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/usgs-earthquakes/internal/store"
)

func newChangesCmd(flags *rootFlags) *cobra.Command {
	var (
		since       string
		changeType  string
		minMagDelta float64
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Stateful diff since the last sync: new events, magnitude/depth/alert revisions, deletions",
		Long: `Detect what changed in the USGS catalog since you last synced.

A 'revisions' table is populated incrementally by 'sync' (compares pre/post
values per event on mag, depth, alert, status, updated time). This command
queries that table for the window you ask about.

Filter by --type:
  new      — events that appeared in the catalog
  revised  — events whose magnitude/depth/alert/status changed (use --min-mag-delta)
  deleted  — events removed from the catalog

On a fresh install, the revisions table is empty until at least two sync
runs have completed. First-run output will simply note 'no revisions
recorded yet'.`,
		Example: strings.Trim(`
  # What's revised since yesterday with at least 0.3 magnitude delta
  usgs-earthquakes-pp-cli changes --since 24h --type revised --min-mag-delta 0.3 --json

  # Everything new in the past hour
  usgs-earthquakes-pp-cli changes --since 1h --type new --json

  # All change types in the past 7 days
  usgs-earthquakes-pp-cli changes --since 7d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			startT, err := parseSinceArg(since)
			if err != nil {
				return usageErr(err)
			}
			db, err := openLocalStore(ctx)
			if err != nil {
				return fmt.Errorf("opening local store (run `usgs-earthquakes-pp-cli sync` first): %w", err)
			}
			defer db.Close()
			if err := ensureRevisionsTable(ctx, db); err != nil {
				return err
			}

			whereType := ""
			argsSQL := []any{startT.UnixMilli()}
			if changeType != "" {
				whereType = " AND change_type = ?"
				argsSQL = append(argsSQL, strings.ToLower(changeType))
			}
			argsSQL = append(argsSQL, limit)
			rows, err := db.DB().QueryContext(ctx, `
				SELECT event_id, change_type, observed_at, pre_mag, post_mag, pre_alert, post_alert, pre_status, post_status, note
				FROM revisions
				WHERE observed_at >= ?`+whereType+`
				ORDER BY observed_at DESC
				LIMIT ?`, argsSQL...)
			if err != nil {
				return fmt.Errorf("query revisions: %w", err)
			}
			defer rows.Close()
			type changeRow struct {
				EventID    string  `json:"event_id"`
				Type       string  `json:"change_type"`
				ObservedAt string  `json:"observed_at"`
				PreMag     float64 `json:"pre_mag"`
				PostMag    float64 `json:"post_mag"`
				MagDelta   float64 `json:"mag_delta"`
				PreAlert   string  `json:"pre_alert"`
				PostAlert  string  `json:"post_alert"`
				PreStatus  string  `json:"pre_status"`
				PostStatus string  `json:"post_status"`
				Note       string  `json:"note"`
			}
			var results []changeRow
			for rows.Next() {
				var id, ct, note sql.NullString
				var observedAt sql.NullInt64
				var preMag, postMag sql.NullFloat64
				var preAlert, postAlert, preStatus, postStatus sql.NullString
				if rows.Scan(&id, &ct, &observedAt, &preMag, &postMag, &preAlert, &postAlert, &preStatus, &postStatus, &note) != nil {
					continue
				}
				delta := postMag.Float64 - preMag.Float64
				if changeType == "revised" && math.Abs(delta) < minMagDelta {
					continue
				}
				results = append(results, changeRow{
					EventID:    id.String,
					Type:       ct.String,
					ObservedAt: time.Unix(observedAt.Int64/1000, 0).UTC().Format(time.RFC3339),
					PreMag:     preMag.Float64,
					PostMag:    postMag.Float64,
					MagDelta:   round2(delta),
					PreAlert:   preAlert.String,
					PostAlert:  postAlert.String,
					PreStatus:  preStatus.String,
					PostStatus: postStatus.String,
					Note:       note.String,
				})
			}

			out := map[string]any{
				"window_start":  startT.Format(time.RFC3339),
				"change_type":   changeType,
				"min_mag_delta": minMagDelta,
				"count":         len(results),
				"changes":       results,
			}
			if len(results) == 0 {
				out["note"] = "no revisions recorded yet — run `usgs-earthquakes-pp-cli sync` at least twice to populate the revisions table"
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(w, "Window\t%s — now\n", startT.Format(time.RFC3339))
			fmt.Fprintf(w, "Type filter\t%s\n", oradefault(changeType, "(all)"))
			fmt.Fprintf(w, "Changes\t%d\n\n", len(results))
			if len(results) == 0 {
				fmt.Fprintln(w, "no revisions recorded yet — run `sync` at least twice to populate the revisions table")
				return w.Flush()
			}
			fmt.Fprintln(w, "TIME\tTYPE\tEVENT_ID\tPRE_MAG\tPOST_MAG\tDELTA\tPRE_ALERT\tPOST_ALERT\tNOTE")
			for _, r := range results {
				fmt.Fprintf(w, "%s\t%s\t%s\tM%.1f\tM%.1f\t%+.2f\t%s\t%s\t%s\n",
					r.ObservedAt, r.Type, r.EventID, r.PreMag, r.PostMag, r.MagDelta,
					oradefault(r.PreAlert, "-"), oradefault(r.PostAlert, "-"), r.Note)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Lookback window (24h, 7d, ISO 8601 timestamp)")
	cmd.Flags().StringVar(&changeType, "type", "", "Change type filter: new | revised | deleted (default: all)")
	cmd.Flags().Float64Var(&minMagDelta, "min-mag-delta", 0, "When --type revised, require absolute magnitude delta >= this value")
	cmd.Flags().IntVar(&limit, "limit", 500, "Max changes to return")
	return cmd
}

func ensureRevisionsTable(ctx context.Context, db *store.Store) error {
	_, err := db.DB().ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS revisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id TEXT NOT NULL,
			change_type TEXT NOT NULL,
			observed_at INTEGER NOT NULL,
			pre_mag REAL,
			post_mag REAL,
			pre_alert TEXT,
			post_alert TEXT,
			pre_status TEXT,
			post_status TEXT,
			note TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_revisions_observed ON revisions(observed_at);
		CREATE INDEX IF NOT EXISTS idx_revisions_event ON revisions(event_id);
	`)
	return err
}

func oradefault(s, dflt string) string {
	if s == "" {
		return dflt
	}
	return s
}
