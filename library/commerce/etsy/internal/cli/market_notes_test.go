package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mktNotesContain(m map[string]any, want string) bool {
	notes, _ := m["notes"].([]any)
	for _, n := range notes {
		if n == want {
			return true
		}
	}
	return false
}

func TestMarketNotesGapsSnapshotNoteVerbatim(t *testing.T) { mktTestNotesHoisting(t, "gaps") }
func TestMarketNotesDemandPerTermPrefixed(t *testing.T)    { mktTestNotesHoisting(t, "demand") }
func TestMarketNotesPriceGroupPrefixed(t *testing.T)       { mktTestNotesHoisting(t, "price") }
func mktTestNotesHoisting(t *testing.T, command string) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "partial", mktFxProg{})
	mktFxResult(t, db.DB(), id, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 9000001, title: "linen apron", price: 2400})
	db.Close()
	args := []string{"market", command, "--snapshot", fmt.Sprint(id)}
	if command == "gaps" {
		path := filepath.Join(t.TempDir(), "targets.json")
		if err := os.WriteFile(path, []byte(`{"targets":[{"name":"linen apron","include":["linen apron"]}]}`), 0600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--rules", path)
	}
	out, err := mktRunCLI(t, args...)
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	m := mktDecode(t, out)
	row := m["results"].([]any)[0].(map[string]any)
	key, text := "term", "note"
	if command == "price" {
		key = "group"
	}
	if command == "gaps" {
		// Gap rows carry claims (statement), not advisories; only snapshot_note is hoisted.
		if !mktNotesContain(m, m["snapshot_note"].(string)) {
			t.Fatalf("NOTES_HOISTING_BROKEN snapshot note: %s", out)
		}
		return
	}
	want := fmt.Sprintf("%q: %s", row[key], row[text])
	if !mktNotesContain(m, want) {
		t.Fatalf("NOTES_HOISTING_BROKEN attributed row %q: %s", want, out)
	}
}

func TestMarketNotesCollapse(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	terms := []string{}
	for i := 1; i <= 7; i++ {
		terms = append(terms, fmt.Sprintf("linen apron %d", i))
	}
	id := mktFxSnapshot(t, db.DB(), time.Now(), terms, 1, "complete", mktFxProg{})
	for i, term := range terms {
		mktFxResult(t, db.DB(), id, mktFxListing{term: term, pos: 1, id: int64(1000000001 + i), shop: 9000001, price: 2400})
	}
	db.Close()
	out, err := mktRunCLI(t, "market", "demand")
	if err != nil {
		t.Fatal(err)
	}
	m := mktDecode(t, out)
	row := m["results"].([]any)[0].(map[string]any)
	want := `"linen apron 1", "linen apron 2", "linen apron 3", "linen apron 4", "linen apron 5" (+2 more): ` + row["note"].(string)
	count := 0
	for _, n := range m["notes"].([]any) {
		if strings.Contains(n.(string), row["note"].(string)) {
			count++
		}
	}
	if count != 1 || !mktNotesContain(m, want) {
		t.Fatalf("NOTES_COLLAPSE_BROKEN want %q once: %s", want, out)
	}
}

func TestMarketCompleteNoResume(t *testing.T) {
	for _, populated := range []bool{true, false} {
		name := "legacy_omitted"
		if populated {
			name = "populated"
		}
		t.Run(name, func(t *testing.T) {
			mktTestEnv(t)
			db := mktOpenFixtureStore(t)
			terms := []string{"linen", "linen apron"}
			id := mktFxSnapshot(t, db.DB(), time.Now(), terms, 2, "complete", mktFxProg{})
			if populated {
				for i, term := range terms {
					mktFxResult(t, db.DB(), id, mktFxListing{term: term, pos: 1, id: int64(1000000001 + i), price: 2400})
				}
			}
			snap, err := mktGetSnapshot(db.DB(), id)
			if err != nil {
				t.Fatal(err)
			}
			wantPages := []string{"linen|1", "linen|2", "linen apron|1", "linen apron|2"}
			if fmt.Sprint(snap.Progress.Pages) != fmt.Sprint(wantPages) {
				t.Fatalf("COMPLETE_NO_RESUME: progress %v", snap.Progress.Pages)
			}
			db.Close()
			out, err := mktRunCLI(t, "market", "demand", "--snapshot", fmt.Sprint(id))
			if err != nil {
				t.Fatalf("COMPLETE_NO_RESUME: %v: %s", err, out)
			}
			envelope := mktDecode(t, out)
			rows := envelope["results"].([]any)
			wantCount := 0
			if populated {
				wantCount = len(terms)
			}
			if len(rows) != wantCount {
				t.Fatalf("COMPLETE_NO_RESUME: rows %s", out)
			}
			for _, value := range rows {
				row := value.(map[string]any)
				if row["state"] != "absent" || row["listings_total"] != float64(1) {
					t.Fatalf("COMPLETE_NO_RESUME: unexpected populated state: %s", out)
				}
			}
			for _, note := range envelope["notes"].([]any) {
				text := strings.ToLower(note.(string))
				if strings.Contains(text, "not yet fetched") || strings.Contains(text, "resume") {
					t.Fatalf("COMPLETE_NO_RESUME: %s", out)
				}
			}
			if !populated && !mktNotesContain(envelope, "2 term(s) were fetched before per-term outcome recording; any with no results are not shown") {
				t.Fatalf("COMPLETE_NO_RESUME: missing legacy note: %s", out)
			}
		})
	}
}

func TestMarketProgressKeyExact(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen", "linen apron", "tea|coffee", "tea"}, 1, "partial", mktFxProg{})
	if _, err := db.DB().Exec(`UPDATE market_snapshots SET progress_json = ? WHERE id = ?`, `{"pages":["linen apron|1","tea|coffee|1"]}`, id); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, err := mktRunCLI(t, "market", "demand", "--snapshot", fmt.Sprint(id))
	if err != nil {
		t.Fatalf("PROGRESS_KEY_EXACT: %v: %s", err, out)
	}
	envelope := mktDecode(t, out)
	rows := envelope["results"].([]any)
	if len(rows) != 2 {
		t.Fatalf("PROGRESS_KEY_EXACT: fetched legacy terms must be omitted: %s", out)
	}
	seen := map[string]bool{}
	for _, value := range rows {
		row := value.(map[string]any)
		term := row["term"].(string)
		if (term != "linen" && term != "tea") || row["state"] != "not_fetched" || row["listings_total"] != nil {
			t.Fatalf("PROGRESS_KEY_EXACT: %s", out)
		}
		seen[term] = true
	}
	if !seen["linen"] || !seen["tea"] {
		t.Fatalf("PROGRESS_KEY_EXACT: %s", out)
	}
	if !mktNotesContain(envelope, "2 term(s) were fetched before per-term outcome recording; any with no results are not shown") {
		t.Fatalf("PROGRESS_KEY_EXACT: missing legacy note: %s", out)
	}
}
