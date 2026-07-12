// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.

package indiapassivefunds

import (
	"encoding/json"
	"testing"
)

func TestListEnvelopeDecode(t *testing.T) {
	env := &ListEnvelope{
		Columns: []ColumnMeta{
			{Field: "f_29", DisplayName: "Scheme Name"},
			{Field: "f_36", DisplayName: "NAV"},
		},
	}
	row := map[string]json.RawMessage{
		"f_29": json.RawMessage(`"HDFC Nifty 50 Index Fund"`),
		"f_36": json.RawMessage(`123.45`),
		"f_99": json.RawMessage(`"unmapped"`),
	}

	got := env.Decode(row)

	if got["Scheme Name"] != "HDFC Nifty 50 Index Fund" {
		t.Errorf("Scheme Name = %v, want HDFC Nifty 50 Index Fund", got["Scheme Name"])
	}
	if got["NAV"] != 123.45 {
		t.Errorf("NAV = %v, want 123.45", got["NAV"])
	}
	// A field with no column-metadata mapping keeps its raw field code as the key.
	if got["f_99"] != "unmapped" {
		t.Errorf("f_99 = %v, want unmapped (field code preserved when unmapped)", got["f_99"])
	}
}

func TestParseListEnvelope(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		raw := json.RawMessage(`{"status":true,"message":"Sucess","response":{"pagingInfo":{"totalRecords":1},"columns":[],"data":[]}}`)
		env, err := parseListEnvelope(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env.PagingInfo.TotalRecords != 1 {
			t.Errorf("TotalRecords = %d, want 1", env.PagingInfo.TotalRecords)
		}
	})
	t.Run("upstream failure", func(t *testing.T) {
		raw := json.RawMessage(`{"status":false,"message":"Requested ranking doesnt exist"}`)
		_, err := parseListEnvelope(raw)
		if err == nil {
			t.Fatal("expected error for status:false, got nil")
		}
	})
}

