// Copyright 2026 usc-tk. Licensed under Apache-2.0. See LICENSE.

// `magazine search "<query>"` runs an FTS5 MATCH against the local
// magazine_fts table, which mirrors magazine_articles. Articles land in
// the table via `magazine list` / `magazine get` / `sync --resources
// magazine`. When the table is empty, the command surfaces a helpful
// run-sync-first error rather than returning no results.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newMagazineSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search over locally-mirrored Kickstarter Magazine articles",
		Long: `FTS5 search against locally-mirrored magazine articles. Each hit returns
slug, title, a 240-char snippet from the body, published_at (unix seconds),
and the canonical URL — ordered by FTS5 rank.

Requires articles to have been synced first. Run any of:
  kickstarter-pp-cli sync --resources magazine
  kickstarter-pp-cli magazine list
  kickstarter-pp-cli magazine get --slug <slug>`,
		Example: `  kickstarter-pp-cli magazine search "ai automation"
  kickstarter-pp-cli magazine search "creator spotlight" --limit 25 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStoreForRead(cmd.Context(), "kickstarter-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("magazine search requires `kickstarter-pp-cli sync --resources magazine` first."))
			}
			defer db.Close()
			if err := db.EnsureV2Schema(cmd.Context()); err != nil {
				return fmt.Errorf("ensuring v2 schema: %w", err)
			}
			results, err := db.MagazineSearch(query, limit)
			if err != nil {
				return fmt.Errorf("magazine fts: %w", err)
			}
			if results == nil {
				return notFoundErr(fmt.Errorf("magazine search requires `kickstarter-pp-cli sync --resources magazine` first."))
			}
			env := map[string]any{
				"query":   query,
				"matches": results,
				"count":   len(results),
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Max FTS hits to return")
	return cmd
}
