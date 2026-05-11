// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

// PATCH: resy-source-port — see .printing-press-patches.json for the change-set rationale.

package resy

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ListReservations returns the user's upcoming Resy reservations. Resy's
// real-world payload omits the venue name from the `venue` object — it lives
// in `share.generic_message` as natural language, so the parser regex-folds
// it out.
func (c *Client) ListReservations(ctx context.Context) ([]UpcomingReservation, error) {
	body, err := c.rawReservations(ctx)
	if err != nil {
		return nil, err
	}
	return ParseReservationsResponse(body)
}

type resyShareMessage struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type resyShare struct {
	GenericMessage string             `json:"generic_message"`
	Message        []resyShareMessage `json:"message"`
}

// resyTimeSlot covers two wire shapes seen on the time_slot field:
//
//	{"date": "YYYY-MM-DD HH:MM:SS"}   ← legacy time_slot object form
//
// AND a third shape seen on the top-level `date` field of some legacy
// reservations:
//
//	{"start": "YYYY-MM-DD HH:MM:SS"}  ← legacy top-level date.start
//
// Both keys are accepted so a single struct can be reused for both wire
// fields. The parser picks whichever is non-empty.
type resyTimeSlot struct {
	Date  string `json:"date"`
	Start string `json:"start"`
}

type resyStatus struct {
	Reservation string `json:"reservation"`
	Finished    int    `json:"finished"`
	NoShow      int    `json:"no_show"`
}

type resyReservationRaw struct {
	ResyToken     string          `json:"resy_token"`
	ReservationID json.RawMessage `json:"reservation_id"`
	Day           string          `json:"day"`
	NumSeats      json.RawMessage `json:"num_seats"`
	Venue         *struct {
		ID   json.RawMessage `json:"id"`
		Name string          `json:"name"`
	} `json:"venue"`
	// time_slot is "HH:MM:SS" string in modern Resy, {date: "..."} object in
	// legacy. We use json.RawMessage and discriminate inside the parser.
	TimeSlot json.RawMessage `json:"time_slot"`
	When     string          `json:"when"`
	Date     *resyTimeSlot   `json:"date"`
	Status   json.RawMessage `json:"status"`
	Share    *resyShare      `json:"share"`
}

type resyReservationsResponse struct {
	Reservations []resyReservationRaw `json:"reservations"`
	Results      *struct {
		Reservations []resyReservationRaw `json:"reservations"`
	} `json:"results"`
}

// Two regexes mirror the TS reference: "Reservation at <name>" is the
// stronger end-of-string anchor; the looser "RSVP for <name> on" pattern is
// a fallback. The two-pass priority matters when both phrasings appear in
// the share payload.
var (
	resVenuePatternAt   = regexp.MustCompile(`(?i)Reservation at\s+(.+?)\s*$`)
	resVenuePatternRSVP = regexp.MustCompile(`(?i)RSVP for\s+(.+?)\s+on\b`)
)

// ExtractVenueNameFromShare pulls the venue name out of Resy's share
// payload. Returns empty string when no pattern matches.
func ExtractVenueNameFromShare(share *resyShare) string {
	if share == nil {
		return ""
	}
	pool := make([]string, 0, 4)
	if share.GenericMessage != "" {
		pool = append(pool, share.GenericMessage)
	}
	for _, m := range share.Message {
		if m.Title != "" {
			pool = append(pool, m.Title)
		}
		if m.Body != "" {
			pool = append(pool, m.Body)
		}
	}
	for _, s := range pool {
		if m := resVenuePatternAt.FindStringSubmatch(s); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}
	for _, s := range pool {
		if m := resVenuePatternRSVP.FindStringSubmatch(s); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

// ParseReservationsResponse turns a /3/user/reservations body into rows.
// Exported for testing.
func ParseReservationsResponse(raw []byte) ([]UpcomingReservation, error) {
	var r resyReservationsResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("resy: parse reservations: %w", err)
	}
	rows := r.Reservations
	if len(rows) == 0 && r.Results != nil {
		rows = r.Results.Reservations
	}
	out := make([]UpcomingReservation, 0, len(rows))
	for _, row := range rows {
		id := row.ResyToken
		if id == "" && len(row.ReservationID) > 0 {
			id = unquoteJSON(row.ReservationID)
		}
		if id == "" {
			continue
		}
		venueID := ""
		venueName := ""
		if row.Venue != nil {
			if len(row.Venue.ID) > 0 {
				venueID = unquoteJSON(row.Venue.ID)
			}
			venueName = row.Venue.Name
		}
		if venueName == "" {
			venueName = ExtractVenueNameFromShare(row.Share)
		}

		// time_slot can be a bare "HH:MM:SS" string or an object with a
		// `date` field carrying "YYYY-MM-DD HH:MM:SS". Try string first;
		// fall back to object decode.
		var timeSource string
		if len(row.TimeSlot) > 0 {
			var asStr string
			if err := json.Unmarshal(row.TimeSlot, &asStr); err == nil && asStr != "" {
				timeSource = asStr
			} else {
				var asObj resyTimeSlot
				if err := json.Unmarshal(row.TimeSlot, &asObj); err == nil {
					timeSource = asObj.Date
				}
			}
		}
		if timeSource == "" && row.Date != nil {
			// Legacy reservation payloads spell the datetime under
			// either `date.start` (older shape) or `date.date` (rarer
			// shape seen alongside the time_slot.date form). Try Start
			// first since it's the dominant legacy variant.
			if row.Date.Start != "" {
				timeSource = row.Date.Start
			} else {
				timeSource = row.Date.Date
			}
		}

		day := row.Day
		if day == "" && timeSource != "" {
			// Legacy datetime "YYYY-MM-DD HH:MM:SS"; the date slice is the
			// first 10 chars iff the prefix looks like a date.
			if len(timeSource) >= 10 && timeSource[4] == '-' && timeSource[7] == '-' {
				day = timeSource[:10]
			}
		}
		timeStr := ""
		if timeSource != "" {
			timeStr = ParseResyTime(timeSource)
		}

		status := ""
		if len(row.Status) > 0 {
			var asStr string
			if err := json.Unmarshal(row.Status, &asStr); err == nil && asStr != "" {
				status = asStr
			} else {
				var asObj resyStatus
				if err := json.Unmarshal(row.Status, &asObj); err == nil {
					switch {
					case asObj.Reservation != "":
						status = asObj.Reservation
					case asObj.Finished == 1:
						status = "Completed"
					case asObj.NoShow == 1:
						status = "No-show"
					}
				}
			}
		}

		out = append(out, UpcomingReservation{
			ID:        id,
			VenueID:   venueID,
			VenueName: venueName,
			Date:      day,
			Time:      timeStr,
			PartySize: parseIntOrZero(row.NumSeats),
			Status:    status,
		})
	}
	return out, nil
}

// FilterUpcoming drops reservations whose date is strictly before today
// (YYYY-MM-DD). Resy returns recent history alongside upcoming rows; the
// CLI's `list --upcoming` path uses this filter.
func FilterUpcoming(rs []UpcomingReservation, today string) []UpcomingReservation {
	out := make([]UpcomingReservation, 0, len(rs))
	for _, r := range rs {
		if r.Date >= today {
			out = append(out, r)
		}
	}
	return out
}
