package pricebook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"F-1921/000", "f 1921 000"},
		{"  PEX Pipe 1\" 20' (TS)  ", "pex pipe 1 20 ts"},
		{"MPCLR30TE1", "mpclr30te1"},
		{"", ""},
		{"___", ""},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeTight(t *testing.T) {
	cases := []struct{ in, want string }{
		{"F-1921-000", "f1921000"},
		{"MPCLR30T-E1", "mpclr30te1"},
		{"  abc 123  ", "abc123"},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeTight(c.in); got != c.want {
			t.Errorf("NormalizeTight(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTokens(t *testing.T) {
	got := Tokens("PEX Pipe pipe 1 (TS)")
	want := map[string]bool{"pex": true, "pipe": true, "1": true, "ts": true}
	if len(got) != len(want) {
		t.Fatalf("Tokens returned %v, want %d unique tokens", got, len(want))
	}
	for _, tok := range got {
		if !want[tok] {
			t.Errorf("Tokens produced unexpected token %q", tok)
		}
	}
}

func TestJaccard(t *testing.T) {
	cases := []struct {
		a, b string
		want float64
	}{
		{"", "", 1},
		{"abc", "", 0},
		{"red pump", "red pump", 1},
		{"red pump", "blue pump", 1.0 / 3.0},
	}
	for _, c := range cases {
		if got := Jaccard(c.a, c.b); !approx(got, c.want) {
			t.Errorf("Jaccard(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"kitten", "sitting", 3},
		{"flaw", "lawn", 2},
	}
	for _, c := range cases {
		if got := Levenshtein(c.a, c.b); got != c.want {
			t.Errorf("Levenshtein(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestLevenshteinRatio(t *testing.T) {
	if r := LevenshteinRatio("", ""); r != 1 {
		t.Errorf("LevenshteinRatio empty = %v, want 1", r)
	}
	if r := LevenshteinRatio("abcd", "abcd"); r != 1 {
		t.Errorf("LevenshteinRatio identical = %v, want 1", r)
	}
	if r := LevenshteinRatio("abcd", "abce"); !approx(r, 0.75) {
		t.Errorf("LevenshteinRatio one-edit-of-4 = %v, want 0.75", r)
	}
}

func TestSimilarity(t *testing.T) {
	// Substring containment boost.
	if s := Similarity("master water", "softener master water tank"); s < 0.85 {
		t.Errorf("Similarity substring case = %v, want >= 0.85", s)
	}
	// Identical strings.
	if s := Similarity("F1921000", "F1921000"); s != 1 {
		t.Errorf("Similarity identical = %v, want 1", s)
	}
	// Totally unrelated.
	if s := Similarity("submersible pump", "vinyl tape"); s > 0.4 {
		t.Errorf("Similarity unrelated = %v, want <= 0.4", s)
	}
	// Empty cases.
	if s := Similarity("", ""); s != 1 {
		t.Errorf("Similarity empty/empty = %v, want 1", s)
	}
	if s := Similarity("abc", ""); s != 0 {
		t.Errorf("Similarity abc/empty = %v, want 0", s)
	}
}

func TestTokenCoverage(t *testing.T) {
	// Every query token appears in the (longer) target — full coverage.
	if c := TokenCoverage("Softener 30k", "30k Grain Softener, Master Water"); !approx(c, 1.0) {
		t.Errorf("TokenCoverage full = %v, want 1.0", c)
	}
	// Half the query tokens present.
	if c := TokenCoverage("submersible pump", "submersible motor"); !approx(c, 0.5) {
		t.Errorf("TokenCoverage half = %v, want 0.5", c)
	}
	// No query tokens present.
	if c := TokenCoverage("vinyl tape", "submersible pump motor"); c != 0 {
		t.Errorf("TokenCoverage none = %v, want 0", c)
	}
	// Empty query covers nothing.
	if c := TokenCoverage("", "anything"); c != 0 {
		t.Errorf("TokenCoverage empty query = %v, want 0", c)
	}
}

func TestPartMatch(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"F1921000", "F-1921-000", true},
		{"MPCLR30TE1", "mpclr30t e1", true},
		{"", "F1921000", false},
		{"F1921000", "", false},
		{"F1921000", "F1921001", false},
	}
	for _, c := range cases {
		if got := PartMatch(c.a, c.b); got != c.want {
			t.Errorf("PartMatch(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestEvalTier(t *testing.T) {
	ladder := []MarkupTier{
		{ID: 1, From: 0, To: 100, Percent: 120},
		{ID: 2, From: 100.01, To: 1000, Percent: 80},
	}
	if tier, ok := EvalTier(ladder, 50); !ok || tier.ID != 1 {
		t.Errorf("EvalTier(50) = %+v ok=%v, want tier 1", tier, ok)
	}
	if tier, ok := EvalTier(ladder, 500); !ok || tier.ID != 2 {
		t.Errorf("EvalTier(500) = %+v ok=%v, want tier 2", tier, ok)
	}
	if _, ok := EvalTier(ladder, 5000); ok {
		t.Errorf("EvalTier(5000) ok=true, want false (above ladder)")
	}
	if _, ok := EvalTier(nil, 50); ok {
		t.Errorf("EvalTier on empty ladder ok=true, want false")
	}
}

func TestExpectedPrice(t *testing.T) {
	ladder := []MarkupTier{{ID: 1, From: 0, To: 1e9, Percent: 100}}
	price, tier, ok := ExpectedPrice(ladder, 1.683)
	if !ok || tier.ID != 1 {
		t.Fatalf("ExpectedPrice ok=%v tier=%+v, want ok tier 1", ok, tier)
	}
	if !approx(price, 3.37) { // 1.683 * 2 = 3.366 -> round2 3.37
		t.Errorf("ExpectedPrice(1.683 @ 100%%) = %v, want 3.37", price)
	}
	if _, _, ok := ExpectedPrice(ladder, -5); ok {
		t.Errorf("ExpectedPrice(-5) ok=true, want false")
	}
}

func TestActualMarkupPercent(t *testing.T) {
	if pct, ok := ActualMarkupPercent(10, 20); !ok || !approx(pct, 100) {
		t.Errorf("ActualMarkupPercent(10,20) = %v ok=%v, want 100 true", pct, ok)
	}
	if _, ok := ActualMarkupPercent(0, 20); ok {
		t.Errorf("ActualMarkupPercent(0,20) ok=true, want false (zero cost)")
	}
	if _, ok := ActualMarkupPercent(-1, 20); ok {
		t.Errorf("ActualMarkupPercent(-1,20) ok=true, want false (negative cost)")
	}
}

func TestBulkPlan(t *testing.T) {
	c1, p1 := 1.50, 3.00
	p2 := 9.99
	changes := []BulkChange{
		{Kind: KindMaterial, ID: 100, Cost: &c1, Price: &p1},
		{Kind: KindEquipment, ID: 200, Price: &p2},
		{Kind: KindMaterial, ID: 300}, // no fields — must be dropped
	}
	got := BulkPlan(changes)
	if len(got.Materials) != 1 {
		t.Fatalf("BulkPlan materials = %d, want 1 (id 300 has no changes, must drop)", len(got.Materials))
	}
	if got.Materials[0]["id"] != int64(100) {
		t.Errorf("BulkPlan material id = %v, want 100", got.Materials[0]["id"])
	}
	if got.Materials[0]["cost"] != 1.5 || got.Materials[0]["price"] != 3.0 {
		t.Errorf("BulkPlan material fields = %+v, want cost 1.5 price 3.0", got.Materials[0])
	}
	if len(got.Equipment) != 1 || got.Equipment[0]["price"] != 9.99 {
		t.Errorf("BulkPlan equipment = %+v, want one item price 9.99", got.Equipment)
	}
	if _, ok := got.Equipment[0]["cost"]; ok {
		t.Errorf("BulkPlan equipment item should not carry cost when only price changed")
	}
}

func TestCategoryRefsUnmarshal(t *testing.T) {
	var ints CategoryRefs
	if err := json.Unmarshal([]byte(`[30105,35228]`), &ints); err != nil {
		t.Fatalf("unmarshal int array: %v", err)
	}
	if len(ints) != 2 || ints[0] != 30105 || ints[1] != 35228 {
		t.Errorf("int-array CategoryRefs = %v, want [30105 35228]", ints)
	}
	var objs CategoryRefs
	if err := json.Unmarshal([]byte(`[{"id":1},{"id":2}]`), &objs); err != nil {
		t.Fatalf("unmarshal object array: %v", err)
	}
	if len(objs) != 2 || objs[0] != 1 || objs[1] != 2 {
		t.Errorf("object-array CategoryRefs = %v, want [1 2]", objs)
	}
	var null CategoryRefs
	if err := json.Unmarshal([]byte(`null`), &null); err != nil || null != nil {
		t.Errorf("unmarshal null = %v err=%v, want nil nil", null, err)
	}
}

func TestParseQuoteFileCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quote.csv")
	content := "vendor_part,cost,description\nF1921000,$1.75,PEX Pipe 1in\nMPCLR30TE1,\"1,250.00\",Softener\n,5.00,blank part is skipped\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := ParseQuoteFile(path, "auto")
	if err != nil {
		t.Fatalf("ParseQuoteFile csv: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("ParseQuoteFile returned %d lines, want 2 (blank part dropped)", len(lines))
	}
	if lines[0].VendorPart != "F1921000" || !approx(lines[0].Cost, 1.75) {
		t.Errorf("line 0 = %+v, want F1921000 / 1.75 ($ stripped)", lines[0])
	}
	if !approx(lines[1].Cost, 1250.0) {
		t.Errorf("line 1 cost = %v, want 1250.0 (comma stripped)", lines[1].Cost)
	}
}

func TestParseQuoteFileJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quote.json")
	content := `[{"vendor_part":"F1921000","cost":1.75,"description":"PEX"},{"vendor_part":"  ","cost":9}]`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := ParseQuoteFile(path, "auto")
	if err != nil {
		t.Fatalf("ParseQuoteFile json: %v", err)
	}
	if len(lines) != 1 || lines[0].VendorPart != "F1921000" {
		t.Errorf("ParseQuoteFile json = %+v, want one line F1921000 (blank-part line dropped)", lines)
	}
}

func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-6
}
