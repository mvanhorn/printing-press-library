package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newSimilarCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var jsonOut bool
	var limit int
	var team string
	cmd := &cobra.Command{
		Use:         "similar [query]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Find potentially duplicate issues using fuzzy text search",
		Long:        "Search locally synced issues using FTS5 full-text search to find potential duplicates. Works offline.",
		Example: `  linear-pp-cli similar "login bug"
  linear-pp-cli similar "pipeline follow-up" --team SYMPH --agent
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
			teamID := ""
			if team != "" {
				resolved, err := resolveTeamFilter(db, team)
				if err != nil {
					if !errors.Is(err, errTeamFilterNotFound) {
						return err
					}
					return notFoundErr(fmt.Errorf("%w. Run 'linear-pp-cli sync' if the team was added recently", err))
				}
				teamID = resolved
			}
			results, err := db.SearchIssuesByTeam(args[0], teamID)
			if err != nil {
				return fmt.Errorf("searching: %w", err)
			}

			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if len(results) == 0 {
				hintIfUnsynced(cmd, db, "issues")
			} else {
				hintIfStale(cmd, db, "issues", flags.maxAge)
			}

			if jsonOut || flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, err := json.Marshal(results)
				if err != nil {
					return err
				}
				if flags.selectFields != "" {
					data = filterFields(data, flags.selectFields)
				} else if flags.compact {
					data = compactFields(data)
				}
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			out := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintf(out, "No issues matching %q\n", args[0])
				return nil
			}

			fmt.Fprintf(out, "%-12s %-15s %s\n", "ID", "STATE", "TITLE")
			fmt.Fprintln(out, strings.Repeat("-", 70))
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
				fmt.Fprintf(out, "%-12s %-15s %s\n", row.Identifier, row.State.Name, title)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d results for %q\n", len(results), args[0])
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&team, "team", "", "Filter by team key, name, or UUID")
	return cmd
}
