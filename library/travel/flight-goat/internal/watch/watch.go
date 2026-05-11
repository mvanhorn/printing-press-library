// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// Package watch implements purchased-flight price monitoring for
// flight-goat-pp-cli. Users register a watch on a specific flight they've
// already booked (airline + flight number + date + route + cabin), and the
// `watch check` command compares the live Google Flights price against the
// paid price and surfaces alerts when the same itinerary becomes cheaper by
// more than the user's threshold.
//
// The package is intentionally separate from internal/store: that store is
// generator-owned and migration-controlled for the FlightAware API surface.
// Watches are a hand-written extension, so we keep them in their own SQLite
// database file (default ~/.local/share/flight-goat-pp-cli/watches.db) to
// avoid migration coupling with the generated schema.
package watch

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Status values for a watch row.
const (
	StatusActive   = "active"
	StatusPaused   = "paused"
	StatusArchived = "archived"
)

// MatchConfidence describes how confident the check is that the cheaper
// price it found is the *same* flight the user holds, vs. just a cheaper
// option on the same route.
type MatchConfidence string

const (
	// MatchExact: same airline + flight number + date + route + cabin.
	// Safe to alert on — this is the user's exact flight.
	MatchExact MatchConfidence = "high"

	// MatchProbable: same airline + date + route + cabin, but the provider
	// did not return a flight number for the cheaper itinerary. Could be
	// the same flight; surface as informational but don't alert by default.
	MatchProbable MatchConfidence = "medium"

	// MatchRouteOnly: cheapest fare on the same route, but a different
	// airline or flight number. NEVER triggers an alert — it's the
	// classic "another carrier is cheaper" case which doesn't help if
	// the user can't move their ticket without losing it.
	MatchRouteOnly MatchConfidence = "low"
)

// Watch is one user-registered flight price watch.
type Watch struct {
	ID            string `json:"id"`
	Origin        string `json:"origin"`
	Destination   string `json:"destination"`
	DepartureDate string `json:"departure_date"`
	// DepartureTime is the optional HH:MM local-departure time of the
	// user's flight. When set, the matcher rejects candidate
	// itineraries whose departure time differs by more than ±30 min —
	// guards against airlines reusing the same flight number on a
	// rescheduled departure later in the same day.
	DepartureTime string `json:"departure_time,omitempty"`
	Airline       string `json:"airline"`
	FlightNumber  string `json:"flight_number"`
	Cabin         string `json:"cabin,omitempty"`
	// FareBrand is free-form: "Main Cabin", "Comfort+", "Polaris",
	// "First Saver", etc. Surfaced in the alert so the user can
	// eyeball whether the cheaper fare is comparable (Google Flights
	// doesn't reliably expose brand codes, so we can't auto-verify).
	FareBrand string `json:"fare_brand,omitempty"`
	// ExcludeBasic, when true, filters out basic-economy fares from
	// the search so a $300 basic-economy result does NOT compare
	// against a $700 main-cabin paid ticket. Defaults to true at
	// watch-add time — opt out via --include-basic if the user
	// actually paid for basic economy.
	ExcludeBasic  bool    `json:"exclude_basic"`
	Passengers    int     `json:"passengers"`
	OriginalPrice float64 `json:"original_price"`
	Threshold     float64 `json:"threshold"`
	Currency      string  `json:"currency"`
	Notify        string  `json:"notify,omitempty"`
	BookingRef    string  `json:"booking_ref,omitempty"`
	Notes         string  `json:"notes,omitempty"`
	Status        string  `json:"status"`

	CreatedAt        time.Time  `json:"created_at"`
	LastCheckedAt    *time.Time `json:"last_checked_at,omitempty"`
	LastSeenPrice    *float64   `json:"last_seen_price,omitempty"`
	LastAlertedPrice *float64   `json:"last_alerted_price,omitempty"`
}

