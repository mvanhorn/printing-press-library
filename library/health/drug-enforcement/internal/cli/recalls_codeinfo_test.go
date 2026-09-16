package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestRecallRecordCodeInfoRoundTrip guards the path the other recall tests
// skip: every one of them builds a recallRecord by hand, so a wrong or missing
// `json:"code_info"` tag leaves them all green while every real record renders
// Lots/Expiry as a dash. The generated struct originally omitted the field, so
// this is the regression a regeneration would reintroduce.
//
// The test decodes a raw openFDA payload through enforcementEnvelope, renders
// it with printRecallRecord, and asserts that every token of code_info reaches
// the output, wrapped under the value column.
func TestRecallRecordCodeInfoRoundTrip(t *testing.T) {
	// Long enough to force wrapping at the shipped 80/16 geometry.
	const codeInfo = "Lot: a) 09JA2530, 31JA2507, expires: 04/30/2027; b) Lot: 09DE2412, " +
		"09JA2528, 29JA2511, expires: 04/30/2027; c) Lot: 17FE2533, expires: 05/31/2027"

	payload := `{
	  "meta": {"results": {"total": 1}},
	  "results": [
	    {
	      "recall_number": "D-1234-2026",
	      "classification": "Class II",
	      "status": "Ongoing",
	      "recalling_firm": "Example Pharma Inc.",
	      "reason_for_recall": "Subpotent drug product.",
	      "product_description": "Ibuprofen tablets, 200 mg, 100-count bottle.",
	      "code_info": "` + codeInfo + `",
	      "recall_initiation_date": "20260115"
	    }
	  ]
	}`

	var env enforcementEnvelope
	if err := json.Unmarshal([]byte(payload), &env); err != nil {
		t.Fatalf("decode enforcement payload: %v", err)
	}
	if len(env.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(env.Results))
	}
	// Guards the struct tag itself: without it every assertion below would be
	// checked against an empty value.
	if env.Results[0].CodeInfo != codeInfo {
		t.Fatalf("CodeInfo did not decode from code_info: got %q", env.Results[0].CodeInfo)
	}

	var buf bytes.Buffer
	printRecallRecord(&buf, env.Results[0])
	out := buf.String()

	lines := strings.Split(out, "\n")
	first := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "  Lots/Expiry:") {
			first = i
			break
		}
	}
	if first == -1 {
		t.Fatalf("no Lots/Expiry: line in output:\n%s", out)
	}
	if strings.TrimSpace(strings.TrimPrefix(lines[first], "  Lots/Expiry:")) == "-" {
		t.Fatalf("Lots/Expiry rendered the empty placeholder:\n%s", out)
	}

	// Every token of the source value must survive: a truncated lot number is
	// wrong rather than merely short, so nothing may be dropped or split.
	for _, tok := range strings.Fields(codeInfo) {
		if !strings.Contains(out, tok) {
			t.Errorf("token %q from code_info missing from output:\n%s", tok, out)
		}
	}

	// Continuation lines belong under the value column, not at the left
	// margin, and must stop before the Reason row that follows.
	cont := 0
	for _, l := range lines[first+1:] {
		if strings.HasPrefix(l, "  Reason:") {
			break
		}
		if strings.TrimSpace(l) == "" {
			continue
		}
		cont++
		if !strings.HasPrefix(l, strings.Repeat(" ", recallLabelWidth)) {
			t.Errorf("continuation line not indented to column %d: %q", recallLabelWidth, l)
		}
		if len(l) > recallLabelWidth && l[recallLabelWidth] == ' ' {
			t.Errorf("continuation line over-indented past the value column: %q", l)
		}
	}
	if cont == 0 {
		t.Errorf("fixture did not wrap, so indentation was not tested:\n%s", out)
	}
}
