// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through eiaFetch in eia_novel_support.go

package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelSpreadCmd(flags *rootFlags) *cobra.Command {
	var route, left, right, data, frequency, typeCode string
	var hours int

	cmd := &cobra.Command{
		Use:         "spread",
		Short:       "Align two facet series by period and refuse subtraction when reported units differ.",
		Example:     "  eia-energy-pp-cli spread --left respondent=CISO --right respondent=ERCO --type D --hours 24 --agent",
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
			ln, lv, err := parseFacet(left)
			if err != nil {
				return err
			}
			rn, rv, err := parseFacet(right)
			if err != nil {
				return err
			}
			if ln != rn {
				return errors.New("--left and --right must use the same facet name")
			}
			end := time.Now().UTC().Truncate(time.Hour)
			start := end.Add(-time.Duration(hours-1) * time.Hour)
			base := map[string][]string{"type": {typeCode}}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			base[ln] = []string{lv}
			lp, err := eiaFetch(ctx, flags, route, seriesParams(data, frequency, start.Format("2006-01-02T15"), end.Format("2006-01-02T15"), base, 5000))
			if err != nil {
				return err
			}
			if err := requireCompletePage(lp, "spread comparison"); err != nil {
				return err
			}
			base[ln] = []string{rv}
			rp, err := eiaFetch(ctx, flags, route, seriesParams(data, frequency, start.Format("2006-01-02T15"), end.Format("2006-01-02T15"), base, 5000))
			if err != nil {
				return err
			}
			if err := requireCompletePage(rp, "spread comparison"); err != nil {
				return err
			}
			lm, rm := mapPeriods(lp.Rows, data), mapPeriods(rp.Rows, data)
			rows, excluded, err := buildSpreadRows(lm, rm, data)
			if err != nil {
				return err
			}
			return emitEIA(cmd, flags, "live", map[string]any{"route": route, "frequency": frequency, "left": left, "right": right, "type": typeCode, "data": data, "hours": hours, "start": start.Format("2006-01-02T15"), "end": end.Format("2006-01-02T15"), "aligned_rows": rows, "excluded_rows": excluded, "left_total": lp.Total, "right_total": rp.Total, "caveats": eiaCaveats()})
		},
	}
	cmd.Flags().StringVar(&route, "route", "electricity/rto/region-data", "EIA data route without /data")
	cmd.Flags().StringVar(&left, "left", "respondent=CISO", "Left facet NAME=VALUE")
	cmd.Flags().StringVar(&right, "right", "respondent=ERCO", "Right facet NAME=VALUE")
	cmd.Flags().StringVar(&typeCode, "type", "D", "Optional grid type facet")
	cmd.Flags().StringVar(&data, "data", "value", "EIA data column")
	cmd.Flags().StringVar(&frequency, "frequency", "hourly", "Shared frequency")
	cmd.Flags().IntVar(&hours, "hours", 24, "Trailing hourly window")
	return cmd
}

func buildSpreadRows(lm, rm map[string]map[string]any, data string) ([]map[string]any, []map[string]any, error) {
	var rows []map[string]any
	var excluded []map[string]any
	for _, period := range sortedStringKeys(lm) {
		rightRow, ok := rm[period]
		if !ok {
			continue
		}
		leftRow := lm[period]
		lu, ru := rowUnit(leftRow, data), rowUnit(rightRow, data)
		if lu == "" || ru == "" {
			return nil, nil, fmt.Errorf("cannot compute spread at %s without reported units", period)
		}
		if lu != ru {
			return nil, nil, fmt.Errorf("unit mismatch at %s: %q vs %q", period, lu, ru)
		}
		a, aok := rowValue(leftRow, data)
		b, bok := rowValue(rightRow, data)
		if !aok || !bok {
			excluded = append(excluded, map[string]any{"period": period, "reason": "non_numeric_or_missing_value", "left": leftRow[data], "right": rightRow[data]})
			continue
		}
		rows = append(rows, map[string]any{"period": period, "left": a, "right": b, "spread": a - b, "unit": lu})
	}
	return rows, excluded, nil
}