// CheckResult is the JSON envelope returned by `watch check` and posted to
// webhook sinks. The shape is stable: SKILL.md documents it and downstream
// tooling consumes it.
type CheckResult struct {
	Schema        string    `json:"schema"`
	WatchID       string    `json:"watch_id"`
	CheckedAt     time.Time `json:"checked_at"`
	Origin        string    `json:"origin"`
	Destination   string    `json:"destination"`
	Date          string    `json:"departure_date"`
	DepartureTime string    `json:"departure_time,omitempty"`
	Airline       string    `json:"airline"`
	FlightNo      string    `json:"flight_number"`
	Cabin         string    `json:"cabin,omitempty"`
	FareBrand     string    `json:"fare_brand,omitempty"`

	OriginalPrice float64 `json:"original_price"`
	Threshold     float64 `json:"threshold"`
	Currency      string  `json:"currency"`

	// BookingURL is a Google Flights search URL pre-filled with the
	// route + date so the user can open the cheaper itinerary in one
	// tap. We deliberately use the search URL (not a deep link to the
	// specific itinerary) because Google's itinerary URLs require an
	// opaque base64 protobuf that's fragile to construct.
	BookingURL string `json:"booking_url"`

	// FoundPrice is the live price for the *matched* itinerary (nil if no
	// itinerary above MatchRouteOnly was found). RouteCheapestPrice is the
	// cheapest fare seen on the route regardless of match — useful for
	// context but never used to trigger an alert.
	FoundPrice          *float64        `json:"found_price,omitempty"`
	RouteCheapestPrice  *float64        `json:"route_cheapest_price,omitempty"`
	Delta               *float64        `json:"delta,omitempty"`
	Confidence          MatchConfidence `json:"confidence"`
	ThresholdCrossed    bool            `json:"threshold_crossed"`
	AlertDispatched     bool            `json:"alert_dispatched"`
	AlertDispatchedTo   string          `json:"alert_dispatched_to,omitempty"`
	AlertSuppressed     bool            `json:"alert_suppressed"`
	AlertSuppressReason string          `json:"alert_suppress_reason,omitempty"`
	// MatchMismatchReason is set when a near-match (same airline +
	// flight number) was rejected for a verifiable mismatch, e.g.
	// departure-time drift outside ±30 min. Surfaced so the user can
	// see "your flight number ran but at a different time" instead of
	// a silent "no match."
	MatchMismatchReason string `json:"match_mismatch_reason,omitempty"`

	// MatchReason is a one-sentence explanation of how the matcher
	// arrived at this Confidence — every constraint that was checked
	// AND passed. Surfaced in the alert so users see the chain of
	// evidence ("same airline DL, flight 668, departure 07:25 within
	// ±30 min of your 07:30, economy cabin, basic-economy excluded")
	// rather than a bare confidence label they have to trust.
	MatchReason string `json:"match_reason,omitempty"`

	// SafetyNotice is the exact rebooking-warning string the alert payload
	// must carry. Surfaced here so JSON consumers (the OpenClaw skill,
	// scripts) cannot accidentally elide it.
	SafetyNotice string `json:"safety_notice"`

	// MatchedFlight describes the live itinerary that matched, when
	// Confidence is MatchExact or MatchProbable.
	MatchedFlight *MatchedFlight `json:"matched_flight,omitempty"`
}

// MatchedFlight is the subset of the gflights.Flight surface we surface
// alongside an alert. We intentionally don't echo every Leg field — alerts
// stay small.
type MatchedFlight struct {
	Airline         string  `json:"airline"`
	FlightNumber    string  `json:"flight_number"`
	Price           float64 `json:"price"`
	Currency        string  `json:"currency"`
	Cabin           string  `json:"cabin,omitempty"`
	FareBrand       string  `json:"fare_brand,omitempty"`
	DepartureTime   string  `json:"departure_time,omitempty"`
	ArrivalTime     string  `json:"arrival_time,omitempty"`
	DurationMinutes int     `json:"duration_minutes,omitempty"`
	Stops           int     `json:"stops"`
}

// SafetyNoticeText is the rebooking-warning string every alert payload
// surfaces. Keep it stable; tests assert on its presence.
const SafetyNoticeText = "Same flight appears cheaper. Verify fare rules, refundability, cancellation fees, credits, and seat/bag differences before canceling or rebooking."

// Validation

var (
	iataAirportRe = regexp.MustCompile(`^[A-Z]{3}$`)
	// Airline codes are typically IATA 2-letter (DL, AA) but a few
	// providers use 3-letter ICAO (DAL, AAL). Accept both.
	iataAirlineRe = regexp.MustCompile(`^[A-Z0-9]{2,3}$`)
	flightNoRe    = regexp.MustCompile(`^[0-9]{1,5}[A-Z]?$`)
	hhmmRe        = regexp.MustCompile(`^([01]?[0-9]|2[0-3]):[0-5][0-9]$`)
	cabinSet      = map[string]struct{}{
		"":                {},
		"economy":         {},
		"premium_economy": {},
		"business":        {},
		"first":           {},
	}
)

