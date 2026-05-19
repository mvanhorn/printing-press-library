// Copyright 2026 conduyt. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
	"github.com/spf13/cobra"
)

func newInsightsContactTrendsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "contact-trends",
		Short:       "Analyze contact creation and tagging patterns over time",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  conduyt-crm-pp-cli insights contact-trends
  conduyt-crm-pp-cli insights contact-trends --limit 30 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("conduyt-crm-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'conduyt-crm-pp-cli sync' first.", err)
			}
			defer db.Close()

			items, err := db.List("contacts", 0)
			if err != nil {
				return fmt.Errorf("listing contacts: %w", err)
			}

			type dayBucket struct {
				Date  string `json:"date"`
				Count int    `json:"count"`
			}

			byDate := make(map[string]int)
			bySource := make(map[string]int)
			total := len(items)

			for _, raw := range items {
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				if created, ok := obj["createdAt"].(string); ok && len(created) >= 10 {
					byDate[created[:10]]++
				}
				if src, ok := obj["source"].(string); ok && src != "" {
					bySource[src]++
				}
			}

			var dates []dayBucket
			for d, c := range byDate {
				dates = append(dates, dayBucket{d, c})
			}
			sort.Slice(dates, func(i, j int) bool { return dates[i].Date > dates[j].Date })
			if limit > 0 && len(dates) > limit {
				dates = dates[:limit]
			}

			type sourceCount struct {
				Source string `json:"source"`
				Count  int    `json:"count"`
				Pct    string `json:"percentage"`
			}
			var sources []sourceCount
			for s, c := range bySource {
				pct := fmt.Sprintf("%.1f%%", float64(c)*100/float64(total))
				sources = append(sources, sourceCount{s, c, pct})
			}
			sort.Slice(sources, func(i, j int) bool { return sources[i].Count > sources[j].Count })

			result := map[string]any{
				"total_contacts":    total,
				"creation_by_date":  dates,
				"sources":           sources,
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Printf("Total contacts: %d\n\n", total)
			fmt.Println("Creation by Date (recent first)")
			fmt.Println("Date\t\tCount")
			fmt.Println("----\t\t-----")
			for _, d := range dates {
				fmt.Printf("%s\t%d\n", d.Date, d.Count)
			}
			fmt.Println("\nBy Source")
			fmt.Println("Source\t\tCount\t%")
			fmt.Println("------\t\t-----\t-")
			for _, s := range sources {
				fmt.Printf("%s\t%d\t%s\n", s.Source, s.Count, s.Pct)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 14, "Max date buckets to show")
	return cmd
}
