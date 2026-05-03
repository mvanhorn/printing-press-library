package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/open-meteo/internal/openmeteo"
	"github.com/mvanhorn/printing-press-library/library/other/open-meteo/internal/wmocode"
)

func newPanelCmd(flags *rootFlags) *cobra.Command {
	var (
		place        string
		latitude     string
		longitude    string
		current      string
		hourly       string
		daily        string
		humanize     bool
		forecastDays int
		timezone     string
	)
	cmd := &cobra.Command{
		Use:   "panel",
		Short: "Multi-location side-by-side weather panel",
		Long: strings.TrimSpace(`
Fetch current/hourly/daily weather for multiple locations in a single batched
Open-Meteo call when possible (the API natively accepts comma-separated lat/lon).
Names provided via --place are geocoded one-by-one, then the actual forecast
fetch happens in a single batched HTTP request.
`),
		Example: strings.Trim(`
  open-meteo-pp-cli panel --place Seattle,Berlin,Tokyo --current temperature_2m,weather_code --json
  open-meteo-pp-cli panel --place "Mavericks, CA,Pipeline, HI" --current temperature_2m,wind_speed_10m --json
  open-meteo-pp-cli panel --latitude 47.6,52.5,35.7 --longitude -122.3,13.4,139.7 --current temperature_2m --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			places, err := openmeteo.CoordsFromFlags(c, place, latitude, longitude)
			if err != nil {
				return err
			}
			if current == "" && hourly == "" && daily == "" {
				current = "temperature_2m,weather_code,wind_speed_10m"
			}
			lats := make([]string, len(places))
			lons := make([]string, len(places))
			for i, p := range places {
				lats[i] = strconv.FormatFloat(p.Latitude, 'f', -1, 64)
				lons[i] = strconv.FormatFloat(p.Longitude, 'f', -1, 64)
			}
			params := map[string]string{
				"latitude":  strings.Join(lats, ","),
				"longitude": strings.Join(lons, ","),
			}
			if current != "" {
				params["current"] = current
			}
			if hourly != "" {
				params["hourly"] = hourly
			}
			if daily != "" {
				params["daily"] = daily
			}
			if forecastDays > 0 {
				params["forecast_days"] = strconv.Itoa(forecastDays)
			}
			if timezone != "" {
				params["timezone"] = timezone
			}
			raw, err := c.Get("https://api.open-meteo.com/v1/forecast", params)
			if err != nil {
				return fmt.Errorf("fetching panel: %w", err)
			}
			// Open-Meteo returns either a single object (1 location) or an
			// array of objects (multi-location). Normalize to an array.
			var results []json.RawMessage
			trimmed := strings.TrimSpace(string(raw))
			if strings.HasPrefix(trimmed, "[") {
				if err := json.Unmarshal(raw, &results); err != nil {
					return fmt.Errorf("parsing array response: %w", err)
				}
			} else {
				results = []json.RawMessage{raw}
			}
			view := make([]map[string]any, 0, len(places))
			for i := 0; i < len(places) && i < len(results); i++ {
				var entry map[string]any
				if err := json.Unmarshal(results[i], &entry); err != nil {
					return fmt.Errorf("parsing entry %d: %w", i, err)
				}
				row := map[string]any{
					"place":     places[i].Name,
					"latitude":  places[i].Latitude,
					"longitude": places[i].Longitude,
					"timezone":  entry["timezone"],
					"current":   entry["current"],
					"hourly":    entry["hourly"],
					"daily":     entry["daily"],
				}
				if humanize {
					if cur, ok := entry["current"].(map[string]any); ok {
						if codeF, ok := cur["weather_code"].(float64); ok {
							cur["weather_description"] = wmocode.Describe(int(codeF))
							row["current"] = cur
						}
					}
				}
				view = append(view, row)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&place, "place", "", "Comma-separated place names (e.g., \"Seattle,Berlin,Tokyo\"). Disambiguate with admin context (\"Springfield, IL\").")
	cmd.Flags().StringVar(&latitude, "latitude", "", "Comma-separated latitudes (alternative to --place).")
	cmd.Flags().StringVar(&longitude, "longitude", "", "Comma-separated longitudes (alternative to --place).")
	cmd.Flags().StringVar(&current, "current", "", "Comma-separated current variables (default temperature_2m,weather_code,wind_speed_10m when no other selection given).")
	cmd.Flags().StringVar(&hourly, "hourly", "", "Comma-separated hourly variables.")
	cmd.Flags().StringVar(&daily, "daily", "", "Comma-separated daily variables.")
	cmd.Flags().IntVar(&forecastDays, "forecast-days", 0, "Forecast days (1-16). Defaults to 7 if unset.")
	cmd.Flags().StringVar(&timezone, "timezone", "auto", "Named timezone, 'auto' (per-location), or UTC.")
	cmd.Flags().BoolVar(&humanize, "humanize", false, "Add human-readable WMO weather descriptions to current output.")
	return cmd
}
