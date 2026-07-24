// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelLastCallCmd(flags *rootFlags) *cobra.Command {
	var flagLatitude string
	var flagLongitude string
	var flagWithin string

	cmd := &cobra.Command{
		Use:         "last-call",
		Short:       "Find soon-starting showtimes that still report inventory and provide direct checkout URLs.",
		Example:     "  atom-tickets-pp-cli last-call --latitude 40.7505 --longitude -73.9934 --within 90m --agent --select options.production,options.venue,options.start,options.checkout_url",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would find imminent inventory-backed Atom showtimes")
				return nil
			}
			lat, lon, err := parseAtomCoordinates(flagLatitude, flagLongitude)
			if err != nil {
				return err
			}
			window, err := time.ParseDuration(defaultString(flagWithin, "90m"))
			if err != nil || window <= 0 || window > 12*time.Hour {
				return usageErr(fmt.Errorf("--within must be a duration between 1 minute and 12 hours"))
			}
			now := time.Now()
			inventory, err := fetchAtomInventory(cmd, flags, lat, lon, 40, now, now.Add(window))
			if err != nil {
				return err
			}
			options := inventory.Options[:0]
			for _, option := range inventory.Options {
				when, ok := atomTime(option.Start)
				if ok && !when.Before(now) && !when.After(now.Add(window)) && option.AvailableInventory > 0 && option.CheckoutURL != "" {
					options = append(options, option)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"within": window.String(), "count": len(options), "options": options, "purchase": "Checkout URLs open Atom checkout; this CLI does not complete payment or seat selection."}, flags)
		},
	}
	cmd.Flags().StringVar(&flagLatitude, "latitude", "", "Latitude for nearby venues")
	cmd.Flags().StringVar(&flagLongitude, "longitude", "", "Longitude for nearby venues")
	cmd.Flags().StringVar(&flagWithin, "within", "90m", "Future window such as 45m or 2h")
	return cmd
}
