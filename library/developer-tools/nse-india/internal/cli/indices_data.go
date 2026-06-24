// Copyright 2026 mayank-lavania. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newIndicesDataCmd(flags *rootFlags) *cobra.Command {
	var flagIndex string

	cmd := &cobra.Command{
		Use:   "data",
		Short: "Full index summary: P/E, P/B, dividend yield, market cap, OHLC, 52w range",
		Example: "  nse-india-pp-cli indices data --index \"NIFTY 50\"\n" +
			"  nse-india-pp-cli indices data --index \"NIFTY BANK\" --json\n" +
			"  nse-india-pp-cli indices data --index \"NIFTY IT\" --select peRatio,pbRatio,dividentYield",
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
				"functionName": "getIndexData",
			}
			if flagIndex != "" {
				params["index"] = flagIndex
			}

			data, prov, err := resolveRead(cmd.Context(), c, flags, "indices", false, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Unwrap {"data": [...]} envelope — getIndexData returns a single-element array
			var envelope struct {
				Data []map[string]any `json:"data"`
			}
			var out json.RawMessage
			if json.Unmarshal(data, &envelope) == nil && len(envelope.Data) > 0 {
				item := envelope.Data[0]
				// Rename for clarity
				if v, ok := item["dividentYield"]; ok {
					item["dividendYield"] = v
					delete(item, "dividentYield")
				}
				if v, ok := item["ffm"]; ok {
					item["freeFloatMarketCap"] = v
					delete(item, "ffm")
				}
				if v, ok := item["full"]; ok {
					item["fullMarketCap"] = v
					delete(item, "full")
				}
				b, _ := json.Marshal(item)
				out = json.RawMessage(b)
			} else {
				out = data
			}

			printProvenance(cmd, 1, prov)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered := out
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
				var item map[string]any
				if json.Unmarshal(out, &item) == nil {
					if err := printAutoTable(cmd.OutOrStdout(), []map[string]any{item}); err != nil {
						return err
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagIndex, "index", "", "Index name (e.g. 'NIFTY 50', 'NIFTY BANK', 'NIFTY IT')")
	return cmd
}
