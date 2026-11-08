// Copyright 2026 Amandeep Khurana and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/consumer"
)

// commuteWindows expands weekdays into local wall-clock windows, keeping times naive
// so the city's time zone (and any DST change inside the range) is applied per window.
func commuteWindows(from time.Time, days []string, start, end string, weeks int) ([][2]string, error) {
	want := map[time.Weekday]bool{}
	names := map[string]time.Weekday{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}
	for _, d := range days {
		wd, ok := names[strings.ToLower(strings.TrimSpace(d))]
		if !ok {
			return nil, fmt.Errorf("unknown day %q; use mon,tue,wed,thu,fri,sat,sun", d)
		}
		want[wd] = true
	}
	var out [][2]string
	for i := 1; i <= weeks*7; i++ {
		day := from.AddDate(0, 0, i)
		if want[day.Weekday()] {
			date := day.Format("2006-01-02")
			endDate := date
			// HH:MM compares as a string. An end at or before the start is an overnight
			// shift, so it finishes the next morning.
			if end <= start {
				endDate = day.AddDate(0, 0, 1).Format("2006-01-02")
			}
			out = append(out, [2]string{date + "T" + start, endDate + "T" + end})
		}
	}
	return out, nil
}

func newNovelCommuteCmd(flags *rootFlags) *cobra.Command {
	var facilityID, weeks int
	var citySlug, days, start, end string
	cmd := &cobra.Command{
		Use:   "commute",
		Short: "Quote a recurring series of parking windows with a total.",
		Long: "Use this command to quote a recurring series of windows at one garage, starting tomorrow. It never places anything.\n" +
			"Do NOT use this command for one window based on a past reservation; use 'rebook' instead.",
		Example:     "  spothero-pp-cli commute --facility-id 2175 --days mon,wed --start 09:00 --end 17:00 --weeks 1 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "commute")
			}
			if facilityID == 0 || days == "" || start == "" || end == "" {
				return usageErr(fmt.Errorf("--facility-id, --days, --start and --end are required"))
			}
			if weeks < 1 || weeks > 8 {
				return usageErr(fmt.Errorf("--weeks must be between 1 and 8"))
			}
			windows, err := commuteWindows(time.Now(), strings.Split(days, ","), start, end, weeks)
			if err != nil {
				return usageErr(err)
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			rows := []map[string]any{}
			total, unavailable, failed := 0, 0, 0
			for _, w := range windows {
				row := map[string]any{"starts": w[0], "ends": w[1]}
				q, err := c.GetFacilityRates(consumer.FacilityRateQuery{FacilityID: facilityID, Starts: w[0], Ends: w[1], CitySlug: citySlug})
				switch {
				case err != nil:
					row["error"] = err.Error()
					unavailable++
					failed++
				case len(q.Rates) == 0 || !q.Rates[0].Available:
					row["available"] = false
					unavailable++
				default:
					row["available"] = true
					row["price_cents"] = q.Rates[0].PriceCents
					total += q.Rates[0].PriceCents
				}
				rows = append(rows, row)
				// Paced because this is many quotes against an unofficial API.
				time.Sleep(250 * time.Millisecond)
			}
			if err := flags.printJSON(cmd, map[string]any{"facility_id": facilityID, "windows": rows, "total_cents": total, "unavailable": unavailable, "placed": false}); err != nil {
				return err
			}
			// Every window erroring means a bad id or an outage, not an empty plan, and a
			// caller reading only the exit code must see that.
			if failed == len(windows) && len(windows) > 0 {
				return apiErr(fmt.Errorf("every quote failed for facility %d", facilityID))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&facilityID, "facility-id", 0, "Facility id")
	cmd.Flags().StringVar(&citySlug, "city-slug", "", "City slug for the local time zone")
	cmd.Flags().StringVar(&days, "days", "", "Comma separated weekdays, for example mon,wed")
	cmd.Flags().StringVar(&start, "start", "", "Local start time, HH:MM")
	cmd.Flags().StringVar(&end, "end", "", "Local end time, HH:MM")
	cmd.Flags().IntVar(&weeks, "weeks", 1, "How many weeks to quote (1 to 8)")
	return cmd
}
