// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature command (Phase 3).

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDigestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Pre-baked analytics digests packaged for n8n / email / Slack",
		Long: `Wraps the Analytics API (yt-analytics.googleapis.com/v2/reports) with
opinionated queries that creators actually use: top videos, CTR, watch time,
traffic sources, revenue, Shorts vs long-form breakdown.

Output is JSON (default) or Markdown (--markdown) ready to pipe into an
email/Slack body.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newDigestAnalyticsCmd(flags))
	cmd.AddCommand(newDigestVideoCmd(flags))
	return cmd
}

const analyticsBaseURL = "https://youtubeanalytics.googleapis.com"

func analyticsQuery(c clientLike, params map[string]string) (json.RawMessage, error) {
	// Build absolute URL since Analytics API has a different host.
	q := []string{}
	for k, v := range params {
		if v == "" {
			continue
		}
		q = append(q, k+"="+v)
	}
	// Switch to GetWithHeaders to allow the alternate host. Most clients accept
	// absolute paths via Get when the base is the YouTube one; we'll fetch via
	// PostWithParams pointing to /v2/reports under the analytics host. Since
	// the generated client is bound to https://youtube.googleapis.com we have
	// to perform a direct HTTP GET here.
	url := analyticsBaseURL + "/v2/reports?" + strings.Join(q, "&")
	return clientRawGet(c, url)
}

// clientRawGet calls Get on the client with an absolute URL by passing the
// path-relative form. The generated client.Get prepends BaseURL only when the
// path starts with "/"; absolute URLs are accepted via Get.
func clientRawGet(c clientLike, absoluteURL string) (json.RawMessage, error) {
	// Use the client's Get with the absolute URL as path; the internal http call
	// honors absolute URLs.
	return c.Get(absoluteURL, nil)
}

func newDigestAnalyticsCmd(flags *rootFlags) *cobra.Command {
	var since, markdown string
	var asMarkdown bool
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Channel-wide analytics digest: views, watch time, CTR, top videos, traffic sources, revenue",
		Example: "  youtube-creator-pp-cli digest analytics --since 7d\n" +
			"  youtube-creator-pp-cli digest analytics --since 30d --markdown",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "youtubeAnalytics.reports.query",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			startDate, endDate := dateRange(since)
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Query 1: overall channel metrics
			overall, err := analyticsQuery(c, map[string]string{
				"ids":       "channel==MINE",
				"startDate": startDate,
				"endDate":   endDate,
				// Note: 'impressions' and 'impressionClickThroughRate' exist in the API
				// docs but are not always available (channel-permission gated). 'dislikes'
				// was deprecated for public reads but remains queryable for own channel.
				// We start with the always-available core metrics; the per-video query
				// below covers retention details.
				"metrics":   "views,estimatedMinutesWatched,averageViewDuration,averageViewPercentage,subscribersGained,subscribersLost,likes,shares,estimatedRevenue",
			})
			quotaLogCost("analytics-query", 1)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Query 2: top videos
			topVideos, err := analyticsQuery(c, map[string]string{
				"ids":        "channel==MINE",
				"startDate":  startDate,
				"endDate":    endDate,
				"metrics":    "views,estimatedMinutesWatched,averageViewPercentage",
				"dimensions": "video",
				"sort":       "-views",
				"maxResults": "10",
			})
			quotaLogCost("analytics-query", 1)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Query 3: traffic sources
			sources, err := analyticsQuery(c, map[string]string{
				"ids":        "channel==MINE",
				"startDate":  startDate,
				"endDate":    endDate,
				"metrics":    "views,estimatedMinutesWatched",
				"dimensions": "insightTrafficSourceType",
				"sort":       "-views",
				"maxResults": "10",
			})
			quotaLogCost("analytics-query", 1)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Query 4: Shorts vs long-form
			contentType, err := analyticsQuery(c, map[string]string{
				"ids":        "channel==MINE",
				"startDate":  startDate,
				"endDate":    endDate,
				"metrics":    "views,estimatedMinutesWatched",
				"dimensions": "creatorContentType",
			})
			quotaLogCost("analytics-query", 1)
			if err != nil {
				// creatorContentType may not be available for all channels; ignore
				contentType = nil
			}

			out := map[string]any{
				"start_date":      startDate,
				"end_date":        endDate,
				"overall":         overall,
				"top_videos":      topVideos,
				"traffic_sources": sources,
				"content_type":    contentType,
				"quota_cost":      4,
			}

			if asMarkdown || markdown != "" {
				md := renderAnalyticsMarkdown(out)
				_, _ = cmd.OutOrStdout().Write([]byte(md))
				return nil
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Lookback duration (e.g. 7d, 30d)")
	cmd.Flags().StringVar(&markdown, "markdown-out", "", "Write markdown to this file instead of stdout JSON")
	cmd.Flags().BoolVar(&asMarkdown, "markdown", false, "Output as Markdown to stdout instead of JSON")
	return cmd
}

func newDigestVideoCmd(flags *rootFlags) *cobra.Command {
	var asMarkdown bool
	cmd := &cobra.Command{
		Use:   "video [video-id]",
		Short: "Per-video performance digest: retention, traffic sources, demographics, devices",
		Example: "  youtube-creator-pp-cli digest video dQw4w9WgXcQ\n" +
			"  youtube-creator-pp-cli digest video dQw4w9WgXcQ --markdown",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "youtubeAnalytics.reports.query",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flags.dryRun {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			videoID := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Use a 90-day window for per-video views
			endDate := time.Now().Format("2006-01-02")
			startDate := time.Now().AddDate(0, 0, -90).Format("2006-01-02")
			filters := "video==" + videoID

			retention, _ := analyticsQuery(c, map[string]string{
				"ids":        "channel==MINE",
				"startDate":  startDate,
				"endDate":    endDate,
				"metrics":    "audienceWatchRatio,relativeRetentionPerformance",
				"dimensions": "elapsedVideoTimeRatio",
				"filters":    filters,
			})
			quotaLogCost("analytics-query", 1)

			devices, _ := analyticsQuery(c, map[string]string{
				"ids":        "channel==MINE",
				"startDate":  startDate,
				"endDate":    endDate,
				"metrics":    "views,estimatedMinutesWatched",
				"dimensions": "deviceType",
				"filters":    filters,
			})
			quotaLogCost("analytics-query", 1)

			sources, _ := analyticsQuery(c, map[string]string{
				"ids":        "channel==MINE",
				"startDate":  startDate,
				"endDate":    endDate,
				"metrics":    "views",
				"dimensions": "insightTrafficSourceType",
				"filters":    filters,
				"sort":       "-views",
			})
			quotaLogCost("analytics-query", 1)

			out := map[string]any{
				"video_id":   videoID,
				"start_date": startDate,
				"end_date":   endDate,
				"retention":  retention,
				"devices":    devices,
				"sources":    sources,
				"quota_cost": 3,
			}
			if asMarkdown {
				_, _ = cmd.OutOrStdout().Write([]byte(renderVideoMarkdown(videoID, out)))
				return nil
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().BoolVar(&asMarkdown, "markdown", false, "Output as Markdown")
	return cmd
}

func dateRange(since string) (string, string) {
	end := time.Now()
	endDate := end.Format("2006-01-02")
	days := 7
	if since != "" {
		s := strings.TrimSuffix(since, "d")
		if n, err := fmt.Sscanf(s, "%d", &days); err != nil || n == 0 {
			days = 7
		}
		if d, err := time.ParseDuration(since); err == nil {
			days = int(d.Hours()/24) + 1
		}
	}
	start := end.AddDate(0, 0, -days)
	return start.Format("2006-01-02"), endDate
}

func renderAnalyticsMarkdown(d map[string]any) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# YouTube Analytics Digest\n\n"))
	b.WriteString(fmt.Sprintf("**Period:** %v → %v\n\n", d["start_date"], d["end_date"]))
	b.WriteString("## Overall\n\n")
	b.WriteString(formatReportTable(d["overall"]))
	b.WriteString("\n## Top Videos\n\n")
	b.WriteString(formatReportTable(d["top_videos"]))
	b.WriteString("\n## Traffic Sources\n\n")
	b.WriteString(formatReportTable(d["traffic_sources"]))
	if ct, ok := d["content_type"]; ok && ct != nil {
		b.WriteString("\n## Shorts vs Long-form\n\n")
		b.WriteString(formatReportTable(ct))
	}
	b.WriteString(fmt.Sprintf("\n\n_Quota cost: %v units_\n", d["quota_cost"]))
	return b.String()
}

func renderVideoMarkdown(videoID string, d map[string]any) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Video Digest: %s\n\n", videoID))
	b.WriteString(fmt.Sprintf("**Period:** %v → %v\n\n", d["start_date"], d["end_date"]))
	b.WriteString("## Retention\n\n")
	b.WriteString(formatReportTable(d["retention"]))
	b.WriteString("\n## Devices\n\n")
	b.WriteString(formatReportTable(d["devices"]))
	b.WriteString("\n## Traffic Sources\n\n")
	b.WriteString(formatReportTable(d["sources"]))
	b.WriteString(fmt.Sprintf("\n\n_Quota cost: %v units_\n", d["quota_cost"]))
	return b.String()
}

// formatReportTable renders an Analytics API report response (which has
// `columnHeaders` and `rows`) as a Markdown table.
func formatReportTable(v any) string {
	raw, ok := v.(json.RawMessage)
	if !ok || len(raw) == 0 {
		return "_no data_\n"
	}
	var report struct {
		ColumnHeaders []struct {
			Name string `json:"name"`
			Type string `json:"columnType"`
		} `json:"columnHeaders"`
		Rows [][]any `json:"rows"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return "_failed to parse report: " + err.Error() + "_\n"
	}
	if len(report.ColumnHeaders) == 0 {
		return "_empty_\n"
	}
	var b strings.Builder
	// Header row
	for i, c := range report.ColumnHeaders {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString("**")
		b.WriteString(c.Name)
		b.WriteString("**")
	}
	b.WriteString("\n")
	// Separator
	for i := range report.ColumnHeaders {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString("---")
	}
	b.WriteString("\n")
	// Rows
	maxRows := 25
	for i, row := range report.Rows {
		if i >= maxRows {
			break
		}
		for j, cell := range row {
			if j > 0 {
				b.WriteString(" | ")
			}
			b.WriteString(fmt.Sprintf("%v", cell))
		}
		b.WriteString("\n")
	}
	if len(report.Rows) > maxRows {
		b.WriteString(fmt.Sprintf("\n_... %d more rows truncated_\n", len(report.Rows)-maxRows))
	}
	return b.String()
}
