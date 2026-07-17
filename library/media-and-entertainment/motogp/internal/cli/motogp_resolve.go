// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0.
// Shared resolution helpers for the novel MotoGP commands. These turn
// human-friendly inputs (year, class name, event/circuit name, rider name)
// into the chained UUIDs the Pulselive API requires, by querying the live API.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/motogp/internal/client"
)

// parseYearArg parses a 4-digit season year from a CLI argument.
func parseYearArg(s string) (int, error) {
	y, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || y < 1949 || y > 2100 {
		return 0, fmt.Errorf("invalid year %q (expected a 4-digit season year like 2024)", s)
	}
	return y, nil
}

// jsonDecode is a thin wrapper so command files don't each import encoding/json.
func jsonDecode(raw json.RawMessage, v any) error {
	return json.Unmarshal(raw, v)
}

// ---- lightweight response models ----

type mgpSeason struct {
	ID      string `json:"id"`
	Year    int    `json:"year"`
	Current bool   `json:"current"`
}

type mgpCategory struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	LegacyID int    `json:"legacy_id"`
}

type mgpNamed struct {
	Name string `json:"name"`
}

type mgpEvent struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ShortName     string   `json:"short_name"`
	SponsoredName string   `json:"sponsored_name"`
	DateStart     string   `json:"date_start"`
	DateEnd       string   `json:"date_end"`
	Test          bool     `json:"test"`
	Circuit       mgpNamed `json:"circuit"`
	Country       mgpNamed `json:"country"`
}

func (e mgpEvent) label() string {
	if strings.TrimSpace(e.Name) != "" {
		return e.Name
	}
	if strings.TrimSpace(e.SponsoredName) != "" {
		return e.SponsoredName
	}
	return e.ShortName
}

type mgpSession struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Number int    `json:"number"`
}

// mgpRider models a rider across two response shapes: the /riders list uses
// name+surname, while session/standings classifications use a flat full_name.
type mgpRider struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Surname      string   `json:"surname"`
	FullNameFlat string   `json:"full_name"`
	Nickname     string   `json:"nickname"`
	Number       int      `json:"number"`
	LegacyID     int      `json:"legacy_id"`
	Country      mgpNamed `json:"country"`
	Step         struct {
		Category mgpNamed `json:"category"`
		Number   int      `json:"number"`
	} `json:"current_career_step"`
}

func (r mgpRider) fullName() string {
	if n := strings.TrimSpace(r.FullNameFlat); n != "" {
		return n
	}
	return strings.TrimSpace(strings.TrimSpace(r.Name) + " " + strings.TrimSpace(r.Surname))
}

// stableKey returns a stable identity for a rider that survives display-name
// variance (casing / spacing / composed-vs-flat) across rounds. It prefers the
// UUID, then the numeric legacy ID, then the race number, and only falls back
// to a normalized full name when no identifier is present. Aggregations (e.g.
// championship points) must key on this, never on fullName(): the API can emit
// "FullNameFlat" one round and "Name"+"Surname" the next for the same rider,
// and a display-name key would then split one rider into two totals.
func (r mgpRider) stableKey() string {
	if id := strings.TrimSpace(r.ID); id != "" {
		return "id:" + id
	}
	if r.LegacyID != 0 {
		return "legacy:" + strconv.Itoa(r.LegacyID)
	}
	if r.Number != 0 {
		return "num:" + strconv.Itoa(r.Number)
	}
	return "name:" + strings.ToLower(strings.Join(strings.Fields(r.fullName()), " "))
}

// novelFetch routes a novel-command read through the shared data-source
// resolver so --data-source is honored end to end: local reads synced data,
// live forces the API, and auto falls back to synced data when the API is
// unreachable. Pass strategy "auto" for resources the sync command persists
// (seasons, categories, events, sessions, riders, broadcast-events,
// standings) so they gain that fallback; pass "live" for endpoints with no
// local copy (session classification, rider stats, live timing) so
// --data-source local is an explicit unsupported error instead of a silent
// live hit. This replaces the earlier command-entry guard, which rejected
// local outright even for commands whose data is fully synceable.
func novelFetch(ctx context.Context, c *client.Client, flags *rootFlags, strategy, resourceType string, isList bool, path string, params map[string]string) (json.RawMessage, error) {
	raw, _, err := resolveReadWithStrategy(ctx, c, flags, strategy, resourceType, isList, path, params, nil, os.Stderr)
	return raw, err
}

// ---- resolvers ----

// resolveSeason finds the season UUID for a given year.
func resolveSeason(ctx context.Context, c *client.Client, flags *rootFlags, year int) (mgpSeason, error) {
	raw, err := novelFetch(ctx, c, flags, "auto", "seasons", true, "/results/seasons", nil)
	if err != nil {
		return mgpSeason{}, err
	}
	var seasons []mgpSeason
	if err := json.Unmarshal(raw, &seasons); err != nil {
		return mgpSeason{}, fmt.Errorf("parsing seasons: %w", err)
	}
	for _, s := range seasons {
		if s.Year == year {
			return s, nil
		}
	}
	return mgpSeason{}, notFoundErr(fmt.Errorf("season %d not found (try a year between %d and the current season)", year, oldestYear(seasons)))
}

