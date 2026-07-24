// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelDealFinderCmd(flags *rootFlags) *cobra.Command {
	var flagLatitude string
	var flagLongitude string
	var flagMovie string

	cmd := &cobra.Command{
		Use:         "deal-finder",
		Short:       "Find the lowest advertised ticket offers across nearby supported venues.",
		Example:     "  atom-tickets-pp-cli deal-finder --latitude 40.7505 --longitude -73.9934 --movie Superman --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare advertised Atom ticket offers")
				return nil
			}
			lat, lon, err := parseAtomCoordinates(flagLatitude, flagLongitude)
			if err != nil {
				return err
			}
			now := time.Now()
			inventory, err := fetchAtomInventory(cmd, flags, lat, lon, 40, now, now.Add(7*24*time.Hour))
			if err != nil {
				return err
			}
			options := inventory.Options[:0]
			for _, option := range inventory.Options {
				if option.Price <= 0 || option.AvailableInventory <= 0 || option.CheckoutURL == "" {
					continue
				}
				if flagMovie != "" && !strings.Contains(strings.ToLower(option.Production), strings.ToLower(flagMovie)) {
					continue
				}
				options = append(options, option)
			}
			sort.SliceStable(options, func(i, j int) bool {
				if options[i].Price != options[j].Price {
					return options[i].Price < options[j].Price
				}
				return options[i].DistanceKM < options[j].DistanceKM
			})
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"count": len(options), "options": options, "price_note": "Prices are advertised offer values and may exclude checkout fees."}, flags)
		},
	}
	cmd.Flags().StringVar(&flagLatitude, "latitude", "", "Latitude for nearby venues")
	cmd.Flags().StringVar(&flagLongitude, "longitude", "", "Longitude for nearby venues")
	cmd.Flags().StringVar(&flagMovie, "movie", "", "Optional movie title filter")
	return cmd
}
