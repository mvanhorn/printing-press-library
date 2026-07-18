// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through eiaFetch in eia_novel_support.go

package cli

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelAnomalyCmd(flags *rootFlags) *cobra.Command {
	var route, facet, data, frequency, typeCode string
	var hours int
	var zThreshold float64

	cmd := &cobra.Command{
		Use:         "anomaly",
		Short:       "Compute deterministic mean and standard-deviation deviations from exact bounded EIA source rows without forecasting.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if hours < 2 || hours > 720 {
				return fmt.Errorf("--hours must be 2-720")
			}
			if zThreshold <= 0 {
				return fmt.Errorf("--z-threshold must be greater than zero")
			}
			name, value, err := parseFacet(facet)
			if err != nil {
				return err
			}
			end := time.Now().UTC().Truncate(time.Hour)
			start := end.Add(-time.Duration(hours-1) * time.Hour)
			facets := map[string][]string{name: {value}}
			if strings.TrimSpace(typeCode) != "" {
				facets["type"] = []string{strings.TrimSpace(typeCode)}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			page, err := eiaFetch(ctx, flags, route, seriesParams(data, frequency, start.Format("2006-01-02T15"), end.Format("2006-01-02T15"), facets, 5000))
			if err != nil {
				return err
			}
			stats := seriesStats(page.Rows, data)
			mean, _ := stats["mean"].(float64)
			sd, _ := stats["standard_deviation"].(float64)
			var anomalies []map[string]any
			if sd > 0 {
				for _, row := range page.Rows {
					v, ok := rowValue(row, data)
					if !ok {
						continue
					}
					z := (v - mean) / sd
					if math.Abs(z) >= zThreshold {
						anomalies = append(anomalies, map[string]any{"period": row["period"], "value": v, "unit": rowUnit(row, data), "z_score": z, "source_row": row})
					}
				}
			}
			return emitEIA(cmd, flags, "live", map[string]any{"route": route, "facet": facet, "frequency": frequency, "data": data, "total": page.Total, "statistics": stats, "threshold_absolute_z": zThreshold, "anomalies": anomalies, "caveats": eiaCaveats()})
		},
	}
	cmd.Flags().StringVar(&route, "route", "electricity/rto/region-data", "EIA data route")
	cmd.Flags().StringVar(&facet, "facet", "respondent=CISO", "Facet NAME=VALUE")
	cmd.Flags().StringVar(&data, "data", "value", "Data column")
	cmd.Flags().StringVar(&frequency, "frequency", "hourly", "Series frequency")
	cmd.Flags().StringVar(&typeCode, "type", "D", "Optional type facet; pass an empty value for routes without a type facet")
	cmd.Flags().IntVar(&hours, "hours", 168, "Trailing hourly window")
	cmd.Flags().Float64Var(&zThreshold, "z-threshold", 2, "Absolute population z-score threshold")
	return cmd
}
