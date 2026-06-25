// Copyright 2026 mayank-lavania. Licensed under Apache-2.0. See LICENSE.
// PATCH: constituents-new-endpoint — switched from dead /api/equity-stockIndices to
// /api/NextApi/apiClient/indexTrackerApi?functionName=getConstituents which is the
// current NSE index tracker API. Adds weightage field and --sort-by flag.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

func newIndicesConstituentsCmd(flags *rootFlags) *cobra.Command {
	var flagIndex string
	var flagSortBy string

	cmd := &cobra.Command{
		Use:   "constituents",
		Short: "Live prices and index weightage for all constituent stocks",
		Example: "  nse-india-pp-cli indices constituents --index \"NIFTY 50\"\n" +
			"  nse-india-pp-cli indices constituents --index \"NIFTY BANK\" --sort-by pchange\n" +
			"  nse-india-pp-cli indices constituents --index \"NIFTY 50\" --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("index") && !flags.dryRun {
				return fmt.Errorf("required flag \"index\" not set")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/api/NextApi/apiClient/indexTrackerApi"
			params := map[string]string{
				"functionName": "getConstituents",
				"noofrecords":  "0",
			}
			if flagIndex != "" {
				params["index"] = flagIndex
			}

			data, prov, err := resolveRead(cmd.Context(), c, flags, "indices", false, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Unwrap {"data": [...]} envelope from indexTrackerApi
			var envelope struct {
				Data []map[string]any `json:"data"`
			}
			var items []map[string]any
			if json.Unmarshal(data, &envelope) == nil && envelope.Data != nil {
				items = envelope.Data
			} else {
				_ = json.Unmarshal(data, &items)
			}

			// Normalise field names: API uses camelCase variants inconsistently
			normalised := make([]map[string]any, 0, len(items))
			for _, item := range items {
				n := map[string]any{
					"symbol":           item["cmSymbol"],
					"indexName":        flagIndex,
					"lastTradedPrice":  item["lasttradedPrice"],
					"change":           item["change"],
					"pChange":          item["pchange"],
					"totalTradedQty":   item["totaltradedquantity"],
					"totalTradedValue": item["totaltradedvalue"],
					"weightage":        item["weightage"],
				}
				normalised = append(normalised, n)
			}

			// Persist normalised rows so index-driver can query the store without
			// re-fetching the live API. Writes under resource_type="index_constituents"
			// keyed by symbol_indexName.
			if flagIndex != "" && len(normalised) > 0 {
				writeConstituentCache(cmd.Context(), flagIndex, normalised)
			}

			switch flagSortBy {
			case "pchange":
				sort.Slice(normalised, func(i, j int) bool {
					return constituentFloat(normalised[i]["pChange"]) > constituentFloat(normalised[j]["pChange"])
				})
			case "volume":
				sort.Slice(normalised, func(i, j int) bool {
					return constituentFloat(normalised[i]["totalTradedQty"]) > constituentFloat(normalised[j]["totalTradedQty"])
				})
			default: // weightage
				sort.Slice(normalised, func(i, j int) bool {
					return constituentFloat(normalised[i]["weightage"]) > constituentFloat(normalised[j]["weightage"])
				})
			}

			printProvenance(cmd, len(normalised), prov)

			out, _ := json.Marshal(normalised)
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
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(normalised) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), normalised); err != nil {
						return err
					}
					if len(normalised) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d constituents. Use --json --select symbol,weightage,pChange for compact output.\n", len(normalised))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagIndex, "index", "", "Index name (e.g. 'NIFTY 50', 'NIFTY BANK', 'NIFTY IT')")
	cmd.Flags().StringVar(&flagSortBy, "sort-by", "weightage", "Sort by: weightage (default), pchange, volume")
	return cmd
}

func constituentFloat(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}
