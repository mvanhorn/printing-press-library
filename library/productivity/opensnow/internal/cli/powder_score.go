package cli

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/spf13/cobra"
)

func newPowderScoreCmd(flags *rootFlags) *cobra.Command {
	var flagDays int

	cmd := &cobra.Command{
		Use:     "powder-score <slug>",
		Short:   "Rate upcoming days 1-10 for powder quality",
		Long:    "Fetches snow forecast detail and scores each day based on expected snow (40%), snow probability (20%), low wind (20%), and cold temperature (20%).",
		Example: "  opensnow-pp-cli powder-score vail\n  opensnow-pp-cli powder-score vail --days 3",
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

			path := "/forecast/snow-detail/" + slug
			data, err := c.Get(path, map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			data = extractResponseData(data)

			var obj map[string]any
			if err := json.Unmarshal(data, &obj); err != nil {
				return fmt.Errorf("parsing forecast: %w", err)
			}

			type scoreRow struct {
				Day        string  `json:"day"`
				Score      float64 `json:"score"`
				Snow       string  `json:"expected_snow"`
				Pop        string  `json:"pop"`
				Wind       string  `json:"wind"`
				Temp       string  `json:"temp"`
				Conditions string  `json:"conditions"`
			}

			rows := extractPowderScores(obj, flagDays)

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			headers := []string{"Day", "Score", "Expected Snow", "Pop", "Wind", "Temp", "Conditions"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{
					r.Day,
					fmt.Sprintf("%.1f", r.Score),
					r.Snow,
					r.Pop,
					r.Wind,
					r.Temp,
					r.Conditions,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().IntVar(&flagDays, "days", 5, "Number of days to score")
	return cmd
}

// powderScoreRow holds a single day's powder score data.
type powderScoreRow struct {
	Day        string  `json:"day"`
	Score      float64 `json:"score"`
	Snow       string  `json:"expected_snow"`
	Pop        string  `json:"pop"`
	Wind       string  `json:"wind"`
	Temp       string  `json:"temp"`
	Conditions string  `json:"conditions"`
}

// extractPowderScores parses forecast data and computes powder scores for each day.
func extractPowderScores(obj map[string]any, maxDays int) []powderScoreRow {
	var rows []powderScoreRow

	// Try forecast_semi_daily first (pairs of day/night)
	if periods, ok := obj["forecast_semi_daily"].([]any); ok {
		dayCount := 0
		for i := 0; i < len(periods) && dayCount < maxDays; i++ {
			pm, ok := periods[i].(map[string]any)
			if !ok {
				continue
			}
			dayNight, _ := pm["day_night"].(string)
			if dayNight == "night" || dayNight == "Night" {
				continue
			}
			dayCount++

			snow := getFloat(pm, "snow", "precip_snow")
			pop := getFloat(pm, "pop", "precip_probability")
			wind := getFloat(pm, "wind_speed", "wind")
			temp := getFloat(pm, "temp", "temperature")
			conditions := getString(pm, "weather", "condition", "conditions")
			dayLabel := getString(pm, "date", "day_long", "day_short")
			if dayLabel == "" {
				dayLabel = fmt.Sprintf("Day %d", dayCount)
			}

			score := computePowderScore(snow, pop, wind, temp)
			rows = append(rows, powderScoreRow{
				Day:        dayLabel,
				Score:      score,
				Snow:       fmt.Sprintf("%.1f\"", snow),
				Pop:        fmt.Sprintf("%.0f%%", pop),
				Wind:       fmt.Sprintf("%.0f mph", wind),
				Temp:       fmt.Sprintf("%.0f°", temp),
				Conditions: conditions,
			})
		}
	}

	// Fall back to forecast_daily if no semi_daily data
	if len(rows) == 0 {
		if daily, ok := obj["forecast_daily"].([]any); ok {
			for i, d := range daily {
				if i >= maxDays {
					break
				}
				dm, ok := d.(map[string]any)
				if !ok {
					continue
				}
				snow := getFloat(dm, "snow", "precip_snow", "snow_day")
				pop := getFloat(dm, "pop", "precip_probability")
				wind := getFloat(dm, "wind_speed", "wind")
				temp := getFloat(dm, "temp_high", "temp", "temperature")
				conditions := getString(dm, "weather", "condition", "conditions")
				dayLabel := getString(dm, "date", "day_long", "day_short")
				if dayLabel == "" {
					dayLabel = fmt.Sprintf("Day %d", i+1)
				}

				score := computePowderScore(snow, pop, wind, temp)
				rows = append(rows, powderScoreRow{
					Day:        dayLabel,
					Score:      score,
					Snow:       fmt.Sprintf("%.1f\"", snow),
					Pop:        fmt.Sprintf("%.0f%%", pop),
					Wind:       fmt.Sprintf("%.0f mph", wind),
					Temp:       fmt.Sprintf("%.0f°", temp),
					Conditions: conditions,
				})
			}
		}
	}

	return rows
}

// computePowderScore scores a day 1-10 based on:
// - Expected snow (weight 0.4): 12"+ = 10
// - Snow probability (weight 0.2): 100% = 10
// - Low wind (weight 0.2): 0 mph = 10, 40+ mph = 0
// - Cold temp (weight 0.2): 20F or below = 10, 45F+ = 0
func computePowderScore(snow, pop, wind, temp float64) float64 {
	// Snow score: 0-12" maps to 0-10
	snowScore := math.Min(snow/12.0*10.0, 10.0)

	// Pop score: 0-100% maps to 0-10
	popScore := pop / 100.0 * 10.0

	// Wind score: low wind is good. 0mph = 10, 40mph+ = 0
	windScore := math.Max(0, (40.0-wind)/40.0*10.0)

	// Temp score: cold is good for powder. 20F or below = 10, 45F+ = 0
	tempScore := math.Max(0, (45.0-temp)/25.0*10.0)
	tempScore = math.Min(tempScore, 10.0)

	score := snowScore*0.4 + popScore*0.2 + windScore*0.2 + tempScore*0.2
	return math.Round(score*10) / 10
}

// getFloat extracts a float64 from a map, trying multiple keys.
func getFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k].(float64); ok {
			return v
		}
	}
	return 0
}

// getString extracts a string from a map, trying multiple keys.
func getString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}
