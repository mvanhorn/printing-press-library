// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import "github.com/spf13/cobra"

func init() {
	// This POST is a search, so the generic import framework has no writable resource.
	delete(resourceWritePaths, "source")
	baseline := []whichEntry{
		{Command: "flights search", Description: "Search live Haneda flights by flight number, airline, destination and status", Group: "Live flight sources"},
		{Command: "flights detail", Description: "Resolve primary, marketing codeshare or padded flight numbers to source detail", Group: "Live flight sources"},
		{Command: "flights disruptions", Description: "Inspect source-reported delayed canceled diverted flights and airport summary", Group: "Live flight sources"},
		{Command: "catalog airports", Description: "Resolve airport city codes and English Japanese airport names", Group: "Source identifiers"},
		{Command: "catalog airlines", Description: "Resolve airline provider ICAO codes and IATA flight prefixes", Group: "Source identifiers"},
		{Command: "schedule search", Description: "Search published monthly flight schedule periods and operating weekdays", Group: "Published schedules"},
	}
	whichIndex = append(baseline, whichIndex...)
	registerNovelCommand(func(root *cobra.Command, _ *rootFlags) {
		if board, _, err := root.Find([]string{"source", "board"}); err == nil && board.Name() == "board" {
			board.Example = "  haneda-airport-pp-cli source board --search-date 20261003 --agent"
		}
		for _, name := range []string{"import", "sync", "search"} {
			if cmd, _, err := root.Find([]string{name}); err == nil && cmd.Name() == name {
				root.RemoveCommand(cmd)
			}
		}
	})
}
