// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/autotempest"

	"github.com/spf13/cobra"
)

// newSourcesCmd lists the nine user-facing AutoTempest source codes and their
// kind: "inline" sources (te/hem/cs/cv/cm/eb/ot) return parsed per-car
// listings; "link" sources (fbm = Facebook Marketplace, st = SearchTempest /
// Craigslist) are comparison-link-only because those sites block scraping or
// require login. Static data — no network, no store.
func newSourcesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "List the AutoTempest search sources (codes, names, country, kind)",
		Example: strings.Trim(`
  autotempest-pp-cli sources --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				rows := make([]map[string]any, 0, len(autotempest.Sources))
				for _, s := range autotempest.Sources {
					rows = append(rows, map[string]any{
						"code":    s.Code,
						"name":    s.Name,
						"country": s.Country,
						"kind":    s.Kind,
					})
				}
				if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
					return err
				}
				fmt.Fprintln(cmd.ErrOrStderr(),
					"\ninline = parsed listings; link = comparison link only (fbm/st block scraping or require login)")
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), autotempest.Sources, flags)
		},
	}
	return cmd
}
