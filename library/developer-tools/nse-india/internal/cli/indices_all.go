// Copyright 2026 mayank-lavania. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var categoryAliases = map[string]string{
	"broad-market": "BROAD MARKET INDICES",
	"sectoral":     "SECTORAL INDICES",
	"thematic":     "THEMATIC INDICES",
	"strategy":     "STRATEGY INDICES",
	"fixed-income": "FIXED INCOME INDICES",
	"derivatives":  "INDICES ELIGIBLE IN DERIVATIVES",
}

func newIndicesAllCmd(flags *rootFlags) *cobra.Command {
	var flagCategory string
	var flagSortBy string

	cmd := &cobra.Command{
		Use:   "all",
		Short: "Dashboard of all 139 NSE indices with P/E, P/B, dividend yield, A/D ratio",
		Example: "  nse-india-pp-cli indices all\n" +
			"  nse-india-pp-cli indices all --category sectoral\n" +
			"  nse-india-pp-cli indices all --category broad-market --sort-by pe\n" +
			"  nse-india-pp-cli indices all --json --select index,last,percentChange,pe,pb",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, prov, err := resolveRead(cmd.Context(), c, flags, "indices", false, "/api/allIndices", nil, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Unwrap {"data": [...]} envelope
			var envelope struct {
				Data []map[string]any `json:"data"`
			}
			var items []map[string]any
			if json.Unmarshal(data, &envelope) == nil && envelope.Data != nil {
				items = envelope.Data
			} else {
				_ = json.Unmarshal(data, &items)
			}

			// Filter by category if requested
			if flagCategory != "" {
				canonical, ok := categoryAliases[strings.ToLower(flagCategory)]
				if !ok {
					// accept the raw value too (e.g. "SECTORAL INDICES")
					canonical = strings.ToUpper(flagCategory)
				}
				filtered := items[:0]
				for _, item := range items {
					if k, _ := item["key"].(string); strings.EqualFold(k, canonical) {
						filtered = append(filtered, item)
					}
				}
				items = filtered
			}

			// Sort
			switch strings.ToLower(flagSortBy) {
			case "pe":
				sort.Slice(items, func(i, j int) bool {
					return allIndicesFloat(items[i]["pe"]) < allIndicesFloat(items[j]["pe"])
				})
			case "pb":
				sort.Slice(items, func(i, j int) bool {
					return allIndicesFloat(items[i]["pb"]) < allIndicesFloat(items[j]["pb"])
				})
			case "dy":
				sort.Slice(items, func(i, j int) bool {
					return allIndicesFloat(items[i]["dy"]) > allIndicesFloat(items[j]["dy"])
				})
			case "percentchange", "pchange":
				sort.Slice(items, func(i, j int) bool {
					return allIndicesFloat(items[i]["percentChange"]) > allIndicesFloat(items[j]["percentChange"])
				})
			case "perchange365d", "1y":
				sort.Slice(items, func(i, j int) bool {
					return allIndicesFloat(items[i]["perChange365d"]) > allIndicesFloat(items[j]["perChange365d"])
				})
			case "perchange30d", "30d":
				sort.Slice(items, func(i, j int) bool {
					return allIndicesFloat(items[i]["perChange30d"]) > allIndicesFloat(items[j]["perChange30d"])
				})
			case "last":
				sort.Slice(items, func(i, j int) bool {
					return allIndicesFloat(items[i]["last"]) > allIndicesFloat(items[j]["last"])
				})
			}

			printProvenance(cmd, len(items), prov)

			out, _ := json.Marshal(items)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered := json.RawMessage(out)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) && len(items) > 0 {
				// For table display use a trimmed view: index, last, %chg, pe, pb, dy, A, D
				tableItems := make([]map[string]any, 0, len(items))
				for _, item := range items {
					tableItems = append(tableItems, map[string]any{
						"index":         item["index"],
						"last":          item["last"],
						"change%":       item["percentChange"],
						"pe":            item["pe"],
						"pb":            item["pb"],
						"dy":            item["dy"],
						"advances":      item["advances"],
						"declines":      item["declines"],
						"perChange365d": item["perChange365d"],
					})
				}
				if err := printAutoTable(cmd.OutOrStdout(), tableItems); err != nil {
					return err
				}
				if len(items) >= 25 {
					fmt.Fprintf(os.Stderr, "\nShowing %d indices. Filter with --category or --sort-by. Use --json for full data.\n", len(items))
				}
				return nil
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&flagCategory, "category", "", "Filter by category: broad-market, sectoral, thematic, strategy, fixed-income, derivatives")
	cmd.Flags().StringVar(&flagSortBy, "sort-by", "", "Sort by: last, percentchange, pe, pb, dy, perchange365d, perchange30d")
	return cmd
}

func allIndicesFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		var f float64
		fmt.Sscanf(x, "%f", &f) // #nosec G104 -- error intentionally ignored; default zero returned on parse failure
		return f
	}
	return 0
}
