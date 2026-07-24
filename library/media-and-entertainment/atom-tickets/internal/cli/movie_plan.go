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

func newNovelMoviePlanCmd(flags *rootFlags) *cobra.Command {
	var flagLatitude string
	var flagLongitude string
	var flagAfter string
	var flagBefore string

	cmd := &cobra.Command{
		Use:         "movie-plan",
		Short:       "Rank bookable nearby showtimes by distance, start time, price, format, rating, and runtime.",
		Example:     "  atom-tickets-pp-cli movie-plan --latitude 40.7505 --longitude -73.9934 --after 18:00 --before 22:00 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank official Atom showtimes")
				return nil
			}
			lat, lon, err := parseAtomCoordinates(flagLatitude, flagLongitude)
			if err != nil {
				return err
			}
			now := time.Now()
			start, err := atomClockOn(now, defaultString(flagAfter, "00:00"))
			if err != nil {
				return usageErr(fmt.Errorf("--after must use HH:MM"))
			}
			end, err := atomClockOn(now, defaultString(flagBefore, "23:59"))
			if err != nil || end.Before(start) {
				return usageErr(fmt.Errorf("--before must use HH:MM and not precede --after"))
			}
			inventory, err := fetchAtomInventory(cmd, flags, lat, lon, 40, start, end)
			if err != nil {
				return err
			}
			options := inventory.Options[:0]
			for _, option := range inventory.Options {
				when, ok := atomTime(option.Start)
				if ok && !when.Before(start) && !when.After(end) && option.AvailableInventory > 0 && option.CheckoutURL != "" {
					options = append(options, option)
				}
			}
			sort.SliceStable(options, func(i, j int) bool {
				if options[i].DistanceKM != options[j].DistanceKM {
					return options[i].DistanceKM < options[j].DistanceKM
				}
				return strings.Compare(options[i].Start, options[j].Start) < 0
			})
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"count": len(options), "options": options, "purchase": "Checkout URLs open Atom checkout; this CLI does not complete payment or seat selection."}, flags)
		},
	}
	cmd.Flags().StringVar(&flagLatitude, "latitude", "", "Latitude for nearby venues")
	cmd.Flags().StringVar(&flagLongitude, "longitude", "", "Longitude for nearby venues")
	cmd.Flags().StringVar(&flagAfter, "after", "", "Earliest local start time (HH:MM)")
	cmd.Flags().StringVar(&flagBefore, "before", "", "Latest local start time (HH:MM)")
	return cmd
}
