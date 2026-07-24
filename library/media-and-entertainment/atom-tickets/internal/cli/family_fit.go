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

func newNovelFamilyFitCmd(flags *rootFlags) *cobra.Command {
	var flagLatitude string
	var flagLongitude string
	var flagRatings string
	var flagEndBefore string

	cmd := &cobra.Command{
		Use:         "family-fit",
		Short:       "Rank showtimes satisfying advisory-rating, runtime, ending-time, distance, and budget constraints.",
		Example:     "  atom-tickets-pp-cli family-fit --latitude 40.7505 --longitude -73.9934 --ratings G,PG --end-before 21:00 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would filter Atom showtimes by family constraints")
				return nil
			}
			lat, lon, err := parseAtomCoordinates(flagLatitude, flagLongitude)
			if err != nil {
				return err
			}
			ratings := splitUpper(flagRatings)
			if len(ratings) == 0 {
				return usageErr(fmt.Errorf("--ratings is required"))
			}
			now := time.Now()
			cutoff, err := atomClockOn(now, defaultString(flagEndBefore, "23:59"))
			if err != nil {
				return usageErr(fmt.Errorf("--end-before must use HH:MM"))
			}
			inventory, err := fetchAtomInventory(cmd, flags, lat, lon, 40, now, now.Add(7*24*time.Hour))
			if err != nil {
				return err
			}
			options := inventory.Options[:0]
			for _, option := range inventory.Options {
				when, ok := atomTime(option.Start)
				if !ok || option.AvailableInventory <= 0 || !ratings[strings.ToUpper(option.Rating)] {
					continue
				}
				ends := when.Add(time.Duration(option.RuntimeMinutes) * time.Minute)
				endLimit := cutoff
				if when.Day() != cutoff.Day() {
					endLimit, _ = atomClockOn(when, defaultString(flagEndBefore, "23:59"))
				}
				if !ends.After(endLimit) {
					options = append(options, option)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"ratings": flagRatings, "count": len(options), "options": options}, flags)
		},
	}
	cmd.Flags().StringVar(&flagLatitude, "latitude", "", "Latitude for nearby venues")
	cmd.Flags().StringVar(&flagLongitude, "longitude", "", "Longitude for nearby venues")
	cmd.Flags().StringVar(&flagRatings, "ratings", "", "Comma-separated acceptable advisory ratings")
	cmd.Flags().StringVar(&flagEndBefore, "end-before", "", "Latest local ending time (HH:MM)")
	return cmd
}
