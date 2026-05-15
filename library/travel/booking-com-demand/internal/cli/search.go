package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com-demand/internal/store"

	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across synced accommodations and reviews",
		Long: `Search uses SQLite FTS5 to find accommodations and reviews matching your query.

Data must be synced first with 'booking-com-demand-pp-cli sync'.
Searches across accommodation names, descriptions, and review text.`,
		Example: strings.Trim(`
  booking-com-demand-pp-cli search "beach resort"
  booking-com-demand-pp-cli search "excellent staff" --json
  booking-com-demand-pp-cli search "downtown" --limit 10`, "\n"),
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

			results, err := s.Search(query)
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if len(results) == 0 {
				return writeNoop(flags, "no_results", fmt.Sprintf("no results for %q", query))
			}

			return flags.printJSON(cmd, results)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store (default: ~/.cache/booking-com-demand-pp-cli/store.db)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of results")

	return cmd
}
