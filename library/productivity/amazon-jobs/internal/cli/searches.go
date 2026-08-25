// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: list and delete saved searches.
// Preserved across `generate --force`.
// pp:data-source local

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/internal/store"
)

func newSearchesCmd(flags *rootFlags) *cobra.Command {
	var deleteName, dbFlag string

	cmd := &cobra.Command{
		Use:   "searches",
		Short: "List (or delete) saved searches",
		Long: strings.Trim(`
List the saved searches created with 'save', or remove one with --delete.
Track a saved search's new reqs with 'new <name>'.`, "\n"),
		Example: strings.Trim(`
  amazon-jobs-pp-cli searches
  amazon-jobs-pp-cli searches --agent
  amazon-jobs-pp-cli searches --delete sde-seattle`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list saved searches")
				return nil
			}
			if err := guardDataSource(flags, "local"); err != nil {
				return err
			}
			dbPath := resolveDBPath(dbFlag)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			if strings.TrimSpace(deleteName) != "" {
				removed, derr := deleteSavedSearch(ctx, db, deleteName)
				if derr != nil {
					return derr
				}
				if !removed {
					return usageErr(fmt.Errorf("no saved search named %q", deleteName))
				}
				result := map[string]any{"deleted": deleteName}
				return emitResult(cmd, flags, result, func(w io.Writer) {
					fmt.Fprintf(w, "deleted saved search %q\n", deleteName)
				})
			}

			list, err := listSavedSearches(ctx, db)
			if err != nil {
				return err
			}

			return emitResult(cmd, flags, list, func(w io.Writer) {
				if len(list) == 0 {
					fmt.Fprintln(w, "no saved searches yet (create one with: amazon-jobs-pp-cli save <name> <query>)")
					return
				}
				for _, s := range list {
					loc := strings.Join(nonEmpty(s.City, s.State, s.Country), ", ")
					fmt.Fprintf(w, "%-20s  query=%q  %s\n", s.Name, s.Query, loc)
					if s.LastSynced != "" {
						fmt.Fprintf(w, "  tracking %d job(s), last checked %s\n", len(s.LastSeen), s.LastSynced)
					}
				}
			})
		},
	}

	cmd.Flags().StringVar(&deleteName, "delete", "", "Delete the named saved search")
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local SQLite store path (default: platform data dir)")

	return cmd
}
