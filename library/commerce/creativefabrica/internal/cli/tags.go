// Hand-authored transcendence: roll up tag/category frequency for a query.
// pp:data-source live
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/algolia"
	"github.com/spf13/cobra"
)

type tagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

func newNovelTagsCmd(flags *rootFlags) *cobra.Command {
	var limit, sample int
	var categoriesToo bool
	cmd := &cobra.Command{
		Use:   "tags <query>",
		Short: "Show the top tags (and categories) co-occurring with a query, to refine it",
		Long: `Roll up the tags across the top results for a query so you can discover better
refinement terms before a deep search. Counts existing tags; it does not invent
styles.`,
		Example:     strings.Trim("\n  creativefabrica-pp-cli tags \"christmas\" --agent\n  creativefabrica-pp-cli tags \"svg\" --limit 25 --categories", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a query is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newAlgoliaClient(flags)
			if sample > 100 {
				sample = 100
			}
			req := algolia.SearchRequest{IndexName: algolia.IndexRelevance, Query: args[0], HitsPerPage: sample}
			if categoriesToo {
				req.Facets = []string{"category"}
				req.MaxValuesPerFacet = 100
			}
			results, err := c.Search(ctx, req)
			if err != nil {
				return apiErr(err)
			}
			tagFreq := map[string]int{}
			catFreq := map[string]int{}
			if len(results) > 0 {
				for _, h := range results[0].Hits {
					for _, t := range h.Tags {
						if t = strings.TrimSpace(t); t != "" {
							tagFreq[t]++
						}
					}
					for _, ct := range h.Category {
						catFreq[ct]++
					}
				}
				for v, n := range results[0].Facets["category"] {
					if n > catFreq[v] {
						catFreq[v] = n
					}
				}
			}
			tags := topTags(tagFreq, limit)
			out := map[string]any{"query": args[0], "tags": tags}
			if categoriesToo {
				out["categories"] = topTags(catFreq, limit)
			}
			if flags.asJSON || flags.agent || !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printJSON(cmd, out)
			}
			rows := make([][]string, 0, len(tags))
			for _, t := range tags {
				rows = append(rows, []string{t.Tag, fmt.Sprintf("%d", t.Count)})
			}
			return flags.printTable(cmd, []string{"TAG", "COUNT"}, rows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max tags to return")
	cmd.Flags().IntVar(&sample, "sample", 100, "Result rows to sample for tag rollup (max 100)")
	cmd.Flags().BoolVar(&categoriesToo, "categories", false, "Also roll up categories")
	return cmd
}

func topTags(freq map[string]int, limit int) []tagCount {
	out := make([]tagCount, 0, len(freq))
	for t, n := range freq {
		out = append(out, tagCount{Tag: t, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
