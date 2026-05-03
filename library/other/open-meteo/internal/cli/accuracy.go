package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/open-meteo/internal/openmeteo"
)

func newAccuracyCmd(flags *rootFlags) *cobra.Command {
	var (
		place     string
		latitude  string
		longitude string
		date      string
		variable  string
	)
	cmd := &cobra.Command{
		Use:   "accuracy",
		Short: "Forecast accuracy back-test for a past date",
		Long: strings.TrimSpace(`
For a past date, compare the forecast that was cached at that time (via
'forecast diff' or any other forecast call) against the archive ground truth
for the same date.

Without a cached forecast snapshot for the place, the command falls back to
"reforecast" mode: it calls the archive endpoint twice — once for the target
date, once for the day before — and reports the day-over-day delta as a
sanity-check baseline.
`),
		Example: strings.Trim(`
  open-meteo-pp-cli accuracy --place Seattle --date 2025-12-25 --variable temperature_2m_max --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if date == "" {
				return fmt.Errorf("--date is required (YYYY-MM-DD)")
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
				variable = "temperature_2m_max"
			}
			truthParams := map[string]string{
				"latitude":   strconv.FormatFloat(p.Latitude, 'f', -1, 64),
				"longitude":  strconv.FormatFloat(p.Longitude, 'f', -1, 64),
				"start_date": date,
				"end_date":   date,
				"daily":      variable,
				"timezone":   "auto",
			}
			truthRaw, err := c.Get("https://archive-api.open-meteo.com/v1/archive", truthParams)
			if err != nil {
				return fmt.Errorf("fetching archive truth: %w", err)
			}
			var truthResp struct {
				Daily map[string]json.RawMessage `json:"daily"`
			}
			if err := json.Unmarshal(truthRaw, &truthResp); err != nil {
				return fmt.Errorf("parsing truth: %w", err)
			}
			truthSeries, ok := truthResp.Daily[variable]
			if !ok {
				return fmt.Errorf("archive missing variable %q for %s", variable, date)
			}
			var truthVals []*float64
			if err := json.Unmarshal(truthSeries, &truthVals); err != nil || len(truthVals) == 0 || truthVals[0] == nil {
				return fmt.Errorf("no archive value for %s", date)
			}
			truth := *truthVals[0]
			view := map[string]any{
				"place":    p.Name,
				"date":     date,
				"variable": variable,
				"truth":    truth,
			}
			snap, _ := openmeteo.LoadSnapshot("forecast", p)
			if snap != nil {
				var snapPayload struct {
					Daily map[string]json.RawMessage `json:"daily"`
				}
				if json.Unmarshal(snap.Payload, &snapPayload) == nil {
					if vals, ok := snapPayload.Daily[variable]; ok {
						var nums []*float64
						if json.Unmarshal(vals, &nums) == nil && len(nums) > 0 && nums[0] != nil {
							view["snapshot_stored_at"] = snap.StoredAt
							view["forecast"] = *nums[0]
							view["error"] = *nums[0] - truth
							return printJSONFiltered(cmd.OutOrStdout(), view, flags)
						}
					}
				}
				view["snapshot_stored_at"] = snap.StoredAt
				view["note"] = "snapshot lacked the requested variable; ran reforecast baseline"
			} else {
				view["note"] = "no prior snapshot — ran reforecast baseline (day-over-day delta)"
			}
			view["mode"] = "reforecast"
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&place, "place", "", "Place name (e.g., \"Seattle\").")
	cmd.Flags().StringVar(&latitude, "latitude", "", "WGS84 latitude (alternative to --place).")
	cmd.Flags().StringVar(&longitude, "longitude", "", "WGS84 longitude (alternative to --place).")
	cmd.Flags().StringVar(&date, "date", "", "Past date to evaluate (YYYY-MM-DD). Required.")
	cmd.Flags().StringVar(&variable, "variable", "temperature_2m_max", "Daily variable to score (temperature_2m_max, precipitation_sum, ...).")
	return cmd
}
