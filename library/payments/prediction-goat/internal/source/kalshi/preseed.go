// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// preseed.go scans the local Kalshi corpus for multi-outcome event
// families (mutually_exclusive=true) and emits a learn.PreseedRow for
// every (parent.title, child.yes_sub_title) pair. Three query-pattern
// variants per child cover the dominant agent phrasings:
//
//   "odds {sub_title} wins {event.title}"
//   "{sub_title} wins {event.title}"
//   "odds {sub_title} {event.title}"
//
// The driver in internal/learn/preseed.go iterates registered scanners
// and writes each row through store.UpsertLearning. We register the
// Kalshi scanner via package init so any binary that imports this
// package picks it up automatically; the sync CLI is the only path
// that calls preseed.Run, so init-time registration is a no-op for
// non-sync codepaths.

package kalshi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
)

// PreseedResourceType is the registry key under which this scanner is
// registered. Visible as a constant for diagnostics and tests; the
// driver iterates the value set so the key value itself doesn't have
// to match the resource_type of the rows it emits.
const PreseedResourceType = "kalshi_markets"

// VenueKalshi is the canonical venue string written into preseed rows.
const VenueKalshi = "kalshi"

// kalshiMultiLegTitlePreseedRE matches Kalshi market titles that are
// comma-concatenated YES/NO outcome legs (e.g.
// "YES the question, NO the question"). Children whose title has this
// shape have no clean entity to key on (the YES/NO leg is the entity
// signal, not the title), so the scanner skips them. Mirrors the
// kalshiMultiLegTitleRE pattern in internal/cli/trending.go; kept as a
// local copy here to avoid pulling the cli package into source/.
var kalshiMultiLegTitlePreseedRE = regexp.MustCompile(`^(?i)(yes|no)\s.+,(yes|no)\s`)

func init() {
	learn.RegisterScanner(PreseedResourceType, ScanForPreseed)
}

// ScanForPreseed walks Kalshi events with mutually_exclusive=true,
// finds each event's child markets in the resources table, and emits
// a learn.PreseedRow for every (event, child, query-pattern variant)
// triple. Exported so tests can call it directly without going through
// the registry.
//
// The query against resources uses json_extract on the JSON-stored
// data column. mutually_exclusive arrives from the Kalshi API as a
// JSON bool, which json_extract returns as the integer 1; for
// defense-in-depth we also accept the string 'true' in case future
// upstream payloads switch shape.
func ScanForPreseed(ctx context.Context, db *sql.DB) ([]learn.PreseedRow, error) {
	if db == nil {
		return nil, fmt.Errorf("kalshi preseed: db is nil")
	}

	events, err := loadMutuallyExclusiveEvents(ctx, db)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}

	out := make([]learn.PreseedRow, 0, len(events)*8)
	for _, ev := range events {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		children, err := loadEventChildren(ctx, db, ev.EventTicker)
		if err != nil {
			return out, err
		}
		for _, child := range children {
			rows := rowsForChild(ev, child)
			out = append(out, rows...)
		}
	}
	return out, nil
}

// preseedEvent is the minimal event projection the scanner uses. Kept
// as a private type so the external `Event` struct in types.go isn't
// pulled into preseed concerns.
type preseedEvent struct {
	EventTicker string
	Title       string
}

// preseedChild is the minimal child-market projection. yes_sub_title
// is the entity signal (e.g., "Portugal"); title may be a multi-leg
// CSV that we want to skip even when yes_sub_title is set, because
// CSV-shaped titles signal the market is non-team-entity-shaped.
type preseedChild struct {
	Ticker      string
	Title       string
	YesSubTitle string
}

// loadMutuallyExclusiveEvents pulls every kalshi_events row whose
// json_extract($.mutually_exclusive) clears the truthy check. SQLite
// json_extract returns 1 for JSON true and the literal text for JSON
// strings; we accept both shapes via an IN clause.
func loadMutuallyExclusiveEvents(ctx context.Context, db *sql.DB) ([]preseedEvent, error) {
	const q = `
		SELECT id, data
		FROM resources
		WHERE resource_type = 'kalshi_events'
		  AND (json_extract(data, '$.mutually_exclusive') = 1
		       OR json_extract(data, '$.mutually_exclusive') = 'true')
	`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("kalshi preseed events: %w", err)
	}
	defer rows.Close()

	events := make([]preseedEvent, 0)
	for rows.Next() {
		var id string
		var data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			return nil, fmt.Errorf("kalshi preseed events scan: %w", err)
		}
		if !data.Valid {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data.String), &obj); err != nil {
			continue
		}
		title := asPreseedString(obj["title"])
		ticker := asPreseedString(obj["event_ticker"])
		if ticker == "" {
			ticker = id
		}
		if title == "" || ticker == "" {
			continue
		}
		events = append(events, preseedEvent{EventTicker: ticker, Title: title})
	}
	return events, rows.Err()
}

