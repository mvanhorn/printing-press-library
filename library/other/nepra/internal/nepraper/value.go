package nepraper

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ValueKind says what a Value actually is. It exists because a bare float64
// cannot distinguish "the report says 0" from "the report does not say", and
// conflating those two is the single worst failure mode for this data: NEPRA
// publishes real zeros (a met target has breach 0.00) right next to entities
// whose data was deliberately excluded.
type ValueKind int

const (
	// KindAbsent means no value was published in this cell / for this key.
	// It is the zero value of ValueKind on purpose: a Value{} is "nothing
	// known", never "zero".
	KindAbsent ValueKind = iota
	// KindNumeric means a number was published and parsed. Num is valid.
	KindNumeric
	// KindQualitative means a non-numeric datum was published, e.g. NEPRA's
	// "Far Away" / "Near to Limit" breach labels. Raw carries it verbatim.
	KindQualitative
	// KindUnverified means something that looks like a figure is present in
	// the source but cannot honestly be read as a number -- for example the
	// truncated Excel chart labels "19,535." whose trailing digits are simply
	// not in the text layer. Raw carries the truncated string; Num is invalid.
	KindUnverified
	// KindUnavailable means the source document itself could not be obtained
	// (e.g. FY2023-24's PER is linked from NEPRA's index but 404s). This is
	// distinct from "the report exists and reports nothing".
	KindUnavailable
	// KindExcluded means the entity was excluded from the report on the
	// record, with a stated reason. Reason carries NEPRA's own wording.
	KindExcluded
)

func (k ValueKind) String() string {
	switch k {
	case KindAbsent:
		return "absent"
	case KindNumeric:
		return "numeric"
	case KindQualitative:
		return "qualitative"
	case KindUnverified:
		return "unverified"
	case KindUnavailable:
		return "unavailable"
	case KindExcluded:
		return "excluded"
	}
	return fmt.Sprintf("ValueKind(%d)", int(k))
}

// valueKindByName is the inverse of [ValueKind.String].
var valueKindByName = map[string]ValueKind{
	"absent":      KindAbsent,
	"numeric":     KindNumeric,
	"qualitative": KindQualitative,
	"unverified":  KindUnverified,
	"unavailable": KindUnavailable,
	"excluded":    KindExcluded,
}

// MarshalJSON emits the kind's name rather than its iota position. A bare
// iota is not a wire format: inserting a kind renumbers every value that was
// ever serialised, and the sibling nepraparse package hit exactly that when
// StateUnset was added at position 0.
func (k ValueKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

// UnmarshalJSON accepts the kind's name only, and refuses a number or an
// unknown name rather than defaulting to KindAbsent — or, worse, to a kind
// that makes Num look valid.
func (k *ValueKind) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return fmt.Errorf("nepraper: value kind must be a name string, not %s: %w", data, err)
	}
	vk, ok := valueKindByName[name]
	if !ok {
		return fmt.Errorf("nepraper: unknown value kind %q", name)
	}
	*k = vk
	return nil
}

// Value is one published (or pointedly unpublished) figure.
//
// Num is meaningful only when Kind == KindNumeric. Every other kind carries its
// evidence in Raw and/or Reason. Callers that want a number must go through
// Float, which refuses to invent one.
type Value struct {
	Kind   ValueKind
	Num    float64
	Raw    string // the source text exactly as extracted, before parsing
	Reason string // why this is not a number, for the non-numeric kinds
	// Label is the canonical form of a recognised non-numeric datum, e.g.
	// "Far Away" for a cell whose glyphs spelled only "Away". It is set
	// ALONGSIDE Raw, never over it: Raw stays the evidence and Label is the
	// interpretation, so a caller can compare verdicts canonically while
	// still seeing what the PDF said.
	Label string
}

// Float returns the numeric value and true only for KindNumeric.
func (v Value) Float() (float64, bool) {
	if v.Kind == KindNumeric {
		return v.Num, true
	}
	return 0, false
}

// IsNumeric reports whether this Value carries a usable number.
func (v Value) IsNumeric() bool { return v.Kind == KindNumeric }

