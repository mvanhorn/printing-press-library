// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package nepraxwalk

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/cliutil"
)

//go:embed crosswalk.json
var crosswalkFS embed.FS

// table is the parsed crosswalk.json. Everything else in this package is a
// view over it; nothing is inferred at runtime that is not derivable from it.
type table struct {
	SchemaVersion     int                 `json:"schema_version"`
	ObservedFYs       []string            `json:"observed_fys"`
	LatestObservedFY  string              `json:"latest_observed_fy"`
	SNoIsNotAKey      string              `json:"sno_is_not_a_key"`
	HeldOutFYs        []string            `json:"held_out_fys"`
	Rows              []Row               `json:"rows"`
	AmbiguousTokens   []AmbiguousToken    `json:"ambiguous_tokens"`
	AbsentParents     []AbsentParent      `json:"absent_parents"`
	PossibleDuplicate []PossibleDuplicate `json:"possible_duplicates"`
}

type index struct {
	tbl *table
	// byKey maps a fold key (of a canonical name or an alias) to a row index.
	byKey map[string]int
	// collidingKeys are fold keys that reached two different rows. A collision
	// means crosswalk.json is malformed; such keys are refused, not guessed.
	collidingKeys map[string]bool
	// byStripped maps the trailing-parenthetical-stripped fold key to the set
	// of row indexes it reaches. More than one means refuse.
	byStripped map[string][]int
	// acronyms maps an uppercased NEPRA parenthetical acronym to row indexes.
	acronyms map[string][]int
	// byTicker maps an uppercased PSX symbol to row indexes.
	byTicker map[string][]int
	// ambiguous maps an uppercased token to its curated refusal.
	ambiguous map[string]AmbiguousToken
	// absent maps an uppercased PSX symbol to its absence record.
	absent  map[string]AbsentParent
	loadErr error
}

var (
	loadOnce sync.Once
	idx      *index
)

func load() *index {
	loadOnce.Do(func() {
		idx = buildIndex()
	})
	return idx
}

func buildIndex() *index {
	ix := &index{
		byKey:         map[string]int{},
		collidingKeys: map[string]bool{},
		byStripped:    map[string][]int{},
		acronyms:      map[string][]int{},
		byTicker:      map[string][]int{},
		ambiguous:     map[string]AmbiguousToken{},
		absent:        map[string]AbsentParent{},
	}
	raw, err := crosswalkFS.ReadFile("crosswalk.json")
	if err != nil {
		ix.loadErr = fmt.Errorf("nepraxwalk: reading embedded crosswalk.json: %w", err)
		return ix
	}
	// crosswalk.json carries underscore-prefixed prose fields for human
	// reviewers (_what, _provenance, _field_semantics), so unknown-field
	// rejection is deliberately not enabled here.
	var tbl table
	if err := json.Unmarshal(raw, &tbl); err != nil {
		ix.loadErr = fmt.Errorf("nepraxwalk: parsing embedded crosswalk.json: %w", err)
		return ix
	}
	ix.tbl = &tbl

	for i := range tbl.Rows {
		r := tbl.Rows[i]
		names := append([]string{r.CanonicalName}, r.Aliases...)
		for _, n := range names {
			k := foldKey(n)
			if k == "" {
				continue
			}
			if prev, ok := ix.byKey[k]; ok && prev != i {
				ix.collidingKeys[k] = true
				continue
			}
			ix.byKey[k] = i
			if sk := foldKey(stripTrailingParen(Normalize(n))); len(sk) >= minStrippedKeyLen {
				ix.byStripped[sk] = appendUnique(ix.byStripped[sk], i)
			}
			if a := trailingAcronym(Normalize(n)); a != "" {
				ix.acronyms[a] = appendUnique(ix.acronyms[a], i)
			}
		}
		if r.PSXTicker != "" {
			t := strings.ToUpper(strings.TrimSpace(r.PSXTicker))
			ix.byTicker[t] = appendUnique(ix.byTicker[t], i)
		}
	}
	for _, a := range tbl.AmbiguousTokens {
		ix.ambiguous[strings.ToUpper(strings.TrimSpace(a.Token))] = a
	}
	for _, a := range tbl.AbsentParents {
		ix.absent[strings.ToUpper(strings.TrimSpace(a.PSXTicker))] = a
	}
	return ix
}

