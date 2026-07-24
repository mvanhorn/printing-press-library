// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func newNovelPreorderRadarCmd(flags *rootFlags) *cobra.Command {
	var flagLatitude string
	var flagLongitude string
	var flagDays int

	cmd := &cobra.Command{
		Use:         "preorder-radar",
		Short:       "Surface upcoming preorder days across nearby venues with production and venue names resolved.",
		Example:     "  atom-tickets-pp-cli preorder-radar --latitude 40.7505 --longitude -73.9934 --days 30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan official Atom preorder records")
				return nil
			}
			lat, lon, err := parseAtomCoordinates(flagLatitude, flagLongitude)
			if err != nil {
				return err
			}
			if flagDays < 1 || flagDays > 90 {
				return usageErr(fmt.Errorf("--days must be between 1 and 90"))
			}
			now := time.Now()
			inventory, err := fetchAtomInventory(cmd, flags, lat, lon, 40, now, now.Add(time.Duration(flagDays)*24*time.Hour))
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"days": strconv.Itoa(flagDays), "count": len(inventory.Preorders), "preorders": inventory.Preorders}, flags)
		},
	}
	cmd.Flags().StringVar(&flagLatitude, "latitude", "", "Latitude for nearby venues")
	cmd.Flags().StringVar(&flagLongitude, "longitude", "", "Longitude for nearby venues")
	cmd.Flags().IntVar(&flagDays, "days", 30, "Future preorder horizon in days (1-90)")
	return cmd
}
