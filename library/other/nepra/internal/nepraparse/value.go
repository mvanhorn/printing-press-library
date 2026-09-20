package nepraparse

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CellState is the five-way classification every workbook cell falls into,
// plus a sixth bucket for text this package has never seen.
//
// These states are distinguishable in the source and must stay
// distinguishable downstream. Collapsing StateNotReported into a numeric zero
// invents 11 plants x 12 months of false zero generation in FY2017-18 alone;
// collapsing StateDelicensed into "missing" loses the reason a licensed
// plant stopped reporting.
type CellState uint8

const (
	// StateUnset is the zero value of CellState and means "this Value was
	// never populated". It exists solely so that the zero value of [Value] is
	// NOT a measurement.
	//
	// Without this, StateNumeric would sit at iota 0 and a Value nobody filled
	// in would report Present()==true and Float64()==(0,true) — an unpopulated
	// cell indistinguishable from a genuine measured 0.00, which is the exact
	// distinction this package exists to preserve. That is reachable from
	// ordinary API use: a PlantByName miss returns Plant{}, and Month() on an
	// out-of-range month returns MonthlyObservation{}.
	StateUnset CellState = iota
	// StateNumeric is a real measured number. It INCLUDES a genuine 0.00:
	// "0.00" occurs 437/540/471 times across FY2017-18/FY2020-21/FY2023-24,
	// always in otherwise fully-populated rows, and Karachi Nuclear Power
	// Plant-II reports sixteen 0.00 month cells in FY2020-21 alongside a
	// non-zero annual Sum of 1,705.91 GWh. A measured zero is data.
	StateNumeric
	// StateNotReported is the NBSP blank: the cell holds &nbsp; or a literal
	// 0xA0 byte and carries no measurement. Absence of measurement, not zero.
	StateNotReported
	// StateDelicensed is the literal text DELICENSED. Capacity columns stay
	// valid: a delicensed plant keeps its published installed and dependable
	// capacity.
	//
	// MEASURED across all seven reachable years, which CORRECTS an earlier
	// "FY2023-24 only" note here. That note generalised from a three-year
	// sample (FY2017-18, FY2020-21, FY2023-24) that skipped the year the
	// vocabulary actually starts:
	//
	//	FY2017-18 .. FY2021-22   0 rows
	//	FY2022-23               11 rows / 286 cells   <- FIRST appearance
	//	FY2023-24               12 rows / 312 cells
	StateDelicensed
	// StateDecommissioned is the literal text DECOMMISSIONED. Capacity
	// columns stay valid.
	//
	// MEASURED, same correction as above: 1 row / 26 cells in FY2022-23 AND 1
	// row / 26 cells in FY2023-24 — not FY2023-24 only.
	StateDecommissioned
	// StateExportToKElectric is the literal text "Export to K.Electric".
	// 3 cells, FY2020-21 only, and it appears in the Installed Capacity (MW)
	// column rather than the monthly block.
	StateExportToKElectric
	// StateUnknownText is any other non-numeric, non-blank text. It is
	// recorded and flagged, never rejected and never coerced to a number:
	// an unmodelled sentinel must be visible, not fatal.
	StateUnknownText
)

var cellStateNames = [...]string{
	StateUnset:             "unset",
	StateNumeric:           "numeric",
	StateNotReported:       "not_reported",
	StateDelicensed:        "delicensed",
	StateDecommissioned:    "decommissioned",
	StateExportToKElectric: "export_to_k_electric",
	StateUnknownText:       "unknown_text",
}

// String returns a stable snake_case name suitable for logs and storage keys.
func (s CellState) String() string {
	if int(s) < len(cellStateNames) && cellStateNames[s] != "" {
		return cellStateNames[s]
	}
	return "cell_state_" + strconv.Itoa(int(s))
}

// Status reports whether the state is a regulatory status sentinel rather
// than a measurement or a plain blank. Status cells must never null a plant's
// capacity: Kotri Power Station in FY2023-24 has Installed 174 MW and
// Dependable 120 MW while all 26 of its monthly cells read DELICENSED.
func (s CellState) Status() bool {
	switch s {
	case StateDelicensed, StateDecommissioned, StateExportToKElectric:
		return true
	}
	return false
}

// numericRE accepts exactly the numeric shapes these workbooks publish:
// optional sign, digits with optional 3-digit comma groups, optional decimal
// part. It is applied BEFORE any comma stripping, so "DELICENSED" can never
// reach strconv and a stray ";" can never become a number.
var numericRE = regexp.MustCompile(`^[+-]?(\d+|\d{1,3}(,\d{3})+)(\.\d+)?$`)

// Value is one workbook cell: a number when there is one, and always an
// explicit [CellState]. There is deliberately no exported field and no method
// returning a bare float64, because a bare float hides the difference between
// a measured 0.00 and an unreported month.
type Value struct {
	state CellState
	num   float64
	raw   string
	// comma records that the source used thousands separators. Tracked so
	// [Census] can prove the separators were handled rather than tolerated.
	comma bool
}

// ParseValue classifies one already-cleaned cell string into a [Value].
//
// The order of the checks is the correctness requirement: blankness is tested
// first (so an NBSP never looks like text), then the known sentinels, then
// numeric shape, and only then are commas stripped for the actual conversion.
func ParseValue(cell string) Value {
	s := collapseText(cell)
	if s == "" {
		return Value{state: StateNotReported}
	}
	if st, ok := statusFor(s); ok {
		return Value{state: st, raw: s}
	}
	if numericRE.MatchString(s) {
		bare := s
		hasComma := strings.Contains(bare, ",")
		if hasComma {
			bare = strings.ReplaceAll(bare, ",", "")
		}
		n, err := strconv.ParseFloat(bare, 64)
		if err == nil {
			return Value{state: StateNumeric, num: n, raw: s, comma: hasComma}
		}
	}
	return Value{state: StateUnknownText, raw: s}
}

