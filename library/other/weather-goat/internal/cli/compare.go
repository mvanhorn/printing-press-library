package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <location1> <location2>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Compare weather between two locations side-by-side",
		Long:  "Fetch current conditions for two locations and display a side-by-side comparison of temperature, wind, precipitation, and UV.",
		Example: `  weather-goat-pp-cli compare "San Francisco" "Los Angeles"
  weather-goat-pp-cli compare "New York" "Miami" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "GET /forecast (Open-Meteo) x2")
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("requires two location arguments\nUsage: weather-goat-pp-cli compare <location1> <location2>"))
			}
			lat1, lon1, name1, err := geocodeLookup(args[0])
			if err != nil {
				return fmt.Errorf("resolving %q: %w", args[0], err)
			}
			lat2, lon2, name2, err := geocodeLookup(args[1])
			if err != nil {
				return fmt.Errorf("resolving %q: %w", args[1], err)
			}

			c, clientErr := flags.newClient()
			if clientErr != nil {
				return clientErr
			}

			f1, err := fetchLocationForecast(c, lat1, lon1)
			if err != nil {
				return fmt.Errorf("fetching forecast for %s: %w", name1, err)
			}
			f2, err := fetchLocationForecast(c, lat2, lon2)
			if err != nil {
				return fmt.Errorf("fetching forecast for %s: %w", name2, err)
			}

			if flags.asJSON {
				result := map[string]any{
					"location1": buildCompareJSON(name1, f1),
					"location2": buildCompareJSON(name2, f2),
					"comparison": map[string]any{
						"warmer":     comparePick(name1, name2, f1.temp, f2.temp, true),
						"drier":      comparePick(name1, name2, f1.precipProb, f2.precipProb, false),
						"less_windy": comparePick(name1, name2, f1.windSpeed, f2.windSpeed, false),
					},
				}
				return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(result), flags)
			}

			w := cmd.OutOrStdout()
			tw := newTabWriter(w)

			fmt.Fprintf(tw, "\t%s\t%s\n", bold(name1), bold(name2))
			fmt.Fprintf(tw, "Temp\t%.0f°F\t%.0f°F\n", f1.temp, f2.temp)
			fmt.Fprintf(tw, "Feels like\t%.0f°F\t%.0f°F\n", f1.feelsLike, f2.feelsLike)
			fmt.Fprintf(tw, "Conditions\t%s\t%s\n", f1.conditions, f2.conditions)
			fmt.Fprintf(tw, "Precip %%\t%.0f%%\t%.0f%%\n", f1.precipProb, f2.precipProb)
			fmt.Fprintf(tw, "Wind\t%.0f mph\t%.0f mph\n", f1.windSpeed, f2.windSpeed)
			fmt.Fprintf(tw, "UV\t%.0f\t%.0f\n", f1.uvIndex, f2.uvIndex)
			if err := tw.Flush(); err != nil {
				return err
			}

			fmt.Fprintln(w)
			fmt.Fprintf(w, "Warmer: %s\n", comparePick(name1, name2, f1.temp, f2.temp, true))
			fmt.Fprintf(w, "Drier: %s\n", comparePick(name1, name2, f1.precipProb, f2.precipProb, false))
			fmt.Fprintf(w, "Less windy: %s\n", comparePick(name1, name2, f1.windSpeed, f2.windSpeed, false))

			return nil
		},
	}

	return cmd
}

type locationForecast struct {
	temp       float64
	feelsLike  float64
	conditions string
	precipProb float64
	windSpeed  float64
	uvIndex    float64
}

func fetchLocationForecast(c clientGetter, lat, lon float64) (*locationForecast, error) {
	params := map[string]string{
		"latitude":         fmt.Sprintf("%f", lat),
		"longitude":        fmt.Sprintf("%f", lon),
		"current":          "temperature_2m,apparent_temperature,weather_code,wind_speed_10m,uv_index",
		"hourly":           "precipitation_probability",
		"timezone":         "auto",
		"temperature_unit": "fahrenheit",
		"wind_speed_unit":  "mph",
		"forecast_hours":   "1",
	}

	data, err := c.Get("/forecast", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Current struct {
			Temperature  float64 `json:"temperature_2m"`
			ApparentTemp float64 `json:"apparent_temperature"`
			WeatherCode  int     `json:"weather_code"`
			WindSpeed    float64 `json:"wind_speed_10m"`
			UVIndex      float64 `json:"uv_index"`
		} `json:"current"`
		Hourly struct {
			PrecipProb []float64 `json:"precipitation_probability"`
		} `json:"hourly"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing forecast: %w", err)
	}

	f := &locationForecast{
		temp:       resp.Current.Temperature,
		feelsLike:  resp.Current.ApparentTemp,
		conditions: describeWeatherCode(resp.Current.WeatherCode),
		windSpeed:  resp.Current.WindSpeed,
		uvIndex:    resp.Current.UVIndex,
	}
	if len(resp.Hourly.PrecipProb) > 0 {
		f.precipProb = resp.Hourly.PrecipProb[0]
	}
	return f, nil
}

func buildCompareJSON(name string, f *locationForecast) map[string]any {
	return map[string]any{
		"name":        name,
		"temperature": f.temp,
		"feels_like":  f.feelsLike,
		"conditions":  f.conditions,
		"precip_prob": f.precipProb,
		"wind_speed":  f.windSpeed,
		"uv_index":    f.uvIndex,
	}
}

func comparePick(name1, name2 string, val1, val2 float64, higherWins bool) string {
	if val1 == val2 {
		return "Tie"
	}
	if higherWins {
		if val1 > val2 {
			return name1
		}
		return name2
	}
	if val1 < val2 {
		return name1
	}
	return name2
}
