package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/tavily/internal/store"

	"github.com/spf13/cobra"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagDB string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Show extracted pages and crawl results older than N days",
		Long:  "Lists locally stored extracted pages and crawl results that are older than the specified threshold, indicating content that may need re-extraction",
		Example: strings.Trim(`
  tavily-pp-cli stale
  tavily-pp-cli stale --days 14
  tavily-pp-cli stale --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dbPath := flagDB
			if dbPath == "" {
				dbPath = store.DefaultDBPath()
			}

			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening db: %w", err)
			}
			defer db.Close()

			items, err := db.GetStaleContent(flagDays)
			if err != nil {
				return fmt.Errorf("querying stale content: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, items)
			}

			if len(items) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No content older than %d days.\n", flagDays)
				return nil
			}

			headers := []string{"TYPE", "URL", "FETCHED", "AGE (DAYS)"}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{
					item.Type,
					truncate(item.URL, 60),
					item.FetchedAt.Format("2006-01-02"),
					fmt.Sprintf("%d", item.AgeDays),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&flagDays, "days", 7, "Show content older than this many days")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to SQLite database (default ~/.tavily-pp-cli/tavily.db)")

	return cmd
}
