// Copyright 2026 dave-morin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/techmeme/internal/store"
	"github.com/spf13/cobra"
)

// extractSignificantWords returns significant (non-stopword, 3+ chars) words from text.
func extractSignificantWords(text string) map[string]bool {
	words := extractWords(text)
	set := map[string]bool{}
	for _, w := range words {
		set[w] = true
	}
	return set
}

// wordOverlap counts the number of shared words between two sets.
func wordOverlap(a, b map[string]bool) int {
	count := 0
	for w := range a {
		if b[w] {
			count++
		}
	}
	return count
}

func newVelocityCmd(flags *rootFlags) *cobra.Command {
	var hours int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "Find stories with multiple sources covering the same topic",
		Long: `Identify stories that are gaining velocity - multiple sources covering
the same topic in a short time window. Groups headlines by topic similarity
and shows which stories are getting the most coverage.

Requires synced data. Run 'techmeme-pp-cli sync' first.`,
		Example: `  # Check velocity in last 6 hours (default)
  techmeme-pp-cli velocity

  # Check velocity in last 24 hours
  techmeme-pp-cli velocity --hours 24

  # Output as JSON
  techmeme-pp-cli velocity --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("techmeme-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			since := time.Now().Add(-time.Duration(hours) * time.Hour)
			items, err := db.HeadlinesSince(since)
			if err != nil {
				return fmt.Errorf("querying headlines: %w", err)
			}

			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No cached headlines. Run 'techmeme-pp-cli sync' first.")
				return nil
			}

			// Parse headline data
			type parsedHeadline struct {
				title  string
				source string
				time   string
				words  map[string]bool
			}

			var headlines []parsedHeadline
			for _, item := range items {
				var obj map[string]any
				if json.Unmarshal(item, &obj) != nil {
					continue
				}
				title := ""
				if v, ok := obj["title"].(string); ok {
					title = v
				} else if v, ok := obj["headline"].(string); ok {
					title = v
				}
				if title == "" {
					continue
				}
				source := ""
				if v, ok := obj["source"].(string); ok {
					source = v
				} else if v, ok := obj["author"].(string); ok {
					source = v
				}
				timeStr := ""
				if v, ok := obj["timestamp"].(string); ok {
					timeStr = v
				} else if v, ok := obj["time"].(string); ok {
					timeStr = v
				}

				headlines = append(headlines, parsedHeadline{
					title:  title,
					source: source,
					time:   timeStr,
					words:  extractSignificantWords(title),
				})
			}

			// Group headlines by topic similarity (>2 shared significant words)
			type storyGroup struct {
				Keywords []string `json:"keywords"`
				Count    int      `json:"count"`
				First    string   `json:"first_seen"`
				Latest   string   `json:"latest"`
				Sources  []string `json:"sources"`
				Titles   []string `json:"titles"`
			}

			assigned := make([]bool, len(headlines))
			var groups []storyGroup

			for i := 0; i < len(headlines); i++ {
				if assigned[i] {
					continue
				}
				group := storyGroup{
					Count:  1,
					First:  headlines[i].time,
					Latest: headlines[i].time,
					Titles: []string{headlines[i].title},
				}
				if headlines[i].source != "" {
					group.Sources = []string{headlines[i].source}
				}
				assigned[i] = true

				// Find similar headlines
				sharedWords := map[string]int{}
				for w := range headlines[i].words {
					sharedWords[w]++
				}

				for j := i + 1; j < len(headlines); j++ {
					if assigned[j] {
						continue
					}
					overlap := wordOverlap(headlines[i].words, headlines[j].words)
					if overlap >= 2 {
						assigned[j] = true
						group.Count++
						group.Titles = append(group.Titles, headlines[j].title)
						if headlines[j].source != "" {
							// Deduplicate sources
							found := false
							for _, s := range group.Sources {
								if s == headlines[j].source {
									found = true
									break
								}
							}
							if !found {
								group.Sources = append(group.Sources, headlines[j].source)
							}
						}
						if headlines[j].time < group.First || group.First == "" {
							group.First = headlines[j].time
						}
						if headlines[j].time > group.Latest {
							group.Latest = headlines[j].time
						}
						for w := range headlines[j].words {
							if headlines[i].words[w] {
								sharedWords[w]++
							}
						}
					}
				}

				// Extract top shared keywords
				type kw struct {
					word  string
					count int
				}
				var kwList []kw
				for w, c := range sharedWords {
					kwList = append(kwList, kw{w, c})
				}
				sort.Slice(kwList, func(a, b int) bool {
					return kwList[a].count > kwList[b].count
				})
				for k := 0; k < len(kwList) && k < 4; k++ {
					group.Keywords = append(group.Keywords, kwList[k].word)
				}

				groups = append(groups, group)
			}

			// Filter groups with 2+ headlines and sort by count
			var filtered []storyGroup
			for _, g := range groups {
				if g.Count >= 2 {
					filtered = append(filtered, g)
				}
			}
			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i].Count > filtered[j].Count
			})

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), filtered, flags)
			}

			if len(filtered) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No multi-source stories found in the time window.")
				return nil
			}

			headers := []string{"TOPIC KEYWORDS", "COUNT", "FIRST SEEN", "LATEST", "SOURCES"}
			rows := make([][]string, 0, len(filtered))
			for _, g := range filtered {
				rows = append(rows, []string{
					strings.Join(g.Keywords, ", "),
					fmt.Sprintf("%d", g.Count),
					truncate(g.First, 16),
					truncate(g.Latest, 16),
					truncate(strings.Join(g.Sources, ", "), 40),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&hours, "hours", 6, "Number of hours to analyze")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techmeme-pp-cli/data.db)")

	return cmd
}
