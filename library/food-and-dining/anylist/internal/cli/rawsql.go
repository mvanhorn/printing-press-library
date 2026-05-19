package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"

	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a raw SQL query against the local AnyList cache",
		Long: `Run arbitrary SQL against the local SQLite cache. Requires sync to have been run first.

Useful aggregation patterns:
  SELECT category_match_id, COUNT(*) as count FROM items GROUP BY category_match_id ORDER BY count DESC
  SELECT list_id, SUM(CASE WHEN checked=1 THEN 1 ELSE 0 END) as done FROM items GROUP BY list_id
  SELECT r.name, AVG(r.rating) as avg_rating FROM recipes r GROUP BY r.name`,
		Example: `  anylist-pp-cli sql "SELECT name, COUNT(*) FROM items GROUP BY name ORDER BY 2 DESC LIMIT 10"
  anylist-pp-cli sql "SELECT list_id, COUNT(*) as total FROM items GROUP BY list_id" --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			st, err := store.Open(cfg)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			query := strings.Join(args, " ")
			db := st.DB()

			rows, err := db.QueryContext(ctx, query)
			if err != nil {
				return fmt.Errorf("query error: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("getting columns: %w", err)
			}

			var allRows []map[string]any
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return fmt.Errorf("scanning row: %w", err)
				}
				row := map[string]any{}
				for i, col := range cols {
					v := vals[i]
					// Convert []byte to string for display
					if b, ok := v.([]byte); ok {
						v = string(b)
					}
					row[col] = v
				}
				allRows = append(allRows, row)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating rows: %w", err)
			}

			w := cmd.OutOrStdout()

			if flags.asJSON {
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(allRows)
			}

			if flags.csv {
				cw := csv.NewWriter(w)
				if err := cw.Write(cols); err != nil {
					return err
				}
				for _, row := range allRows {
					record := make([]string, len(cols))
					for i, col := range cols {
						v := row[col]
						if v == nil {
							record[i] = ""
						} else {
							record[i] = fmt.Sprintf("%v", v)
						}
					}
					if err := cw.Write(record); err != nil {
						return err
					}
				}
				cw.Flush()
				return cw.Error()
			}

			// Tab-separated table
			tw := newTabWriter(w)
			fmt.Fprintln(tw, strings.Join(cols, "\t"))
			for _, row := range allRows {
				vals := make([]string, len(cols))
				for i, col := range cols {
					v := row[col]
					if v == nil {
						vals[i] = ""
					} else {
						vals[i] = fmt.Sprintf("%v", v)
					}
				}
				fmt.Fprintln(tw, strings.Join(vals, "\t"))
			}
			return tw.Flush()
		},
	}
}
