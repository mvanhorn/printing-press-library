package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/weather-goat/internal/config"

	"github.com/spf13/cobra"
)

func newBreatheCmd(flags *rootFlags) *cobra.Command {
	var flagLat float64
	var flagLon float64

	cmd := &cobra.Command{
		Use:   "breathe [location]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Check air quality, pollen levels, and outdoor exercise safety",
		Long:  "Fetch air quality data including AQI, PM2.5, PM10, UV index, and pollen levels. Provides a recommendation for outdoor activity safety.",
		Example: `  weather-goat-pp-cli breathe
  weather-goat-pp-cli breathe "Denver"
  weather-goat-pp-cli breathe --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "GET /air-quality (Open-Meteo)")
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

			params := map[string]string{
				"latitude":  fmt.Sprintf("%f", lat),
				"longitude": fmt.Sprintf("%f", lon),
				"current":   "us_aqi,pm2_5,pm10,uv_index,alder_pollen,birch_pollen,grass_pollen,ragweed_pollen",
				"timezone":  "auto",
			}

			// Air quality API uses a different subdomain than the forecast API
			data, err := openMeteoGet("https://air-quality-api.open-meteo.com/v1/air-quality", params)
			if err != nil {
				return classifyAPIError(err)
			}

			var resp struct {
				Current struct {
					USAQI         float64 `json:"us_aqi"`
					PM25          float64 `json:"pm2_5"`
					PM10          float64 `json:"pm10"`
					UVIndex       float64 `json:"uv_index"`
					AlderPollen   float64 `json:"alder_pollen"`
					BirchPollen   float64 `json:"birch_pollen"`
					GrassPollen   float64 `json:"grass_pollen"`
					RagweedPollen float64 `json:"ragweed_pollen"`
				} `json:"current"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("parsing air quality data: %w", err)
			}

			aqi := resp.Current.USAQI
			aqiLabel := aqiCategory(aqi)
			recommendation := aqiRecommendation(aqi)

			if flags.asJSON {
				result := map[string]any{
					"location":       locName,
					"us_aqi":         aqi,
					"aqi_category":   aqiLabel,
					"pm2_5":          resp.Current.PM25,
					"pm10":           resp.Current.PM10,
					"uv_index":       resp.Current.UVIndex,
					"recommendation": recommendation,
					"pollen": map[string]any{
						"alder":   resp.Current.AlderPollen,
						"birch":   resp.Current.BirchPollen,
						"grass":   resp.Current.GrassPollen,
						"ragweed": resp.Current.RagweedPollen,
					},
				}
				return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(result), flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s — Air Quality\n", bold(locName))
			fmt.Fprintf(w, "AQI: %.0f (%s)\n", aqi, aqiLabel)
			fmt.Fprintf(w, "PM2.5: %.1f  PM10: %.1f\n", resp.Current.PM25, resp.Current.PM10)
			fmt.Fprintf(w, "UV Index: %.0f\n", resp.Current.UVIndex)

			// Pollen
			hasPollen := resp.Current.AlderPollen > 0 || resp.Current.BirchPollen > 0 ||
				resp.Current.GrassPollen > 0 || resp.Current.RagweedPollen > 0
			if hasPollen {
				fmt.Fprintln(w)
				fmt.Fprintln(w, "Pollen:")
				if resp.Current.AlderPollen > 0 {
					fmt.Fprintf(w, "  Alder: %.0f\n", resp.Current.AlderPollen)
				}
				if resp.Current.BirchPollen > 0 {
					fmt.Fprintf(w, "  Birch: %.0f\n", resp.Current.BirchPollen)
				}
				if resp.Current.GrassPollen > 0 {
					fmt.Fprintf(w, "  Grass: %.0f\n", resp.Current.GrassPollen)
				}
				if resp.Current.RagweedPollen > 0 {
					fmt.Fprintf(w, "  Ragweed: %.0f\n", resp.Current.RagweedPollen)
				}
			}

			fmt.Fprintln(w)
			fmt.Fprintln(w, recommendation)

			return nil
		},
	}

	cmd.Flags().Float64Var(&flagLat, "latitude", 0, "Latitude")
	cmd.Flags().Float64Var(&flagLon, "longitude", 0, "Longitude")

	return cmd
}

func aqiCategory(aqi float64) string {
	switch {
	case aqi <= 50:
		return "Good"
	case aqi <= 100:
		return "Moderate"
	case aqi <= 150:
		return "Unhealthy for Sensitive Groups"
	case aqi <= 200:
		return "Unhealthy"
	case aqi <= 300:
		return "Very Unhealthy"
	default:
		return "Hazardous"
	}
}

func aqiRecommendation(aqi float64) string {
	switch {
	case aqi <= 50:
		return "Safe to exercise outdoors."
	case aqi <= 100:
		return "Generally safe. Sensitive individuals should consider reducing prolonged outdoor exertion."
	case aqi <= 150:
		return "Sensitive groups should limit outdoor exertion."
	default:
		return "Everyone should avoid prolonged outdoor exertion."
	}
}
