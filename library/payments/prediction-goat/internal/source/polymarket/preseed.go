// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// preseed.go scans the locally-synced Polymarket events corpus for
// negRisk=true event families and emits a learn.PreseedRow for each
// child market whose `question` carries an identifiable entity. The
// entity is extracted from the canonical Polymarket question shape
// "Will the {entity} win the {topic}" (the "the " article is optional)
// and used to anchor the same three query-pattern variants the Kalshi
// scanner emits.
//
// Polymarket stores parent events with a nested `markets` array, so
// the scanner walks each event's children inline without a second
// round-trip; the local resources row carries everything we need.

package polymarket

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
// registered. Visible as a constant for diagnostics.
const PreseedResourceType = "polymarket_events"

// VenuePolymarket is the canonical venue tag written into preseed
// rows. Lowercase to match what the rest of the CLI emits in venue
// fields (compare, topic, etc.).
const VenuePolymarket = "polymarket"

// polymarketEntityRE captures the "entity" capture from canonical
// Polymarket question shapes. Three forms covered:
//
//   "Will the {entity} win ..."
//   "Will {entity} win ..."
//   "Will {entity} ..." (no "win"; fallback)
//
// Anchored case-insensitively. The first capture group is the entity
// — kept lazy so the regex doesn't swallow the rest of the question
// into the entity slot. Trailing words are then trimmed at "win" /
// "the" / sentence-end heuristically in extractPolymarketEntity.
var polymarketEntityRE = regexp.MustCompile(`(?i)^will\s+(?:the\s+)?([A-Za-z][A-Za-z0-9\.\-' ]+?)\s+(?:win|be|become)\b`)

func init() {
	learn.RegisterScanner(PreseedResourceType, ScanForPreseed)
}

// ScanForPreseed walks events with negRisk=true and emits PreseedRows
// for every child market whose `question` matches the entity-bearing
// shape. Exported so tests can call it directly without going through
// the registry.
func ScanForPreseed(ctx context.Context, db *sql.DB) ([]learn.PreseedRow, error) {
	if db == nil {
		return nil, fmt.Errorf("polymarket preseed: db is nil")
	}

	events, err := loadNegRiskEvents(ctx, db)
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
		for _, child := range ev.Children {
			rows := rowsForChildMarket(ev, child)
			out = append(out, rows...)
		}
	}
	return out, nil
}

// preseedEvent is the minimal parent-event projection. Children come
// from the nested "markets" array on the event row; we extract them
// inline to avoid a second SQL pass.
type preseedEvent struct {
	Slug     string
	Title    string
	Children []preseedChild
}

// preseedChild is the minimal child-market projection. Slug is the
// resource_id in the local resources table; question is what we
// pattern-match for the entity.
type preseedChild struct {
	Slug     string
	Question string
}

// loadNegRiskEvents pulls every event row whose negRisk JSON field is
// truthy. SQLite json_extract returns the integer 1 for JSON true; we
// also accept the literal string 'true' for the legacy shape some
// caches carry.
func loadNegRiskEvents(ctx context.Context, db *sql.DB) ([]preseedEvent, error) {
	const q = `
		SELECT id, data
		FROM resources
		WHERE resource_type = 'events'
		  AND (json_extract(data, '$.negRisk') = 1
		       OR json_extract(data, '$.negRisk') = 'true')
	`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("polymarket preseed events: %w", err)
	}
	defer rows.Close()

	events := make([]preseedEvent, 0)
	for rows.Next() {
		var id string
		var data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			return nil, fmt.Errorf("polymarket preseed events scan: %w", err)
		}
		if !data.Valid {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data.String), &obj); err != nil {
			continue
		}
		ev := preseedEvent{
			Slug:  asPreseedString(obj["slug"]),
			Title: asPreseedString(obj["title"]),
		}
		if ev.Slug == "" {
			ev.Slug = id
		}
		if ev.Title == "" {
			// No title means no topic phrase for the query-pattern
			// variants. Skip rather than emit "odds X wins ".
			continue
		}
		// Walk the nested markets array. Polymarket events carry
		// child markets inline; resource_type='markets' rows are a
		// secondary projection of the same data and may not be
		// synced, so the inline array is the authoritative source.
		marketsRaw, _ := obj["markets"].([]any)
		ev.Children = collectChildren(marketsRaw)
		events = append(events, ev)
	}
	return events, rows.Err()
}

