// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: persist a named search for tracking with 'new'.
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

func newNovelSaveCmd(flags *rootFlags) *cobra.Command {
	var country, state, city, sort, dbFlag string

	cmd := &cobra.Command{
		Use:   "save <name> [query]",
		Short: "Persist a named search (query + filters) to track with 'new'",
		Long: strings.Trim(`
Persist a named search — its query and location filters — into the local store
so 'new' can report reqs unseen since your last check.

Use 'save' to create or update a persisted named query; use 'searches' to list
or delete them, and 'find' for a one-off query that stores nothing.`, "\n"),
		Example: strings.Trim(`
  amazon-jobs-pp-cli save sde-seattle "software engineer" --city Seattle --country USA
  amazon-jobs-pp-cli save aws-remote "solutions architect" --country USA`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would save a named search")
				return nil
			}
			if err := guardDataSource(flags, "local"); err != nil {
				return err
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a saved-search name is required"))
			}
			name := strings.TrimSpace(args[0])
			if name == "" {
				return usageErr(fmt.Errorf("saved-search name cannot be empty"))
			}
			query := ""
			if len(args) > 1 {
				query = strings.TrimSpace(strings.Join(args[1:], " "))
			}
			if query == "" && country == "" && state == "" && city == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide a query and/or at least one of --country/--state/--city"))
			}
			if sort == "" {
				sort = "recent"
			}
			dbPath := resolveDBPath(dbFlag)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			ss := SavedSearch{Name: name, Query: query, Country: country, State: state, City: city, Sort: sort}
			if err := upsertSavedSearch(ctx, db, ss, nowISO()); err != nil {
				return err
			}
			// Read it back so the response reflects stored state.
			stored, err := getSavedSearch(ctx, db, name)
			if err != nil {
				return err
			}

			return emitResult(cmd, flags, stored, func(w io.Writer) {
				fmt.Fprintf(w, "saved search %q\n", name)
				if query != "" {
					fmt.Fprintf(w, "  query: %s\n", query)
				}
				loc := strings.TrimSpace(strings.Join(nonEmpty(city, state, country), ", "))
				if loc != "" {
					fmt.Fprintf(w, "  location: %s\n", loc)
				}
				fmt.Fprintf(w, "  sort: %s\n", sort)
				fmt.Fprintf(w, "\ntrack it with: amazon-jobs-pp-cli new %s\n", name)
			})
		},
	}

	cmd.Flags().StringVar(&country, "country", "", "Filter by ISO alpha-3 country code (USA, GBR, IND, ...)")
	cmd.Flags().StringVar(&state, "state", "", "Filter by full state/region name")
	cmd.Flags().StringVar(&city, "city", "", "Filter by city name")
	cmd.Flags().StringVar(&sort, "sort", "recent", "Sort order: recent or relevant")
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local SQLite store path (default: platform data dir)")

	return cmd
}

// nonEmpty returns the non-empty strings from the arguments, preserving order.
func nonEmpty(vals ...string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
