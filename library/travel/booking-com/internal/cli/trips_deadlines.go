// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/booking"
	"github.com/spf13/cobra"
)

type tripDeadline struct {
	booking.Trip
	FreeCancellationUntil string `json:"free_cancellation_until,omitempty"`
	Note                  string `json:"note,omitempty"`
}

func newTripsDeadlinesCmd(flags *rootFlags) *cobra.Command {
	var within time.Duration
	cmd := &cobra.Command{
		Use:         "deadlines",
		Short:       "List trip cancellation deadlines within a duration",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]tripDeadline, 0))
			}
			c, err := flags.newClient()
			if err != nil {
				return fmt.Errorf("trips deadlines: %w", err)
			}
			data, err := c.Get("/mytrips.html", nil)
			if err != nil {
				return fmt.Errorf("trips deadlines: %w", err)
			}
			parsed, err := booking.ParseTrips(data)
			if err != nil {
				return fmt.Errorf("trips deadlines: %w", err)
			}
			trips := make([]booking.Trip, 0)
			if err := json.Unmarshal(parsed, &trips); err != nil {
				return fmt.Errorf("trips deadlines: %w", err)
			}
			cutoff := time.Now().Add(within)
			out := make([]tripDeadline, 0)
			for _, trip := range trips {
				deadline := extractDeadlineText(string(data))
				if deadline == "" {
					out = append(out, tripDeadline{Trip: trip, Note: "cancellation deadline unavailable"})
					continue
				}
				if t, ok := parseLooseDate(deadline); ok && t.After(time.Now()) && t.Before(cutoff) {
					out = append(out, tripDeadline{Trip: trip, FreeCancellationUntil: t.Format(dateOnly)})
				}
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().DurationVar(&within, "within", 14*24*time.Hour, "Deadline window duration")
	return cmd
}

var deadlineRE = regexp.MustCompile(`(?i)(?:free cancellation until|cancel(?:lation)?[^.]{0,40}until)\s+([A-Za-z]{3,9}\s+\d{1,2},?\s+\d{4}|\d{4}-\d{2}-\d{2})`)

func extractDeadlineText(text string) string {
	if m := deadlineRE.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return ""
}

func parseLooseDate(s string) (time.Time, bool) {
	for _, layout := range []string{dateOnly, "January 2, 2006", "Jan 2, 2006", "January 2 2006", "Jan 2 2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
