package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/tavily/internal/store"

	"github.com/spf13/cobra"
)

func newResearchSearchCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "search [terms...]",
		Short: "Full-text search across stored research reports",
		Long:  "Runs an FTS5 query against all locally stored research reports, returning matching excerpts with dates and original queries",
		Example: strings.Trim(`
  tavily-pp-cli research search golang testing
  tavily-pp-cli research search "machine learning" --limit 5
  tavily-pp-cli research search "API design" --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			terms := strings.Join(args, " ")
			if terms == "" {
				return cmd.Help()
			}
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

			matches, err := db.SearchResearchReports(terms, flagLimit)
			if err != nil {
				return fmt.Errorf("searching reports: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, matches)
			}

			if len(matches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No research reports match %q\n", terms)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Found %d matching reports:\n\n", len(matches))
			for i, m := range matches {
				fmt.Fprintf(cmd.OutOrStdout(), "%d. [%s] Query: %s\n", i+1, m.CreatedAt.Format("2006-01-02"), m.InputQuery)
				fmt.Fprintf(cmd.OutOrStdout(), "   %s\n\n", m.Excerpt)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagDB, "db", "", "Path to SQLite database (default ~/.tavily-pp-cli/tavily.db)")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum number of results to return")

	return cmd
}