// collectChildren extracts the (slug, question) pairs from a Polymarket
// event's nested markets array. Children with empty slug or empty
// question are filtered out (no resource to address, or no entity to
// extract).
func collectChildren(raw []any) []preseedChild {
	out := make([]preseedChild, 0, len(raw))
	for _, m := range raw {
		mObj, ok := m.(map[string]any)
		if !ok {
			continue
		}
		slug := asPreseedString(mObj["slug"])
		question := asPreseedString(mObj["question"])
		if slug == "" || question == "" {
			continue
		}
		out = append(out, preseedChild{Slug: slug, Question: question})
	}
	return out
}

// rowsForChildMarket emits the three query-pattern variants for one
// (event, child) pair. Returns an empty slice when the child's
// question doesn't yield a clean entity via the canonical regex.
func rowsForChildMarket(ev preseedEvent, child preseedChild) []learn.PreseedRow {
	entity := extractPolymarketEntity(child.Question)
	if entity == "" {
		return nil
	}

	// Three query-pattern variants per child. The third uses a
	// topic-words-only form derived from the event title (year
	// prefix and trailing classifier words stripped) for the
	// dominant agent phrasing. Always emitted even when the topic
	// transform is a no-op; learn.Run's dedup pass collapses any
	// rows that normalize to the same query_pattern.
	topicWords := topicWordsFromEventTitle(ev.Title)
	if topicWords == "" {
		topicWords = ev.Title
	}
	queries := []string{
		fmt.Sprintf("odds %s wins %s", entity, ev.Title),
		fmt.Sprintf("%s wins %s", entity, ev.Title),
		fmt.Sprintf("odds %s %s", entity, topicWords),
	}

	out := make([]learn.PreseedRow, 0, len(queries))
	for _, q := range queries {
		out = append(out, learn.PreseedRow{
			QueryPattern: q,
			ResourceID:   child.Slug,
			ResourceType: "markets",
			Venue:        VenuePolymarket,
			Entities:     []string{entity},
			Source:       learn.SourcePreseed,
		})
	}
	return out
}

// extractPolymarketEntity pulls the entity name out of a canonical
// Polymarket question. Returns "" when the question shape isn't
// recognized — the scanner skips those rather than emit a row keyed
// on an arbitrary substring.
func extractPolymarketEntity(question string) string {
	m := polymarketEntityRE.FindStringSubmatch(question)
	if len(m) < 2 {
		return ""
	}
	entity := strings.TrimSpace(m[1])
	// Trim trailing connective words that snuck into the capture
	// because the regex lazy-matched up to the next keyword. The
	// regex already anchors on "win/be/become" but defensive
	// trimming here makes the entity safe to slot into a query
	// pattern verbatim.
	entity = strings.TrimSuffix(entity, " the")
	return strings.TrimSpace(entity)
}

// topicWordsFromEventTitle returns the keyword core of an event title
// for the third query-pattern variant. Strips trailing classifier
// words (Winner, Champion, Outcome), leading 4-digit year prefixes,
// and leading possessive qualifiers (Men's, Women's). See the
// matching helper in source/kalshi/preseed.go for the wider rationale;
// this is a deliberate copy so the two source packages stay
// independent.
func topicWordsFromEventTitle(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return ""
	}
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
	fields := strings.Fields(t)
	if len(fields) > 1 && isFourDigitYear(fields[0]) {
		t = strings.Join(fields[1:], " ")
		fields = strings.Fields(t)
	}
	for len(fields) > 1 && isPossessiveQualifier(fields[0]) {
		fields = fields[1:]
	}
	return strings.TrimSpace(strings.Join(fields, " "))
}

func isPossessiveQualifier(tok string) bool {
	switch strings.ToLower(strings.TrimSpace(tok)) {
	case "men's", "women's", "boys'", "girls'", "mens", "womens":
		return true
	}
	return false
}

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
// Local copy of the same helper in source/kalshi/preseed.go — keeps
// the two source packages independent at the cost of duplication.
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
