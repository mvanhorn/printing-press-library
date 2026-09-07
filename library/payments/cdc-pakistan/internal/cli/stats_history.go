// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/cdcparse"
	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/store"
)

type statRow struct {
	AsOf       string `json:"as_of"`
	MetricKey  string `json:"metric_key"`
	Label      string `json:"label"`
	Raw        string `json:"raw"`
	Known      bool   `json:"known"`
	Provenance string `json:"provenance"`
	FirstSeen  string `json:"first_seen"`
}

type statsView struct {
	Mode          string    `json:"mode"`
	Rows          []statRow `json:"rows"`
	Snapshots     int       `json:"distinct_snapshots"`
	Captured      string    `json:"captured_as_of,omitempty"`
	NewRows       int       `json:"new_rows,omitempty"`
	UnknownLabels []string  `json:"unknown_labels,omitempty"`
	Note          string    `json:"note"`
}

func newNovelStatsHistoryCmd(flags *rootFlags) *cobra.Command {
	var (
		metric   string
		snapshot bool
		limit    int
		dbPath   string
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Track the sixteen CDS aggregate metrics forward from your first sync, with archived rows labelled as such.",
		Long: strings.Trim(`
Append-only history of CDC's CDS aggregate statistics.

The statistics page is OVERWRITTEN every month. A given month's figures cease to
exist the moment the next month lands, so this table can only ever accumulate
forward from the first time you capture it. There is no backfill: with --snapshot
this records TODAY's published figures and nothing else.

WHY THE HISTORY IS THIN AND WILL STAY THIN. Pre-CLI months are recoverable only
from web-archive captures of a Cloudflare-challenged page: 42 distinct
year-months exist between 2015-09 and 2024-06, densely in 2018-19 and only two
or three per year after, with nothing at all after 2024-06. Those rows would
carry provenance='archive' and are NON-CONTIGUOUS. They must never be read as a
continuous series, and nothing here interpolates a missing month.

LABEL DRIFT IS REAL. Between Apr-2024 and Jul-2026 CDC renamed "Number of
Shares" to "Number of Shares/Debt Instruments", SPLIT "Number of Securities"
into Listed and Unlisted, and DELETED the "Sahulat Accounts" row. Rows are keyed
on a normalised identifier so a rewording does not start a new series, and any
label that matches no rule is stored with known=false rather than dropped.
`, "\n"),
		Example: "  cdc-pakistan-pp-cli stats history --metric sub_accounts_individual --agent",
		Annotations: map[string]string{
			"mcp:read-only":   "false",
			"mcp:local-write": "true",
			// --snapshot is deliberate: this is a local-write feature and the coverage
			// gate must see its REAL write path execute, not just a read.
			"pp:happy-args":       "--snapshot;--metric=sub_accounts_individual;--limit=5;--timeout=30m",
			"pp:typed-exit-codes": "0,3,4",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "stats history")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("cdc-pakistan-pp-cli")
			}
			view := &statsView{Mode: "history", Rows: []statRow{}}

			if snapshot {
				db, err := store.OpenWithContext(ctx, dbPath)
				if err != nil {
					return configErr(fmt.Errorf("opening store: %w", err))
				}
				defer db.Close()
				if err := store.EnsureCDCSchema(ctx, db); err != nil {
					return err
				}
				if err := captureStats(ctx, db, flags, view); err != nil {
					return err
				}
				view.Mode = "snapshot"
				if err := readStats(ctx, db, metric, limit, view); err != nil {
					return err
				}
			} else {
				// Route through the table-aware gate like the six sibling read
				// commands. A file-existence check is NOT enough: the generated
				// store creates the DB file on first open, so on a fresh install
				// the file exists while cdc_stats_snapshots does not, and the
				// query leaked a raw "no such table" SQL error at exit 5.
				db, ready, oerr := openCDCMirror(ctx, dbPath, "cdc_stats_snapshots")
				if oerr != nil {
					return oerr
				}
				if !ready {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: cdc-pakistan-pp-cli stats history --snapshot --db %s\n", dbPath, dbPath)
					view.Note = "no local store yet; run with --snapshot to record the first observation"
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), view, flags)
					}
					return nil
				}
				defer db.Close()
				if err := readStats(ctx, db, metric, limit, view); err != nil {
					return err
				}
			}

			if view.Note == "" {
				view.Note = fmt.Sprintf("%d distinct snapshot period(s) stored. This series accumulates FORWARD only; the page is overwritten monthly so earlier months cannot be fetched.", view.Snapshots)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if len(view.Rows) == 0 {
				fmt.Fprintln(w, view.Note)
				return nil
			}
			fmt.Fprintf(w, "%-12s %-34s %s\n", "AS-OF", "METRIC", "VALUE")
			for _, r := range view.Rows {
				k := r.MetricKey
				if k == "" {
					k = "(unmapped: " + truncStr(r.Label, 22) + ")"
				}
				fmt.Fprintf(w, "%-12s %-34s %s\n", r.AsOf, k, truncStr(r.Raw, 40))
			}
			fmt.Fprintf(w, "\n%s\n", view.Note)
			if len(view.UnknownLabels) > 0 {
				fmt.Fprintf(w, "unmapped labels (kept, flagged): %s\n", strings.Join(view.UnknownLabels, "; "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&metric, "metric", "", "filter to one normalised metric key, e.g. sub_accounts_individual")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum rows to return (0 = no limit)")
	cmd.Flags().BoolVar(&snapshot, "snapshot", false, "fetch the live page and append today's observation (requires a clearance cookie)")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

func captureStats(ctx context.Context, db *store.Store, flags *rootFlags, view *statsView) error {
	cl, err := LoadClearance(flags)
	if err != nil {
		return err
	}
	if err := cl.CheckMargin(0); err != nil {
		return err
	}
	f := newClearanceFetcher(cl, 45*time.Second)
	_, body, err := f.GetHTML(ctx, "/about-us/statistics/")
	if err != nil {
		return err
	}
	snap, err := cdcparse.ParseStatistics(body)
	if err != nil {
		return apiErr(fmt.Errorf("parsing statistics page: %w", err))
	}
	if snap.AsOf == "" {
		return apiErr(fmt.Errorf("statistics page carried no 'Facts (As of ...)' period; refusing to store an undated snapshot"))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	const q = `INSERT INTO cdc_stats_snapshots
	  (as_of, metric_key, label, raw, known, provenance, first_seen, last_seen)
	  VALUES (?,?,?,?,?,'live',?,?)
	  ON CONFLICT(as_of, metric_key, label) DO UPDATE SET
	    raw=excluded.raw, last_seen=excluded.last_seen`
	for _, m := range snap.Metrics {
		known := 0
		if m.Known {
			known = 1
		}
		var existed int
		_ = db.DB().QueryRowContext(ctx,
			`SELECT 1 FROM cdc_stats_snapshots WHERE as_of=? AND metric_key=? AND label=?`,
			snap.AsOf, m.Key, m.Label).Scan(&existed)
		if existed == 0 {
			view.NewRows++
		}
		if _, err := db.DB().ExecContext(ctx, q, snap.AsOf, m.Key, m.Label, m.Raw, known, now, now); err != nil {
			return apiErr(fmt.Errorf("storing metric %q: %w", m.Label, err))
		}
	}
	view.Captured = snap.AsOf
	view.UnknownLabels = snap.UnknownLabels
	return nil
}

func readStats(ctx context.Context, db *store.Store, metric string, limit int, view *statsView) error {
	q := `SELECT as_of, metric_key, label, raw, known, provenance, first_seen
	        FROM cdc_stats_snapshots`
	var argv []any
	if m := strings.TrimSpace(metric); m != "" {
		q += ` WHERE metric_key = ?`
		argv = append(argv, m)
	}
	q += ` ORDER BY as_of, metric_key`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := db.DB().QueryContext(ctx, q, argv...)
	if err != nil {
		return apiErr(fmt.Errorf("reading statistics history: %w", err))
	}
	periods := map[string]bool{}
	for rows.Next() {
		var r statRow
		var known int
		if err := rows.Scan(&r.AsOf, &r.MetricKey, &r.Label, &r.Raw, &known, &r.Provenance, &r.FirstSeen); err != nil {
			_ = rows.Close()
			return apiErr(err)
		}
		r.Known = known == 1
		periods[r.AsOf] = true
		view.Rows = append(view.Rows, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return apiErr(err)
	}
	if err := rows.Close(); err != nil {
		return apiErr(err)
	}
	view.Snapshots = len(periods)
	if len(view.Rows) == 0 && metric != "" {
		view.Note = fmt.Sprintf("no rows for metric %q. Run --snapshot first, or check the key with an unfiltered call.", metric)
	}
	return nil
}
