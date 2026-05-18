// Copyright 2026 dave-morin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/techmeme/internal/store"
	"github.com/spf13/cobra"
)

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var date string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Get a day's tech news grouped by source",
		Long: `Show headlines for a specific date, grouped by source publication.
Defaults to today. Requires synced data.`,
		Example: `  # Today's digest
  techmeme-pp-cli digest

  # Digest for a specific date
  techmeme-pp-cli digest --date 2026-05-08

  # Digest as JSON
  techmeme-pp-cli digest --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			if dbPath == "" {
				dbPath = defaultDBPath("techmeme-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			items, err := db.HeadlinesForDate(date)
			if err != nil {
				return fmt.Errorf("querying headlines: %w", err)
			}

			if len(items) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No headlines for %s. Run 'sync' to populate.\n", date)
				return nil
			}

			// Group by source
			type headlineEntry struct {
				Title string `json:"title"`
				Link  string `json:"link,omitempty"`
				Time  string `json:"time,omitempty"`
			}
			type sourceGroup struct {
				Source    string          `json:"source"`
				Headlines []headlineEntry `json:"headlines"`
			}

			groups := map[string][]headlineEntry{}
			groupOrder := []string{}

			for _, item := range items {
				var obj map[string]any
				if json.Unmarshal(item, &obj) != nil {
					continue
				}

				source := "Unknown"
				if v, ok := obj["source"].(string); ok && v != "" {
					source = v
				} else if v, ok := obj["author"].(string); ok && v != "" {
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

				timeStr := ""
				if v, ok := obj["timestamp"].(string); ok {
					timeStr = v
				} else if v, ok := obj["time"].(string); ok {
					timeStr = v
				}

				if title == "" {
					continue
				}

				if _, exists := groups[source]; !exists {
					groupOrder = append(groupOrder, source)
				}
				groups[source] = append(groups[source], headlineEntry{
					Title: title,
					Link:  link,
					Time:  timeStr,
				})
			}

			// Sort groups by number of headlines (most first)
			sort.Slice(groupOrder, func(i, j int) bool {
				return len(groups[groupOrder[i]]) > len(groups[groupOrder[j]])
			})

			if flags.asJSON {
				result := make([]sourceGroup, 0, len(groupOrder))
				for _, src := range groupOrder {
					result = append(result, sourceGroup{
						Source:    src,
						Headlines: groups[src],
					})
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Digest for %s (%d headlines from %d sources)\n\n", date, len(items), len(groupOrder))

			for _, src := range groupOrder {
				headlines := groups[src]
				fmt.Fprintf(cmd.OutOrStdout(), "%s (%d)\n", bold(src), len(headlines))
				for _, h := range headlines {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", h.Title)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Date in YYYY-MM-DD format (default: today)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techmeme-pp-cli/data.db)")

	return cmd
}
