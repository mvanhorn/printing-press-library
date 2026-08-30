// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed
// This command computes results from locally stored history (resource_snapshots)
// built up as the user browses; it does not read a single upstream resource type.

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type dedupePair struct {
	A          string  `json:"a"`
	B          string  `json:"b"`
	AuthorA    string  `json:"author_a"`
	AuthorB    string  `json:"author_b"`
	Similarity float64 `json:"similarity"`
}

var wordSplitPattern = regexp.MustCompile(`[^a-z0-9]+`)

func descriptionTokens(desc string) map[string]bool {
	words := wordSplitPattern.Split(strings.ToLower(desc), -1)
	set := make(map[string]bool, len(words))
	for _, w := range words {
		if len(w) < 3 {
			continue
		}
		set[w] = true
	}
	return set
}

func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for w := range a {
		if b[w] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func newNovelDedupeCmd(flags *rootFlags) *cobra.Command {
	var flagCategory string
	var flagThreshold float64

	cmd := &cobra.Command{
		Use:         "dedupe",
		Short:       "Surface near-identical listings from different authors within a category.",
		Example:     "  mcpmarket-pp-cli dedupe --category api-development --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "dedupe")
			}
			if flagCategory == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--category is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			categorySlug := slugifyCategory(flagCategory)
			items, err := fetchItemList(ctx, c, "/categories/"+categorySlug, nil)
			if err != nil {
				return apiErr(err)
			}
			persistItems(ctx, flags, "server", items)

			type entry struct {
				name   string
				author string
				tokens map[string]bool
			}
			entries := make([]entry, 0, len(items))
			for _, item := range items {
				name, _ := item["name"].(string)
				desc, _ := item["description"].(string)
				author := ""
				if a, ok := item["author"].(map[string]any); ok {
					author, _ = a["name"].(string)
				}
				entries = append(entries, entry{name: name, author: author, tokens: descriptionTokens(desc)})
			}

			pairs := make([]dedupePair, 0)
			for i := 0; i < len(entries); i++ {
				for j := i + 1; j < len(entries); j++ {
					if entries[i].author != "" && entries[i].author == entries[j].author {
						continue
					}
					sim := jaccardSimilarity(entries[i].tokens, entries[j].tokens)
					if sim >= flagThreshold {
						pairs = append(pairs, dedupePair{
							A: entries[i].name, B: entries[j].name,
							AuthorA: entries[i].author, AuthorB: entries[j].author,
							Similarity: sim,
						})
					}
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"category":  categorySlug,
				"scanned":   len(entries),
				"threshold": flagThreshold,
				"pairs":     pairs,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagCategory, "category", "", "category slug to scan for near-duplicate listings")
	cmd.Flags().Float64Var(&flagThreshold, "threshold", 0.5, "minimum description token-overlap ratio (0-1) to flag as a likely duplicate")
	return cmd
}
