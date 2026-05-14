// Copyright 2026 alex-puckhaber. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/ghl/internal/store"

	"github.com/spf13/cobra"
)

// newTagsCmd is a hand-built top-level grouping for novel tag commands.
// The endpoint-mirror `locations tags` and `contacts tags` already cover
// CRUD; this group adds aggregations the API doesn't expose.
func newTagsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tags",
		Short:       "Tag analytics across the synced store",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newTagsStatsCmd(flags))
	return cmd
}

func newTagsStatsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var minCount int

	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "List every tag in the location with contact-count and kill-switch flag",
		Long:        "Aggregates the `tags` array on each synced contact and reports per-tag contact count plus whether the tag is one of the kill-switch values (`ai off`, `human handover`). Pure local SQL; no API call.",
		Example:     "  ghl-pp-cli tags stats --json\n  ghl-pp-cli tags stats --min-count 5",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("ghl-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ghl-pp-cli sync' first", err)
			}
			defer db.Close()

			rows, err := db.Query(`SELECT data FROM "contacts"`)
			if err != nil {
				return fmt.Errorf("querying contacts: %w", err)
			}
			defer rows.Close()

			counts := map[string]int{}
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(data, &obj); err != nil {
					continue
				}
				rawTags, _ := obj["tags"].([]any)
				for _, t := range rawTags {
					if s, ok := t.(string); ok {
						key := strings.TrimSpace(s)
						if key != "" {
							counts[key]++
						}
					}
				}
			}

			type tagStat struct {
				Tag        string `json:"tag"`
				Count      int    `json:"count"`
				Killswitch bool   `json:"killswitch"`
			}
			stats := make([]tagStat, 0, len(counts))
			for tag, n := range counts {
				if n < minCount {
					continue
				}
				ks := killswitchTagFromString(tag)
				stats = append(stats, tagStat{Tag: tag, Count: n, Killswitch: ks != ""})
			}
			sort.Slice(stats, func(i, j int) bool { return stats[i].Count > stats[j].Count })

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"total_tags": len(stats), "tags": stats}, flags)
			}
			if len(stats) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No tags found in the local store.")
				fmt.Fprintln(cmd.OutOrStdout(), "Hint: run 'ghl-pp-cli sync --full' first.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-7s  %-8s  %s\n", "COUNT", "KS?", "TAG")
			fmt.Fprintf(cmd.OutOrStdout(), "%-7s  %-8s  %s\n", strings.Repeat("-", 7), strings.Repeat("-", 8), strings.Repeat("-", 30))
			for _, t := range stats {
				ksMarker := "  "
				if t.Killswitch {
					ksMarker = "**"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-7d  %-8s  %s\n", t.Count, ksMarker, t.Tag)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ghl-pp-cli/data.db)")
	cmd.Flags().IntVar(&minCount, "min-count", 1, "Only include tags applied to at least N contacts")
	return cmd
}

// killswitchTagFromString normalizes a tag string and returns the canonical
// kill-switch label ("ai off" or "human handover") if it matches.
func killswitchTagFromString(s string) string {
	ls := strings.ToLower(strings.TrimSpace(s))
	for _, m := range tagAIOff {
		if ls == m {
			return "ai off"
		}
	}
	for _, m := range tagHandover {
		if ls == m {
			return "human handover"
		}
	}
	return ""
}
