// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

// HubSpot-style --filter grammar for novel-feature query commands.
//
// The grammar mirrors HubSpot's own Agent CLI bulk-operations filter syntax
// verbatim so an operator who knows their tool already knows ours:
//
//   bare 'field'      -> HAS            (field present and non-empty)
//   '!field'          -> NOT_HAS        (field absent or empty)
//   'field=value'     -> EQ             (case-sensitive string equality)
//   'field~token'     -> CONTAINS_TOKEN (case-insensitive substring)
//
//   AND -> space-separated clauses within a single --filter flag
//   OR  -> repeat --filter
//
// One ParseFilters call ingests cobra's StringSlice (one entry per --filter)
// and returns a FilterExpr. The expression supports two evaluation modes:
//
//   1. Match(row map[string]string) — for in-memory filtering of result rows.
//   2. SQLFragment(prefix)          — for store-backed queries; emits a WHERE
//      fragment with `?` placeholders so values can never be concatenated
//      directly into SQL.
//
// The split keeps the grammar one canonical thing while letting each caller
// pick whichever path is simpler — `nurture queue` runs an existing
// multi-join SQL and post-filters; `deals top` injects into its existing
// SELECT and lets SQLite do the work.

package cliutil

import (
	"fmt"
	"sort"
	"strings"
)

// FilterOp enumerates the four supported per-clause operators.
type FilterOp int

const (
	OpHas FilterOp = iota
	OpNotHas
	OpEq
	OpContainsToken
)

// String renders the op for debug output. Mirrors the grammar tokens so an
// agent reading the AST can reconstruct the source filter mentally.
func (o FilterOp) String() string {
	switch o {
	case OpHas:
		return "HAS"
	case OpNotHas:
		return "NOT_HAS"
	case OpEq:
		return "EQ"
	case OpContainsToken:
		return "CONTAINS_TOKEN"
	default:
		return fmt.Sprintf("FilterOp(%d)", int(o))
	}
}

// FilterClause is one term in the expression: a field check.
//
// Value is empty for HAS and NOT_HAS. Field is always non-empty after
// ParseFilters returns (the parser rejects empty-field clauses).
type FilterClause struct {
	Field string
	Op    FilterOp
	Value string
}

// FilterGroup is a single --filter's worth of clauses, all AND-joined.
type FilterGroup struct {
	Clauses []FilterClause
}

// FilterExpr is the whole expression: groups OR-joined.
//
// Zero-group expressions match every row. ParseFilters returns a zero-group
// expression for empty input so callers don't have to special-case "no
// filter" — `Match` and `SQLFragment` both handle it.
type FilterExpr struct {
	Groups []FilterGroup
}

// IsEmpty reports whether the expression has any clauses. Used by callers to
// short-circuit SQL injection when there's nothing to inject.
func (e FilterExpr) IsEmpty() bool {
	for _, g := range e.Groups {
		if len(g.Clauses) > 0 {
			return false
		}
	}
	return true
}

// ParseFilters takes one or more --filter flag values (the slice cobra hands
// us from StringSliceVar) and returns the parsed expression.
//
// Empty input yields a zero-group FilterExpr (matches everything). An
// individual flag that's all whitespace is rejected — passing `--filter ”`
// is almost certainly a quoting mistake and the parser surfaces it rather
// than silently treating it as no-filter.
//
// Grammar precedence inside a clause: the parser checks for `=` and `~` in
// that order. A `~` appearing inside a value after `=` is therefore part of
// the EQ value, not a new operator (matches HubSpot's docs: the first
// operator token wins). Leading `!` is only NOT_HAS when no `=` or `~`
// appears; `!field=value` is a parse error (operators can't combine).
func ParseFilters(rawFlags []string) (FilterExpr, error) {
	var expr FilterExpr
	if len(rawFlags) == 0 {
		return expr, nil
	}
	for i, raw := range rawFlags {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return FilterExpr{}, fmt.Errorf("--filter[%d]: empty filter (omit the flag instead of passing an empty value)", i)
		}
		group, err := parseFilterGroup(trimmed)
		if err != nil {
			return FilterExpr{}, fmt.Errorf("--filter[%d]: %w", i, err)
		}
		expr.Groups = append(expr.Groups, group)
	}
	return expr, nil
}

