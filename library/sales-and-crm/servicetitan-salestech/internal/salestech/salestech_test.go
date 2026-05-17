package salestech

import (
	"strings"
	"testing"
	"time"
)

// ----- match tests ----------------------------------------------------

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"":                               "",
		"Hello World":                    "hello world",
		"Well-Pump #2":                   "well pump 2",
		"a   b\tc":                       "a b c",
		"1.5 HP submersible-pump motor!": "1 5 hp submersible pump motor",
	}
	for in, want := range cases {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSimilarity(t *testing.T) {
	tests := []struct {
		a, b string
		min  float64
	}{
		{"submersible pump", "submersible pump motor", 0.6},
		{"well pump 1hp", "1hp well pump", 0.4},
		{"foo", "completely different bar", 0.0},
	}
	for _, tc := range tests {
		s := similarity(tc.a, tc.b)
		if s < tc.min {
			t.Errorf("similarity(%q,%q) = %.3f, want >= %.3f", tc.a, tc.b, s, tc.min)
		}
	}
	if similarity("", "anything") != 0 {
		t.Errorf("empty query should score 0")
	}
}

func TestTokenCoverage(t *testing.T) {
	if c := tokenCoverage("well pump", "1 hp submersible well pump motor"); c < 0.99 {
		t.Errorf("expected full coverage, got %.3f", c)
	}
	if c := tokenCoverage("blue widget", "submersible well pump"); c > 0 {
		t.Errorf("expected zero coverage on no matches, got %.3f", c)
	}
}

// ----- timestamp / status parse --------------------------------------

func TestParseTimestamp(t *testing.T) {
	good := []string{
		"2026-05-17T18:30:00Z",
		"2026-05-17T18:30:00.123Z",
		"2026-05-17T18:30:00",
	}
	for _, s := range good {
		if _, ok := parseTimestamp(s); !ok {
			t.Errorf("parseTimestamp(%q) should succeed", s)
		}
	}
	if _, ok := parseTimestamp(""); ok {
		t.Errorf("empty timestamp should fail")
	}
}

func TestEstimateStatusUnmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`"Open"`, "Open"},
		{`"Sold"`, "Sold"},
		{`{"value":"Dismissed","name":"Dismissed"}`, "Dismissed"},
		{`{"name":"Open"}`, "Open"},
		{`null`, ""},
	}
	for _, tc := range cases {
		var es EstimateStatus
		if err := es.UnmarshalJSON([]byte(tc.in)); err != nil {
			t.Fatalf("unmarshal(%q): %v", tc.in, err)
		}
		if got := es.String(); got != tc.want {
			t.Errorf("status(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ----- percentile -----------------------------------------------------

func TestPercentile(t *testing.T) {
	s := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	cases := []struct {
		p, want float64
	}{
		{0, 1},
		{0.5, 5.5},
		{0.9, 9.1},
		{1, 10},
	}
	for _, tc := range cases {
		got := percentile(s, tc.p)
		if absFloat(got-tc.want) > 0.001 {
			t.Errorf("percentile(%.2f) = %.3f, want %.3f", tc.p, got, tc.want)
		}
	}
	if percentile(nil, 0.5) != 0 {
		t.Errorf("nil percentile should be 0")
	}
	if percentile([]float64{42}, 0.7) != 42 {
		t.Errorf("single-value percentile should be the value")
	}
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// ----- CSV import ----------------------------------------------------

func TestImportCSV_HappyPath(t *testing.T) {
	in := strings.NewReader(`estimate_key,estimate_name,job_id,sku_id,qty,unit_rate,description
Q1,New well pump install,4471,9001,1,750.00,1 HP submersible pump
Q1,New well pump install,4471,9002,1,150.00,Install labor
Q2,Pressure tank replace,4472,9010,1,420.50,30-gallon pressure tank
`)
	rows, err := ImportCSV(in)
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 grouped rows, got %d", len(rows))
	}
	if rows[0].JobID != 4471 {
		t.Errorf("Q1 job_id = %d, want 4471", rows[0].JobID)
	}
	if len(rows[0].Items) != 2 {
		t.Errorf("Q1 items = %d, want 2", len(rows[0].Items))
	}
	if rows[0].Items[1].UnitRate != 150.00 {
		t.Errorf("Q1 item2 unit_rate = %f, want 150.00", rows[0].Items[1].UnitRate)
	}
	if rows[0].Errors != nil {
		t.Errorf("Q1 should be error-free, got: %v", rows[0].Errors)
	}
	// Q1 has job_id but Q2 has none — Q2 should warn (project_id also blank).
	if len(rows[1].Warnings) == 0 {
		// Q2 has job_id=4472, so it should NOT warn.
		if rows[1].JobID != 4472 {
			t.Errorf("Q2 expected job_id 4472, got %d", rows[1].JobID)
		}
	}
}

func TestImportCSV_MissingRequiredColumn(t *testing.T) {
	in := strings.NewReader(`estimate_key,estimate_name,qty,unit_rate
Q1,no sku,1,750
`)
	if _, err := ImportCSV(in); err == nil {
		t.Errorf("expected error for missing sku_id column, got nil")
	}
}

func TestImportCSV_ValidationErrors(t *testing.T) {
	in := strings.NewReader(`estimate_key,estimate_name,sku_id,qty,unit_rate
Q1,bad qty,9001,abc,750
Q1,bad qty,9002,0,800
Q2,missing rate,9010,1,
`)
	rows, err := ImportCSV(in)
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if len(rows[0].Errors) == 0 {
		t.Errorf("Q1 should have qty errors")
	}
	if len(rows[1].Errors) == 0 {
		t.Errorf("Q2 should have unit_rate errors")
	}
}

func TestCreateRequestPayload(t *testing.T) {
	row := CSVImportRow{
		Name:        "Test",
		Summary:     "summary",
		JobID:       4471,
		Tax:         42.0,
		SoldByID:    100,
		IsRecommend: true,
		Items: []CSVImportItem{
			{SkuID: 9001, Qty: 1, UnitRate: 750.0, Description: "pump"},
		},
	}
	p := row.CreateRequestPayload()
	if p["jobId"] != int64(4471) {
		t.Errorf("jobId = %v, want 4471", p["jobId"])
	}
	if p["soldBy"] != int64(100) {
		t.Errorf("soldBy = %v, want 100", p["soldBy"])
	}
	items, ok := p["items"].([]map[string]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items missing or wrong shape: %v", p["items"])
	}
	if items[0]["skuId"] != int64(9001) {
		t.Errorf("item.skuId = %v, want 9001", items[0]["skuId"])
	}
}

// ----- followup helpers -----------------------------------------------

func TestAddFollowUp_Validation(t *testing.T) {
	// We can't easily mock a real *store.Store in a pure test; assert input
	// validation that doesn't touch the store.
	if _, err := AddFollowUp(nil, 1, "", "", ""); err == nil {
		t.Errorf("empty note should error")
	}
	if _, err := AddFollowUp(nil, 1, "ok", "not-a-date", ""); err == nil {
		t.Errorf("bad remind-on format should error")
	}
}

// ----- truncate -------------------------------------------------------

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short unchanged: got %q", got)
	}
	if got := truncate("hello world", 5); got != "hello…" {
		t.Errorf("truncate long: got %q", got)
	}
	if got := truncate("", 10); got != "" {
		t.Errorf("truncate empty: got %q", got)
	}
}

// ----- daysBetween + Estimate.Total ----------------------------------

func TestDaysBetween(t *testing.T) {
	t1 := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	if d := daysBetween(t1, t2); d != 7 {
		t.Errorf("daysBetween = %d, want 7", d)
	}
}

func TestEstimateTotal(t *testing.T) {
	e := Estimate{Subtotal: 1000, Tax: 87.50}
	if got := e.Total(); got != 1087.50 {
		t.Errorf("Total = %.2f, want 1087.50", got)
	}
}

func TestEstimateSoldByID(t *testing.T) {
	id := int64(42)
	e := Estimate{SoldBy: &id}
	if got := e.SoldByID(); got != 42 {
		t.Errorf("SoldByID = %d, want 42", got)
	}
	e2 := Estimate{}
	if got := e2.SoldByID(); got != 0 {
		t.Errorf("SoldByID nil should be 0, got %d", got)
	}
}

// ----- earliestSoldAt -------------------------------------------------

func TestEarliestSoldAt(t *testing.T) {
	changes := []StatusChange{
		{EstimateID: 1, To: "Sold", ChangedAt: "2026-05-17T12:00:00Z"},
		{EstimateID: 1, To: "Unsold", ChangedAt: "2026-05-18T12:00:00Z"},
		{EstimateID: 1, To: "Sold", ChangedAt: "2026-05-19T12:00:00Z"},
		{EstimateID: 2, To: "Dismissed", ChangedAt: "2026-05-17T12:00:00Z"},
	}
	out := earliestSoldAt(changes)
	if t1, ok := out[1]; !ok || t1.Day() != 17 {
		t.Errorf("estimate 1 earliest Sold should be 2026-05-17, got %v ok=%v", t1, ok)
	}
	if _, ok := out[2]; ok {
		t.Errorf("estimate 2 never sold, should not appear")
	}
}