func appendUnique(dst []int, v int) []int {
	for _, x := range dst {
		if x == v {
			return dst
		}
	}
	return append(dst, v)
}

// minStrippedKeyLen keeps the trailing-parenthetical fallback from firing on a
// stub. "(NPPCL)" alone must not resolve to anything.
const minStrippedKeyLen = 6

// Normalize collapses a published NEPRA name to a single-spaced, trimmed form
// without changing its letters. It decodes HTML entities (workbook cells reach
// callers HTML-escaped), then folds U+00A0 NBSP — which is what an "empty"
// generation-workbook cell actually contains — and every other Unicode space,
// tab and newline into single ASCII spaces.
//
// Normalize is display normalisation. It is deliberately not the lookup key:
// see the package documentation on conservatism.
func Normalize(s string) string {
	s = cliutil.CleanText(s)
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		// unicode.IsSpace covers U+00A0 NBSP, which is what an "empty"
		// generation-workbook cell actually holds. The two explicit runes are
		// zero-width and invisible rather than space-classed.
		if unicode.IsSpace(r) || r == '\ufeff' || r == '\u200b' {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteRune(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// foldKey is the lookup key: Normalize, then case-fold and drop the periods and
// commas NEPRA sprinkles inconsistently ("Ltd." vs "Ltd", "Limited." vs
// "Limited"). Nothing else is removed — no word dropping, no abbreviation
// expansion, no Ltd/Limited equivalence — because those would risk folding two
// genuinely different plants onto one key.
func foldKey(s string) string {
	n := Normalize(s)
	var b strings.Builder
	b.Grow(len(n))
	for _, r := range n {
		if r == '.' || r == ',' {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return strings.TrimSpace(b.String())
}

// stripTrailingParen removes exactly one balanced parenthetical group at the
// end of s, e.g. "Narowal Energy Ltd. (HUBCO)" -> "Narowal Energy Ltd.". Only
// the last group goes, so "... (Private) Ltd-A (TBCCPL-A)" keeps "(Private)".
func stripTrailingParen(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, ")") {
		return s
	}
	depth := 0
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[:i])
			}
		}
	}
	return s
}

// trailingAcronym extracts an all-caps NEPRA parenthetical such as "(KEL)" or
// "(FWEL-I)". It returns "" for descriptive parentheticals like "(Private)" or
// "(Unit-II) Rahim Yar Khan".
func trailingAcronym(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, ")") {
		return ""
	}
	open := strings.LastIndex(s, "(")
	if open < 0 {
		return ""
	}
	inner := strings.TrimSpace(strings.TrimSuffix(s[open+1:], ")"))
	inner = strings.TrimSuffix(inner, ".")
	if len(inner) < 2 || len(inner) > 12 {
		return ""
	}
	for _, r := range inner {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
		default:
			return ""
		}
	}
	return strings.ToUpper(inner)
}

// MatchKind records how a name was resolved, so a caller can audit which joins
// leaned on the parenthetical-drift fallback rather than an exact hit.
type MatchKind string

const (
	// MatchCanonical is an exact hit on the canonical name.
	MatchCanonical MatchKind = "canonical"
	// MatchAlias is an exact hit on a recorded published variant.
	MatchAlias MatchKind = "alias"
	// MatchParenStripped is a hit after removing one trailing parenthetical
	// group, accepted only because exactly one plant remained.
	MatchParenStripped MatchKind = "paren_stripped"
)

// Match is a successful resolution.
type Match struct {
	Row Row
	// Query is the caller's string, untouched.
	Query string
	// NormalizedQuery is Query after Normalize.
	NormalizedQuery string
	Kind            MatchKind
	// MatchedName is the stored canonical name or alias that the query hit.
	MatchedName string
}

