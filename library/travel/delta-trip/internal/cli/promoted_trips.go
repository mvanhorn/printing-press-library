// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newTripsPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagNoCache bool

	cmd := &cobra.Command{
		Use:         "trips <confirmation> <first-name> <last-name>",
		Short:       "Look up a Delta trip by confirmation number",
		Long:        "Fetch full trip details from delta.com using confirmation number and passenger name. No login required.",
		Example:     "  delta-trip trips ABC123 JANE SMITH\n  delta-trip trips ABC123 JANE SMITH --json",
		Annotations: map[string]string{"pp:endpoint": "trips.get", "pp:method": "GET", "mcp:read-only": "true"},
		Args:        cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, first, last := strings.ToUpper(args[0]), strings.ToUpper(args[1]), strings.ToUpper(args[2])
			trip, err := fetchAndCacheTrip(cmd.Context(), conf, first, last, flags, flagNoCache)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				b, _ := json.MarshalIndent(trip, "", "  ")
				prov := DataProvenance{Source: "live", ResourceType: tripCacheType}
				wrapped, _ := wrapWithProvenance(b, prov)
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			return printTripTable(cmd.OutOrStdout(), trip)
		},
	}
	cmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "Bypass local cache and fetch live from delta.com")

	// Wire sibling endpoints and sub-resources as subcommands
	cmd.AddCommand(newTripFlightsCmd(flags))
	cmd.AddCommand(newCheckinStatusCmd(flags))

	return cmd
}

// newTripsGetCmd is the explicit `trips get` subcommand.
func newTripsGetCmd(flags *rootFlags) *cobra.Command {
	var flagNoCache bool
	cmd := &cobra.Command{
		Use:     "get <confirmation> <first-name> <last-name>",
		Short:   "Get trip details by confirmation number",
		Example: "  delta-trip trips get ABC123 JANE SMITH",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, first, last := strings.ToUpper(args[0]), strings.ToUpper(args[1]), strings.ToUpper(args[2])
			trip, err := fetchAndCacheTrip(cmd.Context(), conf, first, last, flags, flagNoCache)
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(trip, "", "  ")
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				prov := DataProvenance{Source: "live", ResourceType: tripCacheType}
				wrapped, _ := wrapWithProvenance(b, prov)
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			return printTripTable(cmd.OutOrStdout(), trip)
		},
	}
	cmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "Bypass local cache and fetch live from delta.com")
	return cmd
}

// newFlightsPromotedCmd is redefined here to use the browser scraper.
// The generated version in promoted_flights.go is replaced by this registration.
func init() {
	_ = fmt.Sprintf // keep import
}
