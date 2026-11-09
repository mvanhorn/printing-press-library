package cli

import (
	"encoding/json"
	"testing"
)

// SEC's companyfacts endpoint does not always return `cik` as a number —
// some filers return a zero-padded string (issue #2445).
func TestCompanyFactsCIKAcceptsStringAndNumber(t *testing.T) {
	var cf companyFacts
	if err := json.Unmarshal([]byte(`{"cik":"0001181412","entityName":"SpaceX"}`), &cf); err != nil {
		t.Fatalf("string cik: %v", err)
	}
	if cf.CIK != 1181412 {
		t.Fatalf("string cik = %d, want 1181412", cf.CIK)
	}

	var cfNum companyFacts
	if err := json.Unmarshal([]byte(`{"cik":320193,"entityName":"Apple"}`), &cfNum); err != nil {
		t.Fatalf("number cik: %v", err)
	}
	if cfNum.CIK != 320193 {
		t.Fatalf("number cik = %d, want 320193", cfNum.CIK)
	}

	var cfNull companyFacts
	if err := json.Unmarshal([]byte(`{"cik":null}`), &cfNull); err != nil {
		t.Fatalf("null cik: %v", err)
	}
	if cfNull.CIK != 0 {
		t.Fatalf("null cik = %d, want 0", cfNull.CIK)
	}

	var cfBad companyFacts
	if err := json.Unmarshal([]byte(`{"cik":"abc"}`), &cfBad); err == nil {
		t.Fatal("non-numeric cik: expected error, got nil")
	}

	// Empty and "null" quoted strings are nonnumeric, not JSON null.
	var cfEmpty companyFacts
	if err := json.Unmarshal([]byte(`{"cik":""}`), &cfEmpty); err == nil {
		t.Fatal(`cik "": expected error, got nil`)
	}
	var cfNullStr companyFacts
	if err := json.Unmarshal([]byte(`{"cik":"null"}`), &cfNullStr); err == nil {
		t.Fatal(`cik "null": expected error, got nil`)
	}
}

func TestFrameUnitCIKAcceptsString(t *testing.T) {
	var fu frameUnit
	body := `{"taxonomy":"us-gaap","tag":"Assets","uom":"USD","label":"Assets","description":"d","pts":1,"data":[{"accn":"a","cik":"0001181412","entityName":"SpaceX","loc":"X","end":"2025-01-01","val":1.5}]}`
	if err := json.Unmarshal([]byte(body), &fu); err != nil {
		t.Fatalf("frameUnit: %v", err)
	}
	if len(fu.Data) != 1 || fu.Data[0].CIK != 1181412 {
		t.Fatalf("frame CIK = %v, want [1181412]", fu.Data)
	}
}

func TestCompanyConceptRespCIKAcceptsString(t *testing.T) {
	var r companyConceptResp
	if err := json.Unmarshal([]byte(`{"cik":"0001181412","entityName":"SpaceX","taxonomy":"us-gaap","tag":"Assets"}`), &r); err != nil {
		t.Fatalf("companyConceptResp: %v", err)
	}
	if r.CIK != 1181412 {
		t.Fatalf("concept CIK = %d, want 1181412", r.CIK)
	}
}
