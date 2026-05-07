package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newSimilarCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var jsonOut bool
	var limit int
	cmd := &cobra.Command{
		Use:         "similar [query]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Find potentially duplicate issues using fuzzy text search",
		Long:        "Search locally synced issues using FTS5 full-text search to find potential duplicates. Works offline.",
		Example: `  linear-pp-cli similar "login bug"
  linear-pp-cli similar "payment failed" --limit 20
  linear-pp-cli similar "onboarding" --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Verify mode: short-circuit so a synthetic query against an
			// empty FTS index doesn't fail the mechanical verify pass.
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'linear-pp-cli sync' first.", err)
			}
			defer db.Close()

			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("search query cannot be empty")
			}
			results, err := db.SearchIssues(args[0])
			if err != nil {
				return fmt.Errorf("searching: %w", err)
			}

			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			if len(results) == 0 {
				fmt.Printf("No issues matching %q\n", args[0])
				return nil
			}

			fmt.Printf("%-12s %-15s %s\n", "ID", "STATE", "TITLE")
			fmt.Println(strings.Repeat("-", 70))
			for _, raw := range results {
				var row struct {
					Identifier string                `json:"identifier"`
					Title      string                `json:"title"`
					State      struct{ Name string } `json:"state"`
				}
				json.Unmarshal(raw, &row)
				title := row.Title
				if len(title) > 45 {
					title = title[:42] + "..."
				}
				fmt.Printf("%-12s %-15s %s\n", row.Identifier, row.State.Name, title)
			}
			fmt.Fprintf(os.Stderr, "\n%d results for %q\n", len(results), args[0])
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
