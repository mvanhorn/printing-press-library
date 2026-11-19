package cli

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func mktCheckNotesInvariant(t *testing.T, m map[string]any) {
	t.Helper()
	raw := m["notes"].([]any)
	notes := make([]string, len(raw))
	seen := map[string]bool{}
	for i, n := range raw {
		notes[i] = n.(string)
		seen[notes[i]] = true
	}
	if note, ok := m["note"].(string); ok && note != "" && !seen[note] && note != strings.Join(notes, "; ") {
		t.Fatalf("NOTE_NOT_IN_NOTES: %v", m)
	}
	for i, n := range notes {
		contained := map[string]bool{}
		for _, other := range notes {
			if other != "" && other != n && strings.Contains(n, other) {
				contained[other] = true
			}
		}
		if len(contained) >= 2 {
			t.Fatalf("NOTES_ENTRY_CONTAINS_OTHERS: %v", notes)
		}
		others := append([]string{}, notes[:i]...)
		others = append(others, notes[i+1:]...)
		for start := range others {
			for end := start + 2; end <= len(others); end++ {
				if n == strings.Join(others[start:end], "; ") {
					t.Fatalf("NOTES_COMPOSITE_BROKEN: %v", notes)
				}
			}
		}
	}
}

func TestMarketNotesDemandMultiple(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron", "ceramic mug", "wool beanie"}, 1, "partial", mktFxProg{})
	mktFxResult(t, db.DB(), id, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 9000001, title: "linen apron", price: 2400})
	if _, err := db.DB().Exec(`INSERT INTO market_term_pages (snapshot_id, term, page, listings) VALUES (?, ?, 1, 0)`, id, "ceramic mug"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, err := mktRunCLI(t, "market", "demand", "--snapshot", fmt.Sprint(id))
	if err != nil {
		t.Fatal(err)
	}
	m := mktDecode(t, out)
	want := []any{
		"no review data collected; listings are ordered by search position, not demand — re-run market snapshot with --with-reviews",
		"1 term(s) returned no results from Etsy's search API",
		fmt.Sprintf("1 term(s) not yet fetched; resume with etsy-pp-cli market snapshot --resume %d", id),
		`"linen apron": no review data was collected for this term (absent is not zero); re-run market snapshot with --with-reviews`,
		`"ceramic mug": Etsy's search returned no results for this term`,
	}
	if !reflect.DeepEqual(m["notes"], want) {
		t.Fatalf("NOTES_DEMAND_EXACT: %s", out)
	}
	mktCheckNotesInvariant(t, m)
	if m["note"] != strings.Join(anyNotesStrings(m), "; ") {
		t.Fatalf("NOTE_NOT_IN_NOTES: %s", out)
	}
}

func anyNotesStrings(m map[string]any) []string {
	out := []string{}
	for _, n := range m["notes"].([]any) {
		out = append(out, n.(string))
	}
	return out
}

func TestMarketNotesCompleteResumeCapped(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "complete", mktFxProg{})
	if _, err := db.DB().Exec(`UPDATE market_snapshots SET progress_json = ? WHERE id = ?`, `{"reviews_capped":[1000000001]}`, id); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, err := mktRunCLI(t, "market", "snapshot", "--resume", fmt.Sprint(id))
	if err != nil {
		t.Fatal(err)
	}
	m := mktDecode(t, out)
	for _, n := range []string{"Review collection capped at 1000 reviews per listing.", "snapshot is already complete; nothing to resume"} {
		if !mktNotesContain(m, n) {
			t.Fatalf("NOTES_RESUME_UNION: %s", out)
		}
	}
	mktCheckNotesInvariant(t, m)
}

func TestMarketNotesDryRunUnreadableTerms(t *testing.T) {
	mktTestEnv(t)
	out, err := mktRunCLI(t, "market", "snapshot", "--terms", filepath.Join(t.TempDir(), "missing.txt"), "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	m := mktDecode(t, out)
	note, ok := m["note"].(string)
	if !ok || note == "" || !reflect.DeepEqual(m["notes"], []any{note}) || m["dry_run"] != true {
		t.Fatalf("NOTES_DRYRUN_UNION: %s", out)
	}
	mktCheckNotesInvariant(t, m)
}
