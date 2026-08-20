package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newStormTrackCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "storm-track <slug>",
		Short:   "Track storm progression showing when snow starts, peaks, and ends",
		Long:    "Fetches hourly forecast data and identifies contiguous snow periods, showing duration, totals, and peak rates.",
		Example: "  opensnow-pp-cli storm-track vail\n  opensnow-pp-cli storm-track vail --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/forecast/detail/" + slug
			data, err := c.Get(path, map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			data = extractResponseData(data)

			var obj map[string]any
			if err := json.Unmarshal(data, &obj); err != nil {
				return fmt.Errorf("parsing forecast: %w", err)
			}

			type stormPeriod struct {
				Start    string  `json:"start"`
				End      string  `json:"end"`
				Duration string  `json:"duration"`
				Total    float64 `json:"total_snow"`
				PeakHour string  `json:"peak_hour"`
				PeakRate float64 `json:"peak_rate"`
			}

			var storms []stormPeriod
			var hourly []any

			// Try forecast_hourly
			if h, ok := obj["forecast_hourly"].([]any); ok {
				hourly = h
			}

			if len(hourly) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"message": "No hourly data available for storm tracking",
						"slug":    slug,
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No hourly forecast data available. Try: opensnow-pp-cli forecast get-detail "+slug)
				return nil
			}

			// Scan for contiguous snow periods
			inStorm := false
			var current stormPeriod
			var totalSnow float64
			var peakRate float64
			var peakHour string
			var hours int

			for _, h := range hourly {
				hm, ok := h.(map[string]any)
				if !ok {
					continue
				}

				precipSnow := getFloat(hm, "precip_snow", "snow")
				precipType := getFloat(hm, "precip_type")
				timeStr := getString(hm, "time", "datetime", "date")

				isSnowing := precipSnow > 0 || precipType == 1 || precipType == 2

				if isSnowing {
					if !inStorm {
						inStorm = true
						current = stormPeriod{Start: timeStr}
						totalSnow = 0
						peakRate = 0
						peakHour = ""
						hours = 0
					}
					totalSnow += precipSnow
					hours++
					if precipSnow > peakRate {
						peakRate = precipSnow
						peakHour = timeStr
					}
					current.End = timeStr
				} else if inStorm {
					current.Total = totalSnow
					current.PeakHour = peakHour
					current.PeakRate = peakRate
					current.Duration = fmt.Sprintf("%dh", hours)
					storms = append(storms, current)
					inStorm = false
				}
			}
			// Close any open storm
			if inStorm {
				current.Total = totalSnow
				current.PeakHour = peakHour
				current.PeakRate = peakRate
				current.Duration = fmt.Sprintf("%dh", hours)
				storms = append(storms, current)
			}

			if len(storms) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"message": "No snow in the forecast",
						"slug":    slug,
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No snow in the forecast")
				return nil
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), storms, flags)
			}

			headers := []string{"Start -> End", "Duration", "Total Snow", "Peak Hour", "Peak Rate"}
			tableRows := make([][]string, 0, len(storms))
			for _, s := range storms {
				tableRows = append(tableRows, []string{
					fmt.Sprintf("%s -> %s", s.Start, s.End),
					s.Duration,
					fmt.Sprintf("%.1f\"", s.Total),
					s.PeakHour,
					fmt.Sprintf("%.2f\"/hr", s.PeakRate),
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
}
