// Copyright 2026 nicolas-correa. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newFlavorsListCmd is the explicit "flavors list" subcommand, equivalent to
// the top-level "flavors" promoted command but with --format support.
func newFlavorsListCmd(flags *rootFlags) *cobra.Command {
	var flagCountry string
	var flagLang string
	var flagType string
	var flagFormat string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List HEETS, TEREA, and other consumable sticks for a market",
		Example: strings.Trim(`
  iqos-pp-cli flavors list --country gb --type heets --json
  iqos-pp-cli flavors list --type terea --country de
  iqos-pp-cli flavors list --type heets --json --format csv`, "\n"),
		Annotations: map[string]string{"pp:endpoint": "flavors.list", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/{country}/{lang}.sitemap_product.xml"
			path = replacePathParam(path, "country", flagCountry)
			path = replacePathParam(path, "lang", flagLang)
			params := map[string]string{}
			if flagType != "" {
				params["type"] = flagType
			}
			data, _, err := resolveRead(cmd.Context(), c, flags, "flavors", false, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			data = extractResponseData(data)

			if strings.ToLower(flagFormat) == "csv" {
				var items []map[string]any
				if err := json.Unmarshal(data, &items); err != nil {
					return err
				}
				w := csv.NewWriter(cmd.OutOrStdout())
				if len(items) > 0 {
					keys := make([]string, 0, len(items[0]))
					for k := range items[0] {
						keys = append(keys, k)
					}
					_ = w.Write(keys)
					for _, item := range items {
						row := make([]string, len(keys))
						for i, k := range keys {
							row[i] = fmt.Sprintf("%v", item[k])
						}
						_ = w.Write(row)
					}
				}
				w.Flush()
				return w.Error()
			}
			return printOutput(cmd.OutOrStdout(), data, flags.asJSON || !isTerminal(cmd.OutOrStdout()))
		},
	}
	cmd.Flags().StringVar(&flagCountry, "country", "gb", "Country code")
	cmd.Flags().StringVar(&flagLang, "lang", "en", "Language code")
	cmd.Flags().StringVar(&flagType, "type", "", "Stick type filter (heets, terea, levia, veev)")
	cmd.Flags().StringVar(&flagFormat, "format", "json", "Output format: json or csv")
	return cmd
}
