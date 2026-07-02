// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed command: latest Points Guy articles via the RSS feed.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/tpg"
)

func newLatestCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var category string

	cmd := &cobra.Command{
		Use:   "latest",
		Short: "Show the latest articles and news from The Points Guy",
		Long: strings.TrimSpace(`
List the most recent articles published by The Points Guy, from its RSS feed.
Filter with --category (matches the article's RSS category, e.g. news, deals,
credit-cards). For a time window across categories, use 'since'.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli latest --limit 10
  thepointsguy-pp-cli latest --category news --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch latest articles")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			items, err := fetchFeed(cmd, flags, ctx)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			filtered := filterFeed(items, category, limit)
			return emitFeed(cmd, flags, filtered)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum articles to return")
	cmd.Flags().StringVar(&category, "category", "", "Filter by RSS category, e.g. news, deals, credit-cards")
	return cmd
}

// fetchFeed pulls the RSS feed live, or reads synced articles when
// --data-source local (or when live fetch fails).
func fetchFeed(cmd *cobra.Command, flags *rootFlags, ctx context.Context) ([]tpg.FeedItem, error) {
	if flags.dataSource != "local" {
		c := newTPGClient(flags)
		items, err := c.Latest(ctx)
		if err == nil {
			return items, nil
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "live feed unavailable (%v); trying local store\n", err)
	}
	st, err := openTPGStore()
	if err != nil {
		return nil, err
	}
	defer st.Close()
	raws, err := st.List(rtArticles, 500)
	if err != nil {
		return nil, err
	}
	items := make([]tpg.FeedItem, 0, len(raws))
	for _, r := range raws {
		var it tpg.FeedItem
		if json.Unmarshal(r, &it) == nil && it.Title != "" {
			items = append(items, it)
		}
	}
	return items, nil
}

func filterFeed(items []tpg.FeedItem, category string, limit int) []tpg.FeedItem {
	out := make([]tpg.FeedItem, 0, len(items))
	for _, it := range items {
		if category != "" && !strings.EqualFold(it.Category, category) &&
			!strings.Contains(strings.ToLower(it.Link), "/"+strings.ToLower(category)+"/") {
			continue
		}
		out = append(out, it)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func emitFeed(cmd *cobra.Command, flags *rootFlags, items []tpg.FeedItem) error {
	if flags.asJSON || flags.agent {
		return emitJSON(cmd, flags, struct {
			Count int            `json:"count"`
			Items []tpg.FeedItem `json:"items"`
		}{len(items), items})
	}
	w := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintf(w, "DATE\tTITLE\tURL\n")
	for _, it := range items {
		date := it.PubDate
		if !it.Published.IsZero() {
			date = it.Published.Format("2006-01-02")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", date, truncRunes(it.Title, 60), it.Link)
	}
	return w.Flush()
}
