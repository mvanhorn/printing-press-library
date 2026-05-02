package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newLaunchesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "launches",
		Short: "Calendar and timing analysis for launch slot picking",
	}
	cmd.AddCommand(newLaunchesCalendarCmd(flags))
	return cmd
}

func newLaunchesCalendarCmd(flags *rootFlags) *cobra.Command {
	var (
		topic string
		week  int
		count int
	)
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Show what launched what day in a week, with hour-of-day distribution",
		Long: strings.Trim(`
Pulls launches in a given ISO week and groups them by day-of-week and
hour-of-day. Helps a founder pick a less-crowded slot in their topic.

If --week is omitted the current ISO week is used. Pass --week 18 (no year)
to pick a week in the current year, or --week 2026-W18 for a specific year.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli launches calendar --topic artificial-intelligence
  producthunt-pp-cli launches calendar --topic developer-tools --week 18 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if topic == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			startDate, endDate, label := weekRange(week)
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			vars := map[string]any{
				"first":        count,
				"topic":        topic,
				"order":        "VOTES",
				"postedAfter":  startDate.Format(time.RFC3339),
				"postedBefore": endDate.Format(time.RFC3339),
			}
			var resp phgql.PostsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostsQuery, vars, &resp); err != nil {
				return err
			}
			byDay := map[string][]postBrief{}
			byHour := map[int]int{}
			for _, e := range resp.Posts.Edges {
				dow := e.Node.CreatedAt.Weekday().String()
				byDay[dow] = append(byDay[dow], postBrief{
					Slug: e.Node.Slug, Name: e.Node.Name,
					VotesCount: e.Node.VotesCount, CommentsCount: e.Node.CommentsCount,
					Tagline: e.Node.Tagline, PosterHandle: e.Node.User.Username,
				})
				byHour[e.Node.CreatedAt.Hour()]++
			}
			for k := range byDay {
				sort.Slice(byDay[k], func(i, j int) bool { return byDay[k][i].VotesCount > byDay[k][j].VotesCount })
			}
			out := launchesCalendarOut{
				Topic:     topic,
				Week:      label,
				StartDate: startDate.Format("2006-01-02"),
				EndDate:   endDate.Format("2006-01-02"),
				ByDay:     byDay,
				ByHourUTC: byHour,
				Total:     len(resp.Posts.Edges),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&topic, "topic", "", "Topic slug (required)")
	cmd.Flags().IntVar(&week, "week", 0, "ISO week number in current year (1-53). Omit for the current week.")
	cmd.Flags().IntVar(&count, "count", 20, "Posts to fetch (max 20 per page)")
	return cmd
}

type launchesCalendarOut struct {
	Topic     string                 `json:"topic"`
	Week      string                 `json:"week"`
	StartDate string                 `json:"start_date"`
	EndDate   string                 `json:"end_date"`
	ByDay     map[string][]postBrief `json:"by_day"`
	ByHourUTC map[int]int            `json:"by_hour_utc_count"`
	Total     int                    `json:"total"`
}

func weekRange(week int) (start, end time.Time, label string) {
	now := time.Now().UTC()
	year, isoWeek := now.ISOWeek()
	if week == 0 {
		week = isoWeek
	}
	// Find Monday of week: ISO weeks start on Monday.
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	_, jan4ISOWeek := jan4.ISOWeek()
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	mondayOfWeek1 := jan4.AddDate(0, 0, -(weekday - 1))
	target := mondayOfWeek1.AddDate(0, 0, (week-jan4ISOWeek)*7)
	end = target.AddDate(0, 0, 7)
	label = fmt.Sprintf("%d-W%02d", year, week)
	return target, end, label
}
