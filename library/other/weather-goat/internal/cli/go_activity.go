package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/weather-goat/internal/config"

	"github.com/spf13/cobra"
)

func newGoCmd(flags *rootFlags) *cobra.Command {
	var flagLat float64
	var flagLon float64

	cmd := &cobra.Command{
		Use:   "go <activity> [location]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Get activity-specific weather verdicts: walk, bike, hike, commute, drive",
		Long: `Check whether conditions are safe for an activity. Each mode applies
domain-specific thresholds and returns a verdict: GO, CAUTION, or STOP.

Activities:
  walk     - Preparation advice based on precip, temperature, UV
  bike     - GO/CAUTION/STOP based on wind, rain, temperature, AQI
  hike     - GO/CAUTION/STOP for thunderstorms, hypothermia risk, UV, wind
  commute  - Compare morning vs evening conditions with umbrella advice
  drive    - GO/CAUTION/STOP for visibility, ice, wind, and NWS warnings`,
		Example: `  weather-goat-pp-cli go walk
  weather-goat-pp-cli go bike "Portland"
  weather-goat-pp-cli go commute
  weather-goat-pp-cli go drive --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "GET /forecast + /air-quality (Open-Meteo)")
				return nil
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("requires an activity argument\nUsage: weather-goat-pp-cli go <activity> [location]\nActivities: walk, bike, hike, commute, drive"))
			}
			activity := strings.ToLower(args[0])
			locationArgs := args[1:]

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			lat, lon, locName, err := resolveLocation(cfg, flagLat, flagLon,
				cmd.Flags().Changed("latitude"), cmd.Flags().Changed("longitude"), locationArgs)
			if err != nil {
				return err
			}

			c, clientErr := flags.newClient()
			if clientErr != nil {
				return clientErr
			}

			switch activity {
			case "walk":
				return runWalk(cmd, flags, c, lat, lon, locName)
			case "bike":
				return runBike(cmd, flags, c, lat, lon, locName)
			case "hike":
				return runHike(cmd, flags, c, lat, lon, locName)
			case "commute":
				return runCommute(cmd, flags, c, cfg, lat, lon, locName)
			case "drive":
				return runDrive(cmd, flags, c, lat, lon, locName)
			default:
				return usageErr(fmt.Errorf("unknown activity %q. Choose: walk, bike, hike, commute, drive", activity))
			}
		},
	}

	cmd.Flags().Float64Var(&flagLat, "latitude", 0, "Latitude")
	cmd.Flags().Float64Var(&flagLon, "longitude", 0, "Longitude")

	return cmd
}

type currentConditions struct {
	Temperature   float64
	FeelsLike     float64
	WeatherCode   int
	WindSpeed     float64
	WindGusts     float64
	Precipitation float64
	PrecipProb    float64
	UVIndex       float64
	Humidity      float64
	USAQI         float64
}

func fetchCurrentForActivity(c clientGetter, lat, lon float64) (*currentConditions, error) {
	params := map[string]string{
		"latitude":         fmt.Sprintf("%f", lat),
		"longitude":        fmt.Sprintf("%f", lon),
		"current":          "temperature_2m,relative_humidity_2m,apparent_temperature,precipitation,weather_code,wind_speed_10m,wind_gusts_10m,uv_index",
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
			Temperature   float64 `json:"temperature_2m"`
			Humidity      float64 `json:"relative_humidity_2m"`
			ApparentTemp  float64 `json:"apparent_temperature"`
			Precipitation float64 `json:"precipitation"`
			WeatherCode   int     `json:"weather_code"`
			WindSpeed     float64 `json:"wind_speed_10m"`
			WindGusts     float64 `json:"wind_gusts_10m"`
			UVIndex       float64 `json:"uv_index"`
		} `json:"current"`
		Hourly struct {
			PrecipProb []float64 `json:"precipitation_probability"`
		} `json:"hourly"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing forecast: %w", err)
	}

	cond := &currentConditions{
		Temperature:   resp.Current.Temperature,
		FeelsLike:     resp.Current.ApparentTemp,
		WeatherCode:   resp.Current.WeatherCode,
		WindSpeed:     resp.Current.WindSpeed,
		WindGusts:     resp.Current.WindGusts,
		Precipitation: resp.Current.Precipitation,
		UVIndex:       resp.Current.UVIndex,
		Humidity:      resp.Current.Humidity,
	}
	if len(resp.Hourly.PrecipProb) > 0 {
		cond.PrecipProb = resp.Hourly.PrecipProb[0]
	}

	return cond, nil
}

