// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: Millard County location registry.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelLocationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "locations",
		Short: "List the curated Millard County town/ZIP set the county sweep covers",
		Long: "List the curated Millard County town/ZIP set that 'millard', 'landuse', and " +
			"'agenda scan' use. County bodies surface under the nearest town, so the union of " +
			"these locations gives full county coverage.",
		Example:     "  utah-pmn-pp-cli locations --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			b, err := json.Marshal(millardLocations)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	return cmd
}
