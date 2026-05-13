// PATCH: hand-authored novel-feature file. See .printing-press-patches.json patch id "novel-series-search".
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// SeriesSearchResult is one row returned by `series search`.
type SeriesSearchResult struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Survey string `json:"survey"`
	Area   string `json:"area,omitempty"`
	Item   string `json:"item,omitempty"`
	Units  string `json:"units,omitempty"`
	Adjust string `json:"adjust,omitempty"`
}

func newSeriesSearchCmd(flags *rootFlags) *cobra.Command {
	var surveyFilter, areaFilter, itemFilter, adjustFilter string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Find a BLS series ID by plain-English title, survey, or area.",
		Long:  "Run an FTS5 search over the locally-synced BLS series catalog. The catalog covers the most common ~120 indicators across CPI, CES, CPS, JOLTS, PPI, ECI, productivity, and LAUS. Pass --survey/--area/--item/--adjust to narrow further. BLS has no public series-search endpoint, so this is the only way to discover IDs without scraping bls.gov data tools.",
		Example: `  bls-pp-cli series search "Los Angeles CPI"
  bls-pp-cli series search "unemployment rate" --survey LN
  bls-pp-cli series search "shelter" --survey CU --adjust seasonal --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return fmt.Errorf("search query is empty; pass a phrase like \"unemployment rate California\"")
			}
			db, err := openBLSStore(cmd.Context(), "")
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer func() { _ = db.Close() }()

			// Build the FTS5 query: split into tokens and AND them together,
			// adding wildcard suffix per token so partial matches work.
			// Each token is wrapped in double quotes so FTS5 treats embedded
			// punctuation (- : ( ) etc.) as literal characters instead of
			// operators — without this, queries like "U-3" or "C++" crash
			// the SQLite FTS layer with `no such column` errors.
			rawTokens := strings.Fields(query)
			tokens := make([]string, 0, len(rawTokens))
			for _, t := range rawTokens {
				t = strings.TrimSpace(t)
				t = strings.ReplaceAll(t, "\"", "")
				if t == "" {
					continue
				}
				tokens = append(tokens, "\""+t+"\"*")
			}
			ftsQuery := strings.Join(tokens, " ")

			// FTS5 ranks via bm25; lower (more negative) = better match.
			sqlStmt := `SELECT c.id, c.title, c.survey, COALESCE(c.area,''), COALESCE(c.item,''), COALESCE(c.units,''), COALESCE(c.adjust,'')
				FROM series_catalog c
				JOIN series_catalog_fts fts ON fts.rowid = c.rowid
				WHERE series_catalog_fts MATCH ?`
			argsSQL := []any{ftsQuery}
			if surveyFilter != "" {
				sqlStmt += " AND UPPER(c.survey) = UPPER(?)"
				argsSQL = append(argsSQL, surveyFilter)
			}
			if areaFilter != "" {
				sqlStmt += " AND LOWER(c.area) LIKE LOWER(?)"
				argsSQL = append(argsSQL, "%"+areaFilter+"%")
			}
			if itemFilter != "" {
				sqlStmt += " AND LOWER(c.item) LIKE LOWER(?)"
				argsSQL = append(argsSQL, "%"+itemFilter+"%")
			}
			if adjustFilter != "" {
				sqlStmt += " AND LOWER(c.adjust) = LOWER(?)"
				argsSQL = append(argsSQL, adjustFilter)
			}
			// When the user has not narrowed to a specific area, bias the
			// ranking toward the canonical national headline (area = "U.S.")
			// so plain-English queries like "unemployment rate" return
			// LNS14000000 before per-state LAUS breakouts.
			if areaFilter == "" {
				sqlStmt += " ORDER BY (CASE WHEN LOWER(COALESCE(c.area,'')) = 'u.s.' THEN 0 ELSE 1 END), bm25(series_catalog_fts) LIMIT ?"
			} else {
				sqlStmt += " ORDER BY bm25(series_catalog_fts) LIMIT ?"
			}
			argsSQL = append(argsSQL, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), sqlStmt, argsSQL...)
			if err != nil {
				return fmt.Errorf("search query: %w", err)
			}
			defer func() { _ = rows.Close() }()
			var results []SeriesSearchResult
			for rows.Next() {
				var r SeriesSearchResult
				if err := rows.Scan(&r.ID, &r.Title, &r.Survey, &r.Area, &r.Item, &r.Units, &r.Adjust); err != nil {
					return err
				}
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(results)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			// Human-friendly table
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				m := make([]map[string]any, 0, len(results))
				for _, r := range results {
					m = append(m, map[string]any{
						"id":     r.ID,
						"title":  r.Title,
						"survey": r.Survey,
						"area":   r.Area,
						"item":   r.Item,
						"adjust": r.Adjust,
					})
				}
				if len(m) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "(no matches; try `bls-pp-cli sync` to refresh the catalog or broaden your query)")
					return nil
				}
				if err := printAutoTable(cmd.OutOrStdout(), m); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d match(es). Use --json --select id,title to feed downstream tools.\n", len(results))
				return nil
			}
			raw, _ := json.Marshal(results)
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&surveyFilter, "survey", "", "Filter by BLS survey abbreviation (e.g. CU, LN, CE, JT).")
	cmd.Flags().StringVar(&areaFilter, "area", "", "Substring match against the area label (e.g. \"Los Angeles\", \"California\").")
	cmd.Flags().StringVar(&itemFilter, "item", "", "Substring match against the item label (e.g. \"shelter\", \"food\").")
	cmd.Flags().StringVar(&adjustFilter, "adjust", "", "Filter by seasonal adjustment: seasonal or nsa.")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of results to return.")
	return cmd
}