type clientGetter interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

func fetchAQI(c clientGetter, lat, lon float64) float64 {
	params := map[string]string{
		"latitude":  fmt.Sprintf("%f", lat),
		"longitude": fmt.Sprintf("%f", lon),
		"current":   "us_aqi",
		"timezone":  "auto",
	}
	data, err := c.Get("/air-quality", params)
	if err != nil {
		return 0
	}
	var resp struct {
		Current struct {
			USAQI float64 `json:"us_aqi"`
		} `json:"current"`
	}
	if json.Unmarshal(data, &resp) == nil {
		return resp.Current.USAQI
	}
	return 0
}

// --- Walk ---

func walkChecks(cond *currentConditions) []checkResult {
	var checks []checkResult
	if cond.PrecipProb > 60 || isActiveRain(cond.WeatherCode) {
		checks = append(checks, checkResult{"Rain", "caution", fmt.Sprintf("%.0f%% chance — take an umbrella", cond.PrecipProb)})
	} else {
		checks = append(checks, checkResult{"Rain", "pass", fmt.Sprintf("%.0f%% chance (low)", cond.PrecipProb)})
	}
	if cond.FeelsLike < 40 {
		checks = append(checks, checkResult{"Temperature", "caution", fmt.Sprintf("Feels like %.0f°F — wear warm layers", cond.FeelsLike)})
	} else if cond.FeelsLike > 90 {
		checks = append(checks, checkResult{"Temperature", "caution", fmt.Sprintf("Feels like %.0f°F — stay hydrated", cond.FeelsLike)})
	} else {
		checks = append(checks, checkResult{"Temperature", "pass", fmt.Sprintf("Feels like %.0f°F (comfortable)", cond.FeelsLike)})
	}
	if cond.UVIndex > 6 {
		checks = append(checks, checkResult{"UV", "caution", fmt.Sprintf("UV index %.0f — wear sunscreen", cond.UVIndex)})
	} else {
		checks = append(checks, checkResult{"UV", "pass", fmt.Sprintf("UV index %.0f (safe under 6)", cond.UVIndex)})
	}
	return checks
}

