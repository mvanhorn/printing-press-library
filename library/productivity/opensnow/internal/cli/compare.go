package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/opensnow/internal/cliutil"

	"github.com/spf13/cobra"
)

func newCompareCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "compare <slug1> <slug2> [slug3...]",
		Short:   "Side-by-side comparison of multiple resorts",
		Long:    "Fetches snow report and forecast data for each location and displays key metrics side by side.",
		Example: "  opensnow-pp-cli compare vail aspen breckenridge\n  opensnow-pp-cli compare vail aspen --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type compareData struct {
				Slug   string
				Report map[string]any
			}

			results, errs := cliutil.FanoutRun(
				cmd.Context(),
				args,
				func(s string) string { return s },
				func(ctx context.Context, slug string) (compareData, error) {
					path := "/snow-report/" + slug
					data, err := c.Get(path, map[string]string{})
					if err != nil {
						return compareData{}, err
					}
					data = extractResponseData(data)
					var obj map[string]any
					if err := json.Unmarshal(data, &obj); err != nil {
						return compareData{}, err
					}
					return compareData{Slug: slug, Report: obj}, nil
				},
			)
			cliutil.FanoutReportErrors(os.Stderr, errs)

			if len(results) == 0 {
				return fmt.Errorf("no data available for any location")
			}

			type compareRow struct {
				Metric string            `json:"metric"`
				Values map[string]string `json:"values"`
			}

			metrics := []struct {
				label string
				keys  []string
			}{
				{"Status", []string{"operating_status", "status"}},
				{"Temp", []string{"current_temperature", "temp"}},
				{"24h Snow", []string{"snow_past_24h", "new_snow_24"}},
				{"Base Depth", []string{"base_depth", "snow_depth"}},
				{"Lifts", []string{"lifts_open"}},
				{"Runs", []string{"runs_open"}},
				{"Conditions", []string{"conditions", "weather"}},
			}

			rows := make([]compareRow, 0, len(metrics))
			for _, m := range metrics {
				row := compareRow{
					Metric: m.label,
					Values: make(map[string]string),
				}
				for _, r := range results {
					slug := r.Value.Slug
					val := ""
					for _, key := range m.keys {
						if v, ok := r.Value.Report[key]; ok && v != nil {
							val = fmt.Sprintf("%v", v)
							break
						}
					}
					row.Values[slug] = val
				}
				rows = append(rows, row)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			// Build table headers: Metric + each slug
			headers := []string{"Metric"}
			for _, r := range results {
				name := r.Value.Slug
				if v, ok := r.Value.Report["name"].(string); ok {
					name = v
				}
				headers = append(headers, name)
			}

			tableRows := make([][]string, 0, len(rows))
			for _, row := range rows {
				tr := []string{row.Metric}
				for _, r := range results {
					tr = append(tr, row.Values[r.Value.Slug])
				}
				tableRows = append(tableRows, tr)
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
}
