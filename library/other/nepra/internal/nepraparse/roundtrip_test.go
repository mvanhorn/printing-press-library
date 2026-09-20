package nepraparse

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestZeroValueIsNotAMeasurement is finding A's first half. Nothing in the
// suite ever CONSTRUCTED a Value{} — the API test only reflected over field
// exports — so for the life of the package the zero value of the type whose
// whole purpose is null discipline went unasserted.
//
// It matters because Value{} is reachable from ordinary use: PlantByName
// returns Plant{} on a miss, and Month() returns MonthlyObservation{} for an
// out-of-range month. Before StateUnset was added at iota 0, every one of
// those zero Values reported Present()==true and Float64()==(0,true): an
// unpopulated cell indistinguishable from a genuine measured 0.00.
func TestZeroValueIsNotAMeasurement(t *testing.T) {
	var v Value
	if got := v.State(); got != StateUnset {
		t.Errorf("Value{}.State() = %v, want %v", got, StateUnset)
	}
	if v.Present() {
		t.Error("Value{}.Present() = true; an unpopulated cell must not be a measurement")
	}
	if n, ok := v.Float64(); ok {
		t.Errorf("Value{}.Float64() = (%v,true); an unpopulated cell must not yield a number", n)
	}
	if v.Raw() != "" {
		t.Errorf("Value{}.Raw() = %q, want empty", v.Raw())
	}
	if v.HadThousandsSeparator() {
		t.Error("Value{}.HadThousandsSeparator() = true")
	}
	// It must not RENDER as a number either: a report printing "0.00" for an
	// unpopulated cell is the same lie one layer up.
	if s := v.String(); s == "0" || s == "0.00" || s == "" {
		t.Errorf("Value{}.String() = %q, which reads as a measurement or as nothing", s)
	}
	// And it must be distinguishable from a real measured zero.
	if zero := ParseValue("0.00"); v.State() == zero.State() {
		t.Error("Value{} and a measured 0.00 report the same state")
	}
	// StateUnset must be the zero value, and must not be a status sentinel.
	if StateUnset != 0 {
		t.Errorf("StateUnset = %d, want 0; the zero value of CellState must be the unpopulated one", StateUnset)
	}
	if StateUnset.Status() {
		t.Error("StateUnset.Status() = true; an unpopulated cell is not a regulatory status")
	}
	if got := StateUnset.String(); got != "unset" {
		t.Errorf("StateUnset.String() = %q, want %q", got, "unset")
	}
}

// TestZeroPlantIsNotAMeasurement is the same requirement one level up, on the
// two paths that actually hand a caller a zero Plant or a zero observation.
func TestZeroPlantIsNotAMeasurement(t *testing.T) {
	var p Plant
	for _, c := range []struct {
		label string
		v     Value
	}{
		{"InstalledCapacity", p.InstalledCapacity},
		{"DependableCapacity", p.DependableCapacity},
		{"Total.Utilisation", p.Total.Utilisation},
		{"Total.Generation", p.Total.Generation},
	} {
		if c.v.Present() {
			t.Errorf("Plant{}.%s is Present", c.label)
		}
		if n, ok := c.v.Float64(); ok {
			t.Errorf("Plant{}.%s yields %v", c.label, n)
		}
	}
	for i, m := range p.Months {
		if m.Utilisation.Present() || m.Generation.Present() {
			t.Errorf("Plant{}.Months[%d] is Present", i)
		}
	}
	if p.Status != StateUnset {
		t.Errorf("Plant{}.Status = %v, want %v: an unpopulated row has no known block status", p.Status, StateUnset)
	}

	// An out-of-range month returns a zero observation, and its cells must be
	// unset rather than measured zeros.
	if obs, ok := p.Month(Month(0)); ok {
		t.Error("Month(0) reported ok")
	} else if obs.Utilisation.Present() || obs.Generation.Present() {
		t.Error("the zero MonthlyObservation carries a measurement")
	}
	if obs, ok := p.Month(Month(13)); ok {
		t.Error("Month(13) reported ok")
	} else if obs.Generation.Present() {
		t.Error("the zero MonthlyObservation carries a measurement")
	}

	// A PlantByName miss must be the same: no measurements anywhere.
	w := parse(t, trimFY2324, "2023-24")
	miss, ok := w.PlantByName("No Such Plant Anywhere")
	if ok {
		t.Fatal("PlantByName matched a name that is not in the workbook")
	}
	if miss.InstalledCapacity.Present() || miss.Total.Generation.Present() {
		t.Error("a PlantByName miss returned a Plant carrying measurements")
	}
	if miss.Status != StateUnset {
		t.Errorf("a PlantByName miss returned Status %v, want %v", miss.Status, StateUnset)
	}
}

