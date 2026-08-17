// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

type kalshiSeriesSearchHit struct {
	Ticker   string `json:"ticker"`
	Title    string `json:"title"`
	Category string `json:"category,omitempty"`
	URL      string `json:"url"`
}

type kalshiSeriesSearchResult struct {
	Query string                   `json:"query"`
	Count int                      `json:"count"`
	Hits  []kalshiSeriesSearchHit `json:"hits"`
}

// newKalshiSeriesSearchCmd is the substring search over local kalshi_series
// rows that should have answered every dogfood session's series-discovery
// question in one call. topic's FTS-only ranker buries series tickers
// like KXNBAWEST below higher-term-frequency matches; this command grep-
// matches ticker + title in the synced store with no ranking.
func newKalshiSeriesSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "kalshi-series-search <term>",
		Short: "Substring search over locally synced Kalshi series tickers and titles",
		Example: `  prediction-goat-pp-cli kalshi-series-search WEST
  prediction-goat-pp-cli kalshi-series-search kanye --agent --limit 30`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("kalshi-series-search: --limit must be greater than zero"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("kalshi-series-search: %w", err)
			}
			defer db.Close()
			term := strings.TrimSpace(strings.Join(args, " "))
			like := "%" + strings.ToUpper(term) + "%"
			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT id, data FROM resources
WHERE resource_type='kalshi_series'
AND (UPPER(id) LIKE ? OR UPPER(COALESCE(json_extract(data,'$.title'),'')) LIKE ?)
ORDER BY id
LIMIT ?`, like, like, limit)
			if err != nil {
				return fmt.Errorf("kalshi-series-search: %w", err)
			}
			defer rows.Close()
			hits := make([]kalshiSeriesSearchHit, 0)
			for rows.Next() {
				var id, data sql.NullString
				if err := rows.Scan(&id, &data); err != nil {
					return fmt.Errorf("kalshi-series-search scan: %w", err)
				}
				if !id.Valid {
					continue
				}
				hit := kalshiSeriesSearchHit{Ticker: id.String, URL: "https://kalshi.com/markets?series=" + id.String}
				if data.Valid {
					var obj map[string]any
					if json.Unmarshal([]byte(data.String), &obj) == nil {
						hit.Title = jsonString(obj, "title")
						hit.Category = jsonString(obj, "category")
					}
				}
				hits = append(hits, hit)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("kalshi-series-search rows: %w", err)
			}
			result := kalshiSeriesSearchResult{Query: term, Count: len(hits), Hits: hits}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			tableRows := make([][]string, 0, len(hits))
			for _, h := range hits {
				tableRows = append(tableRows, []string{h.Ticker, h.Title, h.Category})
			}
			return printSimpleTable(cmd.OutOrStdout(), []string{"Ticker", "Title", "Category"}, tableRows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}
