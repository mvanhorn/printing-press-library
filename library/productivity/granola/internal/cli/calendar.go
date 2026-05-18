// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newCalendarCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Calendar overlay across cached meetings",
	}
	cmd.AddCommand(newCalendarOverlayCmd(flags))
	return cmd
}

func newCalendarOverlayCmd(flags *rootFlags) *cobra.Command {
	var week string
	var missedOnly bool
	cmd := &cobra.Command{
		Use:   "overlay",
		Short: "List calendar events for a week, marking which were recorded",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			var anchor time.Time
			if week != "" {
				t, err := parseAnyDate(week)
				if err != nil {
					return usageErr(err)
				}
				anchor = t
			} else {
				anchor = time.Now()
			}
			// Compute Monday->Sunday range.
			start := anchor.AddDate(0, 0, -int(anchor.Weekday())+1)
			if anchor.Weekday() == time.Sunday {
				start = anchor.AddDate(0, 0, -6)
			}
			end := start.AddDate(0, 0, 7)
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			// Index of cached recordings by calendar event id.
			byCalID := map[string]string{}
			for id, d := range c.Documents {
				if d.GoogleCalendarEvent != nil && d.GoogleCalendarEvent.ID != "" {
					byCalID[d.GoogleCalendarEvent.ID] = id
				}
			}
			w := cmd.OutOrStdout()
			// Walk every metadata block — the cache stores invitee
			// information per meeting; we pivot it onto events by id.
			seen := map[string]bool{}
			for mid, md := range c.MeetingsMetadata {
				d := c.DocumentByID(mid)
				if d == nil || d.GoogleCalendarEvent == nil {
					continue
				}
				ev := d.GoogleCalendarEvent
				startTime := extractCalTimeRaw(ev.Start)
				ts, _ := granola.ParseISO(startTime)
				if ts.Before(start) || !ts.Before(end) {
					continue
				}
				recordedID, recorded := byCalID[ev.ID]
				if missedOnly && recorded {
					continue
				}
				if seen[ev.ID] {
					continue
				}
				seen[ev.ID] = true
				rec := map[string]any{
					"event_id":   ev.ID,
					"summary":    ev.Summary,
					"start":      startTime,
					"end":        extractCalTimeRaw(ev.End),
					"recorded":   recorded,
					"meeting_id": recordedID,
					"attendees":  md.Attendees,
				}
				_ = emitNDJSONLine(w, rec)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&week, "week", "", "Anchor date inside the target week (default: today)")
	cmd.Flags().BoolVar(&missedOnly, "missed-only", false, "Show only calendar events not yet recorded")
	return cmd
}

// Ensure fmt used.
var _ = fmt.Sprintf