// Resolve maps a published NEPRA plant name onto its canonical plant.
//
// It normalises whitespace (including the U+00A0 NBSP the workbooks use for
// padding), case and stray periods, then requires an exact hit on a canonical
// name or a recorded alias. Failing that it tries once more with a single
// trailing parenthetical group removed — the "Narowal Energy Ltd. (HUBCO)" ->
// "Narowal Energy Ltd." drift — and accepts that only when exactly one plant
// remains. Two plants sharing a stripped base (the FWEL-I / FWEL-II case) are
// refused, not guessed.
//
// An unrecognised name returns false. There is no fuzzy fallback of any kind.
func Resolve(publishedName string) (Match, bool) {
	ix := load()
	if ix.loadErr != nil || ix.tbl == nil {
		return Match{}, false
	}
	norm := Normalize(publishedName)
	if norm == "" {
		return Match{}, false
	}
	key := foldKey(norm)
	if ix.collidingKeys[key] {
		return Match{}, false
	}
	if i, ok := ix.byKey[key]; ok {
		row := ix.tbl.Rows[i]
		kind := MatchAlias
		matched := norm
		if foldKey(row.CanonicalName) == key {
			kind = MatchCanonical
			matched = row.CanonicalName
		} else {
			for _, a := range row.Aliases {
				if foldKey(a) == key {
					matched = Normalize(a)
					break
				}
			}
		}
		return Match{Row: row, Query: publishedName, NormalizedQuery: norm, Kind: kind, MatchedName: matched}, true
	}
	base := stripTrailingParen(norm)
	sk := foldKey(base)
	if len(sk) < minStrippedKeyLen {
		return Match{}, false
	}
	hits := ix.byStripped[sk]
	if len(hits) != 1 {
		// 0 hits: unknown name. More than 1: two genuinely different plants
		// share this base string, so refusing is the only safe answer.
		return Match{}, false
	}
	row := ix.tbl.Rows[hits[0]]
	return Match{Row: row, Query: publishedName, NormalizedQuery: norm, Kind: MatchParenStripped, MatchedName: row.CanonicalName}, true
}

// ResolveInFY is Resolve plus a validity check, for callers joining a specific
// fiscal year. A name that resolves to a plant whose validity interval does not
// cover fy is refused, so a renamed or divested plant cannot be back-dated.
func ResolveInFY(publishedName, fy string) (Match, bool) {
	m, ok := Resolve(publishedName)
	if !ok || !m.Row.ValidInFY(fy) {
		return Match{}, false
	}
	return m, true
}

// Rows returns every crosswalk row, sorted by canonical name.
func Rows() []Row {
	ix := load()
	if ix.tbl == nil {
		return nil
	}
	out := make([]Row, len(ix.tbl.Rows))
	copy(out, ix.tbl.Rows)
	sort.Slice(out, func(i, j int) bool { return out[i].CanonicalName < out[j].CanonicalName })
	return out
}

// RowByCanonicalName looks a row up by its exact canonical name.
func RowByCanonicalName(name string) (Row, bool) {
	ix := load()
	if ix.tbl == nil {
		return Row{}, false
	}
	k := foldKey(name)
	if ix.collidingKeys[k] {
		return Row{}, false
	}
	i, ok := ix.byKey[k]
	if !ok || foldKey(ix.tbl.Rows[i].CanonicalName) != k {
		return Row{}, false
	}
	return ix.tbl.Rows[i], true
}

// ObservedFYs lists the fiscal years whose workbooks were fetched to build the
// crosswalk. A plant's absence from a year outside this list says nothing.
func ObservedFYs() []string {
	ix := load()
	if ix.tbl == nil {
		return nil
	}
	return append([]string(nil), ix.tbl.ObservedFYs...)
}

// HeldOutFYs lists fiscal years deliberately excluded when the crosswalk was
// built, so they can serve as an out-of-sample test of the alias table. Never
// treat a held-out year's data as having informed these rows.
func HeldOutFYs() []string {
	ix := load()
	if ix.tbl == nil {
		return nil
	}
	return append([]string(nil), ix.tbl.HeldOutFYs...)
}