// parseFilterGroup splits a single flag value on whitespace into clauses,
// then parses each clause individually. Whitespace inside an EQ value is
// not supported (values can't contain spaces in the HubSpot grammar); use
// CONTAINS_TOKEN for partial matches that span words.
func parseFilterGroup(s string) (FilterGroup, error) {
	var group FilterGroup
	tokens := strings.Fields(s)
	for _, tok := range tokens {
		clause, err := parseClause(tok)
		if err != nil {
			return FilterGroup{}, fmt.Errorf("token %q: %w", tok, err)
		}
		group.Clauses = append(group.Clauses, clause)
	}
	if len(group.Clauses) == 0 {
		return FilterGroup{}, fmt.Errorf("filter group has no clauses")
	}
	return group, nil
}

// parseClause turns one whitespace-delimited token into a FilterClause.
//
// Operator search order: `=` then `~`. NOT_HAS is detected only when neither
// appears (so a literal `!` doesn't get mis-parsed as the start of a value).
func parseClause(tok string) (FilterClause, error) {
	if tok == "" {
		return FilterClause{}, fmt.Errorf("empty clause")
	}
	// EQ wins over CONTAINS_TOKEN if both appear — first byte rule, HubSpot-
	// compatible. The `!` prefix is illegal for EQ/CONTAINS_TOKEN: HubSpot's
	// grammar reserves `!` for NOT_HAS, not negated equality.
	if i := strings.IndexByte(tok, '='); i >= 0 {
		if strings.HasPrefix(tok, "!") {
			return FilterClause{}, fmt.Errorf("'!' prefix is only valid for NOT_HAS (bare field), not for EQ/CONTAINS_TOKEN")
		}
		field := tok[:i]
		value := tok[i+1:]
		if field == "" {
			return FilterClause{}, fmt.Errorf("EQ clause missing field name before '='")
		}
		return FilterClause{Field: field, Op: OpEq, Value: value}, nil
	}
	if i := strings.IndexByte(tok, '~'); i >= 0 {
		if strings.HasPrefix(tok, "!") {
			return FilterClause{}, fmt.Errorf("'!' prefix is only valid for NOT_HAS (bare field), not for EQ/CONTAINS_TOKEN")
		}
		field := tok[:i]
		value := tok[i+1:]
		if field == "" {
			return FilterClause{}, fmt.Errorf("CONTAINS_TOKEN clause missing field name before '~'")
		}
		return FilterClause{Field: field, Op: OpContainsToken, Value: value}, nil
	}
	if strings.HasPrefix(tok, "!") {
		field := tok[1:]
		if field == "" {
			return FilterClause{}, fmt.Errorf("NOT_HAS clause missing field name after '!'")
		}
		return FilterClause{Field: field, Op: OpNotHas}, nil
	}
	return FilterClause{Field: tok, Op: OpHas}, nil
}

// Match reports whether a row (map of field name -> value, typically read
// from json_extract output) satisfies the expression.
//
// Empty expressions match every row — callers can apply Match unconditionally
// and the no-filter case is the no-op.
//
// CONTAINS_TOKEN is case-insensitive on both sides; EQ is case-sensitive.
// HubSpot's docs are explicit about both: field-name match is case-sensitive
// (the map key compare here), and CONTAINS_TOKEN folds case.
func (e FilterExpr) Match(row map[string]string) bool {
	if e.IsEmpty() {
		return true
	}
	for _, g := range e.Groups {
		if matchGroup(g, row) {
			return true
		}
	}
	return false
}

func matchGroup(g FilterGroup, row map[string]string) bool {
	for _, c := range g.Clauses {
		if !matchClause(c, row) {
			return false
		}
	}
	return true
}

func matchClause(c FilterClause, row map[string]string) bool {
	v, present := row[c.Field]
	switch c.Op {
	case OpHas:
		return present && v != ""
	case OpNotHas:
		return !present || v == ""
	case OpEq:
		return present && v == c.Value
	case OpContainsToken:
		if !present {
			return false
		}
		return strings.Contains(strings.ToLower(v), strings.ToLower(c.Value))
	default:
		return false
	}
}

