// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// campaigns groups synced links by tag and aggregates click/lead/sale totals
// per campaign. Reads from the local /store cache (store.Open-backed); does
// not call the live API.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type campaignRow struct {
	Tag         string `json:"tag"`
	TotalLinks  int    `json:"total_links"`
	TotalClicks int    `json:"total_clicks"`
	TotalLeads  int    `json:"total_leads"`
	TotalSales  int    `json:"total_sales"`
	TopLink     string `json:"top_link,omitempty"`
}

func newCampaignsCmd(flags *rootFlags) *cobra.Command {
	var sortBy string
	var minClicks int
	var limit int
	var interval string

	cmd := &cobra.Command{
		Use:   "campaigns",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Tag-grouped performance dashboard — clicks, leads, sales rolled up per tag",
		Long: `Aggregate clicks, leads, and sales for every tag in your workspace. Joins the
local link cache with each link's analytics fields. Surfaces the top campaign tag
and a representative top-performing link per tag.

Reads from the local store. Run sync first.`,
		Example: `  # All tags ranked by total clicks
  dub-pp-cli campaigns --json

  # Limit to top 10 campaigns by sales
  dub-pp-cli campaigns --sort-by sales --limit 10 --agent

  # Hide tags with fewer than 100 cumulative clicks
  dub-pp-cli campaigns --min-clicks 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStoreForRead("dub-pp-cli")
			if err != nil {
				return apiErr(fmt.Errorf("open local store: %w", err))
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local store found. run `dub-pp-cli sync --full` first"))
			}
			defer db.Close()

			rows, err := db.DB().Query(`SELECT data FROM links WHERE archived = 0 OR archived IS NULL`)
			if err != nil {
				return apiErr(fmt.Errorf("query links: %w", err))
			}
			defer rows.Close()

			agg := make(map[string]*campaignRow)
			topPerTag := make(map[string]int) // tag -> max clicks seen
			topLinkPerTag := make(map[string]string)
			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(raw), &obj); err != nil {
					continue
				}
				tags := tagNamesFrom(obj)
				if len(tags) == 0 {
					tags = []string{"(untagged)"}
				}
				clicks := intField(obj, "clicks")
				leads := intField(obj, "leads")
				sales := intField(obj, "sales")
				short := stringField(obj, "shortLink")
				for _, tag := range tags {
					row, ok := agg[tag]
					if !ok {
						row = &campaignRow{Tag: tag}
						agg[tag] = row
					}
					row.TotalLinks++
					row.TotalClicks += clicks
					row.TotalLeads += leads
					row.TotalSales += sales
					if clicks > topPerTag[tag] {
						topPerTag[tag] = clicks
						topLinkPerTag[tag] = short
					}
				}
			}
			out := make([]campaignRow, 0, len(agg))
			for _, r := range agg {
				if r.TotalClicks < minClicks {
					continue
				}
				r.TopLink = topLinkPerTag[r.Tag]
				out = append(out, *r)
			}
			sortCampaigns(out, sortBy)
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no campaigns found (try lowering --min-clicks or running sync)")
				return nil
			}
			headers := []string{"TAG", "LINKS", "CLICKS", "LEADS", "SALES", "TOP_LINK"}
			rowsTbl := make([][]string, 0, len(out))
			for _, r := range out {
				rowsTbl = append(rowsTbl, []string{r.Tag, fmt.Sprintf("%d", r.TotalLinks), fmt.Sprintf("%d", r.TotalClicks), fmt.Sprintf("%d", r.TotalLeads), fmt.Sprintf("%d", r.TotalSales), r.TopLink})
			}
			return flags.printTable(cmd, headers, rowsTbl)
		},
	}

	cmd.Flags().StringVar(&sortBy, "sort-by", "clicks", "Sort by: clicks, leads, sales, links")
	cmd.Flags().IntVar(&minClicks, "min-clicks", 0, "Hide campaigns with fewer total clicks")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap output to top N campaigns (0 = no limit)")
	cmd.Flags().StringVar(&interval, "interval", "all", "Time interval (e.g. 24h, 7d, 30d, all). Reserved for future analytics-cache joins; today campaigns reads the current local snapshot.")
	_ = interval
	return cmd
}

// tagNamesFrom extracts tag names from a link object.
// Dub's API returns either `tags: [{id, name, color}, ...]` (full) or
// `tagNames: ["x", "y"]` (compact). Handle both.
func tagNamesFrom(obj map[string]any) []string {
	out := []string{}
	if names, ok := obj["tagNames"].([]any); ok {
		for _, n := range names {
			if s, ok := n.(string); ok {
				out = append(out, s)
			}
		}
	}
	if tags, ok := obj["tags"].([]any); ok {
		for _, t := range tags {
			if m, ok := t.(map[string]any); ok {
				if name, ok := m["name"].(string); ok {
					out = append(out, name)
				}
			}
		}
	}
	return uniqueStrings(out)
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func sortCampaigns(rows []campaignRow, by string) {
	by = strings.ToLower(strings.TrimSpace(by))
	sort.Slice(rows, func(i, j int) bool {
		switch by {
		case "leads":
			return rows[i].TotalLeads > rows[j].TotalLeads
		case "sales":
			return rows[i].TotalSales > rows[j].TotalSales
		case "links":
			return rows[i].TotalLinks > rows[j].TotalLinks
		default: // clicks
			return rows[i].TotalClicks > rows[j].TotalClicks
		}
	})
}
