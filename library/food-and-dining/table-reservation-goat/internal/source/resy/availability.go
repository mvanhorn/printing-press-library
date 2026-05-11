// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

// PATCH: resy-source-port — see .printing-press-patches.json for the change-set rationale.

package resy

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// AvailabilityParams are the inputs to Availability(). VenueID is Resy's
// numeric venue id stringified — matches the search result's `id` field.
type AvailabilityParams struct {
	VenueID   string
	Date      string // YYYY-MM-DD
	PartySize int
}

// Availability returns the open slots Resy reports for the given
// venue/date/party. An empty result is a valid signal: the venue is on
// Resy but has no openings.
func (c *Client) Availability(ctx context.Context, params AvailabilityParams) ([]Slot, error) {
	body, err := c.rawFind(ctx, params.VenueID, params.Date, params.PartySize)
	if err != nil {
		return nil, err
	}
	return ParseAvailabilityResponse(body)
}

// VenueIdentity is the minimal venue metadata returned alongside slots in
// /4/find responses. Resy has no public restaurant-detail endpoint, so
// this is the most we can lift from the consumer surface without
// scraping a session-cookied SSR page.
type VenueIdentity struct {
	ID   string
	Name string
}

// VenueIdentityByID probes /4/find for the given venue on a single
// date and returns the venue's id+name from the response envelope.
// Returns an empty struct + nil error when /4/find returns no rows
// matching `venueID` on that specific date.
//
// CAVEAT: /4/find sometimes returns an empty results.venues list when
// there are no published slots for the requested date — even for
// venues that demonstrably exist on Resy and have slots on adjacent
// dates (live-tested 2026-05-11 against Le Bernardin id=1387 with a
// probe date 14 days out: empty envelope despite the venue having
// availability 2-5 days out). Callers that want a robust "does this
// venue exist on Resy" probe should use VenueIdentityProbe, which
// walks several forward dates before declaring not-found. Use this
// single-date entry point only when the caller already has a date
// with known availability (e.g., a slot-book pre-flight).
func (c *Client) VenueIdentityByID(ctx context.Context, venueID, date string, partySize int) (VenueIdentity, error) {
	body, err := c.rawFind(ctx, venueID, date, partySize)
	if err != nil {
		return VenueIdentity{}, err
	}
	var r resyFindResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return VenueIdentity{}, fmt.Errorf("resy: parse find for venue identity: %w", err)
	}
	if r.Results == nil {
		return VenueIdentity{}, nil
	}
	// We REQUIRE an exact id match. /4/find takes a venue_id query
	// argument and is targeted, so any row whose `venue.id` doesn't
	// match the requested ID is a different venue (Resy occasionally
	// includes related-venue rows in the envelope) and must not be
	// taken as the identity for `venueID`. The previous "id == "" &&
	// name != """ fallback could silently match an empty-id sibling
	// row and return its name as the requested venue's identity.
	// Returning empty + nil error when no row matches keeps the
	// "not on Resy" signal honest for the caller.
	for _, v := range r.Results.Venues {
		if v.Venue == nil || v.Venue.ID == nil || len(v.Venue.ID.Resy) == 0 {
			continue
		}
		id := unquoteJSON(v.Venue.ID.Resy)
		if id != venueID {
			continue
		}
		return VenueIdentity{ID: id, Name: v.Venue.Name}, nil
	}
	return VenueIdentity{}, nil
}

// VenueIdentityProbe walks several forward dates probing for the
// venue's identity. /4/find can return an empty results.venues list
// for venues with no published slots on the probed date, even when
// the venue has availability on adjacent dates (live-confirmed
// 2026-05-11 against Le Bernardin id=1387). Probing today plus a
// small spread of forward offsets lifts the false-negative rate
// without exhaustively walking the calendar.
//
// Returns the first successful identity. Returns empty + nil error
// when no probed date yields a match — at that point the venue
// either isn't on Resy or has no published inventory in the probed
// window (the latter is rare; most venues publish far enough out
// that one of the offsets lands on a populated date).
//
// `partySize` is forwarded to /4/find unchanged; a sensible default
// is 2 for the broadest match.
func (c *Client) VenueIdentityProbe(ctx context.Context, venueID string, partySize int) (VenueIdentity, error) {
	today := time.Now().UTC()
	// Cover the near (1d), mid (3d, 7d), far (14d, 30d) horizons —
	// max 5 calls, average 1-2 before a hit on a venue with normal
	// inventory.
	for _, days := range []int{1, 3, 7, 14, 30} {
		date := today.AddDate(0, 0, days).Format("2006-01-02")
		identity, err := c.VenueIdentityByID(ctx, venueID, date, partySize)
		if err != nil {
			return VenueIdentity{}, err
		}
		if identity.ID != "" {
			return identity, nil
		}
	}
	return VenueIdentity{}, nil
}

type resyFindConfig struct {
	ID    json.RawMessage `json:"id"`
	Token string          `json:"token"`
	Type  string          `json:"type"`
}

type resyFindDate struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type resyFindSlot struct {
	Date   *resyFindDate   `json:"date"`
	Config *resyFindConfig `json:"config"`
	Size   *resyFindSize   `json:"size"`
}

type resyFindSize struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type resyFindVenue struct {
	Venue *struct {
		ID *struct {
			Resy json.RawMessage `json:"resy"`
		} `json:"id"`
		Name string `json:"name"`
	} `json:"venue"`
	Slots []resyFindSlot `json:"slots"`
}

type resyFindResponse struct {
	Results *struct {
		Venues []resyFindVenue `json:"venues"`
	} `json:"results"`
}

// ParseAvailabilityResponse parses a /4/find body into Slot rows. Exported
// for tests.
func ParseAvailabilityResponse(raw []byte) ([]Slot, error) {
	var r resyFindResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("resy: parse availability: %w", err)
	}
	if r.Results == nil {
		return nil, nil
	}
	out := make([]Slot, 0)
	for _, v := range r.Results.Venues {
		for _, s := range v.Slots {
			if s.Config == nil || s.Config.Token == "" {
				continue
			}
			if s.Date == nil || s.Date.Start == "" {
				continue
			}
			slot := Slot{
				Token: s.Config.Token,
				Time:  ParseResyTime(s.Date.Start),
				Type:  s.Config.Type,
			}
			if len(s.Config.ID) > 0 {
				slot.ConfigID = unquoteJSON(s.Config.ID)
			}
			if s.Size != nil && s.Size.Max > 0 {
				slot.PartySize = s.Size.Max
			}
			out = append(out, slot)
		}
	}
	return out, nil
}

// resyTimeRegexp matches "HH:MM" or "HH:MM:SS" inside a free-form date
// string like "2026-05-15 19:00:00". The regex tolerates the seconds being
// present or absent.
var resyTimeRegexp = regexp.MustCompile(`\b(\d{2}):(\d{2})(?::\d{2})?\b`)

// ParseResyTime extracts "HH:MM" out of a Resy date string. Returns the
// input unchanged if no match is found, matching the TS reference behavior
// — surfaces the raw value to humans without dropping data.
func ParseResyTime(s string) string {
	m := resyTimeRegexp.FindStringSubmatch(s)
	if len(m) < 3 {
		return s
	}
	return m[1] + ":" + m[2]
}

// parseIntOrZero is used in the list parser to fold party-size fields that
// might arrive as numbers or strings.
func parseIntOrZero(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	if n, err := strconv.Atoi(unquoteJSON(raw)); err == nil {
		return n
	}
	return 0
}