// TestValueJSONRoundTrip is finding A's second half, and the defect it guards
// was live: Value has only unexported fields, so json.Marshal emitted `{}`
// for EVERY cell and a decode reconstituted Value{}. Tarbela's 3,478 MW came
// back as a measured 0.00 and Kotri's DELICENSED came back as a measurement.
// JSON is the CLI's primary output path, so this was the whole dataset.
func TestValueJSONRoundTrip(t *testing.T) {
	cells := []string{
		"3,478", "0.00", "94.94", "-1.50", "13,365.93",
		"", " ", "DELICENSED", "DECOMMISSIONED", "Export\n  to K.Electric", ";",
	}
	for _, cell := range cells {
		in := ParseValue(cell)
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("marshalling ParseValue(%q): %v", cell, err)
		}
		if bytes.Equal(b, []byte("{}")) {
			t.Fatalf("ParseValue(%q) serialised as {}; every field is unexported, so Value needs its own MarshalJSON", cell)
		}
		// A cell with no measurement must not carry a `value` key at all: an
		// explicit null or a 0 would both invite a consumer to read zero.
		if !in.Present() && bytes.Contains(b, []byte(`"value"`)) {
			t.Errorf("ParseValue(%q) is not a measurement but serialised with a value key: %s", cell, b)
		}
		if in.Present() && !bytes.Contains(b, []byte(`"value"`)) {
			t.Errorf("ParseValue(%q) is a measurement but serialised without a value key: %s", cell, b)
		}

		var out Value
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("unmarshalling %s: %v", b, err)
		}
		if out.State() != in.State() {
			t.Errorf("ParseValue(%q) round trip: state %v -> %v", cell, in.State(), out.State())
		}
		if out.Present() != in.Present() {
			t.Errorf("ParseValue(%q) round trip: Present %v -> %v", cell, in.Present(), out.Present())
		}
		wantN, wantOK := in.Float64()
		gotN, gotOK := out.Float64()
		if wantOK != gotOK || wantN != gotN {
			t.Errorf("ParseValue(%q) round trip: Float64 (%v,%v) -> (%v,%v)", cell, wantN, wantOK, gotN, gotOK)
		}
		if out.Raw() != in.Raw() {
			t.Errorf("ParseValue(%q) round trip: Raw %q -> %q", cell, in.Raw(), out.Raw())
		}
		if out.HadThousandsSeparator() != in.HadThousandsSeparator() {
			t.Errorf("ParseValue(%q) round trip: comma flag %v -> %v", cell, in.HadThousandsSeparator(), out.HadThousandsSeparator())
		}
	}

	// The zero value must survive as the zero value, not become a zero.
	var zero Value
	b, err := json.Marshal(zero)
	if err != nil {
		t.Fatalf("marshalling Value{}: %v", err)
	}
	var back Value
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshalling Value{} (%s): %v", b, err)
	}
	if back.State() != StateUnset || back.Present() {
		t.Errorf("Value{} round trip gave state %v present %v", back.State(), back.Present())
	}
}

