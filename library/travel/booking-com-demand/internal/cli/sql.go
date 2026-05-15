package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com-demand/internal/store"

	"github.com/spf13/cobra"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run raw SQL against the local SQLite store",
		Long: `Execute arbitrary SQL queries against the synced local store.

Tables: accommodations, reviews, review_scores, availability, orders,
messages, locations_cities, locations_districts, locations_landmarks.

Each table has columns: id (TEXT), data (JSON), synced_at (TIMESTAMP).
Use json_extract(data, '$.field') to query JSON fields.`,
		Example: strings.Trim(`
  booking-com-demand-pp-cli sql "SELECT id, json_extract(data, '$.name') as name FROM accommodations LIMIT 10"
  booking-com-demand-pp-cli sql "SELECT COUNT(*) as total FROM reviews"
  booking-com-demand-pp-cli sql "SELECT id, json_extract(data, '$.price') as price FROM availability ORDER BY json_extract(data, '$.price') ASC LIMIT 5"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			query := strings.Join(args, " ")

			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nhint: run 'booking-com-demand-pp-cli sync' first to populate the local store", err)
			}
			defer s.Close()

			results, err := s.SQL(query)
			if err != nil {
				return fmt.Errorf("sql query failed: %w", err)
			}

			if len(results) == 0 {
				return writeNoop(flags, "no_results", "query returned no rows")
			}

			return flags.printJSON(cmd, results)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store (default: ~/.cache/booking-com-demand-pp-cli/store.db)")

	return cmd
}
