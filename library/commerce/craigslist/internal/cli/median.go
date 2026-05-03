// `median` aggregates p25/p50/p75 of prices for a query in the local store.
// Pure SQL aggregation — Craigslist has no native aggregation endpoint.
//
// FTS5 MATCH narrows by query text; --by-city groups by site so a reseller
// sees price discovery per market.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// medianRow is the typed shape returned per group (or as a single global row).
type medianRow struct {
	Site  string `json:"site,omitempty"`
	P25   int    `json:"p25"`
	P50   int    `json:"p50"`
	P75   int    `json:"p75"`
	Count int    `json:"count"`
}

func newMedianCmd(flags *rootFlags) *cobra.Command {
	var category, since string
	var byCity bool
	cmd := &cobra.Command{
		Use:         "median [query]",
		Short:       "p25/p50/p75 prices for a query in the local store",
		Long:        "Aggregate prices for listings matching a query (FTS5 MATCH on title/body), optionally over a time window or grouped by site.",
		Example:     "  craigslist-pp-cli median \"iphone 15\" --category mob --since 30d --by-city --json\n  craigslist-pp-cli median \"1BR\" --category apa --since 7d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			cutoff, err := parseDuration(since)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := computeMedian(ctx, db.DB(), query, category, cutoff, byCity, time.Now())
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				items := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					items = append(items, map[string]any{
						"site": r.Site, "p25": r.P25, "p50": r.P50, "p75": r.P75, "count": r.Count,
					})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Filter by category abbreviation")
	cmd.Flags().StringVar(&since, "since", "", "Limit window (e.g. 30d, 24h)")
	cmd.Flags().BoolVar(&byCity, "by-city", false, "Group by site")
	return cmd
}

// computeMedian queries the store for prices matching query/category/cutoff,
// then computes percentiles per group. byCity=false produces a single row;
// byCity=true produces one row per site.
func computeMedian(ctx context.Context, db *sql.DB, query, category string, cutoff time.Duration, byCity bool, now time.Time) ([]medianRow, error) {
	q := `SELECT l.site, l.price FROM listings l`
	args := []any{}
	conds := []string{"l.price > 0"}
	if fts := quoteFTS(query); fts != "" {
		// SQLite FTS5 requires the unaliased table name in MATCH; an alias
		// like "f" is interpreted as a missing column. Wrap the query as a
		// phrase via quoteFTS so embedded apostrophes (e.g. "men's") and
		// other reserved characters don't blow up the parser.
		q += ` JOIN listings_fts ON listings_fts.rowid = l.pid`
		conds = append(conds, "listings_fts MATCH ?")
		args = append(args, fts)
	}
	if category != "" {
		conds = append(conds, "l.category_abbr = ?")
		args = append(args, category)
	}
	if cutoff > 0 {
		conds = append(conds, "l.posted_at >= ?")
		args = append(args, now.Add(-cutoff).Unix())
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("median query: %w", err)
	}
	defer rows.Close()
	bySite := map[string][]int{}
	allPrices := []int{}
	for rows.Next() {
		var site string
		var price int
		if err := rows.Scan(&site, &price); err != nil {
			return nil, err
		}
		bySite[site] = append(bySite[site], price)
		allPrices = append(allPrices, price)
	}
	if !byCity {
		return []medianRow{{P25: percentile(allPrices, 0.25), P50: percentile(allPrices, 0.5), P75: percentile(allPrices, 0.75), Count: len(allPrices)}}, nil
	}
	out := make([]medianRow, 0, len(bySite))
	for site, ps := range bySite {
		out = append(out, medianRow{Site: site, P25: percentile(ps, 0.25), P50: percentile(ps, 0.5), P75: percentile(ps, 0.75), Count: len(ps)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Site < out[j].Site })
	return out, nil
}

// percentile returns the p-th percentile (p in [0,1]) of values, or 0 when
// values is empty. Nearest-rank method — fine for price discovery on
// hundreds-to-thousands of rows.
func percentile(values []int, p float64) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	rank := int(float64(len(sorted)-1) * p)
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}
