package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Analytics and performance insights across your campaigns",
	}
	cmd.AddCommand(newStatsTrendsCmd(flags))
	cmd.AddCommand(newStatsFunnelCmd(flags))
	cmd.AddCommand(newStatsBestTimeCmd(flags))
	return cmd
}

// statsCampaign represents a campaign with its stats combined.
type statsCampaign struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Subject     string    `json:"subject"`
	SentAt      time.Time `json:"sent_at"`
	SentCount   int       `json:"sent_count"`
	OpenCount   int       `json:"unique_open_count"`
	ClickCount  int       `json:"unique_click_count"`
	UnsubCount  int       `json:"unsubscribe_count"`
	BounceCount int       `json:"bounce_count"`
	OpenRate    float64   `json:"open_rate"`
	ClickRate   float64   `json:"click_rate"`
	UnsubRate   float64   `json:"unsubscribe_rate"`
}

// fetchCampaignsWithStats fetches all sent campaigns from the API and enriches
// them with stats. This is live — it does not use the local SQLite store.
func fetchCampaignsWithStats(flags *rootFlags, days int) ([]statsCampaign, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	// Fetch all campaigns (paginated)
	var allCampaigns []map[string]any
	page := 1
	for {
		params := map[string]string{"page": fmt.Sprintf("%d", page)}
		data, _, err := resolveRead(nil, c, flags, "campaigns", false, "/campaigns", params, nil)
		if err != nil {
			return nil, fmt.Errorf("fetching campaigns page %d: %w", page, err)
		}
		var resp struct {
			Data        []map[string]any `json:"data"`
			CurrentPage int              `json:"current_page"`
			Total       int              `json:"total"`
			PerPage     int              `json:"per_page"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			// Try direct array
			var direct []map[string]any
			if err2 := json.Unmarshal(data, &direct); err2 == nil {
				allCampaigns = append(allCampaigns, direct...)
				break
			}
			return nil, fmt.Errorf("parsing campaigns: %w", err)
		}
		allCampaigns = append(allCampaigns, resp.Data...)
		if len(resp.Data) < resp.PerPage || resp.Total <= page*resp.PerPage {
			break
		}
		page++
		time.Sleep(200 * time.Millisecond) // respect rate limit
	}

	// Filter to sent campaigns within the time window
	var result []statsCampaign
	for _, camp := range allCampaigns {
		sentAtStr, _ := camp["sent_at"].(string)
		if sentAtStr == "" {
			continue // not sent yet
		}
		sentAt, err := time.Parse(time.RFC3339, sentAtStr)
		if err != nil {
			sentAt, err = time.Parse("2006-01-02T15:04:05.000000Z", sentAtStr)
			if err != nil {
				continue
			}
		}
		if days > 0 && sentAt.Before(cutoff) {
			continue
		}

		id := int(toFloat64(camp["id"]))
		if id == 0 {
			continue
		}

		sc := statsCampaign{
			ID:      id,
			Title:   toString(camp["title"]),
			Subject: toString(camp["subject"]),
			SentAt:  sentAt,
		}

		// Fetch stats for this campaign
		statsData, _, statsErr := resolveRead(nil, c, flags, "campaigns", false,
			fmt.Sprintf("/campaigns/%d/stats", id), nil, nil)
		if statsErr == nil {
			var s map[string]any
			if json.Unmarshal(statsData, &s) == nil {
				sc.SentCount = int(toFloat64(s["sent_count"]))
				sc.OpenCount = int(toFloat64(s["unique_open_count"]))
				sc.ClickCount = int(toFloat64(s["unique_click_count"]))
				sc.UnsubCount = int(toFloat64(s["unsubscribe_count"]))
				sc.BounceCount = int(toFloat64(s["bounce_count"]))
				sc.OpenRate = toFloat64(s["open_rate"])
				sc.ClickRate = toFloat64(s["click_rate"])
				sc.UnsubRate = toFloat64(s["unsubscribe_rate"])
			}
		}
		result = append(result, sc)
		time.Sleep(100 * time.Millisecond) // respect 60 req/min rate limit
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].SentAt.Before(result[j].SentAt)
	})
	return result, nil
}

func newStatsTrendsCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Show how open rates, click rates, and unsubscribes have changed over time",
		Long: `trends fetches your sent campaigns and their stats, then shows how
engagement has trended over the specified number of days.

Note: this command makes one API call per sent campaign to fetch stats.
For large accounts, it may take a moment.`,
		Example: strings.Trim(`
  sendfox-pp-cli stats trends --days 30
  sendfox-pp-cli stats trends --days 90 --json
  sendfox-pp-cli stats trends --days 30 --json --select open_rate,click_rate,date`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil_isVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch campaign trends")
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Fetching campaigns from the last %d days...\n", days)

			campaigns, err := fetchCampaignsWithStats(flags, days)
			if err != nil {
				return fmt.Errorf("fetching stats: %w", err)
			}
			if len(campaigns) == 0 {
				fmt.Fprintf(w, "No sent campaigns found in the last %d days.\n", days)
				return nil
			}

			type trendRow struct {
				Date      string  `json:"date"`
				Subject   string  `json:"subject"`
				SentCount int     `json:"sent_count"`
				OpenRate  float64 `json:"open_rate"`
				ClickRate float64 `json:"click_rate"`
				UnsubRate float64 `json:"unsub_rate"`
			}
			var rows []trendRow
			for _, c := range campaigns {
				rows = append(rows, trendRow{
					Date:      c.SentAt.Format("2006-01-02"),
					Subject:   c.Subject,
					SentCount: c.SentCount,
					OpenRate:  c.OpenRate,
					ClickRate: c.ClickRate,
					UnsubRate: c.UnsubRate,
				})
			}

			if flags.asJSON || (!isTerminal(w) && !flags.csv && !flags.quiet && !flags.plain) {
				data, _ := json.Marshal(rows)
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				return printOutput(w, filtered, true)
			}

			// Human table
			fmt.Fprintln(w, "")
			fmt.Fprintf(w, "%-12s %-45s %6s %8s %8s %8s\n",
				"Date", "Subject", "Sent", "Opens%", "Clicks%", "Unsubs%")
			fmt.Fprintln(w, strings.Repeat("─", 90))
			for _, r := range rows {
				subj := r.Subject
				if len(subj) > 45 {
					subj = subj[:42] + "..."
				}
				fmt.Fprintf(w, "%-12s %-45s %6d %7.1f%% %7.1f%% %7.1f%%\n",
					r.Date, subj, r.SentCount, r.OpenRate*100, r.ClickRate*100, r.UnsubRate*100)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Number of days to look back")
	return cmd
}

func newStatsFunnelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "funnel",
		Short: "Cross-campaign funnel: sent vs opened vs clicked vs unsubscribed",
		Long: `funnel aggregates stats across all your sent campaigns to show your
overall list engagement at a glance.

Requires fetching stats for each campaign — may take a moment for large accounts.`,
		Example: strings.Trim(`
  sendfox-pp-cli stats funnel
  sendfox-pp-cli stats funnel --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil_isVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute engagement funnel")
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}

			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Fetching all campaign stats...")

			campaigns, err := fetchCampaignsWithStats(flags, 0) // 0 = all time
			if err != nil {
				return fmt.Errorf("fetching stats: %w", err)
			}
			if len(campaigns) == 0 {
				fmt.Fprintln(w, "No sent campaigns found.")
				return nil
			}

			var totalSent, totalOpen, totalClick, totalUnsub, totalBounce int
			for _, c := range campaigns {
				totalSent += c.SentCount
				totalOpen += c.OpenCount
				totalClick += c.ClickCount
				totalUnsub += c.UnsubCount
				totalBounce += c.BounceCount
			}

			openPct := pct(totalOpen, totalSent)
			clickPct := pct(totalClick, totalSent)
			unsubPct := pct(totalUnsub, totalSent)
			bouncePct := pct(totalBounce, totalSent)

			type funnelResult struct {
				Campaigns    int     `json:"campaigns"`
				TotalSent    int     `json:"total_sent"`
				TotalOpened  int     `json:"total_opened"`
				TotalClicked int     `json:"total_clicked"`
				TotalUnsubs  int     `json:"total_unsubscribed"`
				TotalBounces int     `json:"total_bounces"`
				OpenRate     float64 `json:"open_rate_pct"`
				ClickRate    float64 `json:"click_rate_pct"`
				UnsubRate    float64 `json:"unsub_rate_pct"`
				BounceRate   float64 `json:"bounce_rate_pct"`
			}
			result := funnelResult{
				Campaigns:    len(campaigns),
				TotalSent:    totalSent,
				TotalOpened:  totalOpen,
				TotalClicked: totalClick,
				TotalUnsubs:  totalUnsub,
				TotalBounces: totalBounce,
				OpenRate:     openPct,
				ClickRate:    clickPct,
				UnsubRate:    unsubPct,
				BounceRate:   bouncePct,
			}

			if flags.asJSON || (!isTerminal(w) && !flags.csv && !flags.quiet && !flags.plain) {
				data, _ := json.Marshal(result)
				return printOutput(w, data, true)
			}

			fmt.Fprintln(w, "")
			fmt.Fprintf(w, "Campaigns analyzed: %d\n\n", len(campaigns))
			fmt.Fprintf(w, "%-18s %10s %8s\n", "Stage", "Count", "Rate")
			fmt.Fprintln(w, strings.Repeat("─", 40))
			fmt.Fprintf(w, "%-18s %10d %7.1f%%\n", "Sent", totalSent, 100.0)
			fmt.Fprintf(w, "%-18s %10d %7.1f%%\n", "Opened", totalOpen, openPct)
			fmt.Fprintf(w, "%-18s %10d %7.1f%%\n", "Clicked", totalClick, clickPct)
			fmt.Fprintf(w, "%-18s %10d %7.1f%%\n", "Unsubscribed", totalUnsub, unsubPct)
			fmt.Fprintf(w, "%-18s %10d %7.1f%%\n", "Bounced", totalBounce, bouncePct)
			return nil
		},
	}
	return cmd
}

func newStatsBestTimeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "best-time",
		Short: "Identify which days and hours correlate with your highest open rates",
		Long: `best-time analyzes your historical campaign data to find which day of
the week and hour of the day tend to produce the best open rates.

Uses all available sent campaigns — the more you have, the more accurate the result.`,
		Example: strings.Trim(`
  sendfox-pp-cli stats best-time
  sendfox-pp-cli stats best-time --json --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil_isVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would analyze best send times")
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}

			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Analyzing historical campaign performance...")

			campaigns, err := fetchCampaignsWithStats(flags, 0)
			if err != nil {
				return fmt.Errorf("fetching stats: %w", err)
			}
			if len(campaigns) == 0 {
				fmt.Fprintln(w, "No sent campaigns found.")
				return nil
			}

			// Aggregate by day-of-week and hour
			type bucket struct {
				count   int
				openSum float64
			}
			byDay := make(map[time.Weekday]*bucket)
			byHour := make(map[int]*bucket)

			for _, c := range campaigns {
				day := c.SentAt.Weekday()
				hour := c.SentAt.Hour()
				if byDay[day] == nil {
					byDay[day] = &bucket{}
				}
				byDay[day].count++
				byDay[day].openSum += c.OpenRate

				if byHour[hour] == nil {
					byHour[hour] = &bucket{}
				}
				byHour[hour].count++
				byHour[hour].openSum += c.OpenRate
			}

			type rankEntry struct {
				Label   string  `json:"label"`
				Count   int     `json:"campaigns"`
				AvgOpen float64 `json:"avg_open_rate_pct"`
			}

			var dayRanks []rankEntry
			days := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}
			for _, d := range days {
				if b := byDay[d]; b != nil && b.count > 0 {
					dayRanks = append(dayRanks, rankEntry{
						Label:   d.String(),
						Count:   b.count,
						AvgOpen: (b.openSum / float64(b.count)) * 100,
					})
				}
			}
			sort.Slice(dayRanks, func(i, j int) bool {
				return dayRanks[i].AvgOpen > dayRanks[j].AvgOpen
			})

			var hourRanks []rankEntry
			for h := 0; h < 24; h++ {
				if b := byHour[h]; b != nil && b.count > 0 {
					label := fmt.Sprintf("%02d:00 UTC", h)
					hourRanks = append(hourRanks, rankEntry{
						Label:   label,
						Count:   b.count,
						AvgOpen: (b.openSum / float64(b.count)) * 100,
					})
				}
			}
			sort.Slice(hourRanks, func(i, j int) bool {
				return hourRanks[i].AvgOpen > hourRanks[j].AvgOpen
			})

			type bestTimeResult struct {
				ByDay  []rankEntry `json:"by_day"`
				ByHour []rankEntry `json:"by_hour"`
				Note   string      `json:"note"`
			}
			note := fmt.Sprintf("Based on %d sent campaigns", len(campaigns))
			if len(campaigns) < 5 {
				note += " (low sample size — results may not be reliable)"
			}
			result := bestTimeResult{
				ByDay:  dayRanks,
				ByHour: hourRanks,
				Note:   note,
			}

			if flags.asJSON || (!isTerminal(w) && !flags.csv && !flags.quiet && !flags.plain) {
				data, _ := json.Marshal(result)
				return printOutput(w, data, true)
			}

			fmt.Fprintln(w, "")
			fmt.Fprintf(w, "Based on %d campaigns\n\n", len(campaigns))

			fmt.Fprintln(w, "Best days to send:")
			fmt.Fprintf(w, "%-12s %10s %10s\n", "Day", "Campaigns", "Avg Opens%")
			fmt.Fprintln(w, strings.Repeat("─", 36))
			for i, r := range dayRanks {
				marker := ""
				if i == 0 {
					marker = " ← best"
				}
				fmt.Fprintf(w, "%-12s %10d %9.1f%%%s\n", r.Label, r.Count, r.AvgOpen, marker)
			}

			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "Best hours to send (UTC):")
			fmt.Fprintf(w, "%-12s %10s %10s\n", "Hour", "Campaigns", "Avg Opens%")
			fmt.Fprintln(w, strings.Repeat("─", 36))
			limit := 8
			if len(hourRanks) < limit {
				limit = len(hourRanks)
			}
			for i, r := range hourRanks[:limit] {
				marker := ""
				if i == 0 {
					marker = " ← best"
				}
				fmt.Fprintf(w, "%-12s %10d %9.1f%%%s\n", r.Label, r.Count, r.AvgOpen, marker)
			}
			if len(campaigns) < 5 {
				fmt.Fprintln(w, yellow("\nLow sample size — send more campaigns for reliable results."))
			}
			return nil
		},
	}
	return cmd
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case json.Number:
		f, _ := val.Float64()
		return f
	}
	return 0
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