func oldestYear(seasons []mgpSeason) int {
	min := 0
	for _, s := range seasons {
		if min == 0 || s.Year < min {
			min = s.Year
		}
	}
	return min
}

// normClass maps a human class name to the canonical category name substring.
func normClass(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "™", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	switch s {
	case "", "motogp", "mgp", "premier":
		return "motogp"
	case "moto2", "m2":
		return "moto2"
	case "moto3", "m3":
		return "moto3"
	case "motoe", "e":
		return "motoe"
	}
	return s
}

// resolveCategory finds the category UUID for a class name within a season.
func resolveCategory(ctx context.Context, c *client.Client, flags *rootFlags, seasonUUID, className string) (mgpCategory, error) {
	raw, err := novelFetch(ctx, c, flags, "auto", "categories", true, "/results/categories", map[string]string{"seasonUuid": seasonUUID})
	if err != nil {
		return mgpCategory{}, err
	}
	var cats []mgpCategory
	if err := json.Unmarshal(raw, &cats); err != nil {
		return mgpCategory{}, fmt.Errorf("parsing categories: %w", err)
	}
	want := normClass(className)
	for _, cat := range cats {
		if normClass(cat.Name) == want {
			return cat, nil
		}
	}
	var have []string
	for _, cat := range cats {
		have = append(have, strings.ReplaceAll(cat.Name, "™", ""))
	}
	return mgpCategory{}, notFoundErr(fmt.Errorf("class %q not found for this season; available: %s", className, strings.Join(have, ", ")))
}

