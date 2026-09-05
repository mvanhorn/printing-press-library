// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `search` runs offline full-text search over the user's synced
// design history (prompts, style, tool, dimensions) using the local SQLite FTS
// index. Run `sync` first to populate the mirror. Never calls the API.
//
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/store"

	"github.com/spf13/cobra"
)

func newNovelSearchCmd(flags *rootFlags) *cobra.Command {
	var status string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Offline full-text search over your synced design history.",
		Long: trimLong(`
Search your local mirror of Artistly designs by prompt text and metadata. This
is entirely offline (no API calls); run 'artistly-pp-cli sync' first to populate
the mirror. Filter by --status (e.g. private, processing).

To regenerate a result you find, pass its id to 'redo'.`),
		Example:     "  artistly-pp-cli search \"pirate ship\" --status private --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			query := strings.Join(args, " ")

			if dbPath == "" {
				dbPath = defaultDBPath("artistly-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: artistly-pp-cli sync --resources designs --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			// PATCH(amend-2026-06-17: search the prompt-bearing scope).
			// Two design scopes are synced: "designs" (from /designs-by-folder,
			// a lightweight shape with NO positive_prompt) and
			// "designs-fetch-personal-designs" (from /fetch-personal-designs,
			// the full record including positive_prompt). search must query the
			// latter or prompt full-text matches never resolve — the headline
			// "offline prompt search" feature returned nothing for real prompt
			// words even after a successful sync.
			const searchScope = "designs-fetch-personal-designs"
			if !hintIfUnsynced(cmd, db, searchScope) {
				hintIfStale(cmd, db, searchScope, flags.maxAge)
			}

			rows, err := db.Search(query, limit, searchScope)
			if err != nil {
				return fmt.Errorf("search: %w", err)
			}

			results := make([]Design, 0, len(rows))
			for _, raw := range rows {
				var d Design
				if json.Unmarshal(raw, &d) != nil {
					continue
				}
				if status != "" && !strings.EqualFold(d.Status, status) {
					continue
				}
				results = append(results, d)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching designs in the local mirror.")
				return nil
			}
			for _, d := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-10d %-9s %s\n", d.ID, d.Status, truncate(d.PositivePrompt, 70))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by design status (e.g. private, processing)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}
