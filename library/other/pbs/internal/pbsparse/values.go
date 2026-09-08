// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Package pbsparse parses Pakistan Bureau of Statistics price releases.
//
// Everything in this package is pure logic over bytes already fetched: no HTTP,
// no filesystem, no clock. That is deliberate so the parser can be tested
// exhaustively against real fixtures, because a silent parse error here would
// make every downstream panel command confidently wrong at once.
package pbsparse

import (
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ValueState classifies a price cell. PBS uses THREE distinct conventions for
// "there is no price here" and they must never be collapsed into one another.
// Measured on Annex_03.09.2026.xlsx: weekly Appendix-A carries 43 numeric-zero
// cells and 51 structurally-absent cells against 3272 real numerics, and
// Appendix-B additionally uses the literal string "N.A." — the file documents
// its own sentinel ("N.A. stands for Not Available.").
//
// Cost of collapsing them, measured on item 3 (Rice IRRI-6/9): the true
// national average is 154.00. Averaging the missing cities' zeros as prices
// yields 128.36, an error of -16.6%. Excluding them yields 155.86.
type ValueState string

const (
	// StatePresent is a real observed price.
	StatePresent ValueState = "present"
	// StateZero is a cell rendered as numeric 0. On PBS this means the price
	// was not collected, NOT that the good is free. It is kept distinct from
	// StateBlank because the two carry different upstream provenance and a
	// future PBS change could make one of them meaningful.
	StateZero ValueState = "zero"
	// StateBlank is a structurally absent cell (omitted from the xlsx XML
	// entirely, or empty in extracted PDF text).
	StateBlank ValueState = "blank"
	// StateNA is the literal string "N.A.", documented upstream as
	// "Not Available". Appears in Appendix-B.
	StateNA ValueState = "na"
	// StateUnparseable is a non-empty cell that is not a number and not a
	// recognised sentinel. Recorded rather than discarded so a new upstream
	// convention surfaces as a count instead of vanishing.
	StateUnparseable ValueState = "unparseable"
)

// Value is a single price cell with its provenance preserved.
//
// Num is only meaningful when State == StatePresent. Callers must gate on
// State, never on Num != 0 — that test cannot distinguish a real zero-priced
// observation from an uncollected one, which is exactly the -16.6% bug.
type Value struct {
	Num   float64    `json:"value,omitempty"`
	State ValueState `json:"value_state"`
	Raw   string     `json:"raw,omitempty"`
}

// Present reports whether this cell carries a usable number.
func (v Value) Present() bool { return v.State == StatePresent }

var (
	// PBS writes plain en-US grouping. Accounting parentheses are NOT used in
	// these files (unlike MUFAP, where "(4.97)" means -4.97), but the form is
	// still decoded here so that if PBS ever adopts it the sign is correct
	// rather than silently dropped as unparseable.
	reParen = regexp.MustCompile(`^\((.*)\)$`)
	reNum   = regexp.MustCompile(`^-?[\d,]*\.?\d+$`)
)

// naTokens are the exact non-numeric strings PBS uses for a missing price.
// Compared after upper-casing and stripping internal spaces and periods so
// "N.A.", "N.A", "n.a." and "NA" all land on StateNA.
var naTokens = map[string]bool{"NA": true}

// ParseValue classifies one raw cell.
//
// absent must be true when the cell was structurally missing (no <c> element
// in the xlsx row, or no token at that column position in PDF text). A blank
// cell is OMITTED from xlsx XML rather than present-and-empty, so a parser
// that only iterates existing cells cannot see blanks at all — that mistake
// reproduced a false "weekly uses zero, monthly uses blank" dichotomy before
// the counts were taken by grid position instead.
func ParseValue(raw string, absent bool) Value {
	if absent {
		return Value{State: StateBlank}
	}
	s := strings.TrimSpace(html.UnescapeString(raw))
	// Strip non-breaking spaces: PBS xlsx sheets declare no charset and carry
	// 0xa0 bytes that fail strict UTF-8, so cp1252 decoding upstream can leave
	// these behind.
	s = strings.NewReplacer(" ", "", "​", "").Replace(s)
	if s == "" {
		return Value{State: StateBlank}
	}

	// Footnote markers live INSIDE label and value strings on PBS
	// ("Electricity Charges for Q1*"). Strip trailing markers before numeric
	// classification so a starred value is not lost as unparseable.
	trimmed := strings.TrimRight(s, "*+ \t")

	key := strings.ToUpper(strings.NewReplacer(".", "", " ", "", "-", "").Replace(trimmed))
	if naTokens[key] {
		return Value{State: StateNA, Raw: s}
	}

	neg := false
	if m := reParen.FindStringSubmatch(trimmed); m != nil {
		neg = true
		trimmed = strings.TrimSpace(m[1])
	}
	if !reNum.MatchString(trimmed) {
		return Value{State: StateUnparseable, Raw: s}
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(trimmed, ",", ""), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return Value{State: StateUnparseable, Raw: s}
	}
	if neg {
		f = -f
	}
	if f == 0 {
		return Value{State: StateZero, Raw: s}
	}
	return Value{Num: f, State: StatePresent, Raw: s}
}

// StateCensus counts each value state across a slice. It is the accounting
// behind `coverage --state-census`, and every panel command prints it so a
// caller can always see how many cells were excluded from an aggregate.
type StateCensus struct {
	Present     int `json:"present"`
	Zero        int `json:"zero"`
	Blank       int `json:"blank"`
	NA          int `json:"na"`
	Unparseable int `json:"unparseable"`
}

// Total returns every cell counted, present or not.
func (c StateCensus) Total() int {
	return c.Present + c.Zero + c.Blank + c.NA + c.Unparseable
}

// Excluded returns the cells that carry no usable number.
func (c StateCensus) Excluded() int { return c.Total() - c.Present }

// Census tallies states over values.
func Census(vals []Value) StateCensus {
	var c StateCensus
	for _, v := range vals {
		switch v.State {
		case StatePresent:
			c.Present++
		case StateZero:
			c.Zero++
		case StateBlank:
			c.Blank++
		case StateNA:
			c.NA++
		default:
			c.Unparseable++
		}
	}
	return c
}

// CleanText normalises a label taken from PBS xlsx or PDF text.
//
// It unescapes HTML entities (an "&amp;" survives into sharedStrings on
// "Nitro. Phosph. &amp; Pot. (Npk)"), collapses whitespace runs (header cells
// concatenate several dates separated by long space runs), and removes soft
// hyphenation that Appendix-B uses for city names while Appendix-A of the same
// file does not ("Islam-abad" vs "Islamabad (01)").
func CleanText(s string) string {
	s = html.UnescapeString(s)
	s = strings.NewReplacer(" ", " ", "­", "", "​", "").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

// reSoftHyphen matches a hyphen between two lower-case letters, which is how
// PBS breaks city names in Appendix-B ("Gujran-wala", "Faisal-abad"). It is
// deliberately narrow: it must not touch real hyphenated tokens such as
// "Rice IRRI-6/9", "Hi-Speed Diesel" or "2015-16".
var reSoftHyphen = regexp.MustCompile(`([a-z])-([a-z])`)

// NormalizeCity reduces any PBS spelling of a city to a lower-case key.
//
// It handles all three forms that ship in ONE weekly file: the Appendix-A form
// with a code ("Islamabad (01)"), the Appendix-B soft-hyphenated form
// ("Islam-abad"), and a bare name. The city code is returned separately when
// present because it is the only stable machine identifier PBS provides.
func NormalizeCity(s string) (key string, code string) {
	s = CleanText(s)
	if m := reCityCode.FindStringSubmatch(s); m != nil {
		code = m[2]
		s = strings.TrimSpace(m[1])
	}
	s = reSoftHyphen.ReplaceAllString(strings.ToLower(s), "$1$2")
	return strings.TrimSpace(s), code
}

var reCityCode = regexp.MustCompile(`^(.*?)\s*\((\d{2})\)\s*$`)
