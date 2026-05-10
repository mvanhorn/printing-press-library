package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"homeassistant-pp-cli/internal/store"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Execute raw SQL against the local data store",
		Example: `  # List all tables
  homeassistant-pp-cli sql "SELECT name FROM sqlite_master WHERE type='table'"

  # Count entities by domain
  homeassistant-pp-cli sql "SELECT count(*) FROM states"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.Open("")
			if err != nil {
				return err
			}

			query := args[0]
			rows, err := db.DB.Query(query)
			if err != nil {
				return err
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return err
			}

			if flags.asJSON {
				var results []map[string]any
				for rows.Next() {
					row := make([]any, len(cols))
					ptr := make([]any, len(cols))
					for i := range row {
						ptr[i] = &row[i]
					}
					if err := rows.Scan(ptr...); err != nil {
						return err
					}
					m := make(map[string]any)
					for i, col := range cols {
						val := row[i]
						if b, ok := val.([]byte); ok {
							val = string(b)
						}
						m[col] = val
					}
					results = append(results, m)
				}
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			var tableRows [][]string
			for rows.Next() {
				row := make([]any, len(cols))
				ptr := make([]any, len(cols))
				for i := range row {
					ptr[i] = &row[i]
				}
				if err := rows.Scan(ptr...); err != nil {
					return err
				}
				var sRow []string
				for _, val := range row {
					if val == nil {
						sRow = append(sRow, "NULL")
					} else {
						sRow = append(sRow, fmt.Sprintf("%v", val))
					}
				}
				tableRows = append(tableRows, sRow)
			}

			if len(tableRows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Empty result set.")
				return nil
			}

			return flags.printTable(cmd, cols, tableRows)
		},
	}
	return cmd
}
