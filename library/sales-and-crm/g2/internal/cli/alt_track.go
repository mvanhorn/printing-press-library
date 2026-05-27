// Copyright 2026 rahul-bansal. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/internal/store"
	"github.com/spf13/cobra"
)

// altTrackRow is the per-company row emitted by `alt-track`.
type altTrackRow struct {
	Company     string `json:"company"`
	Domain      string `json:"domain"`
	Employees   int64  `json:"employees"`
	IntentScore int64  `json:"intent_score"`
	Visits      int64  `json:"visits"`
}

// newAltTrackCmd builds the top-level `alt-track <product>` command.
// Ranks companies showing Alternatives-signal buyer-intent at the
// given product, weighted by visits and employee size.
func newAltTrackCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "alt-track [product]",
		Short:       "Rank companies showing Alternatives buyer intent against a product",
		Long:        "Filters local buyer_intent_events to signal_type='Alternatives' for the given product slug or ID, groups by company, and ranks by employees × visits.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  g2-pp-cli alt-track slack --since 30d --limit 25 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("product (slug or ID) is required"))
			}
			product := strings.TrimSpace(args[0])
			if product == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("product (slug or ID) is required"))
			}
			window, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
			}
			cutoff := time.Now().Add(-window)
			if dbPath == "" {
				dbPath = defaultDBPath("g2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			maybeEmitSyncHints(cmd, db, "buyer_intent", flags.maxAge)

			if cliutil.IsDogfoodEnv() && limit > 50 {
				limit = 50
			}

			rows, err := queryAltTrack(db.DB(), product, cutoff, limit)
			if err != nil {
				return fmt.Errorf("querying buyer-intent: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, rows)
			}
			if flags.csv {
				return printAltTrackCSV(cmd, rows)
			}
			headers := []string{"company", "domain", "employees", "intent_score", "visits"}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					r.Company,
					r.Domain,
					fmt.Sprintf("%d", r.Employees),
					fmt.Sprintf("%d", r.IntentScore),
					fmt.Sprintf("%d", r.Visits),
				})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window for the diff (e.g. 7d, 24h, 1w)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max companies to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.local/share/g2-pp-cli/data.db)")
	return cmd
}

// queryAltTrack runs the SQL aggregation. Returns a stable shape ordered
// by employees * visits desc.
func queryAltTrack(db *sql.DB, product string, cutoff time.Time, limit int) ([]altTrackRow, error) {
	q := `
SELECT
  COALESCE(json_extract(data, '$.attributes.company_name'), '')   AS company,
  COALESCE(json_extract(data, '$.attributes.domain'),
           json_extract(data, '$.attributes.company_domain'), '') AS domain,
  COALESCE(json_extract(data, '$.attributes.employees'),
           json_extract(data, '$.attributes.employee_count'), 0)  AS employees,
  COALESCE(json_extract(data, '$.attributes.company_intent_score'),
           json_extract(data, '$.attributes.intent_score'), 0)    AS intent_score,
  COUNT(*) AS visits
FROM resources
WHERE resource_type IN ('buyer_intent', 'buyer-intent', 'products_buyer_intent')
  AND LOWER(COALESCE(json_extract(data, '$.attributes.signal_type'), '')) = 'alternatives'
  AND (
    json_extract(data, '$.attributes.subject_product_id')   = ?
    OR json_extract(data, '$.attributes.subject_product_slug') = ?
    OR json_extract(data, '$.attributes.product_id')        = ?
    OR json_extract(data, '$.attributes.product_slug')      = ?
  )
  AND synced_at >= ?
GROUP BY company, domain
ORDER BY (employees * visits) DESC
LIMIT ?
`
	rs, err := db.Query(q, product, product, product, product, cutoff, limit)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rs.Close()
	var out []altTrackRow
	for rs.Next() {
		var (
			company   sql.NullString
			domain    sql.NullString
			employees sql.NullInt64
			intent    sql.NullInt64
			visits    sql.NullInt64
		)
		if err := rs.Scan(&company, &domain, &employees, &intent, &visits); err != nil {
			return nil, err
		}
		out = append(out, altTrackRow{
			Company:     company.String,
			Domain:      domain.String,
			Employees:   employees.Int64,
			IntentScore: intent.Int64,
			Visits:      visits.Int64,
		})
	}
	return out, rs.Err()
}

func printAltTrackCSV(cmd *cobra.Command, rows []altTrackRow) error {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "company,domain,employees,intent_score,visits")
	for _, r := range rows {
		fmt.Fprintf(w, "%s,%s,%d,%d,%d\n",
			csvEscape(r.Company),
			csvEscape(r.Domain),
			r.Employees,
			r.IntentScore,
			r.Visits,
		)
	}
	return nil
}

// csvEscape wraps in quotes when the value contains a comma, quote, or newline.
func csvEscape(s string) string {
	if !strings.ContainsAny(s, ",\"\n") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