// LatestObservedFY is the newest fiscal year in the crosswalk, i.e. the year an
// open-ended ValidToFY refers to.
func LatestObservedFY() string {
	ix := load()
	if ix.tbl == nil {
		return ""
	}
	return ix.tbl.LatestObservedFY
}

// AmbiguousTokens returns the curated refusals, sorted by token.
func AmbiguousTokens() []AmbiguousToken {
	ix := load()
	if ix.tbl == nil {
		return nil
	}
	out := append([]AmbiguousToken(nil), ix.tbl.AmbiguousTokens...)
	sort.Slice(out, func(i, j int) bool { return out[i].Token < out[j].Token })
	return out
}

// PossibleDuplicates returns the human-review list: published names that could
// plausibly be one plant, each with the verdict and the reasoning.
func PossibleDuplicates() []PossibleDuplicate {
	ix := load()
	if ix.tbl == nil {
		return nil
	}
	return append([]PossibleDuplicate(nil), ix.tbl.PossibleDuplicate...)
}

// AbsentParents returns operators known to be missing from these workbooks.
func AbsentParents() []AbsentParent {
	ix := load()
	if ix.tbl == nil {
		return nil
	}
	return append([]AbsentParent(nil), ix.tbl.AbsentParents...)
}

// Validate reports structural problems in the embedded crosswalk: a load
// failure, a fold key reaching two different rows, a row missing an identity,
// a ticker asserted without StatusListed, or a ticker claimed on a row whose
// confidence is too weak to carry one.
func Validate() error {
	ix := load()
	if ix.loadErr != nil {
		return ix.loadErr
	}
	if ix.tbl == nil {
		return fmt.Errorf("nepraxwalk: crosswalk not loaded")
	}
	if len(ix.collidingKeys) > 0 {
		keys := make([]string, 0, len(ix.collidingKeys))
		for k := range ix.collidingKeys {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return fmt.Errorf("nepraxwalk: fold keys reach more than one row: %v", keys)
	}
	for _, r := range ix.tbl.Rows {
		switch {
		case r.CanonicalName == "":
			return fmt.Errorf("nepraxwalk: row with empty canonical_name")
		case len(r.Observed) == 0:
			return fmt.Errorf("nepraxwalk: %q has no observations", r.CanonicalName)
		case r.ValidFromFY == "":
			return fmt.Errorf("nepraxwalk: %q has no valid_from_fy", r.CanonicalName)
		case r.Evidence == "":
			return fmt.Errorf("nepraxwalk: %q has no evidence", r.CanonicalName)
		}
		if r.HasTicker() && r.ListedStatus != StatusListed {
			return fmt.Errorf("nepraxwalk: %q asserts ticker %q but listed_status is %q", r.CanonicalName, r.PSXTicker, r.ListedStatus)
		}
		if r.HasTicker() && (r.Confidence == ConfidenceLow || r.Confidence == ConfidenceUnattributed) {
			return fmt.Errorf("nepraxwalk: %q asserts ticker %q at confidence %q", r.CanonicalName, r.PSXTicker, r.Confidence)
		}
		if r.ListedStatus == StatusListed && !r.HasTicker() {
			return fmt.Errorf("nepraxwalk: %q is listed but has no ticker", r.CanonicalName)
		}
		for _, o := range r.Observed {
			// Every observation must carry a recognised generation-block
			// status. A missing one is not a benign default: without it the
			// plant's operating status is unknowable, and the whole point of
			// the field is that 13 FY2023-24 plants are NOT operating.
			if _, ok := o.BlockStatus(); !ok {
				return fmt.Errorf("nepraxwalk: %q FY%s has block_status %q, which is not a known cell state",
					r.CanonicalName, o.FY, o.BlockStatusName)
			}
			// The stored capacity state is a checked redundancy against the
			// state re-derived from the published string. Disagreement means
			// the curated data has drifted from what it claims to describe.
			if got, want := o.InstalledCapacityStateName, o.Capacity().State().String(); got != want {
				return fmt.Errorf("nepraxwalk: %q FY%s records installed_capacity_state %q but %q parses as %q",
					r.CanonicalName, o.FY, got, o.InstalledCapacityMWRaw, want)
			}
		}
	}
	return nil
}
