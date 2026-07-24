// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelAccessibleShowtimesCmd(flags *rootFlags) *cobra.Command {
	var flagLatitude string
	var flagLongitude string
	var flagAttribute string

	cmd := &cobra.Command{
		Use:         "accessible-showtimes",
		Short:       "Return only bookable nearby showtimes matching required accessibility or seating attributes.",
		Example:     "  atom-tickets-pp-cli accessible-showtimes --latitude 40.7505 --longitude -73.9934 --attribute 'Closed Captioning' --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would filter official Atom accessibility attributes")
				return nil
			}
			lat, lon, err := parseAtomCoordinates(flagLatitude, flagLongitude)
			if err != nil {
				return err
			}
			if strings.TrimSpace(flagAttribute) == "" {
				return usageErr(fmt.Errorf("--attribute is required"))
			}
			now := time.Now()
			inventory, err := fetchAtomInventory(cmd, flags, lat, lon, 40, now, now.Add(7*24*time.Hour))
			if err != nil {
				return err
			}
			options := inventory.Options[:0]
			for _, option := range inventory.Options {
				if option.AvailableInventory <= 0 || option.CheckoutURL == "" {
					continue
				}
				for _, attribute := range option.Attributes {
					if strings.Contains(strings.ToLower(attribute), strings.ToLower(flagAttribute)) {
						options = append(options, option)
						break
					}
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"attribute": flagAttribute, "count": len(options), "options": options}, flags)
		},
	}
	cmd.Flags().StringVar(&flagLatitude, "latitude", "", "Latitude for nearby venues")
	cmd.Flags().StringVar(&flagLongitude, "longitude", "", "Longitude for nearby venues")
	cmd.Flags().StringVar(&flagAttribute, "attribute", "", "Required attribute, for example Closed Captioning")
	return cmd
}
