// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared helpers for the hand-built Ikon transcendence commands
// (most-visited, plan, calendar, watch, changes). Hand-authored — not generated.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ikonGetter is the subset of *client.Client the novel commands use. An
// interface keeps the pure-ish fetch helpers testable with a fake.
type ikonGetter interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

// ikonData unwraps the standard {"data": ..., "errors": [...]} Ikon envelope.
func ikonData(raw json.RawMessage) (json.RawMessage, error) {
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("unexpected response shape: %w", err)
	}
	if len(env.Errors) > 0 {
		var errs []string
		if json.Unmarshal(env.Errors, &errs) == nil && len(errs) > 0 {
			return nil, fmt.Errorf("ikon API: %s", strings.Join(errs, "; "))
		}
		var generic any
		if json.Unmarshal(env.Errors, &generic) == nil && !isEmptyJSONValue(generic) {
			return nil, fmt.Errorf("ikon API: %s", compactJSON(env.Errors))
		}
	}
	if env.Data == nil {
		return json.RawMessage("null"), nil
	}
	return env.Data, nil
}

func isEmptyJSONValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len() == 0
	default:
		return false
	}
}

func compactJSON(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err == nil {
		return buf.String()
	}
	return strings.TrimSpace(string(raw))
}

// ----- resorts -----

type ikonResort struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	Code                string `json:"code"`
	Region              string `json:"region"`
	ReservationsEnabled bool   `json:"reservations_enabled"`
}

func fetchResorts(ctx context.Context, c ikonGetter) ([]ikonResort, error) {
	raw, err := c.Get(ctx, "/api/v2/resorts", nil)
	if err != nil {
		return nil, err
	}
	data, err := ikonData(raw)
	if err != nil {
		return nil, err
	}
	var resorts []ikonResort
	if err := json.Unmarshal(data, &resorts); err != nil {
		return nil, fmt.Errorf("parsing resorts: %w", err)
	}
	return resorts, nil
}

func reservableResorts(all []ikonResort) []ikonResort {
	out := make([]ikonResort, 0)
	for _, r := range all {
		if r.ReservationsEnabled {
			out = append(out, r)
		}
	}
	return out
}

// ----- availability -----

type passAvailability struct {
	ID                    int      `json:"id"`
	HasAccess             bool     `json:"has_access"`
	ReservationsAvailable int      `json:"reservations_available"`
	UnavailableDates      []string `json:"unavailable_dates"`
	BlackoutDates         []string `json:"blackout_dates"`
	ClosedDates           []string `json:"closed_dates"`
	MaxReservationDate    string   `json:"max_reservation_date"`
}

func fetchAvailability(ctx context.Context, c ikonGetter, resortID int) ([]passAvailability, error) {
	path := replacePathParam("/api/v2/reservation-availability/{id}", "id", strconv.Itoa(resortID))
	raw, err := c.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	data, err := ikonData(raw)
	if err != nil {
		return nil, err
	}
	var passes []passAvailability
	if err := json.Unmarshal(data, &passes); err != nil {
		return nil, fmt.Errorf("parsing availability: %w", err)
	}
	return passes, nil
}

// passWindow is the precomputed bookability state for one accessible pass.
type passWindow struct {
	full    map[string]bool
	maxDate string
	slots   int
}

// accessiblePasses precomputes the full-date set for each pass that has access.
func accessiblePasses(passes []passAvailability) []passWindow {
	out := make([]passWindow, 0, len(passes))
	for _, p := range passes {
		if !p.HasAccess {
			continue
		}
		full := map[string]bool{}
		for _, d := range p.UnavailableDates {
			full[d] = true
		}
		for _, d := range p.BlackoutDates {
			full[d] = true
		}
		for _, d := range p.ClosedDates {
			full[d] = true
		}
		out = append(out, passWindow{full: full, maxDate: p.MaxReservationDate, slots: p.ReservationsAvailable})
	}
	return out
}

// bookable reports whether the user can reserve date (YYYY-MM-DD) with at least
// one of their accessible passes: slots remaining, within the season window, and
// the date not blacked-out/closed/full for that pass.
func bookable(windows []passWindow, date string) bool {
	for _, w := range windows {
		if w.slots <= 0 {
			continue
		}
		if w.maxDate != "" && date > w.maxDate {
			continue
		}
		if !w.full[date] {
			return true
		}
	}
	return false
}

// fullDateSet returns the union of non-bookable dates across accessible passes —
// used by `changes` to snapshot what was full at a resort.
func fullDateSet(passes []passAvailability) []string {
	set := map[string]bool{}
	for _, p := range passes {
		if !p.HasAccess {
			continue
		}
		for _, d := range p.UnavailableDates {
			set[d] = true
		}
		for _, d := range p.BlackoutDates {
			set[d] = true
		}
		for _, d := range p.ClosedDates {
			set[d] = true
		}
	}
	out := make([]string, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// ----- dates -----

const isoDate = "2006-01-02"

// enumerateDates returns every YYYY-MM-DD from..to inclusive.
func enumerateDates(from, to string) ([]string, error) {
	start, err := time.Parse(isoDate, from)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", from)
	}
	end, err := time.Parse(isoDate, to)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", to)
	}
	if end.Before(start) {
		return nil, fmt.Errorf("end date %s is before start date %s", to, from)
	}
	out := make([]string, 0)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format(isoDate))
	}
	return out, nil
}

