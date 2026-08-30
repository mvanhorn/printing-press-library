// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored command wiring for irail-pp-cli.
//
// Registers through the generated registerNovelCommand hook rather than editing
// root.go, so `generate --force` preserves both the sources and this wiring.
//
// Scope note: "stations facilities" and "disruptions route" are NOT registered
// here. The generator already attaches those to their promoted parents (see
// promoted_stations.go and promoted_disruptions.go); registering them again
// produced a duplicate entry under Available Commands.

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, parent := range root.Commands() {
			if parent.Name() == "stations" {
				parent.AddCommand(newIrailStationsSearchCmd(flags))
			}
		}
		root.AddCommand(newIrailSavedCmd(flags))
		root.AddCommand(newIrailOccupancyCmd(flags))
	})
}