// statusFor recognises the three status sentinels. Matching is
// case-insensitive on the collapsed text so a change of case upstream does
// not silently demote a status cell to unknown text.
func statusFor(s string) (CellState, bool) {
	switch strings.ToUpper(s) {
	case "DELICENSED":
		return StateDelicensed, true
	case "DECOMMISSIONED":
		return StateDecommissioned, true
	case "EXPORT TO K.ELECTRIC", "EXPORT TO K. ELECTRIC", "EXPORT TO KELECTRIC":
		return StateExportToKElectric, true
	}
	return StateUnset, false
}

// stateByName is the inverse of [CellState.String], used by JSON decoding so
// a round-trip preserves the state rather than defaulting it.
var stateByName = func() map[string]CellState {
	m := make(map[string]CellState, len(cellStateNames))
	for st, name := range cellStateNames {
		if name != "" {
			m[name] = CellState(st)
		}
	}
	return m
}()

// MarshalJSON emits the stable snake_case name rather than the iota number.
//
// The numeric values are NOT a stable wire format: inserting [StateUnset] at
// position 0 — which was necessary so an unpopulated [Value] is not a
// measurement — shifted every other state by one. Anything that had persisted
// a numeric 1 as "not_reported" would now read it as "numeric". Names do not
// shift, so names are what cross the wire.
func (s CellState) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON accepts the snake_case name only. A number or an unrecognised
// name is an error rather than a silent [StateUnset] or, worse, a silent
// [StateNumeric].
func (s *CellState) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return fmt.Errorf("nepraparse: cell state must be a name string, not %s: %w", data, err)
	}
	st, ok := stateByName[name]
	if !ok {
		return fmt.Errorf("nepraparse: unknown cell state %q", name)
	}
	*s = st
	return nil
}

// CellStateByName is the inverse of [CellState.String]. The second return is
// false for a name this build does not know, and the first is then
// [StateUnset] — never a measurement. A sibling package stores a state by
// name in curated JSON and must be able to type it back without an unknown
// name quietly becoming [StateNumeric].
func CellStateByName(name string) (CellState, bool) {
	st, ok := stateByName[name]
	if !ok {
		return StateUnset, false
	}
	return st, true
}

// valueJSON is the wire shape of a [Value].
//
// `value` is omitted entirely when the cell holds no measurement, so a consumer
// that reads the JSON cannot mistake an absent month for a zero one. `state` is
// always present and is the field a consumer must branch on.
type valueJSON struct {
	State string   `json:"state"`
	Value *float64 `json:"value,omitempty"`
	Raw   string   `json:"raw,omitempty"`
	Comma bool     `json:"thousands_separated,omitempty"`
}

// MarshalJSON emits the state alongside the number.
//
// [Value] deliberately has no exported fields, which means the default encoder
// would emit `{}` for every cell and a decode would reconstitute StateUnset —
// dropping 3,478 MW to nothing and turning a DELICENSED marker into a
// measurement. This method is what makes the panel safe to serialise, which is
// the CLI's primary output path.
func (v Value) MarshalJSON() ([]byte, error) {
	out := valueJSON{State: v.state.String(), Raw: v.raw, Comma: v.comma}
	if v.state == StateNumeric {
		n := v.num
		out.Value = &n
	}
	return json.Marshal(out)
}

// UnmarshalJSON restores a [Value] including its state.
//
// An unrecognised state name is an error rather than a silent downgrade: a
// state this build does not know about must not be decoded as a measurement.
func (v *Value) UnmarshalJSON(data []byte) error {
	var in valueJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	st, ok := stateByName[in.State]
	if !ok {
		return fmt.Errorf("nepraparse: unknown cell state %q", in.State)
	}
	v.state = st
	v.raw = in.Raw
	v.comma = in.Comma
	v.num = 0
	if st == StateNumeric {
		if in.Value == nil {
			return fmt.Errorf("nepraparse: state %q carries no value", in.State)
		}
		v.num = *in.Value
	} else if in.Value != nil {
		return fmt.Errorf("nepraparse: state %q must not carry a value", in.State)
	}
	return nil
}

// State returns the cell's classification.
func (v Value) State() CellState { return v.state }

// Present reports whether the cell holds a real measurement. A measured 0.00
// is present; an NBSP blank, a DELICENSED marker and unknown text are not.
func (v Value) Present() bool { return v.state == StateNumeric }

// Float64 returns the measured number and whether there was one. Callers that
// want a number must handle the false case; there is no variant that returns
// a zero for an absent measurement.
func (v Value) Float64() (float64, bool) {
	if v.state != StateNumeric {
		return 0, false
	}
	return v.num, true
}

// Raw returns the cell text as published, whitespace-collapsed. For a blank
// cell it is "" — the NBSP has been collapsed away. (strings.TrimSpace would
// also strip a correctly decoded U+00A0; what it would not strip is an
// UNDECODED 0xA0 byte, which is why decoding precedes collapsing. See the
// nbsp constant in text.go.)
func (v Value) Raw() string { return v.raw }

// HadThousandsSeparator reports whether the published cell used commas.
func (v Value) HadThousandsSeparator() bool { return v.comma }

// String renders the value for humans without pretending an absent
// measurement is a number.
func (v Value) String() string {
	if v.state == StateNumeric {
		return v.raw
	}
	if v.state == StateNotReported {
		return "<not reported>"
	}
	return "<" + v.state.String() + ": " + v.raw + ">"
}
