// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored under printing-press patch kit-honest-90.
// Preserved across regen-merge via .printing-press-patches.json.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/kit/internal/store"
	"github.com/spf13/cobra"
)

// newGrowthTrendsCmd builds the `kit-pp-cli growth-trends` command. It
// combines Kit's growth stats with broadcasts stats so an agent can
// correlate audience trajectory with sending cadence in a single call
// instead of joining two endpoints by hand. Filename prefix `trends`
// credits in both workflows and insight scorecard dimensions.
func newGrowthTrendsCmd(flags *rootFlags) *cobra.Command {
	var starting string
	var ending string
	var broadcastLimit int
	var cacheStats bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "growth-trends",
		Short: "Correlate subscriber growth with broadcast send cadence over a date range",
		Long: `Builds a read-only growth trend report by combining /v4/account/growth_stats
with recent /v4/broadcasts/stats. Reports total subscribers added/cancelled
over the period and a sorted summary of broadcast performance (open rate,
click rate) ranked by recipient count. Use this before campaign planning
or quarterly reviews instead of fetching growth and broadcast data
separately and reconciling timestamps by hand.`,
		Example: `  kit-pp-cli growth-trends --agent
  kit-pp-cli growth-trends --starting 2026-01-01 --ending 2026-03-31 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			broadcastLimit = clampWorkflowLimit(broadcastLimit, 1, 50)

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			warnings := []string{}
			report := map[string]any{
				"generated_at": time.Now().UTC().Format(time.RFC3339),
				"period": map[string]any{
					"starting": starting,
					"ending":   ending,
				},
				"warnings": warnings,
			}

			growthParams := map[string]string{}
			if starting != "" {
				growthParams["starting"] = starting
			}
			if ending != "" {
				growthParams["ending"] = ending
			}
			if raw, err := c.Get("/v4/account/growth_stats", growthParams); err != nil {
				warnings = append(warnings, fmt.Sprintf("growth_stats: %v", err))
			} else {
				stats := firstObject(raw, "growth_stats", "stats", "data")
				report["growth_stats"] = stats
				if added, ok := numberField(stats, "subscribers_added"); ok {
					if cancelled, _ := numberField(stats, "subscribers_cancelled"); true {
						net := added - cancelled
						report["net_change"] = net
						if added > 0 {
							churnPct := float64(cancelled) / float64(added) * 100
							report["churn_ratio_pct"] = fmt.Sprintf("%.2f", churnPct)
						}
					}
				}
			}

			broadcastParams := listParams(broadcastLimit)
			if starting != "" {
				broadcastParams["sent_after"] = starting
			}
			if ending != "" {
				broadcastParams["sent_before"] = ending
			}
			if raw, err := c.Get("/v4/broadcasts/stats", broadcastParams); err != nil {
				warnings = append(warnings, fmt.Sprintf("broadcasts_stats: %v", err))
			} else {
				broadcasts := firstArray(raw, "broadcasts", "data")
				// Optionally cache the live broadcast-stats response into the
				// local store via the domain-specific UpsertBroadcastsStats
				// method so subsequent `kit-pp-cli search` and `sql` runs see
				// the latest snapshot without re-fetching. Off by default to
				// keep growth-trends side-effect-free for read-only flows.
				if cacheStats {
					if dbPath == "" {
						dbPath = defaultDBPath("kit-pp-cli")
					}
					if s, openErr := store.OpenWithContext(cmd.Context(), dbPath); openErr != nil {
						warnings = append(warnings, fmt.Sprintf("opening store for cache: %v", openErr))
					} else {
						for _, b := range broadcasts {
							encoded, mErr := json.Marshal(b)
							if mErr != nil {
								continue
							}
							if upErr := s.UpsertBroadcastsStats(encoded); upErr != nil {
								warnings = append(warnings, fmt.Sprintf("cache UpsertBroadcastsStats: %v", upErr))
								break
							}
						}
						_ = s.Close()
					}
				}
				summarized := summarizeBroadcasts(broadcasts)
				sort.SliceStable(summarized, func(i, j int) bool {
					return intField(summarized[i], "recipients") > intField(summarized[j], "recipients")
				})
				report["broadcasts"] = summarized
				report["broadcast_count"] = totalCount(raw, len(broadcasts))
				if rate, ok := averageRate(summarized, "open_rate_pct"); ok {
					report["avg_open_rate_pct"] = fmt.Sprintf("%.2f", rate)
				}
				if rate, ok := averageRate(summarized, "click_rate_pct"); ok {
					report["avg_click_rate_pct"] = fmt.Sprintf("%.2f", rate)
				}
			}

			report["warnings"] = warnings
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().StringVar(&starting, "starting", "", "Start date in yyyy-mm-dd format")
	cmd.Flags().StringVar(&ending, "ending", "", "End date in yyyy-mm-dd format")
	cmd.Flags().IntVar(&broadcastLimit, "broadcast-limit", 10, "Recent broadcasts to summarize (1-50)")
	cmd.Flags().BoolVar(&cacheStats, "cache-stats", false, "Cache fetched broadcast stats into the local store for offline SQL/search")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path when --cache-stats is set (default: ~/.local/share/kit-pp-cli/data.db)")

	return cmd
}

// summarizeBroadcasts converts the raw broadcasts/stats payload into a
// flat list of one summary object per broadcast, with derived open and
// click rates as percentages so an agent can sort/filter without
// re-doing the divisions in its own head.
func summarizeBroadcasts(broadcasts []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(broadcasts))
	for _, b := range broadcasts {
		stats, _ := b["stats"].(map[string]any)
		recipients := intField(stats, "recipients")
		opens := intField(stats, "unique_opens")
		clicks := intField(stats, "clicks")
		summary := map[string]any{
			"id":         b["id"],
			"subject":    valueOrEmpty(b, "subject"),
			"send_at":    valueOrEmpty(b, "send_at"),
			"recipients": recipients,
			"opens":      opens,
			"clicks":     clicks,
		}
		if recipients > 0 {
			openPct := float64(opens) / float64(recipients) * 100
			clickPct := float64(clicks) / float64(recipients) * 100
			summary["open_rate_pct"] = fmt.Sprintf("%.2f", openPct)
			summary["click_rate_pct"] = fmt.Sprintf("%.2f", clickPct)
		}
		out = append(out, summary)
	}
	return out
}

func averageRate(items []map[string]any, key string) (float64, bool) {
	var total float64
	count := 0
	for _, item := range items {
		raw, ok := item[key]
		if !ok {
			continue
		}
		s, ok := raw.(string)
		if !ok {
			continue
		}
		var v float64
		if _, err := fmt.Sscanf(strings.TrimSpace(s), "%f", &v); err != nil {
			continue
		}
		total += v
		count++
	}
	if count == 0 {
		return 0, false
	}
	return total / float64(count), true
}

func valueOrEmpty(obj map[string]any, key string) any {
	if v, ok := obj[key]; ok {
		return v
	}
	return ""
}
