// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelAwayCmd(flags *rootFlags) *cobra.Command {
	var weeks int
	cmd := &cobra.Command{
		Use:   "away",
		Short: "Away games only, location-first, with field, opponent, and date — built for the carpool group chat.",
		Long: "List upcoming away games only, location-first, over the next N weeks.\n\n" +
			"Use this command for away games with their fields and opponents. Do NOT use it for home games " +
			"or the full schedule; use 'week' or 'agenda'.",
		Example:     "  sprocket-pp-cli away --weeks 4\n  sprocket-pp-cli away --weeks 8 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			if weeks < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--weeks must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			events, err := fetchCalendar(ctx, c, now, now.AddDate(0, 0, weeks*7))
			if err != nil {
				return err
			}
			away := filterAway(events)
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(away) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No away games in the next %d weeks.\n", weeks)
					return nil
				}
				headers := []string{"DATE", "TIME", "OPPONENT", "LOCATION", "EVENT"}
				rows := make([][]string, 0, len(away))
				for _, e := range away {
					rows = append(rows, awayRow(e))
				}
				return flags.printTable(cmd, headers, rows)
			}
			raws := make([]json.RawMessage, 0, len(away))
			for _, e := range away {
				raws = append(raws, e.Raw)
			}
			return flags.printJSON(cmd, raws)
		},
	}
	cmd.Flags().IntVar(&weeks, "weeks", 4, "number of weeks ahead to include")
	return cmd
}

// filterAway returns only the away-game events.
func filterAway(events []calEvent) []calEvent {
	out := make([]calEvent, 0, len(events))
	for _, e := range events {
		if e.AwayGame {
			out = append(out, e)
		}
	}
	return out
}

func awayRow(e calEvent) []string {
	date, tm := "TBD", ""
	if e.HasStart {
		date = e.Start.Format("Mon Jan 2")
		tm = e.Start.Format("3:04 PM")
	}
	loc := e.Location
	if loc == "" && e.LocationID != 0 {
		loc = fmt.Sprintf("location #%d", e.LocationID)
	}
	title := e.Title
	if e.Cancelled {
		// Mark cancelled games, like eventRow — a carpool drive list must not
		// send families to a field for a game that is off.
		title = "[CANCELLED] " + title
	}
	return []string{date, tm, e.Opponent, loc, title}
}
