package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/opensnow/internal/cliutil"

	"github.com/spf13/cobra"
)

func newOvernightCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "overnight",
		Short:   "Show the overnight snow forecast for all favorite locations",
		Long:    "For each favorite, fetches the semi-daily snow forecast and extracts the next overnight period (6pm-6am) to show expected snowfall.",
		Example: "  opensnow-pp-cli overnight\n  opensnow-pp-cli overnight --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, slugs, err := loadFavorites(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if len(slugs) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No favorites configured. Add some with: opensnow-pp-cli favorites add <slug>")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type overnightResult struct {
				Slug string
				Data json.RawMessage
			}

			results, errs := cliutil.FanoutRun(
				cmd.Context(),
				slugs,
				func(s string) string { return s },
				func(ctx context.Context, slug string) (overnightResult, error) {
					path := "/forecast/snow-detail/" + slug
					params := map[string]string{}
					data, err := c.Get(path, params)
					if err != nil {
						return overnightResult{}, err
					}
					data = extractResponseData(data)
					return overnightResult{Slug: slug, Data: data}, nil
				},
			)
			cliutil.FanoutReportErrors(os.Stderr, errs)

			if len(results) == 0 {
				return fmt.Errorf("no forecast data available for any favorite")
			}

			type overnightRow struct {
				Name       string `json:"name"`
				Snow       string `json:"overnight_snow"`
				SnowRange  string `json:"snow_range"`
				Conditions string `json:"conditions"`
				Wind       string `json:"wind"`
			}

			rows := make([]overnightRow, 0, len(results))
			for _, r := range results {
				var obj map[string]any
				if err := json.Unmarshal(r.Value.Data, &obj); err != nil {
					continue
				}

				row := overnightRow{Name: r.Value.Slug}
				if v, ok := obj["name"].(string); ok {
					row.Name = v
				}

				// Extract overnight period from forecast_semi_daily
				if periods, ok := obj["forecast_semi_daily"].([]any); ok {
					for _, p := range periods {
						pm, ok := p.(map[string]any)
						if !ok {
							continue
						}
						// Look for night/overnight period
						dayNight, _ := pm["day_night"].(string)
						if dayNight == "night" || dayNight == "Night" {
							if v, ok := pm["snow"].(float64); ok {
								row.Snow = fmt.Sprintf("%.1f\"", v)
							} else if v, ok := pm["precip_snow"]; ok {
								row.Snow = fmt.Sprintf("%v\"", v)
							}
							if lo, ok := pm["snow_low"]; ok {
								if hi, ok2 := pm["snow_high"]; ok2 {
									row.SnowRange = fmt.Sprintf("%v-%v\"", lo, hi)
								}
							}
							if v, ok := pm["weather"].(string); ok {
								row.Conditions = v
							} else if v, ok := pm["condition"].(string); ok {
								row.Conditions = v
							}
							if v, ok := pm["wind_speed"]; ok {
								if dir, ok2 := pm["wind_direction"].(string); ok2 {
									row.Wind = fmt.Sprintf("%v %s", v, dir)
								} else {
									row.Wind = fmt.Sprintf("%v mph", v)
								}
							}
							break
						}
					}
				}

				// If no semi_daily, try forecast_daily for overnight info
				if row.Snow == "" {
					if daily, ok := obj["forecast_daily"].([]any); ok && len(daily) > 0 {
						if d, ok := daily[0].(map[string]any); ok {
							if v, ok := d["snow_night"]; ok {
								row.Snow = fmt.Sprintf("%v\"", v)
							}
						}
					}
				}

				rows = append(rows, row)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			headers := []string{"Name", "Overnight Snow", "Snow Range", "Conditions", "Wind"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{
					r.Name, r.Snow, r.SnowRange, r.Conditions, r.Wind,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
}
