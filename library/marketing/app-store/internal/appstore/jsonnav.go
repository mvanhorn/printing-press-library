// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"encoding/json"
	"html"
	"strconv"
	"strings"
	"time"
)

// Small helpers for walking decoded JSON (map[string]any / []any). Apple's
// page models are deep and polymorphic ($kind-tagged), so the parsers read
// them through these lenient accessors instead of rigid structs: a missing or
// differently-typed field yields the zero value rather than failing the page.

type jsonObject = map[string]any

func asObject(v any) jsonObject {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func asArray(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return nil
}

// at walks a path of object keys and decimal array indexes.
func at(v any, path ...string) any {
	cur := v
	for _, key := range path {
		switch node := cur.(type) {
		case map[string]any:
			cur = node[key]
		case []any:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil
			}
			cur = node[idx]
		default:
			return nil
		}
	}
	return cur
}

func objAt(v any, path ...string) jsonObject { return asObject(at(v, path...)) }

func arrAt(v any, path ...string) []any { return asArray(at(v, path...)) }

// strOf renders strings as-is and numbers without a trailing ".0".
func strOf(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		return x.String()
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}

func strAt(v any, path ...string) string { return strOf(at(v, path...)) }

func floatOf(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	}
	return 0
}

func intOf(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return n
		}
		f, _ := x.Float64()
		return int64(f)
	case string:
		s := strings.TrimSpace(x)
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n
		}
		f, _ := strconv.ParseFloat(s, 64)
		return int64(f)
	}
	return 0
}

func boolOf(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		b, _ := strconv.ParseBool(x)
		return b
	case float64:
		return x != 0
	}
	return false
}

func kindOf(v any) string { return strAt(v, "$kind") }

// cleanText trims, decodes HTML entities, turns <br> tags and non-breaking
// spaces into plain spaces and collapses runs of whitespace (used for
// editorial titles such as "GAME<br>OF THE<br>DAY").
func cleanText(s string) string {
	if s == "" {
		return ""
	}
	s = html.UnescapeString(s)
	for _, br := range []string{"<br>", "<br/>", "<br />", "<BR>"} {
		s = strings.ReplaceAll(s, br, " ")
	}
	s = strings.ReplaceAll(s, " ", " ")
	s = strings.ReplaceAll(s, "‑", "-")
	return strings.Join(strings.Fields(s), " ")
}

// cleanMultiline keeps line breaks (release notes, descriptions) but trims
// each line's trailing space, decodes entities and normalizes NBSP.
func cleanMultiline(s string) string {
	if s == "" {
		return ""
	}
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, " ", " ")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// stripMarkup removes the light NQML/markdown markup Apple uses in
// LinkableText (<b>…</b>, **bold**) for plain-text output.
func stripMarkup(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>' && inTag:
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return html.UnescapeString(b.String())
}

// FormatTime renders a time as RFC3339 in UTC with second precision (the
// store's timestamp format; it sorts lexically).
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

// parseAppleTime parses the date formats Apple's surfaces use and returns UTC:
// RFC3339 with or without fractional seconds ("2026-10-12T07:00:00.000Z",
// "2026-10-21T20:55:47-07:00"), the JavaScript Date.toString form the product
// page uses for version history ("Mon Oct 19 2026 13:57:56 GMT+0000
// (Coordinated Universal Time)") and plain dates ("2026-10-19").
func parseAppleTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if idx := strings.Index(s, " ("); idx > 0 {
		s = s[:idx]
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"Mon Jan 02 2006 15:04:05 GMT-0700",
		"Mon Jan 2 2006 15:04:05 GMT-0700",
		"2006-01-02T15:04:05Z0700",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"Jan 2, 2006",
		"January 2, 2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// normalizeDate returns RFC3339 UTC for a parseable Apple date and the input
// unchanged otherwise (so nothing is silently dropped).
func normalizeDate(s string) string {
	if t, ok := parseAppleTime(s); ok {
		return FormatTime(t)
	}
	return strings.TrimSpace(s)
}