// TestValueJSONRefusesMalformedInput checks the decoder fails loudly instead
// of downgrading. A state this build does not know, or a state/value pair
// that contradicts itself, must not decode into a plausible-looking cell.
func TestValueJSONRefusesMalformedInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
	}{
		{"unknown state name", `{"state":"probationary"}`},
		{"numeric with no value", `{"state":"numeric"}`},
		{"blank carrying a value", `{"state":"not_reported","value":0}`},
		{"delicensed carrying a value", `{"state":"delicensed","value":1292}`},
		{"unset carrying a value", `{"state":"unset","value":0}`},
		{"numeric state as a number", `{"state":1,"value":5}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var v Value
			if err := json.Unmarshal([]byte(tc.in), &v); err == nil {
				n, ok := v.Float64()
				t.Errorf("%s was accepted, giving state %v Float64 (%v,%v); want an error", tc.in, v.State(), n, ok)
			}
		})
	}
}

// TestPlantJSONRoundTrip serialises a real parsed workbook and requires every
// cell to come back identical. This is the end-to-end version of the {} bug:
// it fails if any nested Value, the block Status, or a capacity column loses
// its state on the way through the CLI's own output format.
func TestPlantJSONRoundTrip(t *testing.T) {
	w := parse(t, fullFY2324, "2023-24")

	b, err := json.Marshal(w.Plants)
	if err != nil {
		t.Fatalf("marshalling plants: %v", err)
	}
	var back []Plant
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshalling plants: %v", err)
	}
	if len(back) != len(w.Plants) {
		t.Fatalf("round trip returned %d plants, want %d", len(back), len(w.Plants))
	}

	var cells, statusRows int
	for i, want := range w.Plants {
		got := back[i]
		if got.Name != want.Name || got.SNo != want.SNo {
			t.Fatalf("plant %d identity changed: %q/%d -> %q/%d", i, want.Name, want.SNo, got.Name, got.SNo)
		}
		// The block status must survive as a NAME, not an iota position.
		if got.Status != want.Status {
			t.Errorf("%q: block status %v -> %v", want.Name, want.Status, got.Status)
		}
		if want.Status.Status() {
			statusRows++
		}
		pairs := []struct {
			label string
			a, b  Value
		}{
			{"InstalledCapacity", want.InstalledCapacity, got.InstalledCapacity},
			{"DependableCapacity", want.DependableCapacity, got.DependableCapacity},
			{"Total.Utilisation", want.Total.Utilisation, got.Total.Utilisation},
			{"Total.Generation", want.Total.Generation, got.Total.Generation},
		}
		for m := range want.Months {
			pairs = append(pairs,
				struct {
					label string
					a, b  Value
				}{want.Months[m].Label() + ".Utilisation", want.Months[m].Utilisation, got.Months[m].Utilisation},
				struct {
					label string
					a, b  Value
				}{want.Months[m].Label() + ".Generation", want.Months[m].Generation, got.Months[m].Generation},
			)
			if got.Months[m].Month != want.Months[m].Month {
				t.Errorf("%q month index %d changed: %v -> %v", want.Name, m, want.Months[m].Month, got.Months[m].Month)
			}
		}
		for _, c := range pairs {
			cells++
			if c.a.State() != c.b.State() {
				t.Errorf("%q %s: state %v -> %v", want.Name, c.label, c.a.State(), c.b.State())
			}
			wantN, wantOK := c.a.Float64()
			gotN, gotOK := c.b.Float64()
			if wantOK != gotOK || wantN != gotN {
				t.Errorf("%q %s: (%v,%v) -> (%v,%v)", want.Name, c.label, wantN, wantOK, gotN, gotOK)
			}
		}
	}
	// The two measured anchors from the original defect report.
	if statusRows != 13 {
		t.Errorf("status rows seen = %d, want 13", statusRows)
	}
	if cells != len(w.Plants)*(MonthlyCells+2) {
		t.Errorf("compared %d cells, want %d", cells, len(w.Plants)*(MonthlyCells+2))
	}

	// Tarbela's 3,478 MW is the number that came back as 0.00 before Value
	// had a MarshalJSON. Assert it by name, through the JSON.
	var found bool
	for _, p := range back {
		if p.Name != "Tarbela" && !bytes.Contains([]byte(p.Name), []byte("Tarbela")) {
			continue
		}
		if mw, ok := p.InstalledCapacity.Float64(); ok && mw >= 3400 {
			found = true
			if p.InstalledCapacity.State() != StateNumeric {
				t.Errorf("%q capacity state after round trip = %v", p.Name, p.InstalledCapacity.State())
			}
		}
	}
	if !found {
		t.Error("no Tarbela plant with a >=3400 MW capacity survived the JSON round trip")
	}
}

// TestCellStateJSONIsAName pins the wire format of the state vocabulary.
func TestCellStateJSONIsAName(t *testing.T) {
	for _, st := range []CellState{
		StateUnset, StateNumeric, StateNotReported, StateDelicensed,
		StateDecommissioned, StateExportToKElectric, StateUnknownText,
	} {
		b, err := json.Marshal(st)
		if err != nil {
			t.Fatalf("marshalling %v: %v", st, err)
		}
		if want := `"` + st.String() + `"`; string(b) != want {
			t.Errorf("json.Marshal(%v) = %s, want %s", st, b, want)
		}
		var back CellState
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatalf("unmarshalling %s: %v", b, err)
		}
		if back != st {
			t.Errorf("%s round trip -> %v", b, back)
		}
	}
	// A number must be refused: the iota positions already shifted once when
	// StateUnset was inserted, so a numeric wire value cannot be trusted.
	var st CellState
	if err := json.Unmarshal([]byte("1"), &st); err == nil {
		t.Errorf("a numeric cell state was accepted as %v; iota positions are not a wire format", st)
	}
	if err := json.Unmarshal([]byte(`"probationary"`), &st); err == nil {
		t.Error("an unknown cell state name was accepted")
	}
	// Every name must be recoverable.
	if got, ok := CellStateByName("delicensed"); !ok || got != StateDelicensed {
		t.Errorf(`CellStateByName("delicensed") = (%v,%v)`, got, ok)
	}
	if got, ok := CellStateByName("nonsense"); ok || got != StateUnset {
		t.Errorf(`CellStateByName("nonsense") = (%v,%v), want (unset,false)`, got, ok)
	}
}
