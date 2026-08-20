// Absorbed offline commands — search (full-text), sql (composable SELECT),
// and brokers (aggregated broker activity). All read the local store and
// touch the network zero times.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/loopnet/internal/loopnet"
)

// --- search -----------------------------------------------------------------

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search synced LoopNet listings offline",
		Long: `search runs a full-text query against everything synced into the local
store — listing addresses, descriptions, brokers, and property facts. It
touches the network zero times.`,
		Example: `  loopnet-pp-cli search warehouse
  loopnet-pp-cli search "class a office" --limit 50 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			st, err := lnOpenStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()
			if limit <= 0 {
				limit = 25
			}
			hits, err := st.Search(query, limit)
			if err != nil {
				return apiErr(fmt.Errorf("searching local store: %w", err))
			}
			results := make([]json.RawMessage, 0, len(hits))
			results = append(results, hits...)
			return flags.printJSON(cmd, map[string]any{
				"query": query, "count": len(results), "results": results,
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum results to return")
	return cmd
}

// --- sql --------------------------------------------------------------------

func newSQLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a read-only SQL query against the local store",
		Long: `sql runs a read-only SELECT against the local SQLite store, for composable
analysis the built-in commands do not cover. Useful tables: resources (the
listing/property JSON, queryable with json_extract) and ln_observations (the
append-only price time series).

Only SELECT / WITH queries are permitted.`,
		Example: `  loopnet-pp-cli sql "SELECT COUNT(*) AS n FROM ln_observations"
  loopnet-pp-cli sql "SELECT listing_id, price FROM ln_observations ORDER BY observed_at DESC LIMIT 10"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if err := assertReadOnlySQL(query); err != nil {
				return usageErr(err)
			}
			st, err := lnOpenStoreRO()
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := st.Query(query)
			if err != nil {
				return apiErr(fmt.Errorf("running query: %w", err))
			}
			defer rows.Close()
			cols, err := rows.Columns()
			if err != nil {
				return apiErr(err)
			}
			var out []map[string]any
			for rows.Next() {
				cells := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range cells {
					ptrs[i] = &cells[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					continue
				}
				rec := map[string]any{}
				for i, c := range cols {
					if b, ok := cells[i].([]byte); ok {
						rec[c] = string(b)
					} else {
						rec[c] = cells[i]
					}
				}
				out = append(out, rec)
			}
			return flags.printJSON(cmd, map[string]any{
				"columns": cols, "row_count": len(out), "rows": out,
			})
		},
	}
	return cmd
}

// assertReadOnlySQL rejects anything that is not a single SELECT/WITH query.
func assertReadOnlySQL(q string) error {
	upper := strings.ToUpper(strings.TrimSpace(q))
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("only SELECT / WITH queries are allowed")
	}
	for _, kw := range []string{"INSERT ", "UPDATE ", "DELETE ", "DROP ", "ALTER ", "CREATE ", "REPLACE ", "ATTACH ", "PRAGMA ", "VACUUM"} {
		if strings.Contains(upper, kw) {
			return fmt.Errorf("query contains a write/DDL keyword (%s); sql is read-only", strings.TrimSpace(kw))
		}
	}
	return nil
}

// --- brokers ----------------------------------------------------------------

func newBrokersCmd(flags *rootFlags) *cobra.Command {
	var market lnMarketFlags
	var limit int

	cmd := &cobra.Command{
		Use:   "brokers <location>",
		Short: "Rank brokers by listing activity in a synced submarket",
		Long: `brokers aggregates the brokers behind a synced submarket's listings,
ranking them by active listing count and total asking-price volume.`,
		Example: `  loopnet-pp-cli brokers worcester-ma --type industrial
  loopnet-pp-cli brokers worcester-ma --limit 20 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			location := loopnet.SlugLocation(args[0])
			st, err := lnOpenStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()

			listings, err := lnListings(st, location, market.propertyType, market.listingType)
			if err != nil {
				return apiErr(err)
			}
			if len(listings) == 0 {
				return lnNoData(location)
			}
			type brokerAgg struct {
				Broker      string  `json:"broker"`
				Company     string  `json:"company,omitempty"`
				Listings    int     `json:"listings"`
				TotalVolume float64 `json:"total_volume"`
			}
			agg := map[string]*brokerAgg{}
			for _, l := range listings {
				name := strings.TrimSpace(l.BrokerName)
				if name == "" {
					name = strings.TrimSpace(l.BrokerCompany)
				}
				if name == "" {
					continue
				}
				key := name + "|" + l.BrokerCompany
				b, ok := agg[key]
				if !ok {
					b = &brokerAgg{Broker: name, Company: l.BrokerCompany}
					agg[key] = b
				}
				b.Listings++
				b.TotalVolume += l.Price
			}
			rows := make([]brokerAgg, 0, len(agg))
			for _, b := range agg {
				rows = append(rows, *b)
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Listings != rows[j].Listings {
					return rows[i].Listings > rows[j].Listings
				}
				return rows[i].TotalVolume > rows[j].TotalVolume
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return flags.printJSON(cmd, map[string]any{
				"location": location, "count": len(rows), "brokers": rows,
			})
		},
	}
	addMarketFlags(cmd, &market)
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum brokers to return (0 = all)")
	return cmd
}