// valueJSON is the wire shape of a [Value]. `value` is OMITTED for every
// non-numeric kind, so a JSON consumer cannot read a number that was never
// published.
type valueJSON struct {
	Kind   string   `json:"kind"`
	Value  *float64 `json:"value,omitempty"`
	Raw    string   `json:"raw,omitempty"`
	Label  string   `json:"label,omitempty"`
	Reason string   `json:"reason,omitempty"`
}

// MarshalJSON emits the kind alongside the number, and omits the number
// entirely unless there is one.
//
// The default encoder exported Num unconditionally, so all five non-numeric
// kinds serialised as `"Num":0` — `jq '.Value.Num'` returned 0 for an ABSENT
// cell, for a TESCO row NEPRA excluded on the record, and for a KindUnverified
// truncated chart label such as "19,535." whose whole point is that it must
// never become a number. The Go API honoured the distinction through Float();
// the JSON projection, which is what the CLI actually emits, did not.
func (v Value) MarshalJSON() ([]byte, error) {
	out := valueJSON{Kind: v.Kind.String(), Raw: v.Raw, Label: v.Label, Reason: v.Reason}
	if v.Kind == KindNumeric {
		n := v.Num
		out.Value = &n
	}
	return json.Marshal(out)
}

// UnmarshalJSON restores a [Value], refusing a numeric kind with no number
// and a non-numeric kind that carries one.
func (v *Value) UnmarshalJSON(data []byte) error {
	var in valueJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	kind, ok := valueKindByName[in.Kind]
	if !ok {
		return fmt.Errorf("nepraper: unknown value kind %q", in.Kind)
	}
	switch {
	case kind == KindNumeric && in.Value == nil:
		return fmt.Errorf("nepraper: kind %q carries no value", in.Kind)
	case kind != KindNumeric && in.Value != nil:
		return fmt.Errorf("nepraper: kind %q must not carry a value (got %v)", in.Kind, *in.Value)
	}
	v.Kind, v.Raw, v.Label, v.Reason, v.Num = kind, in.Raw, in.Label, in.Reason, 0
	if in.Value != nil {
		v.Num = *in.Value
	}
	return nil
}

func (v Value) String() string {
	switch v.Kind {
	case KindNumeric:
		return strconv.FormatFloat(v.Num, 'f', -1, 64)
	case KindAbsent:
		return "<absent>"
	case KindQualitative, KindUnverified:
		if v.Reason != "" {
			return fmt.Sprintf("%s(%q: %s)", v.Kind, v.Raw, v.Reason)
		}
		return fmt.Sprintf("%s(%q)", v.Kind, v.Raw)
	default:
		return fmt.Sprintf("%s(%s)", v.Kind, v.Reason)
	}
}

