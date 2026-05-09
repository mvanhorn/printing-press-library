package diggparse

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadFixture reads the trimmed /ai/1000 fixture from testdata.
func loadFixture(t *testing.T) []byte {
	t.Helper()
	// testdata lives at the repo root: ../../testdata/ai-1000-fixture.html
	// (parser package is internal/diggparse).
	candidates := []string{
		filepath.Join("..", "..", "testdata", "ai-1000-fixture.html"),
		filepath.Join("testdata", "ai-1000-fixture.html"),
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			return data
		}
	}
	t.Fatalf("ai-1000-fixture.html not found; tried: %v", candidates)
	return nil
}

func TestParseRoster1000_FixtureReturnsTrimmedRoster(t *testing.T) {
	html := loadFixture(t)
	authors, err := ParseRoster1000(html)
	if err != nil {
		t.Fatalf("ParseRoster1000: %v", err)
	}
	// Trimmed fixture should have at least 30 records — the trim keeps a
	// representative subset (~46 picks) but allow margin in case the
	// fixture is regenerated with fewer slots.
	if len(authors) < 30 {
		t.Fatalf("got %d authors, want >= 30", len(authors))
	}
	// Every record has a non-empty username and a positive rank.
	for _, a := range authors {
		if a.Username == "" {
			t.Errorf("rank %d has empty username", a.Rank)
		}
		if a.Rank <= 0 {
			t.Errorf("@%s has non-positive rank %d", a.Username, a.Rank)
		}
	}
	// Spot-check rank 1 is sama (matches live snapshot at fixture-capture time).
	var sama *Roster1000Author
	for i := range authors {
		if authors[i].Username == "sama" {
			sama = &authors[i]
			break
		}
	}
	if sama == nil {
		t.Fatal("expected fixture to contain sama")
	}
	if sama.Rank != 1 {
		t.Errorf("sama rank = %d, want 1", sama.Rank)
	}
	if sama.Category == "" {
		t.Errorf("sama category empty")
	}
}

func TestParseRoster1000_PreviousRankNullableNeverMissing(t *testing.T) {
	html := loadFixture(t)
	authors, err := ParseRoster1000(html)
	if err != nil {
		t.Fatalf("ParseRoster1000: %v", err)
	}
	// Confirm the fixture contains BOTH a record with previousRank=null and
	// at least one with a non-null value — otherwise the test would pass
	// vacuously.
	sawNull, sawInt := false, false
	for _, a := range authors {
		if a.PreviousRank == nil {
			sawNull = true
		} else {
			sawInt = true
		}
	}
	if !sawNull {
		t.Error("expected at least one record with previousRank=null in fixture")
	}
	if !sawInt {
		t.Error("expected at least one record with a non-null previousRank in fixture")
	}
	// The schema decoder represents "missing key" and "null" identically
	// (both produce nil pointer); since the upstream record always emits
	// the key, every record has either int or null — no record drops it.
	// We assert non-zero records and the null/int split above; nothing more
	// to check at the value-level.
}

func TestParseRoster1000_VibeDistributionDecodedAsMap(t *testing.T) {
	html := loadFixture(t)
	authors, err := ParseRoster1000(html)
	if err != nil {
		t.Fatalf("ParseRoster1000: %v", err)
	}
	requiredKeys := []string{
		"troll", "banter", "hopeful", "humorous", "teaching",
		"combative", "informing", "announcing", "supportive",
		"provocative", "devils_advocate",
	}
	// Find any author with a non-empty vibeDistribution and verify shape.
	var probe *Roster1000Author
	for i := range authors {
		if len(authors[i].VibeDistribution) > 0 {
			probe = &authors[i]
			break
		}
	}
	if probe == nil {
		t.Fatal("no author with vibeDistribution; fixture should include some")
	}
	for _, k := range requiredKeys {
		if _, ok := probe.VibeDistribution[k]; !ok {
			t.Errorf("@%s vibeDistribution missing key %q", probe.Username, k)
		}
	}
}

func TestParseRoster1000_LogangrahamFieldsPopulated(t *testing.T) {
	html := loadFixture(t)
	authors, err := ParseRoster1000(html)
	if err != nil {
		t.Fatalf("ParseRoster1000: %v", err)
	}
	var lg *Roster1000Author
	for i := range authors {
		if authors[i].Username == "logangraham" {
			lg = &authors[i]
			break
		}
	}
	if lg == nil {
		t.Fatal("fixture should contain logangraham (rank 991)")
	}
	if lg.Rank != 991 {
		t.Errorf("logangraham rank = %d, want 991", lg.Rank)
	}
	if !strings.Contains(strings.ToLower(lg.Bio), "frontier red team") {
		t.Errorf("logangraham bio missing 'frontier red team' substring; got %q", lg.Bio)
	}
	if lg.Category != "AI Safety" {
		t.Errorf("logangraham category = %q, want \"AI Safety\"", lg.Category)
	}
}

func TestExtractRoster1000Authors_MalformedChunkIsToleratedNotPanic(t *testing.T) {
	// Inject a half-formed object alongside one valid one. The valid record
	// must still come through; the malformed slice must not panic. The
	// scanObjectsContaining helper already requires balanced braces, so a
	// mid-string truncation gets dropped earlier — the way to surface the
	// "malformed JSON inside a balanced object" path is to keep braces
	// balanced but corrupt the JSON syntax.
	decoded := `{"target_x_id":"1","rank":1,"username":"alice"}` + // valid
		`{"target_x_id":"2","rank":2,"username":"bob","score":not_a_number}` + // malformed
		`{"target_x_id":"3","rank":3,"username":"carol"}` // valid
	authors, err := ExtractRoster1000Authors(decoded)
	if err == nil {
		t.Error("expected an error wrapping the malformed chunk index")
	}
	gotUsers := make(map[string]bool)
	for _, a := range authors {
		gotUsers[a.Username] = true
	}
	if !gotUsers["alice"] || !gotUsers["carol"] {
		t.Errorf("valid records dropped: got %v", gotUsers)
	}
	if gotUsers["bob"] {
		t.Errorf("malformed bob record should not have decoded")
	}
}

func TestParseRoster1000_LiveURL(t *testing.T) {
	if os.Getenv("DIGG_LIVE_TESTS") != "1" {
		t.Skip("set DIGG_LIVE_TESTS=1 to run live /ai/1000 fetch")
	}
	resp, err := http.Get("https://di.gg/ai/1000")
	if err != nil {
		t.Fatalf("live fetch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("live fetch status %d", resp.StatusCode)
	}
	buf := make([]byte, 4*1024*1024)
	n, _ := resp.Body.Read(buf)
	body := buf[:n]
	for {
		more := make([]byte, 4*1024*1024)
		m, _ := resp.Body.Read(more)
		if m == 0 {
			break
		}
		body = append(body, more[:m]...)
	}
	authors, err := ParseRoster1000(body)
	if err != nil && len(authors) == 0 {
		t.Fatalf("live parse: %v", err)
	}
	if len(authors) < 990 {
		t.Errorf("live parse got %d authors, want >= 990", len(authors))
	}
}
