// Copyright 2026 bernard-maltais. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/artyfacts/internal/store"
	"github.com/spf13/cobra"
)

func newOfflineSearchCmd(flags *rootFlags) *cobra.Command {
	var flagDBPath string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "search-local <query>",
		Short: "Search synced artifacts and sections offline (SQLite FTS5)",
		Long: `Search the local SQLite store using full-text search (FTS5).
Matches artifact titles/summaries and section headings/bodies.

Requires a prior 'artyfacts-pp-cli sync' to populate the store.
Zero network calls — results are instant even for large workspaces.`,
		Example:     "  artyfacts-pp-cli search-local \"deployment guide\"\n  artyfacts-pp-cli search-local \"python script\" --limit 5 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := flagDBPath
			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'sync' first to populate the local cache.", err)
			}
			defer db.Close()

			limit := flagLimit
			if limit <= 0 {
				limit = 20
			}

			results, err := db.Search(args[0], limit)
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if flags.asJSON {
				var out []map[string]any
				for _, r := range results {
					m := map[string]any{
						"artifact_id":    r.ArtifactID,
						"artifact_title": r.ArtifactTitle,
						"snippet":        r.Snippet,
					}
					if r.SectionID != "" {
						m["section_id"] = r.SectionID
						m["section_heading"] = r.SectionHeading
					}
					out = append(out, m)
				}
				b, _ := json.Marshal(out)
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No results found. Try 'sync' to refresh the local cache.")
				return nil
			}

			for _, r := range results {
				if r.SectionID != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s › %s\n  %s\n\n",
						r.ArtifactID, r.ArtifactTitle, r.SectionHeading, r.Snippet)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n  %s\n\n",
						r.ArtifactID, r.ArtifactTitle, r.Snippet)
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Max results to return")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "Custom SQLite database path")

	return cmd
}
