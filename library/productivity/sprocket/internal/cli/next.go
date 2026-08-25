// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelNextCmd(flags *rootFlags) *cobra.Command {
	var lookAheadDays int
	cmd := &cobra.Command{
		Use:   "next",
		Short: "The single next upcoming game or practice across all your players' teams: what, when, where, home/away, opponent.",
		Long: "Show the one next upcoming event across all your players' teams.\n\n" +
			"Use this command for the one next event on the family calendar. Do NOT use it for a " +
			"multi-event list; use 'week' or 'agenda'.",
		Example: "  sprocket-pp-cli next\n" +
			"  sprocket-pp-cli next --agent --select clubCalendarEvent.title,clubCalendarEvent.startDate,clubCalendarEvent.opponent,clubCalendarEvent.awayGame",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			events, err := fetchCalendar(ctx, c, now, now.AddDate(0, 0, lookAheadDays))
			if err != nil {
				return err
			}
			next := firstFutureEvent(events, now)
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if next == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "No upcoming events in the next %d days.\n", lookAheadDays)
					return nil
				}
				printEventDetail(cmd, *next)
				return nil
			}
			if next == nil {
				_, err := cmd.OutOrStdout().Write([]byte("null\n"))
				return err
			}
			return flags.printJSON(cmd, json.RawMessage(next.Raw))
		},
	}
	cmd.Flags().IntVar(&lookAheadDays, "look-ahead-days", 60, "how many days ahead to search for the next event")
	return cmd
}

// firstFutureEvent returns the earliest event starting at or after now, or nil.
func firstFutureEvent(events []calEvent, now time.Time) *calEvent {
	for i := range events {
		if events[i].HasStart && !events[i].Start.Before(now) {
			return &events[i]
		}
	}
	return nil
}

// printEventDetail writes a readable multi-line summary of a single event.
func printEventDetail(cmd *cobra.Command, e calEvent) {
	w := cmd.OutOrStdout()
	when := "TBD"
	if e.HasStart {
		when = e.Start.Format("Mon Jan 2, 2006 at 3:04 PM")
	}
	title := e.Title
	if e.Cancelled {
		title = "[CANCELLED] " + title
	}
	fmt.Fprintf(w, "%s\n", title)
	fmt.Fprintf(w, "  When: %s\n", when)
	if t := eventTypeName(e.EventTypeID); t != "" {
		fmt.Fprintf(w, "  Type: %s\n", t)
	}
	if e.Opponent != "" {
		fmt.Fprintf(w, "  Opponent: %s (%s)\n", e.Opponent, homeAway(e))
	}
	if e.Location != "" {
		fmt.Fprintf(w, "  Location: %s\n", e.Location)
	}
}
