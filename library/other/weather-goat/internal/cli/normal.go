package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/weather-goat/internal/config"

	"github.com/spf13/cobra"
)

func newNormalCmd(flags *rootFlags) *cobra.Command {
	var flagLat float64
	var flagLon float64

	cmd := &cobra.Command{
		Use:   "normal [location]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Compare today's temperature to the historical average for this date",
		Long:  "Fetches today's conditions and compares them to the average temperature for the same date across recent years (proxy for 30-year climate normals).",
		Example: `  weather-goat-pp-cli normal
  weather-goat-pp-cli normal "Chicago"
  weather-goat-pp-cli normal --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "GET /forecast + /archive (Open-Meteo)")
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			lat, lon, locName, err := resolveLocation(cfg, flagLat, flagLon,
				cmd.Flags().Changed("latitude"), cmd.Flags().Changed("longitude"), args)
			if err != nil {
				return err
			}

			c, clientErr := flags.newClient()
			if clientErr != nil {
				return clientErr
			}

			// Get today's temperature
			todayParams := map[string]string{
				"latitude":         fmt.Sprintf("%f", lat),
				"longitude":        fmt.Sprintf("%f", lon),
				"current":          "temperature_2m",
				"timezone":         "auto",
				"temperature_unit": "fahrenheit",
			}

			todayData, err := c.Get("/forecast", todayParams)
			if err != nil {
				return classifyAPIError(err)
			}

			var todayResp struct {
				Current struct {
					Temperature float64 `json:"temperature_2m"`
				} `json:"current"`
			}
			if err := json.Unmarshal(todayData, &todayResp); err != nil {
				return fmt.Errorf("parsing today's forecast: %w", err)
			}
			todayTemp := todayResp.Current.Temperature

			// Fetch historical averages: same day across last 5 years
			now := time.Now()
			month := now.Month()
			day := now.Day()
			currentYear := now.Year()

			var historicalTemps []float64
			for y := currentYear - 5; y < currentYear; y++ {
				date := fmt.Sprintf("%d-%02d-%02d", y, month, day)
				histParams := map[string]string{
					"latitude":         fmt.Sprintf("%f", lat),
					"longitude":        fmt.Sprintf("%f", lon),
					"start_date":       date,
					"end_date":         date,
					"daily":            "temperature_2m_mean",
					"timezone":         "auto",
					"temperature_unit": "fahrenheit",
				}
				histData, err := openMeteoGet("https://archive-api.open-meteo.com/v1/archive", histParams)
				if err != nil {
					continue // skip years we can't fetch
				}
				var histResp struct {
					Daily struct {
						TempMean []float64 `json:"temperature_2m_mean"`
					} `json:"daily"`
				}
				if json.Unmarshal(histData, &histResp) == nil && len(histResp.Daily.TempMean) > 0 {
					historicalTemps = append(historicalTemps, histResp.Daily.TempMean[0])
				}
			}

			if len(historicalTemps) == 0 {
				return fmt.Errorf("could not fetch historical data for comparison")
			}

			var sum float64
			for _, t := range historicalTemps {
				sum += t
			}
			avgTemp := sum / float64(len(historicalTemps))
			diff := todayTemp - avgTemp

			dateStr := fmt.Sprintf("%s %d", month.String(), day)

			if flags.asJSON {
				result := map[string]any{
					"location":          locName,
					"date":              dateStr,
					"today_temperature": todayTemp,
					"average":           avgTemp,
					"difference":        diff,
					"years_sampled":     len(historicalTemps),
				}
				return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(result), flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", bold(locName))
			fmt.Fprintf(w, "Today: %.0f°F\n", todayTemp)
			fmt.Fprintf(w, "Average for %s: %.0f°F (%d-year sample)\n", dateStr, avgTemp, len(historicalTemps))

			if diff > 0 {
				fmt.Fprintf(w, "+%.0f°F above average\n", diff)
			} else if diff < 0 {
				fmt.Fprintf(w, "%.0f°F below average\n", diff)
			} else {
				fmt.Fprintln(w, "Right at the average")
			}

			return nil
		},
	}

	cmd.Flags().Float64Var(&flagLat, "latitude", 0, "Latitude")
	cmd.Flags().Float64Var(&flagLon, "longitude", 0, "Longitude")

	return cmd
}