// monthBounds returns the first and last day of the month "YYYY-MM".
func monthBounds(month string) (from, to string, err error) {
	first, err := time.Parse("2006-01", month)
	if err != nil {
		return "", "", fmt.Errorf("invalid month %q (want YYYY-MM)", month)
	}
	last := first.AddDate(0, 1, -1)
	return first.Format(isoDate), last.Format(isoDate), nil
}

// ----- pass usage / most-visited aggregation -----

type redemption struct {
	ResortName     string `json:"resort_name"`
	RedemptionDate string `json:"redemption_date"`
	IsReturn       bool   `json:"is_return"`
}

type seasonUsage struct {
	DisplayName string       `json:"display_name"`
	UsedDays    int          `json:"used_days"`
	MaxDays     *int         `json:"max_days"`
	Redemptions []redemption `json:"redemptions"`
}

type seasonDays struct {
	Season string `json:"season"`
	Days   int    `json:"days"`
}

type resortRollup struct {
	Resort    string       `json:"resort"`
	TotalDays int          `json:"total_days"`
	Seasons   []seasonDays `json:"seasons"`
}

type fetchFailure struct {
	ProductID string `json:"product_id"`
	Error     string `json:"error"`
}

type mostVisitedView struct {
	Resorts        []resortRollup `json:"resorts"`
	TotalDays      int            `json:"total_days"`
	SeasonsTracked int            `json:"seasons_tracked"`
	PassesScanned  int            `json:"passes_scanned"`
	FetchFailures  []fetchFailure `json:"fetch_failures"`
}

var seasonRe = regexp.MustCompile(`Winter \d{4}-\d{4}`)

func parseSeason(displayName string) string {
	if m := seasonRe.FindString(displayName); m != "" {
		return m
	}
	return displayName
}

func resortFromSeasonUsage(s seasonUsage) string {
	for _, r := range s.Redemptions {
		if r.ResortName != "" {
			return r.ResortName
		}
	}
	name := seasonRe.ReplaceAllString(s.DisplayName, "")
	name = strings.TrimSpace(name)
	name = strings.TrimRight(name, ", ")
	return strings.TrimSpace(name)
}

// seasonDaysCount prefers the API's used_days; falls back to counting first-entry
// redemptions (is_return=false) so re-entries on the same day don't double count.
func seasonDaysCount(s seasonUsage) int {
	if s.UsedDays > 0 {
		return s.UsedDays
	}
	n := 0
	for _, r := range s.Redemptions {
		if !r.IsReturn {
			n++
		}
	}
	return n
}

// aggregateMostVisited joins per-pass season usage into a resort ranking. Pure
// function — unit tested in ikon_common_test.go.
func aggregateMostVisited(perProduct [][]seasonUsage, passesScanned int, failures []fetchFailure) mostVisitedView {
	type acc struct {
		total   int
		seasons []seasonDays
	}
	byResort := map[string]*acc{}
	order := []string{}
	seasonsSeen := map[string]bool{}
	total := 0

	for _, seasons := range perProduct {
		for _, s := range seasons {
			resort := resortFromSeasonUsage(s)
			days := seasonDaysCount(s)
			if resort == "" {
				continue
			}
			a := byResort[resort]
			if a == nil {
				a = &acc{}
				byResort[resort] = a
				order = append(order, resort)
			}
			a.total += days
			a.seasons = append(a.seasons, seasonDays{Season: parseSeason(s.DisplayName), Days: days})
			seasonsSeen[parseSeason(s.DisplayName)] = true
			total += days
		}
	}

	rollups := make([]resortRollup, 0, len(order))
	for _, resort := range order {
		a := byResort[resort]
		sort.SliceStable(a.seasons, func(i, j int) bool { return a.seasons[i].Season < a.seasons[j].Season })
		rollups = append(rollups, resortRollup{Resort: resort, TotalDays: a.total, Seasons: a.seasons})
	}
	sort.SliceStable(rollups, func(i, j int) bool { return rollups[i].TotalDays > rollups[j].TotalDays })

	if failures == nil {
		failures = []fetchFailure{}
	}
	return mostVisitedView{
		Resorts:        rollups,
		TotalDays:      total,
		SeasonsTracked: len(seasonsSeen),
		PassesScanned:  passesScanned,
		FetchFailures:  failures,
	}
}

// diffFullDates compares two snapshots of full dates: opened = dates that left
// the full set (freed up), closed = dates newly full. Pure function.
func diffFullDates(prev, cur []string) (opened, closed []string) {
	ps := map[string]bool{}
	for _, d := range prev {
		ps[d] = true
	}
	cs := map[string]bool{}
	for _, d := range cur {
		cs[d] = true
	}
	opened = make([]string, 0)
	closed = make([]string, 0)
	for d := range ps {
		if !cs[d] {
			opened = append(opened, d)
		}
	}
	for d := range cs {
		if !ps[d] {
			closed = append(closed, d)
		}
	}
	sort.Strings(opened)
	sort.Strings(closed)
	return opened, closed
}
