// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

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
		Short: "Stateful diff since the last sync: new events and magnitude/alert/status revisions",
		Long: `Detect what changed in the USGS catalog since you last synced.

A 'revisions' table is populated incrementally by 'sync' (a SQLite BEFORE
INSERT trigger on the resources table compares pre/post values per event on
mag, alert, and status). This command queries that table for the window you
ask about.

Filter by --type:
  new      — events that appeared in the local store on a sync run
  revised  — events whose magnitude/alert/status changed (use --min-mag-delta)

Note: detecting events that USGS removed from the upstream catalog (true
'deleted' semantics) would require a sync-time pass that diffs the local
store against the live FDSN response; that pass is not implemented in this
CLI, so 'deleted' is intentionally NOT a supported --type filter. The
revisions table reflects only what sync upserts touched.

On a fresh install, the revisions table is empty until the first sync run
populates it. First-run output simply notes 'no revisions recorded yet'.`,
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
			if ct := strings.ToLower(strings.TrimSpace(changeType)); ct != "" && ct != "new" && ct != "revised" {
				return usageErr(fmt.Errorf("--type must be one of: new, revised (got %q). 'deleted' is intentionally unsupported — see `usgs-earthquakes-pp-cli changes --help`", changeType))
			}
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
	cmd.Flags().StringVar(&changeType, "type", "", "Change type filter: new | revised (default: all). 'deleted' is intentionally not supported — detecting upstream-removed events requires a sync-time catalog diff that this CLI does not currently implement.")
	cmd.Flags().Float64Var(&minMagDelta, "min-mag-delta", 0, "When --type revised, require absolute magnitude delta >= this value")
	cmd.Flags().IntVar(&limit, "limit", 500, "Max changes to return")
	return cmd
}

// ensureRevisionsTable creates the revisions table and the SQLite trigger
// that actually populates it during sync. Without the trigger, `changes`
// would always print "no revisions recorded yet" — sync.go is
// generator-emitted and does not write to `revisions` directly. Instead the
// trigger fires BEFORE INSERT on the `resources` table, snapshotting deltas
// at the moment sync upserts via INSERT OR REPLACE.
//
// The trigger handles both shapes in a single BEFORE INSERT:
//   - If an event row with the same id already exists with different
//     mag/alert/status, appends a `revised` row capturing pre/post. INSERT OR
//     REPLACE deletes the old row after BEFORE INSERT runs, so the snapshot
//     is captured before the old data is lost.
//   - If no existing row matches, appends a `new` row.
//
// Idempotent (CREATE IF NOT EXISTS). Safe to call from any path that opens
// the store; ensureRevisionsSchemaOnDefaultDB calls it from cobra.OnInitialize
// in root.go so the trigger is in place before sync upserts.
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

		CREATE TRIGGER IF NOT EXISTS events_revision_before
		BEFORE INSERT ON resources
		FOR EACH ROW WHEN NEW.resource_type = 'events'
		BEGIN
			INSERT INTO revisions (event_id, change_type, observed_at,
				pre_mag, post_mag, pre_alert, post_alert, pre_status, post_status, note)
			SELECT
				NEW.id, 'revised',
				CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
				CAST(json_extract(r.data, '$.properties.mag') AS REAL),
				CAST(json_extract(NEW.data, '$.properties.mag') AS REAL),
				json_extract(r.data, '$.properties.alert'),
				json_extract(NEW.data, '$.properties.alert'),
				json_extract(r.data, '$.properties.status'),
				json_extract(NEW.data, '$.properties.status'),
				'sync revised'
			FROM resources r
			WHERE r.resource_type = 'events' AND r.id = NEW.id
			  AND (
				IFNULL(CAST(json_extract(r.data, '$.properties.mag') AS REAL), -999) !=
				IFNULL(CAST(json_extract(NEW.data, '$.properties.mag') AS REAL), -999)
				OR IFNULL(json_extract(r.data, '$.properties.alert'), '') !=
				   IFNULL(json_extract(NEW.data, '$.properties.alert'), '')
				OR IFNULL(json_extract(r.data, '$.properties.status'), '') !=
				   IFNULL(json_extract(NEW.data, '$.properties.status'), '')
			  );

			INSERT INTO revisions (event_id, change_type, observed_at,
				post_mag, post_alert, post_status, note)
			SELECT
				NEW.id, 'new',
				CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER),
				CAST(json_extract(NEW.data, '$.properties.mag') AS REAL),
				json_extract(NEW.data, '$.properties.alert'),
				json_extract(NEW.data, '$.properties.status'),
				'new event'
			WHERE NOT EXISTS (
				SELECT 1 FROM resources r
				WHERE r.resource_type = 'events' AND r.id = NEW.id
			);
		END;
	`)
	return err
}

// ensureRevisionsSchemaOnDefaultDB opens the canonical local store path and
// installs the revisions table + trigger. Called from root.go's
// cobra.OnInitialize so the trigger is live before sync upserts. Best-effort:
// any error (missing dir, locked DB) is suppressed so command startup doesn't
// break; the next `changes` or `sync` invocation retries.
func ensureRevisionsSchemaOnDefaultDB(ctx context.Context) {
	db, err := openLocalStore(ctx)
	if err != nil {
		return
	}
	defer db.Close()
	_ = ensureRevisionsTable(ctx, db)
}

func oradefault(s, dflt string) string {
	if s == "" {
		return dflt
	}
	return s
}