// resolveEvent finds an event by fuzzy match on name/short name/circuit/country.
func resolveEvent(ctx context.Context, c *client.Client, flags *rootFlags, seasonUUID, query string) (mgpEvent, error) {
	events, err := seasonEvents(ctx, c, flags, seasonUUID, false)
	if err != nil {
		return mgpEvent{}, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	// Prefer exact short-name match, then substring match across fields.
	for _, e := range events {
		if strings.ToLower(e.ShortName) == q {
			return e, nil
		}
	}
	for _, e := range events {
		hay := strings.ToLower(strings.Join([]string{e.Name, e.SponsoredName, e.ShortName, e.Circuit.Name, e.Country.Name}, " "))
		if strings.Contains(hay, q) {
			return e, nil
		}
	}
	return mgpEvent{}, notFoundErr(fmt.Errorf("event %q not found in that season", query))
}

// seasonEvents lists events for a season, optionally only finished ones.
func seasonEvents(ctx context.Context, c *client.Client, flags *rootFlags, seasonUUID string, finishedOnly bool) ([]mgpEvent, error) {
	params := map[string]string{"seasonUuid": seasonUUID}
	if finishedOnly {
		params["isFinished"] = "true"
	}
	raw, err := novelFetch(ctx, c, flags, "auto", "events", true, "/results/events", params)
	if err != nil {
		return nil, err
	}
	var all []mgpEvent
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, fmt.Errorf("parsing events: %w", err)
	}
	// Drop preseason/test events so round numbering and replay only cover GPs.
	events := make([]mgpEvent, 0, len(all))
	for _, e := range all {
		if e.Test {
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

// sessionTypeMatch maps a human session token to a canonical (type, number).
// number == 0 means "any number of that type".
func sessionTypeMatch(token string) (string, int) {
	t := strings.ToUpper(strings.TrimSpace(token))
	t = strings.ReplaceAll(t, " ", "")
	switch t {
	case "", "RACE", "RAC", "GP":
		return "RAC", 0
	case "SPRINT", "SPR":
		return "SPR", 0
	case "WARMUP", "WUP":
		return "WUP", 0
	case "PRACTICE", "PR":
		return "PR", 0
	case "Q", "QUALIFYING", "QUALI":
		return "Q", 0
	case "Q1":
		return "Q", 1
	case "Q2":
		return "Q", 2
	case "FP1":
		return "FP", 1
	case "FP2":
		return "FP", 2
	case "FP3":
		return "FP", 3
	case "FP":
		return "FP", 0
	}
	return t, 0
}

// resolveSession finds a session by human token (race, sprint, q, fp1...).
func resolveSession(ctx context.Context, c *client.Client, flags *rootFlags, eventUUID, catUUID, token string) (mgpSession, error) {
	sessions, err := listSessions(ctx, c, flags, eventUUID, catUUID)
	if err != nil {
		return mgpSession{}, err
	}
	wantType, wantNum := sessionTypeMatch(token)
	for _, s := range sessions {
		if strings.ToUpper(s.Type) == wantType && (wantNum == 0 || s.Number == wantNum) {
			return s, nil
		}
	}
	var have []string
	for _, s := range sessions {
		if s.Number > 0 {
			have = append(have, fmt.Sprintf("%s%d", s.Type, s.Number))
		} else {
			have = append(have, s.Type)
		}
	}
	return mgpSession{}, notFoundErr(fmt.Errorf("session %q not found; available: %s", token, strings.Join(have, ", ")))
}

func listSessions(ctx context.Context, c *client.Client, flags *rootFlags, eventUUID, catUUID string) ([]mgpSession, error) {
	raw, err := novelFetch(ctx, c, flags, "auto", "sessions", true, "/results/sessions", map[string]string{"eventUuid": eventUUID, "categoryUuid": catUUID})
	if err != nil {
		return nil, err
	}
	var sessions []mgpSession
	if err := json.Unmarshal(raw, &sessions); err != nil {
		return nil, fmt.Errorf("parsing sessions: %w", err)
	}
	return sessions, nil
}

// classificationRow is one finisher in a session or standings table.
type classificationRow struct {
	Position    int             `json:"position"`
	Points      int             `json:"points"`
	Rider       mgpRider        `json:"rider"`
	Team        mgpNamed        `json:"team"`
	Constructor mgpNamed        `json:"constructor"`
	Time        json.RawMessage `json:"time"`
	Gap         json.RawMessage `json:"gap"`
	Status      string          `json:"status"`
}

type classificationResp struct {
	Classification []classificationRow `json:"classification"`
}

// sessionClassification fetches the finishing order for a session. Session
// classifications are not part of the local sync set, so this is a live-only
// read ("live" strategy): --data-source local returns an explicit unsupported
// error rather than empty data.
func sessionClassification(ctx context.Context, c *client.Client, flags *rootFlags, sessionID string) ([]classificationRow, error) {
	raw, err := novelFetch(ctx, c, flags, "live", "classification", false, "/results/session/"+sessionID+"/classification", map[string]string{"test": "false"})
	if err != nil {
		return nil, err
	}
	var resp classificationResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parsing classification: %w", err)
	}
	return resp.Classification, nil
}

// resolveRider matches a rider by name against the current-season rider list.
func resolveRider(ctx context.Context, c *client.Client, flags *rootFlags, query string) (mgpRider, error) {
	raw, err := novelFetch(ctx, c, flags, "auto", "riders", true, "/riders", nil)
	if err != nil {
		return mgpRider{}, err
	}
	var riders []mgpRider
	if err := json.Unmarshal(raw, &riders); err != nil {
		return mgpRider{}, fmt.Errorf("parsing riders: %w", err)
	}
	q := strings.ToLower(strings.TrimSpace(query))
	// 1) Exact full-name match wins outright.
	for _, r := range riders {
		if strings.ToLower(r.fullName()) == q {
			return r, nil
		}
	}
	// 2) Exact surname match — but report ambiguity (e.g. the Marquez brothers).
	var surnameHits []mgpRider
	for _, r := range riders {
		if strings.ToLower(strings.TrimSpace(r.Surname)) == q {
			surnameHits = append(surnameHits, r)
		}
	}
	if len(surnameHits) == 1 {
		return surnameHits[0], nil
	}
	if len(surnameHits) > 1 {
		return mgpRider{}, ambiguousRiderErr(query, surnameHits)
	}
	// 3) Substring match across full name + nickname.
	var matches []mgpRider
	for _, r := range riders {
		hay := strings.ToLower(r.fullName() + " " + r.Nickname)
		if q != "" && strings.Contains(hay, q) {
			matches = append(matches, r)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return mgpRider{}, ambiguousRiderErr(query, matches)
	}
	return mgpRider{}, notFoundErr(fmt.Errorf("rider %q not found in the current-season roster (only current riders resolve by name)", query))
}

func ambiguousRiderErr(query string, riders []mgpRider) error {
	var names []string
	for _, r := range riders {
		names = append(names, r.fullName())
	}
	sort.Strings(names)
	return usageErr(fmt.Errorf("rider %q is ambiguous: %s (use the full name)", query, strings.Join(names, ", ")))
}

// riderStats fetches career statistics for a rider legacy ID as a generic map.
//
// A zero legacyID means the roster entry decoded without the numeric
// legacy_id this endpoint requires (a partial /riders response). Guard it
// here rather than issuing GET /riders/0/stats, which would return a
// misleading not-found or the wrong entity's statistics. Both callers
// (career, h2h) are protected by this single check.
func riderStats(ctx context.Context, c *client.Client, flags *rootFlags, legacyID int) (map[string]any, error) {
	if legacyID == 0 {
		return nil, notFoundErr(fmt.Errorf("rider has no legacy_id, so career stats cannot be fetched (the roster entry is missing the numeric identifier the stats endpoint requires)"))
	}
	// Rider stats are not synced locally -> live-only ("live" strategy).
	raw, err := novelFetch(ctx, c, flags, "live", "riders", false, fmt.Sprintf("/riders/%d/stats", legacyID), nil)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parsing rider stats: %w", err)
	}
	return m, nil
}
