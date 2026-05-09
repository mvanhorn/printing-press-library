package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/opensnow/internal/cliutil"

	"github.com/spf13/cobra"
)

func newDashboardCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "dashboard",
		Short:   "Show a summary dashboard for all favorite locations",
		Long:    "For each favorite, fetches the snow report and displays a summary table with status, temperature, snowfall, base depth, and operations.",
		Example: "  opensnow-pp-cli dashboard\n  opensnow-pp-cli dashboard --json",
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

			type reportResult struct {
				Slug string
				Data json.RawMessage
			}

			results, errs := cliutil.FanoutRun(
				cmd.Context(),
				slugs,
				func(s string) string { return s },
				func(ctx context.Context, slug string) (reportResult, error) {
					path := "/snow-report/" + slug
					params := map[string]string{}
					data, err := c.Get(path, params)
					if err != nil {
						// Try local fallback
						cached, cacheErr := db.GetCachedSnowReport(slug)
						if cacheErr == nil && cached != nil {
							return reportResult{Slug: slug, Data: cached}, nil
						}
						return reportResult{}, err
					}
					data = extractResponseData(data)
					return reportResult{Slug: slug, Data: data}, nil
				},
			)
			cliutil.FanoutReportErrors(os.Stderr, errs)

			if len(results) == 0 {
				return fmt.Errorf("no data available for any favorite")
			}

			type dashRow struct {
				Name      string `json:"name"`
				Status    string `json:"status"`
				Temp      string `json:"temp"`
				Snow24h   string `json:"snow_24h"`
				BaseDepth string `json:"base_depth"`
				LiftsOpen string `json:"lifts_open"`
				RunsOpen  string `json:"runs_open"`
			}

			rows := make([]dashRow, 0, len(results))
			for _, r := range results {
				var obj map[string]any
				if err := json.Unmarshal(r.Value.Data, &obj); err != nil {
					continue
				}

				row := dashRow{Name: r.Value.Slug}

				if v, ok := obj["name"].(string); ok {
					row.Name = v
				}
				if v, ok := obj["operating_status"].(string); ok {
					row.Status = v
				} else if v, ok := obj["status"].(string); ok {
					row.Status = v
				}
				if v, ok := obj["current_temperature"]; ok {
					row.Temp = fmt.Sprintf("%v°", v)
				} else if v, ok := obj["temp"]; ok {
					row.Temp = fmt.Sprintf("%v°", v)
				}
				if v, ok := obj["snow_past_24h"]; ok {
					row.Snow24h = fmt.Sprintf("%v\"", v)
				} else if v, ok := obj["new_snow_24"]; ok {
					row.Snow24h = fmt.Sprintf("%v\"", v)
				}
				if v, ok := obj["base_depth"]; ok {
					row.BaseDepth = fmt.Sprintf("%v\"", v)
				} else if v, ok := obj["snow_depth"]; ok {
					row.BaseDepth = fmt.Sprintf("%v\"", v)
				}
				if lo, ok := obj["lifts_open"]; ok {
					if lt, ok2 := obj["lifts_total"]; ok2 {
						row.LiftsOpen = fmt.Sprintf("%v/%v", lo, lt)
					} else {
						row.LiftsOpen = fmt.Sprintf("%v", lo)
					}
				}
				if ro, ok := obj["runs_open"]; ok {
					if rt, ok2 := obj["runs_total"]; ok2 {
						row.RunsOpen = fmt.Sprintf("%v/%v", ro, rt)
					} else {
						row.RunsOpen = fmt.Sprintf("%v", ro)
					}
				}
				rows = append(rows, row)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			headers := []string{"Name", "Status", "Temp", "24h Snow", "Base Depth", "Lifts Open", "Runs Open"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{
					r.Name, r.Status, r.Temp, r.Snow24h, r.BaseDepth, r.LiftsOpen, r.RunsOpen,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
}
