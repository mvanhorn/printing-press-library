package cli

// pp:data-source local

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

type nccplPanelRow struct {
	Date       string  `json:"date"`
	Resource   string  `json:"resource"`
	Key        string  `json:"key"`
	Symbol     string  `json:"symbol,omitempty"`
	Metric     string  `json:"metric"`
	Value      float64 `json:"value"`
	ObservedAt string  `json:"observed_at"`
}

type nccplPanelView struct {
	Rows          []nccplPanelRow `json:"rows"`
	Resource      string          `json:"resource"`
	DistinctDates int             `json:"distinct_dates"`
	DistinctKeys  int             `json:"distinct_keys"`
	MissingDates  int             `json:"missing_dates_in_span"`
	FirstDate     string          `json:"first_date,omitempty"`
	LastDate      string          `json:"last_date,omitempty"`
	Emitted       int             `json:"emitted_rows,omitempty"`
	EmitTarget    string          `json:"emit_target,omitempty"`
	Note          string          `json:"note,omitempty"`
}

func newNCCPLPanelCmd(flags *rootFlags) *cobra.Command {
	var (
		resource   string
		fromDate   string
		toDate     string
		metricsCSV string
		emitPath   string
		dbPath     string
		sortBy     string
		currency   string
		pivot      bool
	)

	cmd := &cobra.Command{
		Use:   "panel",
		Short: "Emit a stored resource as a long-format research panel",
		Long: strings.Trim(`
Reshape a synced resource into tidy long format -- one row per
(date, key, metric, value) with the vintage stamp that records when the value was
first observed.

Gaps are never interpolated or forward-filled. Missing sessions are counted and
reported so a consumer can decide what to do about them, and the universe width
(distinct dates and keys actually spanned) is always printed alongside the data.

observed_at is what makes a flow number admissible as an ex-ante input: NCCPL
publishes no timestamp of its own, so first-observation time is recorded at sync and
is not reconstructible afterwards.

--emit appends the panel into a caller-supplied SQLite file as table nccpl_panel,
keyed (resource, date, key, metric) so re-emission is idempotent.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli panel --resource fipi --from 2026-08-01 --to 2026-09-04 --json
  nccpl-pp-cli panel --resource var-margins --metrics free_float --agent
  nccpl-pp-cli panel --resource fipi --emit ~/psx-research/data/research.db
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--resource=fipi",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "panel")
			}
			if strings.TrimSpace(resource) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--resource is required (one of: %s)", strings.Join(nccplResourceNames(), ", ")))
			}
			res, ok := nccplResourceByName(resource)
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown resource %q; valid: %s", resource, strings.Join(nccplResourceNames(), ", ")))
			}
			if !nccplValidCurrency(currency) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown --currency %q; valid: pkr, usd", currency))
			}
			if err := nccplSortPanel(nil, sortBy); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("nccpl-pp-cli")
			}
			view := nccplPanelView{Rows: make([]nccplPanelRow, 0), Resource: resource}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: nccpl-pp-cli sync --resources %s --latest-only --db %s\n", dbPath, resource, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			ready, err := store.NCCPLSchemaReady(ctx, db)
			if err != nil {
				return err
			}
			if !ready {
				fmt.Fprintf(cmd.ErrOrStderr(), "no NCCPL data synced yet at %s\nrun: nccpl-pp-cli sync --resources %s --latest-only --db %s\n", dbPath, resource, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}

			obs, err := store.NCCPLObservations(ctx, db, resource, fromDate, toDate)
			if err != nil {
				return err
			}
			covered, err := store.NCCPLCoveredDates(ctx, db, resource)
			if err != nil {
				return err
			}

			wanted := map[string]bool{}
			for _, m := range strings.Split(metricsCSV, ",") {
				if m = strings.TrimSpace(m); m != "" {
					wanted[m] = true
				}
			}

			dates := map[string]bool{}
			keys := map[string]bool{}
			for _, o := range obs {
				row, err := nccplDecodePayload(o.Payload)
				if err != nil {
					continue
				}
				dates[o.Date] = true
				keys[o.Key] = true
				symbol := nccplSymbolOf(res, row)

				fields := make([]string, 0, len(row))
				for f := range row {
					fields = append(fields, f)
				}
				sort.Strings(fields)
				for _, f := range fields {
					if len(wanted) > 0 && !wanted[f] {
						continue
					}
					if !nccplMetricMatchesCurrency(f, currency) {
						continue
					}
					v, ok := nccplNum(row, f)
					if !ok {
						continue
					}
					view.Rows = append(view.Rows, nccplPanelRow{
						Date: o.Date, Resource: resource, Key: o.Key, Symbol: symbol,
						Metric: f, Value: v, ObservedAt: o.ObservedAt,
					})
				}
			}

			view.DistinctDates = len(dates)
			view.DistinctKeys = len(keys)
			if len(covered) > 0 {
				lo, hi := covered[0].Date, covered[len(covered)-1].Date
				if fromDate != "" && fromDate > lo {
					lo = fromDate
				}
				if toDate != "" && toDate < hi {
					hi = toDate
				}
				view.FirstDate, view.LastDate = lo, hi
				have := map[string]bool{}
				for _, c := range covered {
					have[c.Date] = true
				}
				if sessions, err := nccplSessionDates(lo, hi); err == nil {
					for _, d := range sessions {
						if !have[d] {
							view.MissingDates++
						}
					}
				}
			}
			if view.MissingDates > 0 {
				view.Note = fmt.Sprintf("%d session(s) inside the emitted span were never fetched and are absent, not zero; run 'coverage --resources %s'", view.MissingDates, resource)
			} else if len(view.Rows) == 0 {
				view.Note = fmt.Sprintf("no stored rows for %s in this range; run 'sync --resources %s'", resource, resource)
			}

			if err := nccplSortPanel(view.Rows, sortBy); err != nil {
				return usageErr(err)
			}

			if pivot {
				metric := strings.TrimSpace(metricsCSV)
				if strings.Contains(metric, ",") {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--pivot needs a single field; pass one --metrics value (got %q)", metricsCSV))
				}
				pv := nccplPivotPanel(view.Rows, metric)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), pv, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-34s", "SECTOR")
				for _, inv := range pv.Investors {
					fmt.Fprintf(cmd.OutOrStdout(), "%14s", inv)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%16s\n", "TOTAL")
				for _, c := range pv.Sectors {
					fmt.Fprintf(cmd.OutOrStdout(), "%-34s", truncate(c.Sector, 33))
					for _, inv := range pv.Investors {
						fmt.Fprintf(cmd.OutOrStdout(), "%14.2f", c.By[inv])
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%16.2f\n", c.Total)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d row(s) folded into %d sector(s) x %d investor class(es)\n",
					pv.RowsUsed, len(pv.Sectors), len(pv.Investors))
				if pv.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n", pv.Note)
				}
				return nil
			}

			if emitPath != "" {
				n, err := nccplEmitPanel(emitPath, view.Rows)
				if err != nil {
					return err
				}
				view.Emitted, view.EmitTarget = n, emitPath
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-24s %-28s %16s\n", "DATE", "KEY", "METRIC", "VALUE")
			for _, r := range view.Rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-24s %-28s %16.4f\n", r.Date, r.Key, r.Metric, r.Value)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d row(s) across %d date(s) and %d key(s)",
				len(view.Rows), view.DistinctDates, view.DistinctKeys)
			if view.FirstDate != "" {
				fmt.Fprintf(cmd.OutOrStdout(), ", span %s..%s", view.FirstDate, view.LastDate)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			if view.EmitTarget != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "emitted %d row(s) into %s (table nccpl_panel)\n", view.Emitted, view.EmitTarget)
			}
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resource, "resource", "", "Resource to emit (one of: "+strings.Join(nccplResourceNames(), ", ")+")")
	cmd.Flags().StringVar(&fromDate, "from", "", "First settlement date to emit (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "Last settlement date to emit (YYYY-MM-DD)")
	cmd.Flags().StringVar(&metricsCSV, "metrics", "", "Comma-separated numeric fields to emit; empty means every numeric field")
	cmd.Flags().StringVar(&emitPath, "emit", "", "Append the panel into this SQLite file as table nccpl_panel")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&sortBy, "sort", "date", "Row order: date, value, abs-value (largest absolute flow first), key or metric")
	cmd.Flags().StringVar(&currency, "currency", "", "Restrict flow metrics to one currency: pkr (net_value/buy_value/sell_value) or usd (net_value_USD)")
	cmd.Flags().BoolVar(&pivot, "pivot", false, "Pivot sector-wise rows into a sector x investor-class matrix, the shape the public dashboards publish")
	return cmd
}

// nccplEmitPanel appends panel rows into a caller-supplied SQLite file.
//
// The target is opened directly rather than through the CLI's own store so that
// emitting into a research database does not create this CLI's private tables there.
func nccplEmitPanel(path string, rows []nccplPanelRow) (int, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return 0, fmt.Errorf("opening emit target %s: %w", path, err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS nccpl_panel (
  resource    TEXT NOT NULL,
  date        TEXT NOT NULL,
  key         TEXT NOT NULL,
  symbol      TEXT,
  metric      TEXT NOT NULL,
  value       REAL,
  observed_at TEXT,
  PRIMARY KEY (resource, date, key, metric)
);
CREATE INDEX IF NOT EXISTS idx_nccpl_panel_date   ON nccpl_panel(date);
CREATE INDEX IF NOT EXISTS idx_nccpl_panel_symbol ON nccpl_panel(symbol, date);`); err != nil {
		return 0, fmt.Errorf("creating nccpl_panel in %s: %w", path, err)
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("emit begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
INSERT INTO nccpl_panel (resource, date, key, symbol, metric, value, observed_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(resource, date, key, metric) DO UPDATE SET
  value = excluded.value, symbol = excluded.symbol`)
	if err != nil {
		return 0, fmt.Errorf("emit prepare: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	n := 0
	for _, r := range rows {
		if _, err := stmt.Exec(r.Resource, r.Date, r.Key, r.Symbol, r.Metric, r.Value, r.ObservedAt); err != nil {
			return 0, fmt.Errorf("emit row %s/%s/%s: %w", r.Resource, r.Date, r.Metric, err)
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("emit commit: %w", err)
	}
	return n, nil
}