// hasAlphanumeric reports whether s contains any letter or digit.
func hasAlphanumeric(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// Numeric builds a parsed numeric Value, retaining the raw source text.
func Numeric(n float64, raw string) Value {
	return Value{Kind: KindNumeric, Num: n, Raw: raw}
}

// Qualitative builds a Value for a published non-numeric datum.
func Qualitative(raw string) Value { return Value{Kind: KindQualitative, Raw: raw} }

// Unverified builds a Value for source text that must not be read as a number.
func Unverified(raw, reason string) Value {
	return Value{Kind: KindUnverified, Raw: raw, Reason: reason}
}

// Unavailable builds a Value for a source document that could not be obtained.
func Unavailable(reason string) Value { return Value{Kind: KindUnavailable, Reason: reason} }

// Excluded builds a Value for an entity excluded from the report on the record.
func Excluded(reason string) Value { return Value{Kind: KindExcluded, Reason: reason} }

// Absent builds the explicit "nothing was published here" Value.
func Absent() Value { return Value{Kind: KindAbsent} }

// qualitativeBreach lists the non-numeric breach labels NEPRA uses in place of
// a breach magnitude from FY2020-21 onwards. Matched on the space-stripped,
// lowercased token because the PDF text layer splits and re-orders these.
var qualitativeBreach = map[string]string{
	"faraway":     "Far Away",
	"neartolimit": "Near to Limit",
	"nearlimit":   "Near Limit",
	"achieved":    "Achieved",
	"complied":    "Complied",
	"withinlimit": "Within Limit",
}

// ParseFigure turns one raw table cell into a Value.
//
// It is deliberately strict. A token that only looks like a number is not
// treated as one:
//
//   - "19,535." (a truncated Excel chart label) becomes KindUnverified, never
//     19535 and never 19.535. The trailing digits are absent from the text
//     layer and inventing them would be fabrication.
//   - "(0.16)" becomes -0.16, which is how NEPRA prints a negative breach.
//   - "Far Away" becomes KindQualitative, not zero.
//   - "" and "-" become KindAbsent, not zero.
func ParseFigure(raw string) Value {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Absent()
	}
	compact := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', ' ', '​':
			return -1
		}
		return r
	}, trimmed)
	if compact == "" || compact == "-" || compact == "--" || compact == "N/A" || compact == "n/a" {
		return Absent()
	}
	if label, ok := qualitativeBreach[strings.ToLower(compact)]; ok {
		return Value{Kind: KindQualitative, Raw: trimmed, Reason: "NEPRA breach label", Num: 0}.withLabel(label)
	}

	neg := false
	s := compact
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg = true
		s = s[1 : len(s)-1]
	}
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimPrefix(s, "+")
	if strings.HasPrefix(s, "-") {
		neg = !neg
		s = s[1:]
	}
	s = strings.ReplaceAll(s, ",", "")

	if s == "" {
		// Everything was stripped as sign, parentheses or a percent sign, so
		// nothing was published here. "()" and "%" reach this branch and are
		// stray glyphs, not data.
		if !hasAlphanumeric(trimmed) {
			return Unverified(trimmed,
				"the token carries no letters or digits, so it is a stray glyph or a "+
					"separator rather than a published datum")
		}
		return Qualitative(trimmed)
	}
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			// A token made only of punctuation is a stray glyph, not a
			// published datum. Calling "(" or "%" Qualitative asserted that
			// NEPRA published a non-numeric VALUE there, which is a claim
			// about the document; Unverified says only that this build cannot
			// read it, which is a claim about the token.
			if !hasAlphanumeric(trimmed) {
				return Unverified(trimmed,
					"the token carries no letters or digits, so it is a stray glyph or a "+
						"separator rather than a published datum")
			}
			return Qualitative(trimmed)
		}
	}
	if strings.HasSuffix(s, ".") {
		// The reason must NOT assert the chart-label provenance. Both causes
		// produce this shape and they are different facts: a genuinely
		// truncated Excel chart label (FY2010-11's "19,535.", whose digits
		// really are absent from the document) and a split cell whose sibling
		// fragment was not reassembled by band. In THIS corpus every
		// trailing-dot token that reaches a cell — "4152.", "68.", "28.",
		// "49." — is the second kind and is correctly rejoined; claiming
		// "truncated chart label" for those would attribute a document
		// property to a reconstruction artefact.
		return Unverified(trimmed,
			"trailing decimal point with no digits after it: either a truncated chart label "+
				"whose remaining digits are absent from the text layer, or a cell fragment whose "+
				"sibling was not reassembled; which one is not determinable from this token alone")
	}
	if strings.Count(s, ".") > 1 {
		return Unverified(trimmed, "more than one decimal point; cell reconstruction is ambiguous")
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return Qualitative(trimmed)
	}
	if neg {
		n = -n
	}
	return Numeric(n, trimmed)
}

// withRawLabel normalises a recognised qualitative breach label to NEPRA's own
// spelling while keeping the extracted text in Reason for traceability.
// withLabel attaches the canonical form of a published non-numeric datum
// WITHOUT destroying the source text.
//
// It used to overwrite Raw with the canonical label, which contradicted Raw's
// own documented contract ("the source text exactly as extracted, before
// parsing") and destroyed the evidence: after it ran, there was no way to see
// that the PDF actually spelled "Away" rather than "Far Away". It also made
// "Near to Limit" and "Near Limit" compare as a kind mismatch.
func (v Value) withLabel(label string) Value {
	v.Label = label
	v.Num = 0
	return v
}