// Validate checks the user-supplied fields on a Watch. Returns the first
// problem found; callers should surface this as a usage error.
func (w *Watch) Validate() error {
	if w == nil {
		return errors.New("watch is nil")
	}
	w.Origin = strings.ToUpper(strings.TrimSpace(w.Origin))
	w.Destination = strings.ToUpper(strings.TrimSpace(w.Destination))
	w.Airline = strings.ToUpper(strings.TrimSpace(w.Airline))
	w.FlightNumber = strings.ToUpper(strings.TrimSpace(w.FlightNumber))
	w.Cabin = strings.ToLower(strings.TrimSpace(w.Cabin))
	w.Currency = strings.ToUpper(strings.TrimSpace(w.Currency))

	if !iataAirportRe.MatchString(w.Origin) {
		return fmt.Errorf("origin %q is not a 3-letter IATA airport code", w.Origin)
	}
	if !iataAirportRe.MatchString(w.Destination) {
		return fmt.Errorf("destination %q is not a 3-letter IATA airport code", w.Destination)
	}
	if w.Origin == w.Destination {
		return fmt.Errorf("origin and destination cannot match (%s)", w.Origin)
	}
	if _, err := time.Parse("2006-01-02", w.DepartureDate); err != nil {
		return fmt.Errorf("departure date %q is not YYYY-MM-DD", w.DepartureDate)
	}
	w.DepartureTime = strings.TrimSpace(w.DepartureTime)
	if w.DepartureTime != "" && !hhmmRe.MatchString(w.DepartureTime) {
		return fmt.Errorf("departure time %q must be HH:MM in 24-hour format", w.DepartureTime)
	}
	w.FareBrand = strings.TrimSpace(w.FareBrand)
	if !iataAirlineRe.MatchString(w.Airline) {
		return fmt.Errorf("airline %q is not a 2- or 3-character carrier code", w.Airline)
	}
	if !flightNoRe.MatchString(w.FlightNumber) {
		return fmt.Errorf("flight number %q must be digits with an optional trailing letter (e.g. 669, 1234A)", w.FlightNumber)
	}
	if _, ok := cabinSet[w.Cabin]; !ok {
		return fmt.Errorf("cabin %q must be one of economy, premium_economy, business, first", w.Cabin)
	}
	if w.Passengers <= 0 {
		w.Passengers = 1
	}
	if w.OriginalPrice <= 0 {
		return fmt.Errorf("paid price must be > 0, got %v", w.OriginalPrice)
	}
	if w.Threshold < 0 {
		return fmt.Errorf("threshold must be >= 0, got %v", w.Threshold)
	}
	if w.Currency == "" {
		w.Currency = "USD"
	}
	if len(w.Currency) != 3 {
		return fmt.Errorf("currency %q must be a 3-letter ISO 4217 code", w.Currency)
	}
	if w.Notify != "" {
		if err := validateNotify(w.Notify); err != nil {
			return err
		}
	}
	if w.Status == "" {
		w.Status = StatusActive
	}
	switch w.Status {
	case StatusActive, StatusPaused, StatusArchived:
	default:
		return fmt.Errorf("status %q must be one of active, paused, archived", w.Status)
	}
	return nil
}

// validateNotify accepts the same dispatch surface as alert.go: stdout,
// json, webhook:<https-url>. Anything else is a usage error.
func validateNotify(spec string) error {
	switch {
	case spec == "stdout", spec == "json":
		return nil
	case strings.HasPrefix(spec, "webhook:"):
		raw := strings.TrimPrefix(spec, "webhook:")
		if raw == "" {
			return errors.New("notify webhook: needs a URL")
		}
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("notify webhook URL %q is invalid: %w", raw, err)
		}
		if u.Scheme != "https" && u.Scheme != "http" {
			return fmt.Errorf("notify webhook URL %q must use http or https", raw)
		}
		if u.Host == "" {
			return fmt.Errorf("notify webhook URL %q has no host", raw)
		}
		return nil
	default:
		return fmt.Errorf("notify %q must be stdout, json, or webhook:<url>", spec)
	}
}