// SQLFragment renders the expression as a WHERE fragment + args slice
// suitable for json_extract() against a JSON column.
//
// jsonPathPrefix is the column name to extract from (typically "data") so the
// emitted SQL reads `json_extract(data, '$.properties.<field>')`. The
// `properties.` segment is hard-coded because every HubSpot object table
// nests its mutable fields under `properties` — the JSON path is canonical,
// not parameterized.
//
// All values pass through `?` placeholders. Field names are inlined into the
// JSON path because SQLite doesn't support parameterized json_extract paths,
// but the field name comes from the parser which has already validated it
// against the operator grammar (no whitespace, no quote characters). A
// defensive guard rejects any field containing a single-quote anyway —
// belt-and-suspenders against an upstream parser bug.
//
// Returns ("", nil) on an empty expression so the caller can skip the WHERE
// append entirely.
func (e FilterExpr) SQLFragment(jsonPathPrefix string) (string, []interface{}) {
	if e.IsEmpty() {
		return "", nil
	}
	if jsonPathPrefix == "" {
		jsonPathPrefix = "data"
	}
	var args []interface{}
	groupSQL := make([]string, 0, len(e.Groups))
	for _, g := range e.Groups {
		if len(g.Clauses) == 0 {
			continue
		}
		clauseSQL := make([]string, 0, len(g.Clauses))
		for _, c := range g.Clauses {
			frag, vals := clauseSQL2(jsonPathPrefix, c)
			clauseSQL = append(clauseSQL, frag)
			args = append(args, vals...)
		}
		groupSQL = append(groupSQL, "("+strings.Join(clauseSQL, " AND ")+")")
	}
	if len(groupSQL) == 0 {
		return "", nil
	}
	if len(groupSQL) == 1 {
		return groupSQL[0], args
	}
	return "(" + strings.Join(groupSQL, " OR ") + ")", args
}

// clauseSQL2 emits one clause's SQL fragment + args. Field name is inlined
// into the JSON path; values always go through `?`.
func clauseSQL2(prefix string, c FilterClause) (string, []interface{}) {
	field := sanitizeFieldForPath(c.Field)
	path := fmt.Sprintf("json_extract(%s, '$.properties.%s')", prefix, field)
	switch c.Op {
	case OpHas:
		return fmt.Sprintf("(%s IS NOT NULL AND %s != '')", path, path), nil
	case OpNotHas:
		return fmt.Sprintf("(%s IS NULL OR %s = '')", path, path), nil
	case OpEq:
		return fmt.Sprintf("%s = ?", path), []interface{}{c.Value}
	case OpContainsToken:
		return fmt.Sprintf("LOWER(%s) LIKE ?", path), []interface{}{"%" + strings.ToLower(c.Value) + "%"}
	default:
		// Unreachable in practice (parser would have rejected); emit a no-op
		// so a regression doesn't silently match everything.
		return "0 = 1", nil
	}
}

// sanitizeFieldForPath strips characters that could escape the inline JSON
// path. The parser already rejects whitespace and the operator characters
// (`=`, `~`, `!` mid-token); this is a final guard against quote-injection
// even if a future parser change widens accepted characters.
func sanitizeFieldForPath(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\'', '"', '\\', 0:
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// DebugString renders the expression in a human-readable form, suitable for
// `--debug` printing to stderr. Format matches the brief in the plan:
//
//	filter expr (N groups, OR):
//	  group 1 (AND): a = "x", !b
//	  group 2 (AND): c ~ "y"
//
// Returns "(empty)" for a no-filter expression so a `--debug` invocation
// without a `--filter` doesn't print a misleading "0 groups" line.
func (e FilterExpr) DebugString() string {
	if e.IsEmpty() {
		return "filter expr: (empty — matches all)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "filter expr (%d groups, OR):\n", len(e.Groups))
	for gi, g := range e.Groups {
		fmt.Fprintf(&b, "  group %d (AND): ", gi+1)
		parts := make([]string, 0, len(g.Clauses))
		for _, c := range g.Clauses {
			parts = append(parts, debugClause(c))
		}
		// Sort within a group is intentionally NOT applied — preserve source
		// order so the operator can map debug output back to their flag.
		b.WriteString(strings.Join(parts, ", "))
		b.WriteByte('\n')
	}
	return b.String()
}

func debugClause(c FilterClause) string {
	switch c.Op {
	case OpHas:
		return c.Field
	case OpNotHas:
		return "!" + c.Field
	case OpEq:
		return fmt.Sprintf("%s = %q", c.Field, c.Value)
	case OpContainsToken:
		return fmt.Sprintf("%s ~ %q", c.Field, c.Value)
	default:
		return fmt.Sprintf("?%s", c.Field)
	}
}

// FieldsReferenced returns the unique field names referenced by the
// expression, sorted for stable output. Used by callers that need to project
// the right subset of columns out of json_extract before calling Match.
func (e FilterExpr) FieldsReferenced() []string {
	seen := map[string]struct{}{}
	for _, g := range e.Groups {
		for _, c := range g.Clauses {
			seen[c.Field] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
