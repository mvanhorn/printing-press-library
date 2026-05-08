// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/store"
)

type rosterRow struct {
	LocationID    string `json:"location_id"`
	LocationName  string `json:"location_name"`
	Leads24h      int    `json:"leads_24h"`
	Unread        int    `json:"unread"`
	UnpaidInvoice int    `json:"unpaid_invoices"`
	StaleOpps     int    `json:"stale_opps"`
}

func newRosterCmd(flags *rootFlags) *cobra.Command {
	var metrics string
	var threshold int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "roster",
		Short:       "Multi-location operational health rollup (one row per sub-account)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Aggregate operational health metrics across every synced sub-account
(location). One row per location, columns are aggregated against the local
SQLite store. Replaces a multi-tab UI walk for the Monday-standup view.

Available --metric values (comma-separated):
  leads-24h         New contacts created in the last 24 hours
  unread            Conversations with unreadCount > 0
  unpaid-invoices   Invoices in any unpaid status
  stale-opps        Opportunities with no activity in --threshold days
`,
		Example: strings.Trim(`
  # All metrics across every synced location
  gohighlevel-pp-cli roster --json

  # Just standup essentials, narrowed via --select
  gohighlevel-pp-cli roster --metric leads-24h,unread --json --select rows.location_name,rows.unread
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gohighlevel-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'gohighlevel-pp-cli sync' first.", err)
			}
			defer db.Close()

			selected := map[string]bool{}
			for _, m := range strings.Split(metrics, ",") {
				m = strings.TrimSpace(m)
				if m != "" {
					selected[m] = true
				}
			}
			if len(selected) == 0 {
				selected["leads-24h"] = true
				selected["unread"] = true
				selected["unpaid-invoices"] = true
				selected["stale-opps"] = true
			}

			rows := map[string]*rosterRow{}
			locRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, COALESCE(json_extract(data, '$.name'), id) AS name
				FROM locations
			`)
			if err == nil {
				for locRows.Next() {
					var id, name string
					if scanErr := locRows.Scan(&id, &name); scanErr == nil {
						rows[id] = &rosterRow{LocationID: id, LocationName: name}
					}
				}
				locRows.Close()
			}
			if len(rows) == 0 {
				rows["unknown"] = &rosterRow{LocationID: "unknown", LocationName: "(no locations synced)"}
			}

			ctx := cmd.Context()
			cutoff24h := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
			thresholdTS := time.Now().Add(-time.Duration(threshold) * 24 * time.Hour).Format(time.RFC3339)

			if selected["leads-24h"] {
				addLocationCounts(ctx, db.DB(), `
					SELECT COALESCE(json_extract(data, '$.locationId'), 'unknown') AS loc, COUNT(*)
					FROM contacts
					WHERE COALESCE(json_extract(data, '$.dateAdded'), '') >= ?
					GROUP BY loc
				`, []any{cutoff24h}, rows, "leads-24h")
			}
			if selected["unread"] {
				addLocationCounts(ctx, db.DB(), `
					SELECT COALESCE(json_extract(data, '$.locationId'), 'unknown') AS loc, COUNT(*)
					FROM conversations
					WHERE COALESCE(json_extract(data, '$.unreadCount'), 0) > 0
					GROUP BY loc
				`, nil, rows, "unread")
			}
			if selected["unpaid-invoices"] {
				addLocationCounts(ctx, db.DB(), `
					SELECT COALESCE(json_extract(data, '$.altId'), json_extract(data, '$.locationId'), 'unknown') AS loc, COUNT(*)
					FROM invoices
					WHERE LOWER(COALESCE(json_extract(data, '$.status'), '')) IN ('draft','sent','overdue','partially_paid','past_due','open')
					GROUP BY loc
				`, nil, rows, "unpaid-invoices")
			}
			if selected["stale-opps"] {
				addLocationCounts(ctx, db.DB(), `
					SELECT COALESCE(json_extract(data, '$.locationId'), 'unknown') AS loc, COUNT(*)
					FROM opportunities
					WHERE COALESCE(json_extract(data, '$.updatedAt'), json_extract(data, '$.dateUpdated'), '') < ?
					GROUP BY loc
				`, []any{thresholdTS}, rows, "stale-opps")
			}

			out := struct {
				GeneratedAt string       `json:"generated_at"`
				Threshold   int          `json:"threshold_days"`
				Metrics     []string     `json:"metrics"`
				Rows        []*rosterRow `json:"rows"`
			}{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Threshold:   threshold,
				Metrics:     sortedKeys(selected),
				Rows:        sortedRows(rows),
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Roster — %d locations (threshold=%dd)\n", len(out.Rows), threshold)
			fmt.Fprintln(cmd.OutOrStdout(), "Location\tLeads24h\tUnread\tUnpaid\tStaleOpps")
			for _, r := range out.Rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\t%d\t%d\t%d\n",
					r.LocationName, r.Leads24h, r.Unread, r.UnpaidInvoice, r.StaleOpps)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&metrics, "metric", "", "Comma-separated metrics: leads-24h,unread,unpaid-invoices,stale-opps")
	cmd.Flags().IntVar(&threshold, "threshold", 14, "Stale threshold in days (used by stale-opps metric)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}

func addLocationCounts(ctx context.Context, db *sql.DB, q string, args []any, rows map[string]*rosterRow, metric string) {
	r, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return
	}
	defer r.Close()
	for r.Next() {
		var loc string
		var n int
		if scanErr := r.Scan(&loc, &n); scanErr != nil {
			continue
		}
		row, ok := rows[loc]
		if !ok {
			row = &rosterRow{LocationID: loc, LocationName: loc}
			rows[loc] = row
		}
		switch metric {
		case "leads-24h":
			row.Leads24h += n
		case "unread":
			row.Unread += n
		case "unpaid-invoices":
			row.UnpaidInvoice += n
		case "stale-opps":
			row.StaleOpps += n
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedRows(m map[string]*rosterRow) []*rosterRow {
	rows := make([]*rosterRow, 0, len(m))
	for _, v := range m {
		rows = append(rows, v)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].LocationName < rows[j].LocationName })
	return rows
}