// loadEventChildren pulls every kalshi_markets row whose event_ticker
// matches the supplied parent. We project to (ticker, title,
// yes_sub_title) which is all the row builder needs.
func loadEventChildren(ctx context.Context, db *sql.DB, eventTicker string) ([]preseedChild, error) {
	const q = `
		SELECT id, data
		FROM resources
		WHERE resource_type = 'kalshi_markets'
		  AND json_extract(data, '$.event_ticker') = ?
	`
	rows, err := db.QueryContext(ctx, q, eventTicker)
	if err != nil {
		return nil, fmt.Errorf("kalshi preseed children for %s: %w", eventTicker, err)
	}
	defer rows.Close()

	children := make([]preseedChild, 0)
	for rows.Next() {
		var id string
		var data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			return nil, fmt.Errorf("kalshi preseed children scan: %w", err)
		}
		if !data.Valid {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data.String), &obj); err != nil {
			continue
		}
		ticker := asPreseedString(obj["ticker"])
		if ticker == "" {
			ticker = id
		}
		children = append(children, preseedChild{
			Ticker:      ticker,
			Title:       asPreseedString(obj["title"]),
			YesSubTitle: asPreseedString(obj["yes_sub_title"]),
		})
	}
	return children, rows.Err()
}

// rowsForChild emits the three query-pattern variants for one
// (event, child) pair. Returns an empty slice when the child has no
// usable entity (empty yes_sub_title or CSV-shaped title).
func rowsForChild(ev preseedEvent, child preseedChild) []learn.PreseedRow {
	sub := strings.TrimSpace(child.YesSubTitle)
	if sub == "" {
		return nil
	}
	if kalshiMultiLegTitlePreseedRE.MatchString(strings.TrimSpace(child.Title)) {
		return nil
	}
	if child.Ticker == "" {
		return nil
	}

	// Three query-pattern variants per child. The third variant uses
	// a topic-words-only form derived from the event title (year
	// prefix and trailing classifier words stripped) which is the
	// dominant agent phrasing for the family question — "odds
	// Portugal World Cup" instead of the full "2026 Men's World Cup
	// Winner" tail. Variant 3 is always emitted even when the topic
	// transform is a no-op; the dedup pass in learn.Run collapses any
	// rows that normalize to the same query_pattern.
	topicWords := topicWordsFromEventTitle(ev.Title)
	if topicWords == "" {
		topicWords = ev.Title
	}
	queries := []string{
		fmt.Sprintf("odds %s wins %s", sub, ev.Title),
		fmt.Sprintf("%s wins %s", sub, ev.Title),
		fmt.Sprintf("odds %s %s", sub, topicWords),
	}

	out := make([]learn.PreseedRow, 0, len(queries))
	for _, q := range queries {
		out = append(out, learn.PreseedRow{
			QueryPattern: q,
			ResourceID:   child.Ticker,
			ResourceType: "kalshi_markets",
			Venue:        VenueKalshi,
			Entities:     []string{sub},
			Source:       learn.SourcePreseed,
		})
	}
	return out
}

// topicWordsFromEventTitle returns a short topic phrase derived from
// the event title. Designed to produce the keyword core a user would
// actually type: "Portugal World Cup" rather than "Portugal 2026
// Men's World Cup Winner". Strips, in order:
//
//   1. Trailing classifier words (Winner, Champion, Outcome).
//   2. Leading 4-digit year (2026, 2024 ...).
//   3. Leading possessive qualifiers ("Men's", "Women's") that gender-
//      tag a tournament without being part of the topic phrase users
//      actually search for.
//
// The transforms are conservative — a title that doesn't carry any of
// these markers passes through unchanged. The intent is the narrowest
// stable substring that the entity-aware recall path will Jaccard-
// match against a typical user query.
func topicWordsFromEventTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return ""
	}
	// Drop trailing classifier words. Looped because a title may
	// carry "Outcome Winner" stacked at the tail.
	classifierSuffixes := []string{" Winner", " winner", " Champion", " champion", " Outcome", " outcome"}
	for changed := true; changed; {
		changed = false
		for _, suffix := range classifierSuffixes {
			if strings.HasSuffix(t, suffix) {
				t = strings.TrimSuffix(t, suffix)
				changed = true
			}
		}
		t = strings.TrimSpace(t)
	}
	// Drop leading 4-digit year.
	fields := strings.Fields(t)
	if len(fields) > 1 && isFourDigitYear(fields[0]) {
		t = strings.Join(fields[1:], " ")
		fields = strings.Fields(t)
	}
	// Drop leading possessive qualifier tokens. "Men's" / "Women's"
	// tag the tournament gender but don't appear in typical user
	// queries about the event ("USA wins world cup" not "USA wins
	// men's world cup").
	for len(fields) > 1 && isPossessiveQualifier(fields[0]) {
		fields = fields[1:]
	}
	return strings.TrimSpace(strings.Join(fields, " "))
}

// isPossessiveQualifier reports whether a token is a gendered or
// otherwise qualifying possessive that prefixes tournament titles
// without being part of the topic phrase ("Men's", "Women's", "Boys'",
// "Girls'"). Conservative: only the exact possessive shape.
func isPossessiveQualifier(tok string) bool {
	switch strings.ToLower(strings.TrimSpace(tok)) {
	case "men's", "women's", "boys'", "girls'", "mens", "womens":
		return true
	}
	return false
}

// isFourDigitYear reports whether s is a 4-character all-digit token
// in the rough range we'd expect for a year prefix on an event title.
// Used by topicWordsFromEventTitle to strip leading-year prefixes.
func isFourDigitYear(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// asPreseedString coerces a JSON-decoded any into a trimmed string.
// Mirrors jsonString in internal/cli/topic.go but lives here so the
// source/kalshi package doesn't pull in the cli package.
func asPreseedString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	case json.Number:
		return strings.TrimSpace(t.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}
