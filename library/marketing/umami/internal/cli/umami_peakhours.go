// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored composite: publishing-window recommendation from the 7x24
// sessions heatmap, absorbed from frontedu's get_peak_hours_analysis.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var weekdayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// pp:data-source live
func newPeakHoursCmd(flags *rootFlags) *cobra.Command {
	var period string
	var timezone string
	var top int
	cmd := &cobra.Command{
		Use:   "peak-hours [site]",
		Short: "Find the day/hour windows when visitors are most active — the best publishing slots",
		Long: `Read the 7x24 sessions heatmap and rank day-of-week x hour windows by
visitor activity. The top windows are when publishing or posting reaches
the most engaged audience.`,
		Example:     "  umami-pp-cli peak-hours restodom.fr --period 30d --timezone Europe/Paris",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=30d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank day/hour windows from the sessions heatmap")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site argument (UUID or domain) is required"))
			}
			if top < 1 {
				return usageErr(fmt.Errorf("--top must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			websiteID, err := resolveWebsiteID(ctx, c, args[0])
			if err != nil {
				return err
			}
			w, err := parsePeriod(period, time.Now())
			if err != nil {
				return usageErr(err)
			}
			params := map[string]string{
				"startAt":  strconv.FormatInt(w.StartMs, 10),
				"endAt":    strconv.FormatInt(w.EndMs, 10),
				"timezone": timezone,
			}
			data, err := c.Get(ctx, "/api/websites/"+websiteID+"/sessions/weekly", params)
			if err != nil {
				return apiErr(fmt.Errorf("fetching weekly heatmap: %w", err))
			}
			var grid [][]float64
			if err := json.Unmarshal(data, &grid); err != nil {
				return apiErr(fmt.Errorf("parsing weekly heatmap: %w", err))
			}
			type slot struct {
				Day      string  `json:"day"`
				Hour     int     `json:"hour"`
				Sessions float64 `json:"sessions"`
			}
			var slots []slot
			var total float64
			for d, hours := range grid {
				for h, v := range hours {
					if d < len(weekdayNames) {
						slots = append(slots, slot{Day: weekdayNames[d], Hour: h, Sessions: v})
						total += v
					}
				}
			}
			if total == 0 {
				out := map[string]any{"website": websiteID, "period": period, "peak_windows": []slot{},
					"note": "no session activity in this period"}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			sort.Slice(slots, func(i, j int) bool { return slots[i].Sessions > slots[j].Sessions })
			if len(slots) > top {
				slots = slots[:top]
			}
			out := map[string]any{
				"website":        websiteID,
				"period":         period,
				"timezone":       timezone,
				"peak_windows":   slots,
				"recommendation": fmt.Sprintf("publish around %s %02d:00 (%s)", slots[0].Day, slots[0].Hour, timezone),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&period, "period", "30d", "Period (7d, 30d, this-month, ...)")
	cmd.Flags().StringVar(&timezone, "timezone", "UTC", "IANA timezone for hour bucketing")
	cmd.Flags().IntVar(&top, "top", 5, "Number of peak windows to return")
	return cmd
}
