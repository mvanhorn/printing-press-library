// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/store"
)

// mergeSearchResults runs an FTS5 search scoped to each of resourceTypes in
// turn and merges the results, deduping by raw JSON so a row that somehow
// matches under multiple types isn't shown twice.
func mergeSearchResults(db *store.Store, query string, limit int, resourceTypes ...string) ([]json.RawMessage, error) {
	seen := map[string]bool{}
	out := make([]json.RawMessage, 0)
	for _, t := range resourceTypes {
		rows, err := db.Search(query, limit, t)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			key := string(r)
			if !seen[key] {
				seen[key] = true
				out = append(out, r)
			}
		}
	}
	return out, nil
}

// pp:data-source local
func newNovelTrendsHistorySearchCmd(flags *rootFlags) *cobra.Command {
	var flagTable string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Full-text search across every related-term and trending-topic result you've ever synced, offline.",
		Example:     "  google-trends-pp-cli trends history search \"electric vehicle\" --table related --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("search query argument is required"))
			}
			query := strings.Join(args, " ")

			switch flagTable {
			case "", "all", "related", "trending":
			default:
				return usageErr(fmt.Errorf("--table must be one of: related, trending (got %q)", flagTable))
			}

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "google-trends-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return noLocalMirrorHint(cmd, flags, "google-trends-pp-cli trends related <keyword>' or 'google-trends-pp-cli trends trending", make([]json.RawMessage, 0))
			}
			defer db.Close()

			var results []json.RawMessage
			switch flagTable {
			case "related":
				results, err = db.Search(query, flagLimit, "gt_related_term")
			case "trending":
				results, err = db.Search(query, flagLimit, "gt_trending_topic")
			default:
				results, err = mergeSearchResults(db, query, flagLimit, "gt_related_term", "gt_trending_topic")
			}
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}
			if results == nil {
				results = make([]json.RawMessage, 0)
			}

			return printLocalResult(cmd, flags, results)
		},
	}
	cmd.Flags().StringVar(&flagTable, "table", "", "Restrict search to one table: related, trending (default: both)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum results to return")
	return cmd
}
