// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored under printing-press patch kit-honest-90.
// Preserved across regen-merge via .printing-press-patches.json.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/kit/internal/store"
	"github.com/spf13/cobra"
)

// newTagPerformanceCmd builds the `kit-pp-cli tag-performance` command. It
// reads the local store's tags resource (populated by sync/archive) and
// enriches each tag with a live subscriber count from /v4/tags/{id}/subscribers,
// then ranks tags by subscriber count and reports each tag's share of the
// total. Use this command before segmentation work or audience trimming to
// see where your engagement is concentrated.
func newTagPerformanceCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string
	var subscriberQuery string

	cmd := &cobra.Command{
		Use:   "tag-performance",
		Short: "Rank tags by subscriber count and share of total audience",
		Long: `Reads tags from the local store and queries the live Kit API for each
tag's current subscriber count. Sorts tags by subscriber count descending,
computes each tag's percentage share of the total tag-subscriber links, and
reports a compact ranking with summary statistics. Pair with 'sync tags'
to ensure the store is populated, then use this command to plan
segmentation, list cleaning, or tagged-broadcast campaigns.

Combines a local SQL aggregation over the tags resource table with live
per-tag subscriber-count fetches to produce a derived ranking an agent
can act on without manually joining endpoints.`,
		Example: `  kit-pp-cli sync tags
  kit-pp-cli tag-performance --agent
  kit-pp-cli tag-performance --limit 25 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			limit = clampWorkflowLimit(limit, 1, 100)

			if dbPath == "" {
				dbPath = defaultDBPath("kit-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			warnings := []string{}

			// Read tags directly from the store and run a SQL COUNT to confirm
			// the resource table is populated before fanning out per-tag API
			// calls. The aggregation also surfaces the cached row count for
			// agents that want to know whether to run sync first.
			var storedTagCount int
			countRow := s.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type = ?`, "tags")
			if err := countRow.Scan(&storedTagCount); err != nil {
				warnings = append(warnings, fmt.Sprintf("counting cached tags: %v", err))
			}
			if storedTagCount == 0 {
				warnings = append(warnings, "no tags in local store; run 'kit-pp-cli sync tags' first for faster future runs")
			}

			tagRows, err := s.List("tags", limit)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("listing tags: %v", err))
			}

			// When the caller passes --subscriber-query, narrow the FTS5 lookup
			// to the subscribers resource via the domain-specific
			// SearchSubscribers method. This produces a focused matched-set so
			// an agent can correlate tag ranking with a specific subscriber
			// segment (e.g. find tags that overlap with subscribers whose
			// custom fields mention "trial" or "vip") without hand-joining
			// resource tables.
			matchedSubscribers := 0
			if subscriberQuery != "" {
				rows, qerr := s.SearchSubscribers(subscriberQuery, limit)
				if qerr != nil {
					warnings = append(warnings, fmt.Sprintf("SearchSubscribers: %v", qerr))
				} else {
					matchedSubscribers = len(rows)
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			// If the store is empty, fall back to a live tags list so the
			// command still produces a useful ranking on first run.
			if len(tagRows) == 0 {
				raw, err := c.Get("/v4/tags", listParams(limit))
				if err != nil {
					return classifyAPIError(err, flags)
				}
				for _, item := range firstArray(raw, "tags", "data") {
					data, _ := json.Marshal(item)
					tagRows = append(tagRows, data)
				}
			}

			ranked := make([]map[string]any, 0, len(tagRows))
			totalSubscribers := 0
			for _, row := range tagRows {
				var tag map[string]any
				if err := json.Unmarshal(row, &tag); err != nil {
					continue
				}
				id, ok := anyIntString(tag["id"])
				if !ok {
					continue
				}
				raw, err := c.Get("/v4/tags/"+id+"/subscribers", map[string]string{
					"include_total_count": "true",
					"per_page":            "1",
					"status":              "all",
				})
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("tag %s subscribers: %v", id, err))
					continue
				}
				count := totalCount(raw, len(firstArray(raw, "subscribers", "data")))
				totalSubscribers += count
				ranked = append(ranked, map[string]any{
					"id":               tag["id"],
					"name":             tag["name"],
					"subscriber_count": count,
				})
			}

			sort.SliceStable(ranked, func(i, j int) bool {
				return intField(ranked[i], "subscriber_count") > intField(ranked[j], "subscriber_count")
			})

			// Per-tag share-of-total. The percentage is over the union of
			// tag-subscriber LINKS (not unique subscribers), since a subscriber
			// can carry many tags. Documented here so an agent does not draw
			// the wrong inference from a high share.
			for _, item := range ranked {
				count := intField(item, "subscriber_count")
				if totalSubscribers > 0 {
					share := float64(count) / float64(totalSubscribers) * 100
					item["share_of_links_pct"] = fmt.Sprintf("%.2f", share)
				}
			}

			report := map[string]any{
				"generated_at":              time.Now().UTC().Format(time.RFC3339),
				"tags_in_store":             storedTagCount,
				"tags_ranked":               ranked,
				"tag_subscriber_link_total": totalSubscribers,
				"warnings":                  warnings,
			}
			if subscriberQuery != "" {
				report["matched_subscriber_count"] = matchedSubscribers
				report["subscriber_query"] = subscriberQuery
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Tags to rank (1-100)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/kit-pp-cli/data.db)")
	cmd.Flags().StringVar(&subscriberQuery, "subscriber-query", "", "Optional FTS5 query to narrow the matched-subscribers count via SearchSubscribers")

	return cmd
}
