// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through eiaFetch in eia_novel_support.go

package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelStateCompareCmd(flags *rootFlags) *cobra.Command {
	var route, states, facetName, data, frequency, start, end, sector string

	cmd := &cobra.Command{
		Use:         "compare",
		Short:       "Compare explicitly selected state series only when route, data column, frequency, periods, and units align.",
		Example:     "  eia-energy-pp-cli state compare --states CA,TX --start 2024 --end 2024 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			values := strings.Split(states, ",")
			if len(values) < 2 {
				return errors.New("--states requires at least two comma-separated values")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			var series []map[string]any
			periodSets := map[string]map[string]map[string]any{}
			unit := ""
			for _, state := range values {
				state = strings.TrimSpace(state)
				facets := map[string][]string{facetName: {state}}
				if strings.TrimSpace(sector) != "" {
					facets["sectorid"] = []string{strings.TrimSpace(sector)}
				}
				page, err := eiaFetch(ctx, flags, route, seriesParams(data, frequency, start, end, facets, 5000))
				if err != nil {
					return err
				}
				if len(page.Rows) == 0 {
					return fmt.Errorf("no rows returned for state %s", state)
				}
				if pageIsTruncated(page) {
					return fmt.Errorf("state %s response is truncated (%d of %s rows); narrow the period bounds", state, len(page.Rows), page.Total)
				}
				stateUnit, err := consistentSeriesUnit(page.Rows, data)
				if err != nil {
					return fmt.Errorf("state %s: %w", state, err)
				}
				if unit == "" {
					unit = stateUnit
				} else if stateUnit != unit {
					return fmt.Errorf("unit mismatch for %s: %q vs %q", state, stateUnit, unit)
				}
				periodSets[state] = mapPeriods(page.Rows, data)
				series = append(series, map[string]any{"state": state, "total": page.Total, "returned_rows": len(page.Rows), "unit": stateUnit, "statistics": seriesStats(page.Rows, data), "source_rows": page.Rows})
			}
			baseState := strings.TrimSpace(values[0])
			aligned := []map[string]any{}
			for period, baseRow := range periodSets[baseState] {
				row := map[string]any{"period": period, baseState: baseRow[data]}
				complete := true
				for _, rawState := range values[1:] {
					state := strings.TrimSpace(rawState)
					other, ok := periodSets[state][period]
					if !ok {
						complete = false
						break
					}
					row[state] = other[data]
				}
				if complete {
					aligned = append(aligned, row)
				}
			}
			sort.Slice(aligned, func(i, j int) bool { return fmt.Sprint(aligned[i]["period"]) < fmt.Sprint(aligned[j]["period"]) })
			if len(aligned) == 0 {
				return errors.New("selected series have no overlapping periods")
			}
			return emitEIA(cmd, flags, "live", map[string]any{"route": route, "facet": facetName, "sector": sector, "frequency": frequency, "data": data, "start": start, "end": end, "series": series, "aligned_periods": aligned, "alignment": "Rows include only periods present in every selected state after route, data column, frequency, requested bounds, sector, and reported-unit validation.", "caveats": eiaCaveats()})
		},
	}
	cmd.Flags().StringVar(&route, "route", "electricity/retail-sales", "EIA data route")
	cmd.Flags().StringVar(&states, "states", "CA,TX", "Comma-separated state facet values")
	cmd.Flags().StringVar(&facetName, "facet-name", "stateid", "State facet name for the route")
	cmd.Flags().StringVar(&sector, "sector", "ALL", "Optional sectorid facet; empty disables it")
	cmd.Flags().StringVar(&data, "data", "sales", "Data column")
	cmd.Flags().StringVar(&frequency, "frequency", "annual", "Shared frequency")
	cmd.Flags().StringVar(&start, "start", "2020", "Inclusive start period")
	cmd.Flags().StringVar(&end, "end", "2024", "End period")
	return cmd
}
