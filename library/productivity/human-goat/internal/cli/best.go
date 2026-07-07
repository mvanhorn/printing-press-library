// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/pricing"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/source/taskrabbit"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newNovelBestCmd(flags *rootFlags) *cobra.Command {
	var flagOn string
	var flagMinRating float64
	var flagMaxRate float64
	var flagLat float64
	var flagLng float64
	var flagState string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "best <job-query>",
		Short:       "Folds TaskRabbit's hidden service (~15%) and trust-and-support (5-15%) fees into the displayed hourly rate",
		Example:     "human-goat-pp-cli best help moving --on saturday --min-rating 4.9 --lat 37.7749 --lng -122.4194 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !commandHasChangedFlags(cmd) {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if dryRunOK(flags) {
				if query == "" {
					query = "<job-query>"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would rank Taskers for %s\n", query)
				return nil
			}
			if query == "" {
				return usageErr(fmt.Errorf("missing job-query"))
			}
			if !cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lng") {
				return usageErr(fmt.Errorf("pass --lat and --lng for your location"))
			}
			if flagLimit < 0 {
				return usageErr(fmt.Errorf("--limit must be non-negative"))
			}

			date, err := parseOnDate(flagOn)
			if err != nil {
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			category, err := resolveTaskRabbitCategory(ctx, c, query)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			tr := taskrabbit.New(c)
			rows, err := rankedTaskRabbitRecommendations(ctx, tr, category, taskrabbitRankOptions{
				Date:      date,
				MinRating: flagMinRating,
				MaxRate:   flagMaxRate,
				Lat:       flagLat,
				Lng:       flagLng,
				State:     flagState,
				Limit:     flagLimit,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printTaskerRankRows(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&flagOn, "on", "", "Date to search: YYYY-MM-DD, today, tomorrow, or weekday")
	cmd.Flags().Float64Var(&flagMinRating, "min-rating", 0, "Minimum Tasker rating")
	cmd.Flags().Float64Var(&flagMaxRate, "max-rate", 0, "Maximum all-in hourly rate in dollars (0 for no ceiling)")
	cmd.Flags().Float64Var(&flagLat, "lat", 0, "Latitude for TaskRabbit recommendations")
	cmd.Flags().Float64Var(&flagLng, "lng", 0, "Longitude for TaskRabbit recommendations")
	cmd.Flags().StringVar(&flagState, "state", "", "State for CA/MA service-fee-only pricing rule")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum number of Taskers to return")
	return cmd
}

type taskrabbitCategoryMatch struct {
	Title             string `json:"title"`
	CategoryName      string `json:"category_name"`
	CategoryID        int    `json:"category_id"`
	DefaultTemplateID int    `json:"default_template_id"`
}

type taskrabbitRankOptions struct {
	Date      string
	MinRating float64
	MaxRate   float64
	Lat       float64
	Lng       float64
	State     string
	Limit     int
}

type taskerRankRow struct {
	Name             string  `json:"name"`
	AllInHourly      float64 `json:"all_in_hourly"`
	BaseHourly       float64 `json:"base_hourly"`
	Rating           float64 `json:"rating"`
	Reviews          int     `json:"reviews"`
	TasksComplete    int     `json:"tasks_completed"`
	Elite            bool    `json:"elite"`
	NextAvailable    string  `json:"next_available"`
	IsFavorite       bool    `json:"is_favorite"`
	RecommendationID string  `json:"recommendation_id,omitempty"`
	allInCents       int
	baseCents        int
}

func parseOnDate(s string) (string, error) {
	clean := strings.ToLower(strings.TrimSpace(s))
	now := time.Now()
	if clean == "" || clean == "today" {
		return now.Format("2006-01-02"), nil
	}
	if clean == "tomorrow" {
		return now.AddDate(0, 0, 1).Format("2006-01-02"), nil
	}
	if parsed, err := time.Parse("2006-01-02", clean); err == nil {
		return parsed.Format("2006-01-02"), nil
	}

	weekdays := map[string]time.Weekday{
		"sunday":    time.Sunday,
		"sun":       time.Sunday,
		"monday":    time.Monday,
		"mon":       time.Monday,
		"tuesday":   time.Tuesday,
		"tue":       time.Tuesday,
		"tues":      time.Tuesday,
		"wednesday": time.Wednesday,
		"wed":       time.Wednesday,
		"thursday":  time.Thursday,
		"thu":       time.Thursday,
		"thur":      time.Thursday,
		"thurs":     time.Thursday,
		"friday":    time.Friday,
		"fri":       time.Friday,
		"saturday":  time.Saturday,
		"sat":       time.Saturday,
	}
	target, ok := weekdays[clean]
	if !ok {
		return "", fmt.Errorf("invalid --on %q: expected YYYY-MM-DD, today, tomorrow, or weekday", s)
	}
	daysAhead := (int(target) - int(now.Weekday()) + 7) % 7
	if daysAhead == 0 {
		daysAhead = 7
	}
	return now.AddDate(0, 0, daysAhead).Format("2006-01-02"), nil
}

func resolveTaskRabbitCategory(ctx context.Context, c *client.Client, query string) (taskrabbitCategoryMatch, error) {
	body, err := c.Get(ctx, "/api/v3/web-client/metro_task_template.json", nil)
	if err != nil {
		return taskrabbitCategoryMatch{}, err
	}
	var decoded struct {
		InitialTaskTemplates []taskrabbitCategoryMatch `json:"initial_task_templates"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return taskrabbitCategoryMatch{}, fmt.Errorf("decode TaskRabbit task templates: %w", err)
	}

	needle := strings.ToLower(strings.TrimSpace(query))
	titles := make([]string, 0, len(decoded.InitialTaskTemplates))
	for _, template := range decoded.InitialTaskTemplates {
		if template.Title != "" {
			titles = append(titles, template.Title)
		}
		if taskrabbitCategoryMatches(needle, template.Title, template.CategoryName) {
			return template, nil
		}
	}
	sort.Strings(titles)
	return taskrabbitCategoryMatch{}, usageErr(fmt.Errorf("no TaskRabbit category matched %q (available titles: %s)", query, strings.Join(titles, ", ")))
}

func taskrabbitCategoryMatches(query, title, categoryName string) bool {
	if query == "" {
		return false
	}
	title = strings.ToLower(strings.TrimSpace(title))
	categoryName = strings.ToLower(strings.TrimSpace(categoryName))
	return strings.Contains(title, query) ||
		strings.Contains(categoryName, query) ||
		(title != "" && strings.Contains(query, title)) ||
		(categoryName != "" && strings.Contains(query, categoryName))
}

func rankedTaskRabbitRecommendations(ctx context.Context, tr *taskrabbit.Client, category taskrabbitCategoryMatch, opts taskrabbitRankOptions) ([]taskerRankRow, error) {
	taskers, _, recommendationID, err := tr.Recommendations(ctx, taskrabbit.BuildRecommendationsInput(category.CategoryID, category.DefaultTemplateID, opts.Lat, opts.Lng, []string{opts.Date}))
	if err != nil {
		return nil, err
	}

	rows := make([]taskerRankRow, 0, len(taskers))
	for _, tasker := range taskers {
		if opts.MinRating > 0 && tasker.RabbitRating < opts.MinRating {
			continue
		}
		breakdown := pricing.AllIn(tasker.PosterHourlyRateCents, opts.State)
		allInHourly := float64(breakdown.AllInCents) / 100.0
		if opts.MaxRate > 0 && allInHourly > opts.MaxRate {
			continue
		}
		rows = append(rows, taskerRankRow{
			Name:             taskerDisplayName(tasker),
			AllInHourly:      allInHourly,
			BaseHourly:       float64(breakdown.BaseCents) / 100.0,
			Rating:           tasker.RabbitRating,
			Reviews:          tasker.RabbitReviews,
			TasksComplete:    tasker.CategoryInvoicesCount,
			Elite:            tasker.Elite,
			NextAvailable:    tasker.NextAvailableAt,
			IsFavorite:       tasker.IsFavorite,
			RecommendationID: recommendationID,
			allInCents:       breakdown.AllInCents,
			baseCents:        breakdown.BaseCents,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].allInCents != rows[j].allInCents {
			return rows[i].allInCents < rows[j].allInCents
		}
		if rows[i].Rating != rows[j].Rating {
			return rows[i].Rating > rows[j].Rating
		}
		return rows[i].TasksComplete > rows[j].TasksComplete
	})
	if opts.Limit > 0 && len(rows) > opts.Limit {
		rows = rows[:opts.Limit]
	}
	return rows, nil
}

func taskerDisplayName(tasker taskrabbit.Tasker) string {
	if strings.TrimSpace(tasker.DisplayName) != "" {
		return tasker.DisplayName
	}
	return tasker.FirstName
}

func printTaskerRankRows(cmd *cobra.Command, flags *rootFlags, rows []taskerRankRow) error {
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no qualifying Taskers after filtering")
		return nil
	}
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, []string{
			row.Name,
			pricing.FormatCents(row.allInCents),
			pricing.FormatCents(row.baseCents),
			fmt.Sprintf("%.2f", row.Rating),
			fmt.Sprintf("%d", row.Reviews),
			fmt.Sprintf("%d", row.TasksComplete),
			fmt.Sprintf("%t", row.Elite),
			row.NextAvailable,
			fmt.Sprintf("%t", row.IsFavorite),
		})
	}
	return flags.printTable(cmd, []string{"NAME", "ALL-IN/HR", "BASE/HR", "RATING", "REVIEWS", "TASKS", "ELITE", "NEXT", "FAVORITE"}, tableRows)
}

func printTaskerRankRow(cmd *cobra.Command, flags *rootFlags, row taskerRankRow) error {
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), row, flags)
	}
	return printTaskerRankRows(cmd, flags, []taskerRankRow{row})
}

func commandHasChangedFlags(cmd *cobra.Command) bool {
	changed := false
	visit := func(_ *pflag.Flag) {
		changed = true
	}
	cmd.Flags().Visit(visit)
	cmd.InheritedFlags().Visit(visit)
	return changed
}
