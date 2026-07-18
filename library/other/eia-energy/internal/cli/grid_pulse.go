// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through eiaFetch in eia_novel_support.go

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelGridPulseCmd(flags *rootFlags) *cobra.Command {
	var respondent string
	var hours int

	cmd := &cobra.Command{
		Use:         "pulse",
		Short:       "Fetch recent balancing-authority measures and show per-type latest values, units, freshness",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if hours < 1 || hours > 720 {
				return fmt.Errorf("--hours must be 1-720")
			}
			end := time.Now().UTC().Truncate(time.Hour)
			start := end.Add(-time.Duration(hours-1) * time.Hour)
			params := seriesParams("value", "hourly", start.Format("2006-01-02T15"), end.Format("2006-01-02T15"), map[string][]string{"respondent": {respondent}}, 5000)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			page, err := eiaFetch(ctx, flags, "electricity/rto/region-data", params)
			if err != nil {
				return err
			}
			groups := map[string][]map[string]any{}
			for _, row := range page.Rows {
				groups[fmt.Sprint(row["type-name"])] = append(groups[fmt.Sprint(row["type-name"])], row)
			}
			pulse := map[string]any{}
			for name, rows := range groups {
				latest := rows[len(rows)-1]
				pulse[name] = map[string]any{"latest_period": latest["period"], "latest_value": latest["value"], "unit": rowUnit(latest, "value"), "trailing": seriesStats(rows, "value")}
			}
			return emitEIA(cmd, flags, "live", map[string]any{"respondent": respondent, "frequency": "hourly", "hours": hours, "total": page.Total, "returned_rows": len(page.Rows), "measures": pulse, "source_rows": page.Rows, "caveats": eiaCaveats()})
		},
	}
	cmd.Flags().StringVar(&respondent, "respondent", "CISO", "Balancing authority respondent code")
	cmd.Flags().IntVar(&hours, "hours", 24, "Trailing hourly window (1-720)")
	return cmd
}
