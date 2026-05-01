package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Run read-only SQL against the local store",
		Long:  "Execute arbitrary SELECT queries against the local SQLite database. Useful for ad-hoc analysis and debugging.",
		Example: `  linear-pp-cli sql "SELECT count(*) as cnt FROM issues"
  linear-pp-cli sql "SELECT identifier, title FROM issues WHERE priority = 1"
  linear-pp-cli sql "SELECT team_id, count(*) as cnt FROM issues GROUP BY team_id"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'linear-pp-cli sync' first.", err)
			}
			defer db.Close()

			results, err := db.SQL(args[0])
			if err != nil {
				return fmt.Errorf("query error: %w", err)
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
