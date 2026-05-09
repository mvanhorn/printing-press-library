package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	var flagDays int

	cmd := &cobra.Command{
		Use:     "history <slug>",
		Short:   "Show historical snow data from locally cached reports",
		Long:    "Queries the local database for historical snow report snapshots and displays snowfall trends, base depth, and operating status over time.",
		Example: "  opensnow-pp-cli history vail\n  opensnow-pp-cli history vail --days 7",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := args[0]

			db, err := openStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			// Query local resources for historical snow-report data
			rows, qErr := db.Query(
				`SELECT data, synced_at FROM resources
				 WHERE resource_type = 'snow-report' AND id = ?
				 ORDER BY synced_at DESC LIMIT ?`,
				slug, flagDays,
			)
			if qErr != nil {
				return fmt.Errorf("querying history: %w", qErr)
			}
			defer rows.Close()

			type historyRow struct {
				Date      string `json:"date"`
				Snow      string `json:"snow"`
				BaseDepth string `json:"base_depth"`
				Status    string `json:"status"`
				Lifts     string `json:"lifts"`
			}

			var histRows []historyRow
			for rows.Next() {
				var dataStr, syncedAt string
				if err := rows.Scan(&dataStr, &syncedAt); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(dataStr), &obj); err != nil {
					continue
				}

				hr := historyRow{Date: syncedAt}
				if v, ok := obj["snow_past_24h"]; ok {
					hr.Snow = fmt.Sprintf("%v\"", v)
				} else if v, ok := obj["new_snow_24"]; ok {
					hr.Snow = fmt.Sprintf("%v\"", v)
				}
				if v, ok := obj["base_depth"]; ok {
					hr.BaseDepth = fmt.Sprintf("%v\"", v)
				} else if v, ok := obj["snow_depth"]; ok {
					hr.BaseDepth = fmt.Sprintf("%v\"", v)
				}
				if v, ok := obj["operating_status"].(string); ok {
					hr.Status = v
				} else if v, ok := obj["status"].(string); ok {
					hr.Status = v
				}
				if lo, ok := obj["lifts_open"]; ok {
					if lt, ok2 := obj["lifts_total"]; ok2 {
						hr.Lifts = fmt.Sprintf("%v/%v", lo, lt)
					} else {
						hr.Lifts = fmt.Sprintf("%v", lo)
					}
				}
				histRows = append(histRows, hr)
			}

			if len(histRows) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"slug":    slug,
						"message": "No historical data. Run 'sync' to start collecting.",
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No historical data. Run 'sync' to start collecting.")
				return nil
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), histRows, flags)
			}

			headers := []string{"Date", "Snow", "Base Depth", "Status", "Lifts"}
			tableData := make([][]string, 0, len(histRows))
			for _, r := range histRows {
				tableData = append(tableData, []string{
					r.Date, r.Snow, r.BaseDepth, r.Status, r.Lifts,
				})
			}
			return flags.printTable(cmd, headers, tableData)
		},
	}

	cmd.Flags().IntVar(&flagDays, "days", 30, "Number of historical records to show")
	return cmd
}
