// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newTripsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "trips",
		Short: "List your upcoming and past reservations",
		Long: `List your Airbnb reservations. The trips query bundle only loads on the
/trips page, so if the operation hash isn't known yet, run 'airbnb-outreach-pp-cli ops
refresh' once to harvest it.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c := newAirbnbClient(flags)
			data, err := c.Trips()
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			return flags.printJSON(cmd, data)
		},
	}
}
