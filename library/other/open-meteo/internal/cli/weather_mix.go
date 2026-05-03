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
	"github.com/mvanhorn/printing-press-library/library/other/open-meteo/internal/wmocode"
)

func newWeatherMixCmd(flags *rootFlags) *cobra.Command {
	var (
		place     string
		latitude  string
		longitude string
		startDate string
		endDate   string
		pastDays  int
	)
	cmd := &cobra.Command{
		Use:   "weather-mix",
		Short: "Distribution of WMO weather conditions over a historical window",
		Long: strings.TrimSpace(`
Aggregate WMO weather codes from the ERA5 archive into category buckets
(% clear, partly_cloudy, overcast, fog, drizzle, rain, snow, showers,
thunderstorm) for a given (place, time window) pair. Useful for travel
planning ("how often does it rain in Seattle in October?") and
climatology summaries.
`),
		Example: strings.Trim(`
  open-meteo-pp-cli weather-mix --place Seattle --start-date 2024-10-01 --end-date 2024-10-31 --json
  open-meteo-pp-cli weather-mix --place Berlin --past-days 90 --json
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
			params := map[string]string{
				"latitude":  strconv.FormatFloat(p.Latitude, 'f', -1, 64),
				"longitude": strconv.FormatFloat(p.Longitude, 'f', -1, 64),
				"hourly":    "weather_code",
				"timezone":  "auto",
			}
			if startDate != "" && endDate != "" {
				params["start_date"] = startDate
				params["end_date"] = endDate
			} else if pastDays > 0 {
				end := time.Now().UTC()
				start := end.AddDate(0, 0, -pastDays)
				params["start_date"] = start.Format("2006-01-02")
				params["end_date"] = end.Format("2006-01-02")
			} else {
				return fmt.Errorf("provide --start-date+--end-date OR --past-days")
			}
			raw, err := c.Get("https://archive-api.open-meteo.com/v1/archive", params)
			if err != nil {
				return fmt.Errorf("fetching archive: %w", err)
			}
			var resp struct {
				Hourly struct {
					Time        []string `json:"time"`
					WeatherCode []*int   `json:"weather_code"`
				} `json:"hourly"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return fmt.Errorf("parsing archive: %w", err)
			}
			counts := map[string]int{}
			total := 0
			for _, codePtr := range resp.Hourly.WeatherCode {
				if codePtr == nil {
					continue
				}
				counts[wmocode.Bucket(*codePtr)]++
				total++
			}
			type bucketRow struct {
				Bucket  string  `json:"bucket"`
				Hours   int     `json:"hours"`
				Percent float64 `json:"percent"`
			}
			rows := make([]bucketRow, 0, len(counts))
			for k, v := range counts {
				pct := 0.0
				if total > 0 {
					pct = float64(v) / float64(total) * 100
				}
				rows = append(rows, bucketRow{Bucket: k, Hours: v, Percent: pct})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Hours > rows[j].Hours })
			view := map[string]any{
				"place":      p.Name,
				"latitude":   p.Latitude,
				"longitude":  p.Longitude,
				"start_date": params["start_date"],
				"end_date":   params["end_date"],
				"hours":      total,
				"buckets":    rows,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&place, "place", "", "Place name (e.g., \"Seattle\"). Use --latitude/--longitude as the alternative.")
	cmd.Flags().StringVar(&latitude, "latitude", "", "WGS84 latitude (alternative to --place).")
	cmd.Flags().StringVar(&longitude, "longitude", "", "WGS84 longitude (alternative to --place).")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Window start date (YYYY-MM-DD). Use with --end-date.")
	cmd.Flags().StringVar(&endDate, "end-date", "", "Window end date (YYYY-MM-DD).")
	cmd.Flags().IntVar(&pastDays, "past-days", 0, "Window covering the last N days (alternative to --start-date/--end-date).")
	return cmd
}
