// Copyright 2026 rahul-bansal. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/internal/store"
	"github.com/spf13/cobra"
)

// buyerIntentListRow is the flat sales-ready row emitted by buyer-intent list.
type buyerIntentListRow struct {
	Company     string `json:"company"`
	Domain      string `json:"domain"`
	Employees   int64  `json:"employees"`
	Industry    string `json:"industry"`
	Country     string `json:"country"`
	PageType    string `json:"page_type"`
	SignalType  string `json:"signal_type"`
	IntentScore int64  `json:"intent_score"`
	VisitedOn   string `json:"visited_on"`
}

// newBuyerIntentListCmd is the `buyer-intent list` child that filters the
// local buyer_intent rows for triage by recency, score, and optional product.
func newBuyerIntentListCmd(flags *rootFlags) *cobra.Command {
	var since string
	var minScore int
	var product string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "list [flags]",
		Short:       "Filter local buyer-intent rows by recency, score, and product for sales triage",
		Long:        "Flattens nested firmographics into a sales-ready table (company, domain, employees, industry, country, page_type, signal_type, intent_score, visited_on). Reads only locally synced buyer_intent rows.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  g2-pp-cli buyer-intent list --since 24h --min-score 50 --csv",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
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

			rows, err := queryBuyerIntentList(db.DB(), cutoff, minScore, product, limit)
			if err != nil {
				return fmt.Errorf("querying buyer-intent rows: %w", err)
			}

			if flags.csv {
				// Route through printOutputWithFlags via JSON to keep CSV
				// rendering consistent with the rest of the CLI.
				raw, _ := json.Marshal(rows)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, rows)
			}
			headers := []string{"company", "domain", "employees", "industry", "country", "page_type", "signal_type", "intent_score", "visited_on"}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					r.Company,
					r.Domain,
					fmt.Sprintf("%d", r.Employees),
					r.Industry,
					r.Country,
					r.PageType,
					r.SignalType,
					fmt.Sprintf("%d", r.IntentScore),
					r.VisitedOn,
				})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Window of recency (e.g. 24h, 7d, 1w)")
	cmd.Flags().IntVar(&minScore, "min-score", 0, "Minimum intent score to include")
	cmd.Flags().StringVar(&product, "product", "", "Filter by product slug or ID (optional)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.local/share/g2-pp-cli/data.db)")
	return cmd
}

// queryBuyerIntentList runs the filter+project SQL against the local store.
func queryBuyerIntentList(db *sql.DB, cutoff time.Time, minScore int, product string, limit int) ([]buyerIntentListRow, error) {
	conds := []string{
		`resource_type IN ('buyer_intent', 'buyer-intent', 'products_buyer_intent')`,
		`synced_at >= ?`,
	}
	argv := []any{cutoff}
	conds = append(conds, `COALESCE(json_extract(data, '$.attributes.company_intent_score'),
                            json_extract(data, '$.attributes.intent_score'), 0) >= ?`)
	argv = append(argv, minScore)
	if strings.TrimSpace(product) != "" {
		conds = append(conds, `(
            json_extract(data, '$.attributes.subject_product_id')   = ?
            OR json_extract(data, '$.attributes.subject_product_slug') = ?
            OR json_extract(data, '$.attributes.product_id')        = ?
            OR json_extract(data, '$.attributes.product_slug')      = ?
        )`)
		argv = append(argv, product, product, product, product)
	}
	q := `
SELECT
  COALESCE(json_extract(data, '$.attributes.company_name'), '')        AS company,
  COALESCE(json_extract(data, '$.attributes.domain'),
           json_extract(data, '$.attributes.company_domain'), '')      AS domain,
  COALESCE(json_extract(data, '$.attributes.employees'),
           json_extract(data, '$.attributes.employee_count'), 0)       AS employees,
  COALESCE(json_extract(data, '$.attributes.industry'), '')            AS industry,
  COALESCE(json_extract(data, '$.attributes.country'),
           json_extract(data, '$.attributes.company_country'), '')     AS country,
  COALESCE(json_extract(data, '$.attributes.page_type'), '')           AS page_type,
  COALESCE(json_extract(data, '$.attributes.signal_type'), '')         AS signal_type,
  COALESCE(json_extract(data, '$.attributes.company_intent_score'),
           json_extract(data, '$.attributes.intent_score'), 0)         AS intent_score,
  COALESCE(json_extract(data, '$.attributes.visited_on'),
           json_extract(data, '$.attributes.day'), '')                 AS visited_on
FROM resources
WHERE ` + strings.Join(conds, " AND ") + `
ORDER BY intent_score DESC
LIMIT ?
`
	argv = append(argv, limit)
	rs, err := db.Query(q, argv...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rs.Close()
	var out []buyerIntentListRow
	for rs.Next() {
		var (
			company    sql.NullString
			domain     sql.NullString
			employees  sql.NullInt64
			industry   sql.NullString
			country    sql.NullString
			pageType   sql.NullString
			signalType sql.NullString
			intent     sql.NullInt64
			visited    sql.NullString
		)
		if err := rs.Scan(&company, &domain, &employees, &industry, &country, &pageType, &signalType, &intent, &visited); err != nil {
			return nil, err
		}
		out = append(out, buyerIntentListRow{
			Company:     company.String,
			Domain:      domain.String,
			Employees:   employees.Int64,
			Industry:    industry.String,
			Country:     country.String,
			PageType:    pageType.String,
			SignalType:  signalType.String,
			IntentScore: intent.Int64,
			VisitedOn:   visited.String,
		})
	}
	return out, rs.Err()
}
