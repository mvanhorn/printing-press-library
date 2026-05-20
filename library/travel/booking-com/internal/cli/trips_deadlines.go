// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/booking"
	"github.com/PuerkitoBio/goquery"
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
			// secure.booking.com hosts authenticated trip data; www.booking.com returns 404.
			data, err := c.Get("https://secure.booking.com/mytrips.html", nil)
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

			// Build a confirmation -> deadline-string map by walking each
			// trip card in the list-page DOM and running the regex against
			// just that card's subtree text. The previous version ran the
			// regex against the whole document, so every trip got the same
			// (or missing) deadline regardless of its actual cancellation
			// terms. Per-trip detail fetches would be more accurate but
			// would require a per-trip URL pattern the browser-sniff didn't
			// capture (the printer's account had no upcoming trips), so we
			// scope to per-card text and surface "cancellation deadline
			// unavailable" honestly when a card has no deadline text.
			perTripDeadline := perTripCardDeadlines(data)

			cutoff := time.Now().Add(within)
			out := make([]tripDeadline, 0)
			for _, trip := range trips {
				deadline := perTripDeadline[trip.ConfirmationNumber]
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

// perTripCardDeadlines walks /mytrips.html and returns a confirmation-number
// keyed map of per-card cancellation-deadline strings (empty when the card
// has no deadline text). The selector matches booking.ParseTrips so the two
// loops stay aligned. Returns an empty map if the HTML can't be parsed; the
// caller falls through to "cancellation deadline unavailable" per trip.
func perTripCardDeadlines(data []byte) map[string]string {
	out := map[string]string{}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return out
	}
	doc.Find(`[data-testid="trip-card"], .booking-card, .trip__upcoming-list-item`).Each(func(_ int, card *goquery.Selection) {
		text := card.Text()
		conf := perTripConfirmation(text)
		if conf == "" {
			return
		}
		if dl := extractDeadlineText(text); dl != "" {
			out[conf] = dl
		}
	})
	return out
}

// perTripConfirmation mirrors booking.labeledValue's "Confirmation number" /
// "Booking number" lookup but is local to this file so the cli package does
// not need to import booking's private helpers.
var confirmationRE = regexp.MustCompile(`(?i)(?:confirmation number|booking number)\s*[:\-]?\s*([A-Za-z0-9\-]{5,20})`)

func perTripConfirmation(text string) string {
	if m := confirmationRE.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return ""
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
