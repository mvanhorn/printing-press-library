package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const analyticsBase = "https://youtubeanalytics.googleapis.com/v2/reports"

func runAnalyticsReport(flags *rootFlags, params map[string]string) (json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	u, _ := url.Parse(analyticsBase)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return c.Get(u.String(), nil)
}

func defaultDateRange(days int) (string, string) {
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -days)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

// ---- retention-leaderboard ----

func newRetentionLeaderboardCmd(flags *rootFlags) *cobra.Command {
	var days int
	var limit int
	cmd := &cobra.Command{
		Use:   "retention-leaderboard",
		Short: "Top videos by average view percentage (Analytics API)",
		Long: `Top videos by average view percentage from the YouTube Analytics API.

Statistical floor: with fewer than ~30 videos in the lookback window the
ranking is noisy — averageViewPercentage swings on small denominators.
Treat short-tail results as directional, not as definitive winners/losers.`,
		Example:     "  youtube-creator-analytics-pp-cli retention-leaderboard --days 90 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			start, end := defaultDateRange(days)
			data, err := runAnalyticsReport(flags, map[string]string{
				"ids":        "channel==MINE",
				"startDate":  start,
				"endDate":    end,
				"metrics":    "averageViewPercentage,averageViewDuration,views",
				"dimensions": "video",
				"sort":       "-averageViewPercentage",
				"maxResults": fmt.Sprintf("%d", limit),
			})
			if err != nil {
				return fmt.Errorf("analytics report: %w (ensure `youtube-creator-analytics-pp-cli auth login` was run)", err)
			}
			return flags.printJSON(cmd, json.RawMessage(data))
		},
	}
	cmd.Flags().IntVar(&days, "days", 28, "Lookback window in days")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max videos")
	return cmd
}

// ---- sub-velocity ----

func newSubVelocityCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:         "sub-velocity",
		Short:       "Daily subscriber gain/loss rate from Analytics",
		Example:     "  youtube-creator-analytics-pp-cli sub-velocity --days 28 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			start, end := defaultDateRange(days)
			data, err := runAnalyticsReport(flags, map[string]string{
				"ids":        "channel==MINE",
				"startDate":  start,
				"endDate":    end,
				"metrics":    "subscribersGained,subscribersLost,views",
				"dimensions": "day",
				"sort":       "day",
			})
			if err != nil {
				return fmt.Errorf("analytics report: %w", err)
			}
			return flags.printJSON(cmd, json.RawMessage(data))
		},
	}
	cmd.Flags().IntVar(&days, "days", 28, "Lookback window in days")
	return cmd
}

// ---- ctr-cohort ----

func newCTRCohortCmd(flags *rootFlags) *cobra.Command {
	var days int
	var groupBy string
	cmd := &cobra.Command{
		Use:         "ctr-cohort",
		Short:       "Impressions and click-through-rate per video (Analytics API)",
		Example:     "  youtube-creator-analytics-pp-cli ctr-cohort --days 90 --json --select rows",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			start, end := defaultDateRange(days)
			dim := "video"
			if groupBy != "" && strings.ToLower(groupBy) == "day" {
				dim = "day"
			}
			data, err := runAnalyticsReport(flags, map[string]string{
				"ids":        "channel==MINE",
				"startDate":  start,
				"endDate":    end,
				"metrics":    "cardImpressions,cardClicks,cardClickRate,views",
				"dimensions": dim,
				"sort":       "-cardClickRate",
				"maxResults": "50",
			})
			if err != nil {
				return fmt.Errorf("analytics report: %w", err)
			}
			return flags.printJSON(cmd, json.RawMessage(data))
		},
	}
	cmd.Flags().IntVar(&days, "days", 90, "Lookback window in days")
	cmd.Flags().StringVar(&groupBy, "group-by", "video", "Group dimension: video | day")
	return cmd
}