func TestFindUnderlyingIndexValue(t *testing.T) {
	taxonomy := &ScreenerFilterTaxonomy{
		UnderlyingIndex: []FilterOption{
			{Text: "Nifty 50 TRI", Value: float64(320)},
			{Text: "Nifty 500 TRI", Value: float64(321)},
			{Text: "Nifty 50 Equal Weight TRI", Value: float64(322)},
		},
	}

	t.Run("exact match prefers shortest, not greedy substring", func(t *testing.T) {
		value, matched, found := FindUnderlyingIndexValue(taxonomy, "NIFTY 50")
		if !found {
			t.Fatal("expected a match for NIFTY 50")
		}
		if value != float64(320) || matched != "Nifty 50 TRI" {
			t.Errorf("got value=%v matched=%q, want value=320 matched=\"Nifty 50 TRI\" (not NIFTY 500 or Equal Weight)", value, matched)
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, _, found := FindUnderlyingIndexValue(taxonomy, "BSE SENSEX")
		if found {
			t.Error("expected no match for an index absent from the taxonomy")
		}
	})

	// niftyindices and indiapassivefunds disagree on whether a space belongs
	// between a name and a trailing number: niftyindices' index_live/index
	// list gives "NIFTY MIDSMALLCAP400 MOMENTUM QUALITY 100", but
	// indiapassivefunds' own taxonomy spells the same index "Nifty
	// MidSmallcap 400 Momentum Quality 100 TRI" (space before "400"). A
	// genuinely matching index must not be reported as absent just because
	// the two sites format the number boundary differently.
	t.Run("letter-digit boundary spacing differs between sites", func(t *testing.T) {
		taxonomy := &ScreenerFilterTaxonomy{
			UnderlyingIndex: []FilterOption{
				{Text: "Nifty MidSmallcap 400 Momentum Quality 100 TRI", Value: float64(595)},
			},
		}
		value, matched, found := FindUnderlyingIndexValue(taxonomy, "NIFTY MIDSMALLCAP400 MOMENTUM QUALITY 100")
		if !found {
			t.Fatal("expected a match despite the space-before-400 formatting difference")
		}
		if value != float64(595) || matched != "Nifty MidSmallcap 400 Momentum Quality 100 TRI" {
			t.Errorf("got value=%v matched=%q, want value=595 matched=%q", value, matched, "Nifty MidSmallcap 400 Momentum Quality 100 TRI")
		}
	})
}

func TestNormalizeIndexName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"letter then digit, no space", "MIDSMALLCAP400", "midsmallcap 400"},
		{"already spaced", "MidSmallcap 400", "midsmallcap 400"},
		{"digit then letter, no space", "200ALPHA", "200 alpha"},
		{"strips tri suffix", "Nifty 50 TRI", "nifty 50"},
		{"no digits, unaffected", "NIFTY BANK", "nifty bank"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeIndexName(tc.in); got != tc.want {
				t.Errorf("normalizeIndexName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFindAMCValue(t *testing.T) {
	taxonomy := &ScreenerFilterTaxonomy{
		AMC: []FilterOption{
			{Text: "HDFC Mutual Fund", Value: float64(7)},
			{Text: "ICICI Prudential Mutual Fund", Value: float64(12)},
		},
	}

	t.Run("substring match", func(t *testing.T) {
		value, matched, found := FindAMCValue(taxonomy, "HDFC")
		if !found || value != float64(7) || matched != "HDFC Mutual Fund" {
			t.Errorf("got value=%v matched=%q found=%v, want value=7 matched=\"HDFC Mutual Fund\" found=true", value, matched, found)
		}
	})

	t.Run("empty query", func(t *testing.T) {
		_, _, found := FindAMCValue(taxonomy, "")
		if found {
			t.Error("expected no match for an empty AMC query")
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, _, found := FindAMCValue(taxonomy, "Vanguard")
		if found {
			t.Error("expected no match for an AMC absent from the taxonomy")
		}
	})
}

func TestFirstDataRow(t *testing.T) {
	raw := json.RawMessage(`{"columns":[],"data":[{"f_01":"a"},{"f_01":"b"}]}`)
	row := firstDataRow(raw)
	if row == nil {
		t.Fatal("expected a row, got nil")
	}
	if string(row["f_01"]) != `"a"` {
		t.Errorf("firstDataRow returned row %v, want the first data row", row)
	}

	if firstDataRow(nil) != nil {
		t.Error("firstDataRow(nil) should return nil")
	}
	if firstDataRow(json.RawMessage(`{"data":[]}`)) != nil {
		t.Error("firstDataRow with empty data should return nil")
	}
}

func TestFieldByDisplayName(t *testing.T) {
	section := json.RawMessage(`{"columns":[{"field":"f_01","displayName":"Scheme Name"}],"data":[]}`)
	row := map[string]json.RawMessage{"f_01": json.RawMessage(`"HDFC Nifty 50 Index Fund"`)}

	got := fieldByDisplayName(row, section, "scheme name")
	if got != "HDFC Nifty 50 Index Fund" {
		t.Errorf("fieldByDisplayName = %q, want HDFC Nifty 50 Index Fund (case-insensitive match)", got)
	}

	if got := fieldByDisplayName(row, section, "nonexistent field"); got != "" {
		t.Errorf("fieldByDisplayName for unmatched displayName = %q, want empty", got)
	}
}

func TestRawToString(t *testing.T) {
	cases := []struct {
		name string
		in   json.RawMessage
		want string
	}{
		{"string", json.RawMessage(`"hello"`), "hello"},
		{"float", json.RawMessage(`123.45`), "123.45"},
		{"integer-valued float", json.RawMessage(`100`), "100"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rawToString(tc.in); got != tc.want {
				t.Errorf("rawToString(%s) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRawToFloat(t *testing.T) {
	f, ok := rawToFloat(json.RawMessage(`42.5`))
	if !ok || f != 42.5 {
		t.Errorf("rawToFloat(42.5) = (%v, %v), want (42.5, true)", f, ok)
	}
	if _, ok := rawToFloat(json.RawMessage(`"not a number"`)); ok {
		t.Error("rawToFloat on a non-numeric value should return ok=false")
	}
}

// TestDeepFindBenchmarkIndex covers the specific reason this function
// exists: funddescription nests a "section1" sub-object mid-array with
// its own columns/data, so a flat top-level scan would miss the
// benchmark field entirely.
func TestDeepFindBenchmarkIndex(t *testing.T) {
	raw := json.RawMessage(`{
		"data": [
			{"someOtherKey": "value"},
			{"section1": {
				"columns": [{"field": "f_05", "displayName": "Benchmark Index"}],
				"data": [{"f_05": "Nifty 50 TRI"}]
			}}
		]
	}`)
	got := deepFindBenchmarkIndex(raw)
	if got != "Nifty 50 TRI" {
		t.Errorf("deepFindBenchmarkIndex = %q, want Nifty 50 TRI", got)
	}

	if got := deepFindBenchmarkIndex(nil); got != "" {
		t.Errorf("deepFindBenchmarkIndex(nil) = %q, want empty", got)
	}
}

func TestLabelValueTriplets(t *testing.T) {
	raw := json.RawMessage(`{"data":[
		{"f_01":"Expense Ratio","f_02":0.03,"f_03":"2026-07-31"},
		{"f_01":"AUM","f_02":"42099.54"}
	]}`)
	got := labelValueTriplets(raw)

	er, ok := got["Expense Ratio"]
	if !ok || er.value != "0.03" || er.asOf != "2026-07-31" {
		t.Errorf("labelValueTriplets[Expense Ratio] = %+v, want value=0.03 asOf=2026-07-31", er)
	}
	aum, ok := got["AUM"]
	if !ok || aum.value != "42099.54" || aum.asOf != "" {
		t.Errorf("labelValueTriplets[AUM] = %+v, want value=42099.54 asOf=\"\" (no f_03)", aum)
	}
}

// TestLatestRatiosRow covers the "0 = undisclosed" distinction this
// function is responsible for: a future reporting period whose fields
// are all null must not win over an older period with real data.
func TestLatestRatiosRow(t *testing.T) {
	raw := json.RawMessage(`{"data":[
		{"f_0":"2026-06-30","f_1":0.03},
		{"f_0":"2026-07-31","f_1":null},
		{"f_0":"2026-05-31","f_1":0.04}
	]}`)
	row, date := latestRatiosRow(raw)
	if date != "2026-06-30" {
		t.Errorf("latestRatiosRow date = %q, want 2026-06-30 (the most recent row with real data, skipping the null-only 2026-07-31 row)", date)
	}
	if row == nil || string(row["f_1"]) != "0.03" {
		t.Errorf("latestRatiosRow row = %v, want f_1=0.03", row)
	}
}

func TestNamedPercentRows(t *testing.T) {
	raw := json.RawMessage(`{
		"columns": [
			{"field": "f_01", "displayName": "Sector Name"},
			{"field": "f_02", "displayName": "Weight Percent"}
		],
		"data": [
			{"f_01": "Banks", "f_02": 30.15},
			{"f_01": "IT", "f_02": 7.38}
		]
	}`)
	got := namedPercentRows(raw, "name", "percent")
	if len(got) != 2 {
		t.Fatalf("namedPercentRows returned %d rows, want 2", len(got))
	}
	if got[0].Name != "Banks" || got[0].Percent != 30.15 {
		t.Errorf("row[0] = %+v, want {Banks 30.15}", got[0])
	}

	if got := namedPercentRows(raw, "nonexistent", "alsomissing"); got != nil {
		t.Errorf("namedPercentRows with unmatched columns = %v, want nil", got)
	}
}

func TestParseSimilarFunds(t *testing.T) {
	raw := json.RawMessage(`{
		"columns": [
			{"field": "f_01", "displayName": "SchemeName"},
			{"field": "f_02", "displayName": "CategoryName"},
			{"field": "f_03", "displayName": "scheme_id"}
		],
		"data": [
			{"f_01": "UTI Nifty 50 Index Fund", "f_02": "Index Fund", "f_03": "1272"}
		]
	}`)
	got := parseSimilarFunds(raw)
	if len(got) != 1 {
		t.Fatalf("parseSimilarFunds returned %d funds, want 1", len(got))
	}
	want := SimilarFund{SchemeID: "1272", SchemeName: "UTI Nifty 50 Index Fund", Category: "Index Fund"}
	if got[0] != want {
		t.Errorf("parseSimilarFunds[0] = %+v, want %+v", got[0], want)
	}
}

// TestMergeScreenerEnvelope covers the reason ScreenAll exists: three
// screener templates share scheme_id but each carry different columns
// (Overview/Returns/Risk), so a fund's row must accumulate fields across
// templates instead of the later template clobbering the earlier one, and
// a scheme_id absent from an earlier template but present in a later one
// must still be appended (not silently dropped).
func TestMergeScreenerEnvelope(t *testing.T) {
	overview := json.RawMessage(`{
		"columns": [
			{"field": "f_42", "displayName": "Fund Name"},
			{"field": "f_45", "displayName": "Tracking Error (%)"},
			{"field": "f_114", "displayName": "scheme_id"}
		],
		"data": [
			{"f_42": "ICICI Prudential BSE Sensex ETF", "f_45": 0.02, "f_114": 1537}
		]
	}`)
	returns := json.RawMessage(`{
		"columns": [
			{"field": "f_54", "displayName": "1 Year %"},
			{"field": "f_114", "displayName": "scheme_id"}
		],
		"data": [
			{"f_54": -5.8, "f_114": 1537},
			{"f_54": 12.3, "f_114": 9999}
		]
	}`)

	var overviewEnv, returnsEnv ListEnvelope
	if err := json.Unmarshal(overview, &overviewEnv); err != nil {
		t.Fatalf("unmarshaling overview fixture: %v", err)
	}
	if err := json.Unmarshal(returns, &returnsEnv); err != nil {
		t.Fatalf("unmarshaling returns fixture: %v", err)
	}

	merged := make(map[string]map[string]any)
	var order []string
	mergeScreenerEnvelope(&overviewEnv, merged, &order)
	mergeScreenerEnvelope(&returnsEnv, merged, &order)

	if len(order) != 2 {
		t.Fatalf("order = %v, want 2 entries (1537 then 9999)", order)
	}
	if order[0] != "1537" || order[1] != "9999" {
		t.Errorf("order = %v, want [1537 9999] (first-seen position preserved)", order)
	}

	row1537 := merged["1537"]
	if row1537["Fund Name"] != "ICICI Prudential BSE Sensex ETF" {
		t.Errorf("scheme 1537 Fund Name = %v, want overview's value preserved after returns merge", row1537["Fund Name"])
	}
	if row1537["Tracking Error (%)"] != 0.02 {
		t.Errorf("scheme 1537 Tracking Error (%%) = %v, want 0.02 (overview field kept)", row1537["Tracking Error (%)"])
	}
	if row1537["1 Year %"] != -5.8 {
		t.Errorf("scheme 1537 1 Year %% = %v, want -5.8 (returns field added, not dropped)", row1537["1 Year %"])
	}

	row9999 := merged["9999"]
	if row9999 == nil {
		t.Fatal("scheme 9999 (present only in returns template) was dropped instead of appended")
	}
	if row9999["1 Year %"] != 12.3 {
		t.Errorf("scheme 9999 1 Year %% = %v, want 12.3", row9999["1 Year %"])
	}
	if _, hasFundName := row9999["Fund Name"]; hasFundName {
		t.Error("scheme 9999 should have no Fund Name field (never appeared in overview template)")
	}
}
