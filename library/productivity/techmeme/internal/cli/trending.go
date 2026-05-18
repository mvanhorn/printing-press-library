// Copyright 2026 dave-morin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/productivity/techmeme/internal/store"
	"github.com/spf13/cobra"
)

// stopwords is a set of common English words to filter from trending analysis.
var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "in": true, "on": true, "at": true,
	"to": true, "for": true, "of": true, "and": true, "or": true,
	"but": true, "its": true, "it": true, "be": true, "been": true,
	"being": true, "have": true, "has": true, "had": true, "do": true,
	"does": true, "did": true, "will": true, "would": true, "could": true,
	"should": true, "may": true, "might": true, "can": true, "with": true,
	"from": true, "by": true, "about": true, "into": true, "through": true,
	"after": true, "before": true, "between": true, "under": true,
	"above": true, "up": true, "down": true, "out": true, "off": true,
	"over": true, "then": true, "than": true, "that": true, "this": true,
	"these": true, "those": true, "not": true, "no": true, "nor": true,
	"only": true, "own": true, "same": true, "so": true, "very": true,
	"just": true, "how": true, "all": true, "each": true, "every": true,
	"both": true, "few": true, "more": true, "most": true, "other": true,
	"some": true, "such": true, "too": true, "what": true, "which": true,
	"who": true, "whom": true, "why": true, "where": true, "when": true,
	"new": true, "says": true, "said": true, "also": true, "now": true,
	"get": true, "got": true, "here": true, "there": true, "they": true,
	"their": true, "them": true, "his": true, "her": true, "him": true,
	"she": true, "he": true, "you": true, "your": true, "our": true,
	"we": true, "us": true, "my": true, "me": true, "i": true,
}

// extractWords splits text into lowercase words, filtering stopwords and short words.
func extractWords(text string) []string {
	var words []string
	// Split on non-letter/non-digit boundaries
	var current strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(unicode.ToLower(r))
		} else {
			if current.Len() >= 3 {
				word := current.String()
				if !stopwords[word] {
					words = append(words, word)
				}
			}
			current.Reset()
		}
	}
	if current.Len() >= 3 {
		word := current.String()
		if !stopwords[word] {
			words = append(words, word)
		}
	}
	return words
}

func newTrendingCmd(flags *rootFlags) *cobra.Command {
	var hours int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "trending",
		Short: "Show trending topics from recent headlines",
		Long: `Analyze cached headlines to find the most mentioned topics.
Uses frequency analysis on headline text, filtering common stopwords.

Requires synced data. Run 'techmeme-pp-cli sync' first.`,
		Example: `  # Show trending topics from last 6 hours (default)
  techmeme-pp-cli trending

  # Show trending topics from last 24 hours
  techmeme-pp-cli trending --hours 24

  # Show trending as JSON
  techmeme-pp-cli trending --json`,
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

			// Count word frequencies and track example headlines
			wordCounts := map[string]int{}
			wordExamples := map[string]string{}
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
				words := extractWords(title)
				seen := map[string]bool{}
				for _, w := range words {
					if !seen[w] {
						wordCounts[w]++
						if _, exists := wordExamples[w]; !exists {
							wordExamples[w] = title
						}
						seen[w] = true
					}
				}
			}

			type trendEntry struct {
				Rank     int    `json:"rank"`
				Topic    string `json:"topic"`
				Mentions int    `json:"mentions"`
				Example  string `json:"example"`
			}

			// Sort by count
			type wordCount struct {
				word  string
				count int
			}
			var sorted []wordCount
			for w, c := range wordCounts {
				if c >= 2 {
					sorted = append(sorted, wordCount{w, c})
				}
			}
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].count > sorted[j].count
			})

			// Limit to top 20
			if len(sorted) > 20 {
				sorted = sorted[:20]
			}

			trends := make([]trendEntry, 0, len(sorted))
			for i, wc := range sorted {
				trends = append(trends, trendEntry{
					Rank:     i + 1,
					Topic:    wc.word,
					Mentions: wc.count,
					Example:  wordExamples[wc.word],
				})
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), trends, flags)
			}

			if len(trends) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Not enough data for trending analysis. Sync more data first.")
				return nil
			}

			headers := []string{"RANK", "TOPIC", "MENTIONS", "EXAMPLE HEADLINE"}
			rows := make([][]string, 0, len(trends))
			for _, t := range trends {
				rows = append(rows, []string{
					fmt.Sprintf("%d", t.Rank),
					t.Topic,
					fmt.Sprintf("%d", t.Mentions),
					truncate(t.Example, 55),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&hours, "hours", 6, "Number of hours to analyze")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techmeme-pp-cli/data.db)")

	return cmd
}
