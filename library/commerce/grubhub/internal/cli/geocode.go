// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/grubhub"
	"github.com/spf13/cobra"
)

func newGeocodeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "geocode <address>",
		Short: "Resolve a street address to Grubhub coordinates",
		Long: "Resolve a street address to the latitude/longitude and WKT POINT Grubhub uses for delivery search.\n\n" +
			"Other commands (near, compare, dish, deals, pick) geocode addresses for you automatically; use this when you want the raw coordinates.",
		Example:     "  grubhub-pp-cli geocode \"350 5th Ave, New York, NY\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would geocode the given address")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an address is required, e.g. geocode \"350 5th Ave, New York, NY\""))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := grubhubClient(ctx, flags)
			if err != nil {
				return err
			}
			coord, err := geocodeAddress(ctx, c, args[0])
			if err != nil {
				return err
			}
			view := map[string]any{
				"address":   args[0],
				"latitude":  coord.Lat,
				"longitude": coord.Lng,
				"point":     grubhub.FormatPoint(coord.Lng, coord.Lat),
				"locality":  coord.Locality,
				"region":    coord.Region,
				"postal":    coord.Postal,
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s, %s %s\n%.6f, %.6f\n%s\n",
				coord.Locality, coord.Region, coord.Postal, coord.Lat, coord.Lng, grubhub.FormatPoint(coord.Lng, coord.Lat))
			return nil
		},
	}
	return cmd
}
