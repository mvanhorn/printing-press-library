package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newLibrarySimilarCmd(flags *rootFlags) *cobra.Command {
	var threshold float64

	cmd := &cobra.Command{
		Use:   "similar",
		Short: "Find books similar to a given book by shared authors, tags, and series",
		Long: `Analyzes the library to find books similar to a given book based on:
  - Shared authors (highest weight)
  - Shared tags/subjects
  - Same series
  - Same publisher

Returns ranked results with similarity score and match reasons.`,
		Example:     "  calibre-ebook-pp-cli library similar 42 --threshold 0.3",
		Annotations: map[string]string{"pp:endpoint": "library.similar", "pp:method": "GET", "pp:path": "/library/similar", "mcp:read-only": "true", "pp:insight": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/books", map[string]string{
				"fields": "id,title,authors,formats,series,series_index,tags,publisher",
				"limit":  "0",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var books []map[string]any
			if err := json.Unmarshal(data, &books); err != nil {
				return fmt.Errorf("parsing books: %w", err)
			}

			var targetID float64
			fmt.Sscanf(args[0], "%f", &targetID)

			var target map[string]any
			for _, b := range books {
				if id, ok := b["id"].(float64); ok && id == targetID {
					target = b
					break
				}
			}
			if target == nil {
				return fmt.Errorf("book ID %s not found", args[0])
			}

			targetAuthors := toLowerSet(splitField(target["authors"]))
			targetTags := toLowerSet(splitField(target["tags"]))
			targetSeries := strings.ToLower(fmt.Sprintf("%v", target["series"]))
			targetPublisher := strings.ToLower(fmt.Sprintf("%v", target["publisher"]))

			type similarBook struct {
				ID       int      `json:"id"`
				Title    string   `json:"title"`
				Authors  string   `json:"authors"`
				Score    float64  `json:"score"`
				Reasons  []string `json:"reasons"`
			}

			var results []similarBook
			for _, b := range books {
				id, ok := b["id"].(float64)
				if !ok || id == targetID {
					continue
				}

				score := 0.0
				var reasons []string

				authors := toLowerSet(splitField(b["authors"]))
				shared := intersection(targetAuthors, authors)
				if len(shared) > 0 {
					authorScore := float64(len(shared)) / float64(max(len(targetAuthors), 1)) * 40
					score += authorScore
					reasons = append(reasons, fmt.Sprintf("shared author(s): %s", strings.Join(shared, ", ")))
				}

				tags := toLowerSet(splitField(b["tags"]))
				sharedTags := intersection(targetTags, tags)
				if len(sharedTags) > 0 {
					tagScore := float64(len(sharedTags)) / float64(max(len(targetTags), 1)) * 25
					score += tagScore
					reasons = append(reasons, fmt.Sprintf("%d shared tags", len(sharedTags)))
				}

				series := strings.ToLower(fmt.Sprintf("%v", b["series"]))
				if series != "" && series == targetSeries && series != "<nil>" {
					score += 25
					reasons = append(reasons, "same series")
				}

				pub := strings.ToLower(fmt.Sprintf("%v", b["publisher"]))
				if pub != "" && pub == targetPublisher && pub != "<nil>" {
					score += 10
					reasons = append(reasons, "same publisher")
				}

				if score >= threshold && len(reasons) > 0 {
					results = append(results, similarBook{
						ID:      int(id),
						Title:   fmt.Sprintf("%v", b["title"]),
						Authors: fmt.Sprintf("%v", b["authors"]),
						Score:   score,
						Reasons: reasons,
					})
				}
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].Score > results[j].Score
			})

			result := map[string]any{
				"target_book":    fmt.Sprintf("%v", target["title"]),
				"target_id":      int(targetID),
				"similar_count":  len(results),
				"similar_books":  results,
				"threshold":      threshold,
				"total_analyzed": len(books) - 1,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				wrapped, _ := wrapWithProvenance(mustMarshal(result), DataProvenance{Source: "live"})
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n  Books similar to: %s\n", bold(fmt.Sprintf("%v", target["title"])))
			fmt.Fprintf(cmd.OutOrStdout(), "  Found %d similar books (threshold: %.1f)\n\n", len(results), threshold)
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "    [%.0f] %s by %s\n", r.Score, r.Title, truncate(r.Authors, 40))
				for _, reason := range r.Reasons {
					fmt.Fprintf(cmd.OutOrStdout(), "          %s\n", reason)
				}
			}
			return nil
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 20, "Minimum similarity score to include")
	return cmd
}

func splitField(v any) []string {
	raw := fmt.Sprintf("%v", v)
	if raw == "" || raw == "<nil>" || raw == "[]" {
		return nil
	}
	if raw[0] == '[' && raw[len(raw)-1] == ']' {
		raw = raw[1 : len(raw)-1]
	}
	parts := strings.Split(raw, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func toLowerSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[strings.ToLower(strings.TrimSpace(item))] = true
	}
	return set
}

func intersection(a, b map[string]bool) []string {
	var shared []string
	for k := range a {
		if b[k] {
			shared = append(shared, k)
		}
	}
	return shared
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
