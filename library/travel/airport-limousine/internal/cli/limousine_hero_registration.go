// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import "github.com/spf13/cobra"

// Register implemented heroes through durable, explicit root wiring. This also
// keeps the root timetable distinct from the pages timetable endpoint.
func init() {
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		heroes := map[string]bool{"timetable": true, "travel-times": true, "transfers": true, "fare": true, "conditions": true}
		for _, existing := range rootCmd.Commands() {
			if heroes[existing.Name()] {
				rootCmd.RemoveCommand(existing)
			}
		}
		rootCmd.AddCommand(newNovelTimetableCmd(flags))
		rootCmd.AddCommand(newNovelTravelTimesCmd(flags))
		rootCmd.AddCommand(newNovelTransfersCmd(flags))
		rootCmd.AddCommand(newNovelFareCmd(flags))
		rootCmd.AddCommand(newNovelConditionsCmd(flags))
		// The generated stop helper is a public GET despite the generic stop-name
		// mutation heuristic. Keep its concrete read-only example in a durable hook.
		if stop, _, err := rootCmd.Find([]string{"pages", "stop"}); err == nil && stop.Name() == "stop" {
			stop.Example = "  airport-limousine-pp-cli pages stop HanedaAirportTerminal3 --json"
			if stop.Annotations == nil {
				stop.Annotations = map[string]string{}
			}
			stop.Annotations["mcp:read-only"] = "true"
		}

	})
}
