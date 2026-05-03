package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/open-meteo/internal/openmeteo"
)

func newIsGoodForCmd(flags *rootFlags) *cobra.Command {
	var (
		place     string
		latitude  string
		longitude string
	)
	cmd := &cobra.Command{
		Use:   "is-good-for [activity]",
		Short: "GO / CAUTION / STOP verdict for an outdoor activity",
		Long: strings.TrimSpace(`
Combine forecast, marine, and air-quality endpoints to render a verdict for a
named outdoor activity. Activity options:

  surfing   wave height 1-5 m, wind <30 km/h offshore preferred, AQI <100
  hiking    no rain, wind <40 km/h, AQI <100, temperature -5 to 30 C
  running   no rain, wind <30 km/h, AQI <50, temperature -5 to 28 C
  biking    no rain, wind <30 km/h, AQI <100, temperature -5 to 30 C
  skiing    snowfall present, wind <40 km/h, temperature <2 C

Verdict is one of GO, CAUTION, STOP, plus the underlying signals so an agent
can override the verdict with custom thresholds.
`),
		Example: strings.Trim(`
  open-meteo-pp-cli is-good-for surfing --place "Mavericks, CA" --json
  open-meteo-pp-cli is-good-for hiking --place "Mt Rainier, WA" --json
  open-meteo-pp-cli is-good-for running --place Seattle --json
`, "\n"),
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			activity := strings.ToLower(args[0])
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			places, err := openmeteo.CoordsFromFlags(c, place, latitude, longitude)
			if err != nil {
				return err
			}
			p := places[0]
			latStr := strconv.FormatFloat(p.Latitude, 'f', -1, 64)
			lonStr := strconv.FormatFloat(p.Longitude, 'f', -1, 64)
			signals := map[string]any{}

			// Forecast: temperature, wind, precipitation, weather code
			fcRaw, err := c.Get("https://api.open-meteo.com/v1/forecast", map[string]string{
				"latitude":      latStr,
				"longitude":     lonStr,
				"current":       "temperature_2m,wind_speed_10m,precipitation,weather_code,snowfall",
				"timezone":      "auto",
				"forecast_days": "1",
			})
			if err != nil {
				return fmt.Errorf("fetching forecast: %w", err)
			}
			var fc struct {
				Current map[string]any `json:"current"`
			}
			_ = json.Unmarshal(fcRaw, &fc)
			signals["forecast_current"] = fc.Current

			// Air quality: AQI for biking/running/hiking/surfing
			if activity != "skiing" {
				aqRaw, err := c.Get("https://air-quality-api.open-meteo.com/v1/air-quality", map[string]string{
					"latitude":  latStr,
					"longitude": lonStr,
					"current":   "european_aqi,us_aqi,pm2_5,pm10,uv_index",
					"timezone":  "auto",
				})
				if err == nil {
					var aq struct {
						Current map[string]any `json:"current"`
					}
					if json.Unmarshal(aqRaw, &aq) == nil {
						signals["air_quality_current"] = aq.Current
					}
				}
			}

			// Marine: wave height for surfing
			if activity == "surfing" {
				mRaw, err := c.Get("https://marine-api.open-meteo.com/v1/marine", map[string]string{
					"latitude":  latStr,
					"longitude": lonStr,
					"current":   "wave_height,wave_period,wave_direction",
					"timezone":  "auto",
				})
				if err == nil {
					var m struct {
						Current map[string]any `json:"current"`
					}
					if json.Unmarshal(mRaw, &m) == nil {
						signals["marine_current"] = m.Current
					}
				}
			}

			verdict, reasons := scoreActivity(activity, signals)
			view := map[string]any{
				"place":     p.Name,
				"latitude":  p.Latitude,
				"longitude": p.Longitude,
				"activity":  activity,
				"verdict":   verdict,
				"reasons":   reasons,
				"signals":   signals,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&place, "place", "", "Place name (e.g., \"Mavericks, CA\").")
	cmd.Flags().StringVar(&latitude, "latitude", "", "WGS84 latitude (alternative to --place).")
	cmd.Flags().StringVar(&longitude, "longitude", "", "WGS84 longitude (alternative to --place).")
	return cmd
}

// scoreActivity returns a verdict ("GO", "CAUTION", "STOP", or "UNKNOWN")
// plus a list of human-readable reasons that contributed to it. Thresholds are
// per-activity; anything outside the supported set returns UNKNOWN.
func scoreActivity(activity string, signals map[string]any) (string, []string) {
	current, _ := signals["forecast_current"].(map[string]any)
	aq, _ := signals["air_quality_current"].(map[string]any)
	marine, _ := signals["marine_current"].(map[string]any)
	temp := numAt(current, "temperature_2m")
	wind := numAt(current, "wind_speed_10m")
	precip := numAt(current, "precipitation")
	snow := numAt(current, "snowfall")
	euAqi := numAt(aq, "european_aqi")
	usAqi := numAt(aq, "us_aqi")
	wave := numAt(marine, "wave_height")
	var reasons []string
	add := func(r string) { reasons = append(reasons, r) }
	stop := false
	caution := false
	switch activity {
	case "surfing":
		if wave == nil {
			add("no marine wave data — try a coastal place")
			return "UNKNOWN", reasons
		}
		if *wave < 1 {
			add(fmt.Sprintf("wave height %.1fm is too small", *wave))
			caution = true
		} else if *wave > 5 {
			add(fmt.Sprintf("wave height %.1fm is dangerous", *wave))
			stop = true
		} else {
			add(fmt.Sprintf("wave height %.1fm is in the comfort zone", *wave))
		}
		if wind != nil && *wind > 30 {
			add(fmt.Sprintf("wind %.0f km/h is choppy", *wind))
			caution = true
		}
		if usAqi != nil && *usAqi > 150 {
			add(fmt.Sprintf("US AQI %.0f (unhealthy)", *usAqi))
			stop = true
		}
	case "hiking", "biking":
		if precip != nil && *precip > 0.5 {
			add(fmt.Sprintf("precipitation %.1f mm — wet trail", *precip))
			caution = true
		}
		if wind != nil && *wind > 40 {
			add(fmt.Sprintf("wind %.0f km/h is high", *wind))
			caution = true
		}
		if temp != nil && (*temp < -5 || *temp > 32) {
			add(fmt.Sprintf("temperature %.1f C is uncomfortable", *temp))
			caution = true
		}
		if euAqi != nil && *euAqi > 100 {
			add(fmt.Sprintf("European AQI %.0f", *euAqi))
			caution = true
		}
		if usAqi != nil && *usAqi > 150 {
			add(fmt.Sprintf("US AQI %.0f (unhealthy)", *usAqi))
			stop = true
		}
	case "running":
		if precip != nil && *precip > 0.5 {
			add(fmt.Sprintf("precipitation %.1f mm — wet pavement", *precip))
			caution = true
		}
		if wind != nil && *wind > 30 {
			add(fmt.Sprintf("wind %.0f km/h", *wind))
			caution = true
		}
		if temp != nil && (*temp < -5 || *temp > 28) {
			add(fmt.Sprintf("temperature %.1f C", *temp))
			caution = true
		}
		if usAqi != nil && *usAqi > 100 {
			add(fmt.Sprintf("US AQI %.0f (unhealthy for sensitive)", *usAqi))
			caution = true
		}
		if usAqi != nil && *usAqi > 150 {
			stop = true
		}
	case "skiing":
		if snow == nil || *snow < 0.1 {
			add("no snowfall right now")
			caution = true
		} else {
			add(fmt.Sprintf("snowfall %.1f cm", *snow*100))
		}
		if wind != nil && *wind > 40 {
			add(fmt.Sprintf("wind %.0f km/h", *wind))
			caution = true
		}
		if temp != nil && *temp > 2 {
			add(fmt.Sprintf("temperature %.1f C is too warm", *temp))
			caution = true
		}
	default:
		return "UNKNOWN", []string{fmt.Sprintf("unknown activity %q (try surfing, hiking, biking, running, skiing)", activity)}
	}
	switch {
	case stop:
		return "STOP", reasons
	case caution:
		return "CAUTION", reasons
	default:
		add("conditions look good")
		return "GO", reasons
	}
}

// numAt extracts a float64 pointer from a map[string]any. Returns nil when the
// key is missing or the value is not a number.
func numAt(m map[string]any, key string) *float64 {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch x := v.(type) {
	case float64:
		return &x
	case int:
		f := float64(x)
		return &f
	}
	return nil
}
