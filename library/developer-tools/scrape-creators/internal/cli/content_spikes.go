// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: surface content that beat a creator's own baseline.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type contentSpikeRow struct {
	ID          string  `json:"id"`
	URL         string  `json:"url,omitempty"`
	Metric      int64   `json:"metric"`
	RatioToMean float64 `json:"ratio_to_mean"`
}

func newNovelContentSpikesCmd(flags *rootFlags) *cobra.Command {
	var platform string
	var threshold float64
	var limit int

	cmd := &cobra.Command{
		Use:         "spikes <handle>",
		Short:       "Surface the content that performed far above a creator's own baseline — the posts that actually went viral.",
		Example:     "  scrape-creators-pp-cli content spikes mrbeast --threshold 2.0",
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
				return usageErr(fmt.Errorf("handle is required"))
			}
			handle := args[0]
			if platform == "" {
				platform = "tiktok"
			}
			if threshold <= 0 {
				threshold = 2.0
			}
			cs, ok := contentSources[platform]
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unsupported --platform %q (supported: tiktok, youtube, instagram)", platform))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get(ctx, cs.path, map[string]string{cs.handleParam: handle})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			items := resultArray(data, cs.arrayKey)
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}

			// Metric is view count when present, else total engagement.
			type scored struct {
				id     string
				url    string
				metric int64
			}
			scoredItems := make([]scored, 0, len(items))
			var sum int64
			for _, it := range items {
				m := extractContentMetrics(it)
				metric := m.views
				if metric == 0 {
					metric = m.engagement()
				}
				var obj map[string]any
				_ = json.Unmarshal(it, &obj)
				id := extractItemID(it, "aweme_id")
				url := firstStringField(obj, "share_url", "url", "shareUrl")
				scoredItems = append(scoredItems, scored{id: id, url: url, metric: metric})
				sum += metric
			}

			spikes := make([]contentSpikeRow, 0)
			var mean float64
			if len(scoredItems) > 0 {
				mean = float64(sum) / float64(len(scoredItems))
			}
			if mean > 0 {
				for _, s := range scoredItems {
					ratio := float64(s.metric) / mean
					if ratio >= threshold {
						spikes = append(spikes, contentSpikeRow{ID: s.id, URL: s.url, Metric: s.metric, RatioToMean: ratio})
					}
				}
			}
			sort.Slice(spikes, func(i, j int) bool { return spikes[i].Metric > spikes[j].Metric })

			if novelWantsMachine(cmd.OutOrStdout(), flags) {
				envelope := map[string]any{
					"handle":      handle,
					"platform":    platform,
					"threshold":   threshold,
					"sampled":     len(scoredItems),
					"mean_metric": int64(mean),
					"spikes":      spikes,
					"spike_count": len(spikes),
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			w := cmd.OutOrStdout()
			if len(scoredItems) == 0 {
				fmt.Fprintf(w, "No content returned for %q on %s.\n", handle, platform)
				return nil
			}
			fmt.Fprintf(w, "%d/%d items spiked >= %.1fx the mean (%d) for %q on %s\n\n",
				len(spikes), len(scoredItems), threshold, int64(mean), handle, platform)
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "ID\tMETRIC\tRATIO\tURL")
			for _, s := range spikes {
				fmt.Fprintf(tw, "%s\t%d\t%.2fx\t%s\n", s.ID, s.Metric, s.RatioToMean, s.URL)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "tiktok", "platform to pull recent content from (tiktok, youtube, instagram)")
	cmd.Flags().Float64Var(&threshold, "threshold", 2.0, "minimum ratio to the mean for a post to count as a spike")
	cmd.Flags().IntVar(&limit, "limit", 0, "cap the number of recent items examined (0 = all returned)")
	return cmd
}