func runWalk(cmd *cobra.Command, flags *rootFlags, c clientGetter, lat, lon float64, loc string) error {
	cond, err := fetchCurrentForActivity(c, lat, lon)
	if err != nil {
		return classifyAPIError(err)
	}

	checks := walkChecks(cond)
	var advice []string
	for _, ch := range checks {
		if ch.Status != "pass" {
			advice = append(advice, ch.Detail)
		}
	}

	if flags.asJSON {
		result := map[string]any{
			"activity":    "walk",
			"location":    loc,
			"conditions":  describeWeatherCode(cond.WeatherCode),
			"temperature": cond.Temperature,
			"feels_like":  cond.FeelsLike,
			"precip_prob": cond.PrecipProb,
			"uv_index":    cond.UVIndex,
			"advice":      advice,
			"checks":      checks,
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(result), flags)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — Walk\n", bold(loc))
	fmt.Fprintf(w, "%.0f°F (feels like %.0f°F), %s\n\n", cond.Temperature, cond.FeelsLike, describeWeatherCode(cond.WeatherCode))
	for _, ch := range checks {
		fmt.Fprintf(w, "  %s %s: %s\n", checkMark(ch.Status), ch.Label, ch.Detail)
	}
	return nil
}

// --- Bike ---

func runBike(cmd *cobra.Command, flags *rootFlags, c clientGetter, lat, lon float64, loc string) error {
	cond, err := fetchCurrentForActivity(c, lat, lon)
	if err != nil {
		return classifyAPIError(err)
	}
	cond.USAQI = fetchAQI(c, lat, lon)

	checks := bikeChecks(cond)
	verdict, reasons := verdictFromChecks(checks)
	return printVerdictWithChecks(cmd, flags, "bike", loc, verdict, reasons, cond, checks)
}

// --- Hike ---

func runHike(cmd *cobra.Command, flags *rootFlags, c clientGetter, lat, lon float64, loc string) error {
	cond, err := fetchCurrentForActivity(c, lat, lon)
	if err != nil {
		return classifyAPIError(err)
	}

	checks := hikeChecks(cond)
	verdict, reasons := verdictFromChecks(checks)
	return printVerdictWithChecks(cmd, flags, "hike", loc, verdict, reasons, cond, checks)
}

// --- Commute ---

func runCommute(cmd *cobra.Command, flags *rootFlags, c clientGetter, cfg *config.Config, lat, lon float64, loc string) error {
	departTime := cfg.CommuteDepartTime
	if departTime == "" {
		departTime = "08:00"
	}
	returnTime := cfg.CommuteReturnTime
	if returnTime == "" {
		returnTime = "18:00"
	}

	// Fetch hourly forecast
	params := map[string]string{
		"latitude":         fmt.Sprintf("%f", lat),
		"longitude":        fmt.Sprintf("%f", lon),
		"hourly":           "temperature_2m,apparent_temperature,precipitation_probability,precipitation,weather_code,wind_speed_10m",
		"timezone":         "auto",
		"temperature_unit": "fahrenheit",
		"wind_speed_unit":  "mph",
		"forecast_hours":   "24",
	}

	data, err := c.Get("/forecast", params)
	if err != nil {
		return classifyAPIError(err)
	}

	var resp struct {
		Hourly struct {
			Time        []string  `json:"time"`
			Temp        []float64 `json:"temperature_2m"`
			FeelsLike   []float64 `json:"apparent_temperature"`
			PrecipProb  []float64 `json:"precipitation_probability"`
			Precip      []float64 `json:"precipitation"`
			WeatherCode []int     `json:"weather_code"`
			WindSpeed   []float64 `json:"wind_speed_10m"`
		} `json:"hourly"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing hourly forecast: %w", err)
	}

	// Find depart and return hour indices
	departIdx := findHourIndex(resp.Hourly.Time, departTime)
	returnIdx := findHourIndex(resp.Hourly.Time, returnTime)

	if departIdx < 0 || returnIdx < 0 || departIdx >= len(resp.Hourly.WeatherCode) || returnIdx >= len(resp.Hourly.WeatherCode) {
		return fmt.Errorf("could not find commute hours in forecast data")
	}

	departCond := describeWeatherCode(resp.Hourly.WeatherCode[departIdx])
	returnCond := describeWeatherCode(resp.Hourly.WeatherCode[returnIdx])
	departTemp := resp.Hourly.Temp[departIdx]
	returnTemp := resp.Hourly.Temp[returnIdx]
	departPrecipProb := resp.Hourly.PrecipProb[departIdx]
	returnPrecipProb := resp.Hourly.PrecipProb[returnIdx]

	var advice []string
	if isActiveRain(resp.Hourly.WeatherCode[departIdx]) {
		advice = append(advice, "Rain this morning — take umbrella for departure")
	}
	if isActiveRain(resp.Hourly.WeatherCode[returnIdx]) && !isActiveRain(resp.Hourly.WeatherCode[departIdx]) {
		advice = append(advice, fmt.Sprintf("%s by %s — take umbrella for the ride home", returnCond, returnTime))
	}
	if resp.Hourly.WindSpeed[departIdx] > 25 || resp.Hourly.WindSpeed[returnIdx] > 25 {
		advice = append(advice, "Strong winds expected — allow extra travel time")
	}

	if flags.asJSON {
		result := map[string]any{
			"activity": "commute",
			"location": loc,
			"depart": map[string]any{
				"time":        departTime,
				"temperature": departTemp,
				"conditions":  departCond,
				"precip_prob": departPrecipProb,
			},
			"return": map[string]any{
				"time":        returnTime,
				"temperature": returnTemp,
				"conditions":  returnCond,
				"precip_prob": returnPrecipProb,
			},
			"advice": advice,
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(result), flags)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — Commute\n", bold(loc))
	fmt.Fprintf(w, "Depart (%s): %.0f°F, %s (%.0f%% precip)\n", departTime, departTemp, departCond, departPrecipProb)
	fmt.Fprintf(w, "Return (%s): %.0f°F, %s (%.0f%% precip)\n", returnTime, returnTemp, returnCond, returnPrecipProb)
	if len(advice) > 0 {
		fmt.Fprintln(w)
		for _, a := range advice {
			fmt.Fprintf(w, "  - %s\n", a)
		}
	} else {
		fmt.Fprintln(w, "Clear commute expected.")
	}
	return nil
}

func findHourIndex(times []string, target string) int {
	for i, t := range times {
		// times are like "2026-04-11T08:00"
		if len(t) >= 16 && t[11:16] == target {
			return i
		}
	}
	return -1
}

// --- Drive ---

func runDrive(cmd *cobra.Command, flags *rootFlags, c clientGetter, lat, lon float64, loc string) error {
	cond, err := fetchCurrentForActivity(c, lat, lon)
	if err != nil {
		return classifyAPIError(err)
	}

	alerts, _ := nwsAlerts(lat, lon)
	checks := driveChecks(cond, alerts)
	verdict, reasons := verdictFromChecks(checks)
	return printVerdictWithChecks(cmd, flags, "drive", loc, verdict, reasons, cond, checks)
}

// --- Shared verdict helpers ---

func maxVerdict(current, proposed string) string {
	order := map[string]int{"GO": 0, "CAUTION": 1, "STOP": 2}
	if order[proposed] > order[current] {
		return proposed
	}
	return current
}

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// checkResult represents a single threshold check with its status.
type checkResult struct {
	Label  string `json:"label"`
	Status string `json:"status"` // pass, caution, stop
	Detail string `json:"detail"`
}

// activityChecks returns the threshold breakdown for an activity.
func bikeChecks(cond *currentConditions) []checkResult {
	checks := []checkResult{
		windCheck(cond.WindSpeed, 20, 30),
		precipCheck(cond.PrecipProb, cond.WeatherCode, 60),
		tempCheck(cond.Temperature, 20, 32, "ice on roads", "frostbite risk"),
		aqiCheck(cond.USAQI, 100, 150),
	}
	return checks
}

func hikeChecks(cond *currentConditions) []checkResult {
	var checks []checkResult
	if isThunderstorm(cond.WeatherCode) {
		checks = append(checks, checkResult{"Lightning", "stop", "Thunderstorm active — do not hike"})
	} else {
		checks = append(checks, checkResult{"Lightning", "pass", "No thunderstorm activity"})
	}
	if isActiveRain(cond.WeatherCode) && cond.Temperature < 40 {
		checks = append(checks, checkResult{"Hypothermia", "caution", fmt.Sprintf("Rain + %.0f°F — exposure risk", cond.Temperature)})
	} else {
		checks = append(checks, checkResult{"Hypothermia", "pass", fmt.Sprintf("%.0f°F, no rain/cold combo", cond.Temperature)})
	}
	if cond.UVIndex > 8 {
		checks = append(checks, checkResult{"UV", "caution", fmt.Sprintf("UV index %.0f — high altitude exposure", cond.UVIndex)})
	} else {
		checks = append(checks, checkResult{"UV", "pass", fmt.Sprintf("UV index %.0f (safe under 8)", cond.UVIndex)})
	}
	if cond.WindGusts > 40 {
		checks = append(checks, checkResult{"Wind gusts", "caution", fmt.Sprintf("%.0f mph — exposed ridges dangerous", cond.WindGusts)})
	} else {
		checks = append(checks, checkResult{"Wind gusts", "pass", fmt.Sprintf("%.0f mph (safe under 40)", cond.WindGusts)})
	}
	return checks
}

func driveChecks(cond *currentConditions, alerts []map[string]any) []checkResult {
	var checks []checkResult
	if isLowVisibility(cond.WeatherCode) {
		checks = append(checks, checkResult{"Visibility", "caution", describeWeatherCode(cond.WeatherCode)})
	} else {
		checks = append(checks, checkResult{"Visibility", "pass", "Clear visibility"})
	}
	if cond.WeatherCode == 66 || cond.WeatherCode == 67 {
		checks = append(checks, checkResult{"Road surface", "stop", "Freezing rain — extremely dangerous"})
	} else if isSnow(cond.WeatherCode) {
		checks = append(checks, checkResult{"Road surface", "caution", "Snow — reduced traction"})
	} else {
		checks = append(checks, checkResult{"Road surface", "pass", "Dry roads"})
	}
	if cond.WindGusts > 60 {
		checks = append(checks, checkResult{"Wind gusts", "stop", fmt.Sprintf("%.0f mph — dangerous for all vehicles", cond.WindGusts)})
	} else if cond.WindGusts > 45 {
		checks = append(checks, checkResult{"Wind gusts", "caution", fmt.Sprintf("%.0f mph — dangerous for high-profile vehicles", cond.WindGusts)})
	} else {
		checks = append(checks, checkResult{"Wind gusts", "pass", fmt.Sprintf("%.0f mph (safe under 45)", cond.WindGusts)})
	}
	alertCheck := checkResult{"NWS alerts", "pass", "No active warnings"}
	for _, a := range alerts {
		event, _ := a["event"].(string)
		severity, _ := a["severity"].(string)
		if strings.Contains(strings.ToLower(event), "warning") || severity == "Extreme" || severity == "Severe" {
			alertCheck = checkResult{"NWS alerts", "caution", event}
			break
		}
	}
	checks = append(checks, alertCheck)
	return checks
}

func windCheck(speed, cautionThresh, stopThresh float64) checkResult {
	if speed > stopThresh {
		return checkResult{"Wind", "stop", fmt.Sprintf("%.0f mph — dangerous crosswinds", speed)}
	}
	if speed > cautionThresh {
		return checkResult{"Wind", "caution", fmt.Sprintf("%.0f mph — strong headwinds", speed)}
	}
	return checkResult{"Wind", "pass", fmt.Sprintf("%.0f mph (safe under %.0f)", speed, cautionThresh)}
}

func precipCheck(prob float64, code int, cautionThresh float64) checkResult {
	if isActiveRain(code) {
		return checkResult{"Rain", "stop", "Active rain — slippery roads"}
	}
	if prob > cautionThresh {
		return checkResult{"Rain", "caution", fmt.Sprintf("%.0f%% chance of rain", prob)}
	}
	return checkResult{"Rain", "pass", fmt.Sprintf("%.0f%% chance (safe under %.0f%%)", prob, cautionThresh)}
}

func tempCheck(temp, stopThresh, cautionThresh float64, cautionReason, stopReason string) checkResult {
	if temp < stopThresh {
		return checkResult{"Temperature", "stop", fmt.Sprintf("%.0f°F — %s", temp, stopReason)}
	}
	if temp < cautionThresh {
		return checkResult{"Temperature", "caution", fmt.Sprintf("%.0f°F — %s", temp, cautionReason)}
	}
	return checkResult{"Temperature", "pass", fmt.Sprintf("%.0f°F (above %.0f°F)", temp, cautionThresh)}
}

func aqiCheck(aqi, cautionThresh, stopThresh float64) checkResult {
	if aqi > stopThresh {
		return checkResult{"AQI", "stop", fmt.Sprintf("%.0f — unhealthy for all", aqi)}
	}
	if aqi > cautionThresh {
		return checkResult{"AQI", "caution", fmt.Sprintf("%.0f — unhealthy for sensitive groups", aqi)}
	}
	if aqi > 0 {
		return checkResult{"AQI", "pass", fmt.Sprintf("%.0f (safe under %.0f)", aqi, cautionThresh)}
	}
	return checkResult{"AQI", "pass", "Not available"}
}

func verdictFromChecks(checks []checkResult) (string, []string) {
	verdict := "GO"
	var reasons []string
	for _, ch := range checks {
		switch ch.Status {
		case "stop":
			verdict = "STOP"
			reasons = append(reasons, ch.Detail)
		case "caution":
			verdict = maxVerdict(verdict, "CAUTION")
			reasons = append(reasons, ch.Detail)
		}
	}
	return verdict, reasons
}

func checkMark(status string) string {
	switch status {
	case "pass":
		return "✓"
	case "caution":
		return "⚠"
	case "stop":
		return "✗"
	}
	return "?"
}

func printVerdict(cmd *cobra.Command, flags *rootFlags, activity, loc, verdict string, reasons []string, cond *currentConditions) error {
	return printVerdictWithChecks(cmd, flags, activity, loc, verdict, reasons, cond, nil)
}

func printVerdictWithChecks(cmd *cobra.Command, flags *rootFlags, activity, loc, verdict string, reasons []string, cond *currentConditions, checks []checkResult) error {
	if flags.asJSON {
		result := map[string]any{
			"activity":    activity,
			"location":    loc,
			"verdict":     verdict,
			"conditions":  describeWeatherCode(cond.WeatherCode),
			"temperature": cond.Temperature,
			"feels_like":  cond.FeelsLike,
			"wind_speed":  cond.WindSpeed,
			"wind_gusts":  cond.WindGusts,
			"reasons":     reasons,
			"checks":      checks,
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(result), flags)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — %s: %s\n", bold(loc), titleCase(activity), verdict)
	fmt.Fprintf(w, "%.0f°F (feels like %.0f°F), %s, wind %.0f mph\n\n",
		cond.Temperature, cond.FeelsLike, describeWeatherCode(cond.WeatherCode), cond.WindSpeed)

	if len(checks) > 0 {
		for _, ch := range checks {
			fmt.Fprintf(w, "  %s %s: %s\n", checkMark(ch.Status), ch.Label, ch.Detail)
		}
	} else if len(reasons) > 0 {
		for _, r := range reasons {
			fmt.Fprintf(w, "  ⚠ %s\n", r)
		}
	} else {
		fmt.Fprintln(w, "  All checks passed.")
	}
	return nil
}
