// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source local

// priceDropRow is one product's price decline over the lookback window.
type priceDropRow struct {
	RetailerID  string  `json:"retailer_id"`
	ProductID   string  `json:"product_id"`
	ProductName *string `json:"product_name"`
	OldPrice    float64 `json:"old_price"`
	NewPrice    float64 `json:"new_price"`
	Drop        float64 `json:"drop"`
	DropPct     float64 `json:"drop_pct"`
	OldTS       string  `json:"old_ts"`
	NewTS       string  `json:"new_ts"`
}

func newNovelPriceDropsCmd(flags *rootFlags) *cobra.Command {
	var flagRetailer []string
	var flagWeeks int
	var flagMinDropPct float64
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:         "price-drops",
		Short:       "Rank the products that fell the most over the last week (or N weeks) across everything synced",
		Example:     "  shopping-pp-cli price-drops --weeks 1 --min-drop-pct 15 --limit 25 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank the biggest price drops from local price history")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}

			weeks := flagWeeks
			if weeks <= 0 {
				weeks = 1
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 25
			}

			ctx := cmd.Context()
			db, err := openShopStore(ctx, resolveShopDBPath(flagDB))
			if err != nil {
				return err
			}
			defer db.Close()

			where := []string{`pp.price IS NOT NULL`}
			var qArgs []any
			if len(flagRetailer) > 0 {
				ph := make([]string, len(flagRetailer))
				for i, r := range flagRetailer {
					ph[i] = "?"
					qArgs = append(qArgs, r)
				}
				where = append(where, `pp.retailers_id IN (`+strings.Join(ph, ",")+`)`)
			}

			// Pull every priced observation, grouped by product, ordered by ts.
			// The window selection happens in Go so the "closest point to
			// weeks*7 days before the latest" rule is exact rather than an
			// approximation forced into SQL date arithmetic on a free-form
			// RFC3339 string. The product_name is a LEFT-JOIN-style correlated
			// subquery so price history without a matching products row still
			// reports (name null).
			query := `SELECT pp.retailers_id, pp.product_id, pp.ts, pp.price,
				(SELECT json_extract(p.data,'$.product_name')
				   FROM products p
				  WHERE p.retailers_id = pp.retailers_id AND p.id = pp.product_id)
			FROM price_points pp
			WHERE ` + strings.Join(where, " AND ") + `
			ORDER BY pp.retailers_id, pp.product_id, pp.ts ASC`

			rows, err := db.Query(query, qArgs...)
			if err != nil {
				return fmt.Errorf("query price points: %w", err)
			}
			defer rows.Close()

			type obs struct {
				ts    string
				tsT   time.Time
				price float64
			}
			type key struct{ rid, pid string }
			series := map[key][]obs{}
			names := map[key]*string{}
			order := []key{}
			for rows.Next() {
				var (
					rid   string
					pid   string
					ts    string
					price float64
					name  sql.NullString
				)
				if err := rows.Scan(&rid, &pid, &ts, &price, &name); err != nil {
					return fmt.Errorf("scan price point: %w", err)
				}
				// Skip points with an unparseable timestamp: a zero time.Time
				// is before any realistic target and would win baseline
				// selection, corrupting the drop/drop_pct ranking.
				parsed, err := time.Parse(time.RFC3339, ts)
				if err != nil {
					continue
				}
				k := key{rid, pid}
				if _, seen := series[k]; !seen {
					order = append(order, k)
					names[k] = nullableString(name)
				}
				series[k] = append(series[k], obs{ts: ts, tsT: parsed, price: price})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating price points: %w", err)
			}

			window := time.Duration(weeks) * 7 * 24 * time.Hour
			results := []priceDropRow{}
			for _, k := range order {
				pts := series[k]
				if len(pts) < 2 {
					continue
				}
				latest := pts[len(pts)-1]
				target := latest.tsT.Add(-window)
				// Pick the observation whose timestamp is closest to the target
				// (the latest point on/before target, else the earliest point).
				baseline := pts[0]
				for _, p := range pts[:len(pts)-1] {
					if !p.tsT.After(target) {
						baseline = p
					}
				}
				if baseline.price <= 0 {
					continue
				}
				drop := baseline.price - latest.price
				if drop <= 0 {
					continue
				}
				pct := drop / baseline.price * 100
				if pct < flagMinDropPct {
					continue
				}
				results = append(results, priceDropRow{
					RetailerID:  k.rid,
					ProductID:   k.pid,
					ProductName: names[k],
					OldPrice:    baseline.price,
					NewPrice:    latest.price,
					Drop:        drop,
					DropPct:     pct,
					OldTS:       baseline.ts,
					NewTS:       latest.ts,
				})
			}

			sort.SliceStable(results, func(i, j int) bool {
				return results[i].DropPct > results[j].DropPct
			})
			if len(results) > limit {
				results = results[:limit]
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringSliceVar(&flagRetailer, "retailer", nil, "Restrict to these retailer IDs (repeatable)")
	cmd.Flags().IntVar(&flagWeeks, "weeks", 1, "Lookback window in weeks")
	cmd.Flags().Float64Var(&flagMinDropPct, "min-drop-pct", 0, "Minimum percent drop to include")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum number of rows to return")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local store (defaults to SHOPPING_DB or the share path)")
	return cmd
}
