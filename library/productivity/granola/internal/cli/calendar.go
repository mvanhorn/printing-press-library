// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/spf13/cobra"
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
			// PATCH(dual-path-store-read): store first, cache fallback.
			// Calendar events themselves (summary, start, end) live only in
			// the cache's google_calendar_event block, so when the cache is
			// unreadable the overlay is rebuilt from the store's
			// calendar_event_id / started_at / ended_at columns instead of
			// silently emitting nothing.
			c, err := openGranolaRead(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()
			w := cmd.OutOrStdout()
			if !c.hasCache() {
				return emitStoreCalendarOverlay(cmd.Context(), c, w, start, end, missedOnly)
			}
			cache := c.Cache()
			// Index of cached recordings by calendar event id.
			byCalID := map[string]string{}
			for id, d := range c.Documents() {
				if d.GoogleCalendarEvent != nil && d.GoogleCalendarEvent.ID != "" {
					byCalID[d.GoogleCalendarEvent.ID] = id
				}
			}
			// Walk every metadata block — the cache stores invitee
			// information per meeting; we pivot it onto events by id.
			seen := map[string]bool{}
			for mid, md := range cache.MeetingsMetadata {
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

// emitStoreCalendarOverlay rebuilds the overlay from the store when no
// desktop cache is readable. Every store meeting carrying a
// calendar_event_id is by definition a recorded event, so --missed-only
// yields nothing on this path: the store only knows about meetings Granola
// actually captured, never about invitations it skipped.
func emitStoreCalendarOverlay(ctx context.Context, v *granolaRead, w io.Writer, start, end time.Time, missedOnly bool) error {
	if missedOnly || v == nil || v.store == nil {
		return nil
	}
	rows, err := v.store.DB().QueryContext(ctx, `
		SELECT id, title, calendar_event_id, started_at, ended_at
		FROM meetings
		WHERE calendar_event_id IS NOT NULL AND calendar_event_id != ''
		ORDER BY started_at DESC
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var id, title, eventID, startedAt, endedAt string
		if err := rows.Scan(&id, &title, &eventID, &startedAt, &endedAt); err != nil {
			return nil
		}
		ts, _ := granola.ParseISO(startedAt)
		if ts.Before(start) || !ts.Before(end) {
			continue
		}
		if seen[eventID] {
			continue
		}
		seen[eventID] = true
		var attendees []granola.CalendarInvitee
		if md := v.MeetingMetadataByID(id); md != nil {
			attendees = md.Attendees
		}
		_ = emitNDJSONLine(w, map[string]any{
			"event_id":   eventID,
			"summary":    title,
			"start":      startedAt,
			"end":        endedAt,
			"recorded":   true,
			"meeting_id": id,
			"attendees":  attendees,
		})
	}
	return nil
}

// Ensure fmt used.
var _ = fmt.Sprintf
