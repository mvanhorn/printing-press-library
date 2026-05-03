package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/open-meteo/internal/openmeteo"
)

func newNormalsCmd(flags *rootFlags) *cobra.Command {
	var (
		place     string
		latitude  string
		longitude string
		variable  string
		years     int
		month     int
		day       int
	)
	cmd := &cobra.Command{
		Use:   "normals",
		Short: "Climate normals (multi-decade averages) for a place and date",
		Long: strings.TrimSpace(`
Compute the N-year climate normal for any (place, date or month, daily variable)
combination. The normal is built locally by aggregating ERA5 archive data — no
single API call returns this.

When --month is given without --day, the normal covers the entire month
across all selected years. When both --month and --day are given, only that
calendar day is used.
`),
		Example: strings.Trim(`
  open-meteo-pp-cli normals --place Seattle --month 7 --variable temperature_2m_max --years 30 --json
  open-meteo-pp-cli normals --place "Mavericks, CA" --month 12 --day 15 --variable wind_speed_10m_max --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if month < 1 || month > 12 {
				return fmt.Errorf("--month must be 1-12")
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
			if variable == "" {
				variable = "temperature_2m_mean"
			}
			if years <= 0 {
				years = 30
			}
			thisYear := time.Now().UTC().Year()
			var samples []float64
			perYear := make(map[int]float64, years)
			for i := 1; i <= years; i++ {
				y := thisYear - i
				var startDate, endDate string
				if day > 0 {
					t := time.Date(y, time.Month(month), day, 0, 0, 0, 0, time.UTC)
					startDate = t.Format("2006-01-02")
					endDate = startDate
				} else {
					t := time.Date(y, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
					startDate = t.Format("2006-01-02")
					endDate = t.AddDate(0, 1, -1).Format("2006-01-02")
				}
				params := map[string]string{
					"latitude":   strconv.FormatFloat(p.Latitude, 'f', -1, 64),
					"longitude":  strconv.FormatFloat(p.Longitude, 'f', -1, 64),
					"start_date": startDate,
					"end_date":   endDate,
					"daily":      variable,
					"timezone":   "auto",
				}
				raw, err := c.Get("https://archive-api.open-meteo.com/v1/archive", params)
				if err != nil {
					return fmt.Errorf("archive year %d: %w", y, err)
				}
				var resp struct {
					Daily map[string]json.RawMessage `json:"daily"`
				}
				if err := json.Unmarshal(raw, &resp); err != nil {
					return fmt.Errorf("parsing year %d: %w", y, err)
				}
				series, ok := resp.Daily[variable]
				if !ok {
					continue
				}
				var nums []*float64
				if err := json.Unmarshal(series, &nums); err != nil {
					continue
				}
				ySum := 0.0
				yCount := 0
				for _, vp := range nums {
					if vp != nil {
						samples = append(samples, *vp)
						ySum += *vp
						yCount++
					}
				}
				if yCount > 0 {
					perYear[y] = ySum / float64(yCount)
				}
			}
			if len(samples) == 0 {
				return fmt.Errorf("no archive samples for variable %q", variable)
			}
			sort.Float64s(samples)
			var sum float64
			for _, v := range samples {
				sum += v
			}
			mean := sum / float64(len(samples))
			median := samples[len(samples)/2]
			view := map[string]any{
				"place":     p.Name,
				"latitude":  p.Latitude,
				"longitude": p.Longitude,
				"variable":  variable,
				"month":     month,
				"day":       day,
				"years":     years,
				"samples":   len(samples),
				"mean":      mean,
				"median":    median,
				"min":       samples[0],
				"max":       samples[len(samples)-1],
				"per_year":  perYear,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&place, "place", "", "Place name (e.g., \"Seattle\").")
	cmd.Flags().StringVar(&latitude, "latitude", "", "WGS84 latitude (alternative to --place).")
	cmd.Flags().StringVar(&longitude, "longitude", "", "WGS84 longitude (alternative to --place).")
	cmd.Flags().StringVar(&variable, "variable", "temperature_2m_mean", "Daily archive variable (temperature_2m_max, temperature_2m_min, temperature_2m_mean, precipitation_sum, ...).")
	cmd.Flags().IntVar(&years, "years", 30, "Number of years to average (default 30).")
	cmd.Flags().IntVar(&month, "month", 0, "Calendar month (1-12). Required.")
	cmd.Flags().IntVar(&day, "day", 0, "Day of month (1-31). When omitted, covers the whole month.")
	return cmd
}
