// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/gtrends"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/store"
)

// pp:data-source live
func newTrendsInterestCmd(flags *rootFlags) *cobra.Command {
	var flagCompare string
	var flagGeo string
	var flagTimeframe string
	var flagCategory int
	var flagProperty string

	cmd := &cobra.Command{
		Use:         "interest <keyword>",
		Short:       "Interest-over-time for a keyword, optionally compared against up to 4 others.",
		Example:     "  google-trends-pp-cli trends interest coffee --compare tea,latte --geo US --agent",
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
			keywords := []string{keyword}
			for _, kw := range strings.Split(flagCompare, ",") {
				kw = strings.TrimSpace(kw)
				if kw != "" {
					keywords = append(keywords, kw)
				}
			}
			if len(keywords) > 5 {
				return usageErr(fmt.Errorf("at most 5 keywords total (1 + up to 4 in --compare); got %d", len(keywords)))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			explore, err := gtrends.Explore(ctx, c, keywords, []string{flagGeo}, flagTimeframe, flagCategory, flagProperty)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			widget, ok := gtrends.FindWidget(explore.Widgets, "TIMESERIES")
			if !ok {
				return apiErr(fmt.Errorf("explore response did not include a TIMESERIES widget"))
			}
			points, err := gtrends.InterestOverTime(ctx, c, widget)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			syncedAt := time.Now().UTC().Format(time.RFC3339)
			out := make([]gtInterestPointRecord, 0, len(points)*len(keywords))

			db, dbErr := store.OpenWithContext(ctx, defaultDBPath("google-trends-pp-cli"))
			if dbErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not open local store to cache interest data: %v\n", dbErr)
			} else {
				defer db.Close()
			}

			// Google Trends normalizes interest values relative to the full
			// comparison set, and category/property independently change which
			// data is returned -- so all three (not just keyword/geo/timeframe/
			// date) must be part of the cache identity, or a later sync with a
			// different category, property, or compare set silently overwrites a
			// differently-scoped series under the same ID. compareScope is
			// sorted so the same peer set in a different --compare order still
			// maps to the same cache identity.
			sortedKeywords := append([]string(nil), keywords...)
			sort.Strings(sortedKeywords)
			compareScope := strings.Join(sortedKeywords, ",")
			categoryStr := strconv.Itoa(flagCategory)

			for _, p := range points {
				date := p.Time.UTC().Format("2006-01-02")
				for i, kw := range keywords {
					if i >= len(p.Values) {
						continue
					}
					row := gtInterestPointRecord{Keyword: kw, Geo: flagGeo, Timeframe: flagTimeframe, Date: date, Value: p.Values[i], Category: flagCategory, Property: flagProperty, CompareScope: compareScope, SyncedAt: syncedAt}
					out = append(out, row)
					if db != nil {
						body, _ := json.Marshal(row)
						id := sha256ID(kw, flagGeo, flagTimeframe, categoryStr, flagProperty, compareScope, date)
						if err := db.Upsert("gt_interest_point", id, body); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to cache interest point for %q: %v\n", kw, err)
						}
					}
				}
			}
			if db != nil {
				for _, kw := range keywords {
					q := gtKeywordQueryRecord{Keyword: kw, Geo: flagGeo, Timeframe: flagTimeframe, Category: flagCategory, Property: flagProperty, CompareScope: compareScope, LastSyncedAt: syncedAt}
					body, _ := json.Marshal(q)
					id := sha256ID(kw, flagGeo, flagTimeframe, categoryStr, flagProperty, compareScope)
					if err := db.Upsert("gt_keyword_query", id, body); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to cache keyword query for %q: %v\n", kw, err)
					}
				}
			}

			if len(out) == 0 {
				return notFoundErr(fmt.Errorf("no interest-over-time data for %q", strings.Join(keywords, ", ")))
			}
			return printLiveResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagCompare, "compare", "", "Comma-separated additional keywords to compare (up to 4 more, 5 total)")
	cmd.Flags().StringVar(&flagGeo, "geo", "", "Geo code (e.g. US); default: worldwide")
	cmd.Flags().StringVar(&flagTimeframe, "timeframe", "today 12-m", "Time range, e.g. 'today 12-m', 'now 7-d'")
	cmd.Flags().IntVar(&flagCategory, "category", 0, "Google Trends category ID")
	cmd.Flags().StringVar(&flagProperty, "property", "", "Search property: '' (web), images, news, youtube, froogle")
	return cmd
}
