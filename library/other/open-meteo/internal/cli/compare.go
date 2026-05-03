package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/open-meteo/internal/openmeteo"
)

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var (
		place     string
		latitude  string
		longitude string
		metric    string
		years     int
		date      string
	)
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare today's (or a date's) weather to the N-year climate normal",
		Long: strings.TrimSpace(`
Pulls the same calendar date from the ERA5 archive across the last N years for a
given place, computes the mean and the historical range, then compares the
current forecast (or a target date's archive value) against that baseline.

Output includes the absolute value, the historical mean, the anomaly (delta),
and a percentile-style classification.
`),
		Example: strings.Trim(`
  open-meteo-pp-cli compare --place Seattle --metric temperature_2m_mean --years 30 --json
  open-meteo-pp-cli compare --place Berlin --metric precipitation_sum --years 20 --date 2025-12-25 --json
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
			p := places[0]
			if metric == "" {
				metric = "temperature_2m_mean"
			}
			if years <= 0 {
				years = 30
			}
			target, err := time.Parse("2006-01-02", date)
			if date == "" {
				target = time.Now().UTC()
			} else if err != nil {
				return fmt.Errorf("invalid --date %q: %w", date, err)
			}
			// Pull the same month-day across years using the archive's
			// start_date/end_date filter; the archive endpoint supports
			// arbitrary contiguous windows, so we issue one call per year.
			samples := make([]float64, 0, years)
			perYear := make(map[int]float64, years)
			for i := 1; i <= years; i++ {
				ts := target.AddDate(-i, 0, 0).Format("2006-01-02")
				params := map[string]string{
					"latitude":   strconv.FormatFloat(p.Latitude, 'f', -1, 64),
					"longitude":  strconv.FormatFloat(p.Longitude, 'f', -1, 64),
					"start_date": ts,
					"end_date":   ts,
					"daily":      metric,
					"timezone":   "auto",
				}
				raw, err := c.Get("https://archive-api.open-meteo.com/v1/archive", params)
				if err != nil {
					return fmt.Errorf("archive year %d: %w", target.Year()-i, err)
				}
				var resp struct {
					Daily map[string]json.RawMessage `json:"daily"`
				}
				if err := json.Unmarshal(raw, &resp); err != nil {
					return fmt.Errorf("parsing year %d: %w", target.Year()-i, err)
				}
				series, ok := resp.Daily[metric]
				if !ok {
					continue
				}
				var nums []*float64
				if err := json.Unmarshal(series, &nums); err != nil {
					continue
				}
				for _, vp := range nums {
					if vp != nil {
						samples = append(samples, *vp)
						perYear[target.Year()-i] = *vp
					}
				}
			}
			if len(samples) == 0 {
				return fmt.Errorf("no archive samples returned for metric %q", metric)
			}
			var sum, minV, maxV float64
			minV, maxV = samples[0], samples[0]
			for _, v := range samples {
				sum += v
				if v < minV {
					minV = v
				}
				if v > maxV {
					maxV = v
				}
			}
			mean := sum / float64(len(samples))
			// Fetch the comparand: forecast for today (no --date) or archive for a past date.
			var current float64
			var currentSource string
			if date == "" {
				// Today's forecast: pull the daily metric for today.
				params := map[string]string{
					"latitude":      strconv.FormatFloat(p.Latitude, 'f', -1, 64),
					"longitude":     strconv.FormatFloat(p.Longitude, 'f', -1, 64),
					"daily":         metric,
					"timezone":      "auto",
					"forecast_days": "1",
				}
				raw, err := c.Get("https://api.open-meteo.com/v1/forecast", params)
				if err != nil {
					return fmt.Errorf("fetching forecast for comparand: %w", err)
				}
				var fr struct {
					Daily map[string]json.RawMessage `json:"daily"`
				}
				if err := json.Unmarshal(raw, &fr); err != nil {
					return fmt.Errorf("parsing forecast: %w", err)
				}
				series, ok := fr.Daily[metric]
				if !ok {
					return fmt.Errorf("forecast does not return metric %q", metric)
				}
				var nums []*float64
				if err := json.Unmarshal(series, &nums); err != nil || len(nums) == 0 || nums[0] == nil {
					return fmt.Errorf("no forecast value for metric %q today", metric)
				}
				current = *nums[0]
				currentSource = "forecast"
			} else {
				params := map[string]string{
					"latitude":   strconv.FormatFloat(p.Latitude, 'f', -1, 64),
					"longitude":  strconv.FormatFloat(p.Longitude, 'f', -1, 64),
					"start_date": date,
					"end_date":   date,
					"daily":      metric,
					"timezone":   "auto",
				}
				raw, err := c.Get("https://archive-api.open-meteo.com/v1/archive", params)
				if err != nil {
					return fmt.Errorf("fetching target archive: %w", err)
				}
				var ar struct {
					Daily map[string]json.RawMessage `json:"daily"`
				}
				if err := json.Unmarshal(raw, &ar); err != nil {
					return fmt.Errorf("parsing target archive: %w", err)
				}
				series, ok := ar.Daily[metric]
				if !ok {
					return fmt.Errorf("archive does not return metric %q", metric)
				}
				var nums []*float64
				if err := json.Unmarshal(series, &nums); err != nil || len(nums) == 0 || nums[0] == nil {
					return fmt.Errorf("no archive value for metric %q on %s", metric, date)
				}
				current = *nums[0]
				currentSource = "archive"
			}
			anomaly := current - mean
			classification := classifyAnomaly(anomaly, minV, maxV, mean)
			view := map[string]any{
				"place":            p.Name,
				"latitude":         p.Latitude,
				"longitude":        p.Longitude,
				"metric":           metric,
				"date":             target.Format("2006-01-02"),
				"current":          current,
				"current_source":   currentSource,
				"normal_mean":      mean,
				"normal_min":       minV,
				"normal_max":       maxV,
				"normal_years":     len(samples),
				"anomaly":          anomaly,
				"classification":   classification,
				"per_year_samples": perYear,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&place, "place", "", "Place name (e.g., \"Seattle\").")
	cmd.Flags().StringVar(&latitude, "latitude", "", "WGS84 latitude (alternative to --place).")
	cmd.Flags().StringVar(&longitude, "longitude", "", "WGS84 longitude (alternative to --place).")
	cmd.Flags().StringVar(&metric, "metric", "temperature_2m_mean", "Daily metric to compare (temperature_2m_mean, temperature_2m_max, temperature_2m_min, precipitation_sum, ...).")
	cmd.Flags().IntVar(&years, "years", 30, "Number of past years to use for the normal (default 30).")
	cmd.Flags().StringVar(&date, "date", "", "Compare against a specific date (YYYY-MM-DD). Defaults to today's forecast.")
	return cmd
}

func classifyAnomaly(anomaly, minV, maxV, mean float64) string {
	span := maxV - minV
	if span <= 0 {
		return "no_variation"
	}
	rel := anomaly / span
	switch {
	case rel >= 0.5:
		return "much_higher_than_normal"
	case rel >= 0.15:
		return "higher_than_normal"
	case rel <= -0.5:
		return "much_lower_than_normal"
	case rel <= -0.15:
		return "lower_than_normal"
	default:
		return "near_normal"
	}
}
