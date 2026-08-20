// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored shared rendering + data-source guards for the schedule novel
// commands. JSON output passes the raw event elements through so --select can
// reach nested fields (e.g. clubCalendarEvent.title); human output is a flat
// table.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// rejectLocalDataSource fails fast when a live-only command is asked for local
// data via --data-source local. These commands always query the live API.
func rejectLocalDataSource(flags *rootFlags) error {
	if flags != nil && flags.dataSource == "local" {
		return usageErr(fmt.Errorf("this command has no local data source; it always queries the live API (omit --data-source local)"))
	}
	return nil
}

// renderEventList emits events as JSON (raw passthrough for --select fidelity)
// or as a human table. emptyNote is printed to stdout for the human path when
// there are no events; the JSON path emits an empty array.
func renderEventList(cmd *cobra.Command, flags *rootFlags, events []calEvent, emptyNote string) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if len(events) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), emptyNote)
			return nil
		}
		headers := []string{"DATE", "TIME", "TYPE", "EVENT", "OPPONENT", "H/A"}
		rows := make([][]string, 0, len(events))
		for _, e := range events {
			rows = append(rows, eventRow(e))
		}
		return flags.printTable(cmd, headers, rows)
	}
	raws := make([]json.RawMessage, 0, len(events))
	for _, e := range events {
		raws = append(raws, e.Raw)
	}
	return flags.printJSON(cmd, raws)
}

// eventRow renders one event as table cells: date, time, type, title,
// opponent, home/away.
func eventRow(e calEvent) []string {
	date, tm := "TBD", ""
	if e.HasStart {
		date = e.Start.Format("Mon Jan 2")
		tm = e.Start.Format("3:04 PM")
	}
	title := e.Title
	if e.Cancelled {
		title = "[CANCELLED] " + title
	}
	ha := ""
	if e.Opponent != "" || e.AwayGame {
		ha = homeAway(e)
	}
	return []string{date, tm, eventTypeName(e.EventTypeID), title, e.Opponent, ha}
}
