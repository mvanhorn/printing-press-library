// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/gtrends"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/store"
)

// collectRelatedTerms fetches a RELATED_QUERIES/RELATED_TOPICS widget's top
// and rising terms and, if db is non-nil, persists each as a gt_related_term
// row. facetLabel is "query" or "topic" per the spec's stored field. category
// is part of the cache identity (not just geo/timeframe): Google's Explore
// request includes it, and a different category can return different related
// terms for the same keyword/geo/timeframe -- omitting it from the ID lets a
// later sync with a different category silently overwrite a differently
// scoped snapshot under the same row.
func collectRelatedTerms(ctx context.Context, c *client.Client, db *store.Store, cmd *cobra.Command, keyword string, widget gtrends.Widget, facetLabel, geo, timeframe string, category int, syncedAt string) ([]gtRelatedTermRecord, error) {
	top, rising, err := gtrends.RelatedSearches(ctx, c, widget)
	if err != nil {
		return nil, err
	}
	categoryStr := strconv.Itoa(category)
	out := make([]gtRelatedTermRecord, 0, len(top)+len(rising))
	add := func(kind string, terms []gtrends.RelatedTerm) {
		for _, t := range terms {
			rec := gtRelatedTermRecord{
				Keyword: keyword, Term: t.Query, Kind: kind, Facet: facetLabel,
				Value: t.Value, IsBreakout: t.IsBreakout, Geo: geo, Timeframe: timeframe, Category: category, SyncedAt: syncedAt,
			}
			out = append(out, rec)
			if db != nil {
				body, _ := json.Marshal(rec)
				id := sha256ID(keyword, rec.Term, kind, facetLabel, geo, timeframe, categoryStr)
				if err := db.Upsert("gt_related_term", id, body); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to cache related term %q: %v\n", rec.Term, err)
				}
			}
		}
	}
	add("top", top)
	add("rising", rising)
	return out, nil
}

// pp:data-source live
func newTrendsRelatedCmd(flags *rootFlags) *cobra.Command {
	var flagFacet string
	var flagGeo string
	var flagTimeframe string
	var flagCategory int

	cmd := &cobra.Command{
		Use:         "related <keyword>",
		Short:       "Related queries and/or topics for a keyword (top + rising).",
		Example:     "  google-trends-pp-cli trends related coffee --facet queries --geo US --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("keyword argument is required"))
			}
			keyword := args[0]
			facetFilter := flagFacet
			if facetFilter == "" {
				facetFilter = "both"
			}
			switch facetFilter {
			case "queries", "topics", "both":
			default:
				return usageErr(fmt.Errorf("--facet must be one of: queries, topics, both (got %q)", flagFacet))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			explore, err := gtrends.Explore(ctx, c, []string{keyword}, []string{flagGeo}, flagTimeframe, flagCategory, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}

			syncedAt := time.Now().UTC().Format(time.RFC3339)
			db, dbErr := store.OpenWithContext(ctx, defaultDBPath("google-trends-pp-cli"))
			if dbErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not open local store to cache related terms: %v\n", dbErr)
			} else {
				defer db.Close()
			}

			out := make([]gtRelatedTermRecord, 0)
			if facetFilter == "queries" || facetFilter == "both" {
				if widget, ok := gtrends.FindWidget(explore.Widgets, "RELATED_QUERIES"); ok {
					rows, err := collectRelatedTerms(ctx, c, db, cmd, keyword, widget, "query", flagGeo, flagTimeframe, flagCategory, syncedAt)
					if err != nil {
						return classifyAPIError(err, flags)
					}
					out = append(out, rows...)
				}
			}
			if facetFilter == "topics" || facetFilter == "both" {
				if widget, ok := gtrends.FindWidget(explore.Widgets, "RELATED_TOPICS"); ok {
					rows, err := collectRelatedTerms(ctx, c, db, cmd, keyword, widget, "topic", flagGeo, flagTimeframe, flagCategory, syncedAt)
					if err != nil {
						return classifyAPIError(err, flags)
					}
					out = append(out, rows...)
				}
			}

			if len(out) == 0 {
				return notFoundErr(fmt.Errorf("no related terms for %q", keyword))
			}
			return printLiveResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagFacet, "facet", "both", "Which related data to fetch: queries, topics, or both")
	cmd.Flags().StringVar(&flagGeo, "geo", "", "Geo code (e.g. US); default: worldwide")
	cmd.Flags().StringVar(&flagTimeframe, "timeframe", "today 12-m", "Time range, e.g. 'today 12-m'")
	cmd.Flags().IntVar(&flagCategory, "category", 0, "Google Trends category ID")
	return cmd
}
