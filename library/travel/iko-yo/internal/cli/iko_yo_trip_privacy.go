// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		previous := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			isTrip := false
			for c := cmd; c != nil; c = c.Parent() {
				if c.Name() == "trip" {
					isTrip = true
					break
				}
			}
			if isTrip {
				flags.noLearn = true
			}
			var err error
			if previous != nil {
				err = previous(cmd, args)
			}
			// A named profile may reset noLearn during the root hook. Trip
			// constraints must remain invocation-only after that hook too.
			if isTrip {
				flags.noLearn = true
			}
			return err
		}
	})
}
