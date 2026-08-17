// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through fetchCPSC in cpsc_novel_support.go

package cli

import (
	"github.com/spf13/cobra"
	"net/url"
	"sort"
	"strings"
)

func newNovelHazardPulseCmd(flags *rootFlags) *cobra.Command {
	var window string

	cmd := &cobra.Command{
		Use:         "hazard-pulse",
		Short:       "Flatten recent product, hazard, remedy, and injury relationships without inventing incident rates.",
		Example:     "cpsc-recalls-pp-cli hazard-pulse --window 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitCPSCDryRun(cmd, flags, "hazard-pulse", map[string]any{"window": window, "request_parameters": []string{"RecallDateStart", "RecallDateEnd"}})
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			start, end, err := cpscWindow(window)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			rows, err := newCPSCFetcher(flags).fetch(ctx, url.Values{"RecallDateStart": {start.Format("2006-01-02")}, "RecallDateEnd": {end.Format("2006-01-02")}})
			if err != nil {
				return err
			}
			hazards, remedies, injuries, products := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
			for _, row := range rows {
				addDistinctRecallLabels(hazards, nestedNames(row, "Hazards"))
				addDistinctRecallLabels(remedies, nestedNames(row, "RemedyOptions"))
				addDistinctRecallLabels(injuries, nestedNames(row, "Injuries"))
				addDistinctRecallLabels(products, nestedNames(row, "Products"))
			}
			return emitCPSC(cmd, flags, "live", map[string]any{"window": map[string]any{"duration": window, "start": start.Format("2006-01-02"), "end": end.Format("2006-01-02")}, "recall_count": len(rows), "products": countRows(products), "hazards": countRows(hazards), "remedy_options": countRows(remedies), "injury_reports": countRows(injuries), "caveats": cpscCaveats()})
		},
	}
	cmd.Flags().StringVar(&window, "window", "30d", "Recent recall window")
	return cmd
}

func addDistinctRecallLabels(counts map[string]int, values []string) {
	seen := map[string]bool{}
	for _, value := range values {
		label := strings.ToLower(strings.Join(strings.Fields(value), " "))
		if label != "" && !seen[label] {
			counts[label]++
			seen[label] = true
		}
	}
}

func countRows(values map[string]int) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for value, count := range values {
		out = append(out, map[string]any{"value": value, "recall_count": count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["recall_count"].(int) > out[j]["recall_count"].(int) })
	return out
}
