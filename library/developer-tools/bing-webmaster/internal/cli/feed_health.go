// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// feed-health: sitemap/feed health monitor. Tracks submitted/discovered/indexed
// counts per feed over time and flags drops. Hand-authored transcendence command.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type bFeedRow struct {
	URL          string  `json:"url"`
	Submitted    float64 `json:"submitted"`
	Discovered   float64 `json:"discovered"`
	Indexed      float64 `json:"indexed"`
	IndexedDelta float64 `json:"indexed_delta_since_last"`
}

type bFeedHealthResult struct {
	Site   string     `json:"site"`
	Feeds  []bFeedRow `json:"feeds"`
	Alerts []string   `json:"alerts"`
}

// parseFeedRows normalizes a GetFeeds array. Field names vary across Bing
// responses, so each count is looked up under several likely keys. Pure.
func parseFeedRows(data json.RawMessage) []bFeedRow {
	out := []bFeedRow{}
	for _, it := range bArray(data) {
		m := bCIMap(it)
		if m == nil {
			continue
		}
		url := bStr(m, "Url")
		if url == "" {
			url = bStr(m, "FeedUrl")
		}
		if url == "" {
			continue
		}
		row := bFeedRow{URL: url}
		row.Submitted = firstNum(m, "UrlCount", "SubmittedUrlCount", "Submitted")
		row.Discovered = firstNum(m, "DiscoveredUrlCount", "Discovered", "DiscoveredCount")
		row.Indexed = firstNum(m, "IndexedUrlCount", "Indexed", "IndexedCount")
		out = append(out, row)
	}
	return out
}

func firstNum(m map[string]json.RawMessage, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := bNum(m, k); ok {
			return v
		}
	}
	return 0
}

func newFeedHealthCmd(flags *rootFlags) *cobra.Command {
	var site string
	cmd := &cobra.Command{
		Use:         "feed-health",
		Short:       "Track sitemap/feed submitted/discovered/indexed counts over time and flag drops",
		Long:        "Read all feeds, snapshot their submitted/discovered/indexed counts, and compare against the previous snapshot to flag any feed whose indexed count dropped — catching a sitemap that silently stopped being indexed.",
		Example:     "  bing-webmaster-pp-cli feed-health --site https://example.com",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			proceed, err := siteOrDryRun(cmd, flags, site, fmt.Sprintf("would read feed health for %q", site))
			if err != nil || !proceed {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), "/json/GetFeeds", map[string]string{"siteUrl": site})
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			feeds := parseFeedRows(data)

			db, err := openSnapshots()
			if err != nil {
				return err
			}
			defer db.Close()
			now := time.Now()
			prev, hadPrev, err := db.Latest(site, "feeds")
			if err != nil {
				return err
			}
			if err := db.Capture(site, "feeds", data, now); err != nil {
				return err
			}

			var result bFeedHealthResult
			result.Site = site
			result.Alerts = []string{}
			prevIndexed := map[string]float64{}
			if hadPrev {
				for _, pr := range parseFeedRows(prev.Data) {
					prevIndexed[pr.URL] = pr.Indexed
				}
			}
			for i := range feeds {
				if old, ok := prevIndexed[feeds[i].URL]; ok {
					feeds[i].IndexedDelta = feeds[i].Indexed - old
					if feeds[i].IndexedDelta < 0 {
						result.Alerts = append(result.Alerts,
							fmt.Sprintf("%s indexed dropped by %.0f (%.0f -> %.0f)", feeds[i].URL, -feeds[i].IndexedDelta, old, feeds[i].Indexed))
					}
				}
			}
			result.Feeds = feeds
			return emitIntel(cmd, flags, result, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "Feed health for %s: %d feeds\n", site, len(feeds))
				sort.Slice(feeds, func(i, j int) bool { return feeds[i].URL < feeds[j].URL })
				for _, f := range feeds {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-50s submitted %.0f  discovered %.0f  indexed %.0f (%.0f)\n",
						f.URL, f.Submitted, f.Discovered, f.Indexed, f.IndexedDelta)
				}
				for _, a := range result.Alerts {
					fmt.Fprintf(cmd.OutOrStdout(), "  ALERT: %s\n", a)
				}
			})
		},
	}
	cmd.Flags().StringVar(&site, "site", "", "Verified site URL (required)")
	return cmd
}
