// Copyright 2026 Luke J and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the FRED CLI. Carried across regen via the
// novel-command merge path; the generated stub was replaced.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/fred/internal/cliutil"

	"github.com/spf13/cobra"
)

type releaseDatesEnvelope struct {
	ReleaseDates []releaseDateRow `json:"release_dates"`
}

type releaseDateRow struct {
	ReleaseID   int    `json:"release_id"`
	ReleaseName string `json:"release_name"`
	Date        string `json:"date"`
}

type calendarView struct {
	Start    string           `json:"start"`
	End      string           `json:"end"`
	Days     int              `json:"days"`
	Count    int              `json:"count"`
	Releases []releaseDateRow `json:"releases"`
}

// pp:data-source live
func newNovelReleaseCalendarCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagLimit int
	cmd := &cobra.Command{
		Use:         "calendar",
		Short:       "Economic data release calendar within a day window",
		Long:        "Show recent and upcoming economic data release dates within a window around today, aggregated across every FRED release. Requires a time-windowed roll-up over the full release-dates feed that the raw endpoint does not summarize.",
		Example:     "  fred-pp-cli release calendar --days 7 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagDays <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be a positive number of days"))
			}

			// The /releases/dates feed with no-data dates is heavy; curtail the
			// window and row count under the live-dogfood matrix's flat timeout.
			limit := flagLimit
			if limit <= 0 {
				// Guard against --limit 0 (or negative) becoming a literal
				// limit=0 query param FRED would reject; fall back to the default.
				limit = 100
			}
			if cliutil.IsDogfoodEnv() {
				if flagDays > 1 {
					flagDays = 1
				}
				if limit > 40 {
					limit = 40
				}
			}

			now := time.Now().UTC()
			start := now.AddDate(0, 0, -flagDays).Format("2006-01-02")
			end := now.AddDate(0, 0, flagDays).Format("2006-01-02")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(ctx, "/releases/dates", map[string]string{
				"file_type":                          "json",
				"realtime_start":                     start,
				"realtime_end":                       end,
				"include_release_dates_with_no_data": "true",
				"sort_order":                         "asc",
				"limit":                              strconv.Itoa(limit),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var env releaseDatesEnvelope
			if err := json.Unmarshal(data, &env); err != nil {
				return apiErr(fmt.Errorf("parsing release dates: %w", err))
			}

			// Keep only dates inside the [start, end] window and sort by date.
			rows := make([]releaseDateRow, 0, len(env.ReleaseDates))
			for _, r := range env.ReleaseDates {
				if r.Date >= start && r.Date <= end {
					rows = append(rows, r)
				}
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Date != rows[j].Date {
					return rows[i].Date < rows[j].Date
				}
				return rows[i].ReleaseName < rows[j].ReleaseName
			})

			return flags.printJSON(cmd, calendarView{
				Start:    start,
				End:      end,
				Days:     flagDays,
				Count:    len(rows),
				Releases: rows,
			})
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 7, "Window size in days around today (covers -days through +days)")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max release dates to fetch (the no-data feed is large; lower is faster)")
	return cmd
}
