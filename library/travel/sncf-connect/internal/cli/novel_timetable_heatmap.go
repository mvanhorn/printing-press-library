// Copyright 2026 jmbernabotto and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newTimetableCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "timetable",
		Short: "Timetable analysis for lines and stops",
	}
	cmd.AddCommand(newTimetableHeatmapCmd(flags))
	return cmd
}

func newTimetableHeatmapCmd(flags *rootFlags) *cobra.Command {
	var lineURI, stopURI, coverage, date string

	cmd := &cobra.Command{
		Use:   "heatmap",
		Short: "ASCII bar chart of train frequency per hour at a stop",
		Long: `Fetches the full-day stop schedule for a line at a stop, buckets
departures by hour, and renders an ASCII bar chart with peak annotation.

Useful for understanding frequency patterns for commute planning.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  sncf-connect-pp-cli timetable heatmap --line "line:SNCF:D" --stop "stop_area:SNCF:87686006"
  sncf-connect-pp-cli timetable heatmap --line "line:SNCF:D" --stop "stop_area:SNCF:87686006" --date 20260601 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if lineURI == "" || stopURI == "" {
				return fmt.Errorf("--line and --stop are required")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if date == "" {
				date = time.Now().Format("20060102")
			}

			path := fmt.Sprintf("/coverage/%s/stop_areas/%s/stop_schedules", coverage, stopURI)
			params := map[string]string{
				"from_datetime":      date + "T000000",
				"until_datetime":     date + "T235959",
				"items_per_schedule": "1000",
				"filter":             "line.uri=" + lineURI,
			}

			data, _, err := resolveRead(cmd.Context(), c, flags, "stop_schedules", false, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			buckets, peak, err := buildHourlyBuckets(data)
			if err != nil {
				return fmt.Errorf("building heatmap: %w", err)
			}

			if flags.asJSON {
				type hourBucket struct {
					Hour  int  `json:"hour"`
					Count int  `json:"count"`
					Peak  bool `json:"peak"`
				}
				var out []hourBucket
				for h := 0; h < 24; h++ {
					out = append(out, hourBucket{Hour: h, Count: buckets[h], Peak: h == peak})
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"line":      lineURI,
					"stop":      stopURI,
					"date":      date,
					"coverage":  coverage,
					"peak_hour": peak,
					"buckets":   out,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Departures per hour — %s at %s (%s)\n\n", lineURI, stopURI, date)

			maxCount := 0
			for _, v := range buckets {
				if v > maxCount {
					maxCount = v
				}
			}
			if maxCount == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  No departures found for this date.")
				return nil
			}

			barWidth := 30
			for h := 0; h < 24; h++ {
				count := buckets[h]
				barLen := 0
				if maxCount > 0 {
					barLen = (count * barWidth) / maxCount
				}
				bar := strings.Repeat("█", barLen)
				peakMarker := ""
				if h == peak {
					peakMarker = " ← peak"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %02d:00  %-30s %3d%s\n", h, bar, count, peakMarker)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&lineURI, "line", "", "Line URI (e.g. line:OCE:TGV)")
	cmd.Flags().StringVar(&stopURI, "stop", "", "Stop area URI (e.g. stop_area:OCE:SA:87391003)")
	cmd.Flags().StringVar(&coverage, "coverage", "sncf", "Navitia coverage region")
	cmd.Flags().StringVar(&date, "date", "", "Date in YYYYMMDD format (default: today)")
	return cmd
}

func buildHourlyBuckets(data json.RawMessage) (buckets [24]int, peakHour int, err error) {
	var resp map[string]any
	if jsonErr := json.Unmarshal(data, &resp); jsonErr != nil {
		return buckets, 0, jsonErr
	}

	schedules, _ := resp["stop_schedules"].([]any)
	if len(schedules) == 0 {
		var list []map[string]any
		if json.Unmarshal(data, &list) == nil {
			for _, s := range list {
				schedules = append(schedules, s)
			}
		}
	}

	for _, sch := range schedules {
		s, _ := sch.(map[string]any)
		dateTimes, _ := s["date_times"].([]any)
		for _, dtEntry := range dateTimes {
			dt, _ := dtEntry.(map[string]any)
			dtStr, _ := dt["date_time"].(string)
			// Format: YYYYMMDDTHHmmss
			if len(dtStr) >= 11 {
				hour := 0
				fmt.Sscanf(dtStr[9:11], "%d", &hour)
				if hour >= 0 && hour < 24 {
					buckets[hour]++
				}
			}
		}
	}

	maxVal := 0
	for h, v := range buckets {
		if v > maxVal {
			maxVal = v
			peakHour = h
		}
	}
	return buckets, peakHour, nil
}
