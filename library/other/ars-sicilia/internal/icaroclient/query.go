package icaroclient

import (
	"fmt"
	"sort"
	"strings"
)

// BuildQuery turns a friendly param map (CLI flag values) into the ISIS query
// expression the Icaro engine accepts. The shape mirrors what the JSP form
// produces server-side after a POST: `(<v>.<FIELD> E <v>.<FIELD>) E (<free>)`.
//
// Empty params produce the universal selector `all`, matching every record
// in the archive. Unknown flag names (not in arc.FieldMap) are passed through
// as free-text search terms.
//
// When isisRaw is non-empty it overrides everything: the caller has its own
// fully-formed expression and we ship it verbatim.
func BuildQuery(arc Archive, params map[string]string, isisRaw string) string {
	if isisRaw = strings.TrimSpace(isisRaw); isisRaw != "" {
		return isisRaw
	}
	var fielded []string
	var freeText []string

	// Stable key order so identical inputs produce identical queries (helps
	// session caching, dogfood reproducibility).
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := strings.TrimSpace(params[k])
		if v == "" {
			continue
		}
		if k == "testo" || k == "free" || k == "terms" || k == "q" {
			freeText = append(freeText, v)
			continue
		}
		field, ok := arc.FieldMap[k]
		if !ok {
			// Unmapped flag: drop into free-text as fallback.
			freeText = append(freeText, v)
			continue
		}
		fielded = append(fielded, fmt.Sprintf("%s.%s", quoteValue(v), field))
	}

	var parts []string
	if len(fielded) > 0 {
		parts = append(parts, "("+strings.Join(fielded, " E ")+")")
	}
	if len(freeText) > 0 {
		// Free text uses AND ("E") between terms unless caller already wrote
		// boolean operators (we don't try to detect this — keep it simple).
		parts = append(parts, "("+strings.Join(freeText, " E ")+")")
	}
	if len(parts) == 0 {
		return "all"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts, " E ")
}

// quoteValue returns the value as-is for purely alphanumeric/whitespace
// content, and parenthesizes/quotes anything that looks structurally complex
// to keep the ISIS parser happy. The portal's own JSP form just emits the
// value verbatim for typical inputs, so we don't escape over-aggressively.
func quoteValue(v string) string {
	if needsQuoting(v) {
		return "(" + v + ")"
	}
	return v
}

func needsQuoting(v string) bool {
	for _, r := range v {
		if r == ' ' || r == '\t' || r == '(' || r == ')' || r == '.' {
			return true
		}
	}
	return false
}
