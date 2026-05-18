// Copyright 2026 dave-morin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/techmeme/internal/store"
	"github.com/spf13/cobra"
)

func newAuthorCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "author <name>",
		Short: "Find headlines by a specific journalist or author",
		Long: `Search cached headlines by author name (case-insensitive).
Shows all Techmeme headlines attributed to the author.

Requires synced data. Run 'techmeme-pp-cli sync' first.`,
		Example: `  # Find headlines by an author
  techmeme-pp-cli author "Kara Swisher"

  # Author search as JSON
  techmeme-pp-cli author "Mark Gurman" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			name := strings.Join(args, " ")

			if dbPath == "" {
				dbPath = defaultDBPath("techmeme-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			items, err := db.SearchByAuthor(name)
			if err != nil {
				return fmt.Errorf("searching by author: %w", err)
			}

			if len(items) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No headlines found for author %q. Run 'sync' to populate data.\n", name)
				return nil
			}

			type authorResult struct {
				Date     string `json:"date"`
				Source   string `json:"source"`
				Headline string `json:"headline"`
				Link     string `json:"link,omitempty"`
			}

			var results []authorResult
			for _, item := range items {
				var obj map[string]any
				if json.Unmarshal(item, &obj) != nil {
					continue
				}

				dateStr := ""
				if v, ok := obj["timestamp"].(string); ok {
					dateStr = v
				} else if v, ok := obj["pubDate"].(string); ok {
					dateStr = v
				} else if v, ok := obj["time"].(string); ok {
					dateStr = v
				}

				source := ""
				if v, ok := obj["source"].(string); ok {
					source = v
				} else if v, ok := obj["author"].(string); ok {
					source = v
				}

				title := ""
				if v, ok := obj["title"].(string); ok {
					title = v
				} else if v, ok := obj["headline"].(string); ok {
					title = v
				}

				link := ""
				if v, ok := obj["link"].(string); ok {
					link = v
				} else if v, ok := obj["url"].(string); ok {
					link = v
				}

				results = append(results, authorResult{
					Date:     dateStr,
					Source:   source,
					Headline: title,
					Link:     link,
				})
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "%d headlines by %q\n", len(results), name)

			headers := []string{"DATE", "SOURCE", "HEADLINE"}
			rows := make([][]string, 0, len(results))
			for _, r := range results {
				rows = append(rows, []string{
					truncate(r.Date, 20),
					truncate(r.Source, 25),
					truncate(r.Headline, 60),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techmeme-pp-cli/data.db)")

	return cmd
}
