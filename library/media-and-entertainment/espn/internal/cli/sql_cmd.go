package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/store"
	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sql <query>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Run read-only SQL queries against the local database",
		Long: `Execute raw SQL SELECT queries against the local SQLite database.
Write operations (INSERT, UPDATE, DELETE, DROP, ALTER, CREATE) are blocked.
Available tables: events, teams_domain, news_domain, standings, resources.`,
		Example: `  espn-pp-cli sql "SELECT short_name, home_score, away_score FROM events WHERE league='nfl' LIMIT 10"
  espn-pp-cli sql "SELECT display_name, abbreviation FROM teams_domain WHERE league='nba'"
  espn-pp-cli sql "SELECT COUNT(*) as total FROM events GROUP BY league" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return usageErr(fmt.Errorf("SQL query is required\nUsage: sql <query>"))
			}
			query := args[0]

			// Block write operations
			upper := strings.ToUpper(strings.TrimSpace(query))
			for _, keyword := range []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE"} {
				if strings.HasPrefix(upper, keyword) {
					return usageErr(fmt.Errorf("write operations are not allowed. Only SELECT queries are supported"))
				}
			}

			if dbPath == "" {
				dbPath = defaultDBPath("espn-pp-cli")
			}

			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nhint: run 'espn-pp-cli sync' first", err)
			}
			defer db.Close()

			results, err := db.QueryRaw(query)
			if err != nil {
				return fmt.Errorf("query error: %w", err)
			}

			if results == nil {
				results = []map[string]any{}
			}

			// JSON output
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			// Table output
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No results.")
				return nil
			}

			if err := printAutoTable(cmd.OutOrStdout(), results); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d rows\n", len(results))
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
