// Hand-authored coverage for curate --sort velocity (recent-window citations). Not generated.

package lancet

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// velSeed is one OpenAlex work as the refresh would receive it. counts is the
// counts_by_year content (nil omits the key, as an un-refreshed mirror would
// lack it); fwci and pct are nil for works OpenAlex has not scored yet.
type velSeed struct {
	id      string
	title   string
	pubYear int
	cited   int
	counts  map[int]int
	fwci    *float64
	pct     *float64
}

func fptr(v float64) *float64 { return &v }

// velPayload builds an OpenAlex /works page for the seeds.
func velPayload(t *testing.T, seeds ...velSeed) json.RawMessage {
	t.Helper()
	results := make([]map[string]any, 0, len(seeds))
	for _, s := range seeds {
		r := map[string]any{
			"id":               "https://openalex.org/W" + s.id,
			"title":            s.title,
			"publication_year": s.pubYear,
			"cited_by_count":   s.cited,
			"open_access":      map[string]any{"is_oa": false},
			"primary_topic":    map[string]any{"display_name": "Rate"},
			"fwci":             nil,

			"citation_normalized_percentile": nil,
		}
		if s.fwci != nil {
			r["fwci"] = *s.fwci
		}
		if s.pct != nil {
			r["citation_normalized_percentile"] = map[string]any{"value": *s.pct, "is_in_top_1_percent": false, "is_in_top_10_percent": false}
		}
		if s.counts != nil {
			years := make([]int, 0, len(s.counts))
			for y := range s.counts {
				years = append(years, y)
			}
			sort.Sort(sort.Reverse(sort.IntSlice(years))) // OpenAlex lists newest first
			list := make([]map[string]int, 0, len(years))
			for _, y := range years {
				list = append(list, map[string]int{"year": y, "cited_by_count": s.counts[y]})
			}
			r["counts_by_year"] = list
		}
		results = append(results, r)
	}
	raw, err := json.Marshal(map[string]any{"meta": map[string]any{"count": len(results), "next_cursor": ""}, "results": results})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

var velJournal = []Journal{{Slug: "lancet", ISSN: "0140-6736", Display: "The Lancet"}}

// velDB stores the seeds through the real Refresh path.
func velDB(t *testing.T, seeds ...velSeed) *sql.DB {
	t.Helper()
	db := wordDB(t)
	if _, err := Refresh(context.Background(), stubFetcher{payload: velPayload(t, seeds...)}, db, velJournal, 0, 0, 1, nil); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return db
}

// velCurate runs Curate and returns the rows as their JSON objects.
func velCurate(db *sql.DB, sortBy string, limit int) ([]map[string]any, error) {
	rows, err := Curate(context.Background(), db, "rate", "", sortBy, false, limit)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	return out, json.Unmarshal(raw, &out)
}

func mustVelCurate(t *testing.T, db *sql.DB, sortBy string, limit int) []map[string]any {
	t.Helper()
	rows, err := velCurate(db, sortBy, limit)
	if err != nil {
		t.Fatalf("Curate(%s): %v", sortBy, err)
	}
	return rows
}

func byTitle(rows []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, r := range rows {
		out[r["title"].(string)] = r
	}
	return out
}

// sameYearOrSkip runs body with the current UTC calendar year and skips the
// test if the year changed meanwhile (the store reads the year from its clock).
func sameYearOrSkip(t *testing.T, body func(cy int)) {
	t.Helper()
	before := time.Now().UTC().Year()
	body(before)
	if after := time.Now().UTC().Year(); after != before {
		t.Skip("calendar year changed during the test")
	}
}

// wantField asserts that key is present in the row, null when want is nil and
// otherwise within 0.05 of want.
func wantField(t *testing.T, label string, row map[string]any, key string, want *float64) {
	t.Helper()
	got, present := row[key]
	if !present {
		t.Errorf("%s: field %q missing from the JSON row", label, key)
		return
	}
	if want == nil {
		if got != nil {
			t.Errorf("%s: %s = %v, want null", label, key, got)
		}
		return
	}
	f, ok := got.(float64)
	if !ok || math.Abs(f-*want) > 0.05 {
		t.Errorf("%s: %s = %v, want %v", label, key, got, *want)
	}
}

// a) formula
func TestVelocityFormula(t *testing.T) {
	sameYearOrSkip(t, func(cy int) {
		db := velDB(t,
			// 0.5*100 + 0.3*50 + 0.2*10; the current year and cy-4 are outside the window.
			velSeed{id: "1", title: "Three years", pubYear: cy - 10, cited: 1000, counts: map[int]int{cy: 9999, cy - 1: 100, cy - 2: 50, cy - 3: 10, cy - 4: 7777}},
			// Published cy-3: only cy-1 and cy-2 count, weights renormalised: (0.5*80 + 0.3*40)/0.8.
			velSeed{id: "2", title: "Two years", pubYear: cy - 3, cited: 1000, counts: map[int]int{cy: 9999, cy - 1: 80, cy - 2: 40, cy - 3: 777}},
			// Published cy-2: only cy-1 counts, the publication year itself and earlier years do not.
			velSeed{id: "3", title: "One year", pubYear: cy - 2, cited: 1000, counts: map[int]int{cy - 1: 40, cy - 2: 500, cy - 3: 900}},
			// A year missing from counts_by_year is 0 (weight kept in the denominator).
			velSeed{id: "4", title: "Missing years are zero", pubYear: cy - 10, cited: 1000, counts: map[int]int{cy - 1: 100}},
			velSeed{id: "5", title: "Only the oldest window year", pubYear: cy - 10, cited: 1000, counts: map[int]int{cy - 3: 10}},
			// No yearly rows at all for this work while others have rows: 0, not null.
			velSeed{id: "6", title: "Mature work without rows", pubYear: cy - 10, cited: 1000, counts: map[int]int{}},
			// No complete year after publication yet: null.
			velSeed{id: "7", title: "Published last year", pubYear: cy - 1, cited: 50, counts: map[int]int{cy - 1: 50, cy: 5}},
			velSeed{id: "8", title: "Published this year", pubYear: cy, cited: 5, counts: map[int]int{cy: 5}},
			velSeed{id: "9", title: "Unknown year", pubYear: 0, cited: 5, counts: map[int]int{cy - 1: 5}},
		)
		got := byTitle(mustVelCurate(t, db, "velocity", 100))
		want := map[string]*float64{
			"Three years":                 fptr(67.0),
			"Two years":                   fptr(65.0),
			"One year":                    fptr(40.0),
			"Missing years are zero":      fptr(50.0),
			"Only the oldest window year": fptr(2.0),
			"Mature work without rows":    fptr(0),
			"Published last year":         nil,
			"Published this year":         nil,
			"Unknown year":                nil,
		}
		if len(got) != len(want) {
			t.Fatalf("rows = %d, want %d (velocity must not drop works)", len(got), len(want))
		}
		for title, v := range want {
			row, ok := got[title]
			if !ok {
				t.Errorf("%q missing from the velocity result", title)
				continue
			}
			wantField(t, title, row, "velocity", v)
		}
	})
}

// b) ranking happens over all matches before LIMIT
func TestVelocityRanksBeforeLimit(t *testing.T) {
	sameYearOrSkip(t, func(cy int) {
		var seeds []velSeed
		for i := 0; i < 30; i++ {
			// Many citations and low ids, but a trickle of recent citations.
			seeds = append(seeds, velSeed{id: string(rune('A'+i/10)) + string(rune('0'+i%10)), title: "Filler " + string(rune('a'+i%26)) + string(rune('a'+i/26)), pubYear: cy - 10, cited: 5000 - i, counts: map[int]int{cy - 1: 1}})
		}
		seeds = append(seeds, velSeed{id: "Z999", title: "Hidden gem", pubYear: cy - 5, cited: 3, counts: map[int]int{cy - 1: 400}})
		db := velDB(t, seeds...)
		rows := mustVelCurate(t, db, "velocity", 3)
		if len(rows) != 3 || rows[0]["title"] != "Hidden gem" {
			t.Fatalf("limit 3 velocity = %v, want Hidden gem first (ranked over all matches, not the first 3 by id or citations)", titleOrder(rows))
		}
		// The same data under the citations sort would not even list it.
		if got := titleOrder(mustVelCurate(t, db, "citations", 3)); contains(got, "Hidden gem") {
			t.Fatalf("fixture invalid: citations top 3 already contains the gem: %v", got)
		}
	})
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// c) nulls last; tie order cited_by_count desc, then id
func TestVelocityNullsLastAndTieOrder(t *testing.T) {
	sameYearOrSkip(t, func(cy int) {
		same := map[int]int{cy - 1: 20, cy - 2: 20, cy - 3: 20}
		db := velDB(t,
			// Inserted in reverse of the expected order so insertion order cannot pass the test.
			velSeed{id: "W102", title: "Tie high cited second id", pubYear: cy - 8, cited: 50, counts: same},
			velSeed{id: "W100", title: "Tie low cited", pubYear: cy - 8, cited: 1, counts: same},
			velSeed{id: "W101", title: "Tie high cited first id", pubYear: cy - 8, cited: 50, counts: same},
			velSeed{id: "W200", title: "Null but huge", pubYear: cy - 1, cited: 100000, counts: map[int]int{cy - 1: 99}},
			velSeed{id: "W300", title: "Slower", pubYear: cy - 8, cited: 9999, counts: map[int]int{cy - 1: 2}},
		)
		got := titleOrder(mustVelCurate(t, db, "velocity", 10))
		want := []string{"Tie high cited first id", "Tie high cited second id", "Tie low cited", "Slower", "Null but huge"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("velocity order = %v, want %v", got, want)
		}
	})
}

// d) display fields on every sort
func TestVelocityDisplayFieldsOnEverySort(t *testing.T) {
	sameYearOrSkip(t, func(cy int) {
		db := velDB(t,
			velSeed{id: "1", title: "Mature", pubYear: cy - 10, cited: 500, counts: map[int]int{cy - 1: 100, cy - 2: 50, cy - 3: 10}, fwci: fptr(2.5), pct: fptr(0.97)},
			velSeed{id: "2", title: "Young", pubYear: cy - 2, cited: 40, counts: map[int]int{cy - 1: 40}},
			velSeed{id: "3", title: "Newborn", pubYear: cy - 1, cited: 9, counts: map[int]int{cy - 1: 9}},
			velSeed{id: "4", title: "Edge complete", pubYear: cy - 4, cited: 60, counts: map[int]int{cy - 1: 10, cy - 2: 20, cy - 3: 30}},
			velSeed{id: "5", title: "Edge incomplete", pubYear: cy - 3, cited: 60, counts: map[int]int{cy - 1: 10, cy - 2: 20, cy - 3: 30}},
		)
		type want struct{ vel, last, accel, fwci, pct *float64 }
		cases := map[string]want{
			"Mature":          {fptr(67), fptr(100), fptr(70), fptr(2.5), fptr(0.97)},
			"Young":           {fptr(40), fptr(40), nil, nil, nil},
			"Newborn":         {nil, nil, nil, nil, nil},
			"Edge complete":   {fptr(17), fptr(10), fptr(-15), nil, nil},
			"Edge incomplete": {fptr(13.75), fptr(10), nil, nil, nil}, // (0.5*10 + 0.3*20)/0.8
		}
		for _, sortBy := range []string{"citations", "date", "per-year", "velocity"} {
			got := byTitle(mustVelCurate(t, db, sortBy, 20))
			if len(got) != len(cases) {
				t.Errorf("%s: rows = %d, want %d", sortBy, len(got), len(cases))
			}
			for title, w := range cases {
				row, ok := got[title]
				if !ok {
					t.Errorf("%s: %q missing", sortBy, title)
					continue
				}
				label := sortBy + "/" + title
				wantField(t, label, row, "velocity", w.vel)
				wantField(t, label, row, "citations_last_year", w.last)
				wantField(t, label, row, "acceleration", w.accel)
				wantField(t, label, row, "fwci", w.fwci)
				wantField(t, label, row, "citation_normalized_percentile", w.pct)
			}
		}
	})
}

// e) migration of an old store
func TestEnsureSchemaMigratesPreVelocityStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if _, err := db.Exec(`CREATE TABLE lancet_works (work_id TEXT PRIMARY KEY, doi TEXT, title TEXT, journal_issn TEXT,
		journal_name TEXT, pub_year INTEGER, pub_date TEXT, cited_count INTEGER, is_oa INTEGER, topic TEXT, synced_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO lancet_works(work_id,title,topic,cited_count,pub_year,pub_date,is_oa) VALUES ('W1','Old row','Rate',7,2015,'2015-01-01',0)`); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ { // the second run must be a no-op, not a duplicate-column error
		if err := EnsureSchema(ctx, db); err != nil {
			t.Fatalf("EnsureSchema #%d: %v", i, err)
		}
		cols := map[string]bool{}
		rs, err := db.Query(`PRAGMA table_info(lancet_works)`)
		if err != nil {
			t.Fatal(err)
		}
		for rs.Next() {
			var cid int
			var name, typ string
			var notnull, pk int
			var dflt sql.NullString
			if err := rs.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				t.Fatal(err)
			}
			cols[name] = true
		}
		rs.Close()
		for _, c := range []string{"fwci", "citation_normalized_percentile"} {
			if !cols[c] {
				t.Errorf("run %d: lancet_works lacks column %s", i, c)
			}
		}
		rs, err = db.Query(`PRAGMA table_info(lancet_work_year_counts)`)
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]int{}
		for rs.Next() {
			var cid int
			var name, typ string
			var notnull, pk int
			var dflt sql.NullString
			if err := rs.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				t.Fatal(err)
			}
			names[name] = pk
		}
		rs.Close()
		if _, ok := names["cited_by_count"]; !ok || names["work_id"] == 0 || names["year"] == 0 {
			t.Errorf("run %d: lancet_work_year_counts columns/pk = %v, want work_id+year primary key and cited_by_count", i, names)
		}
		var title string
		if err := db.QueryRow(`SELECT title FROM lancet_works WHERE work_id='W1'`).Scan(&title); err != nil || title != "Old row" {
			t.Errorf("run %d: legacy row lost: %q %v", i, title, err)
		}
	}
	// The migrated store still takes refresh upserts (FTS update trigger included).
	for i := 0; i < 2; i++ {
		if _, err := Refresh(ctx, stubFetcher{payload: velPayload(t, velSeed{id: "1", title: "Old row", pubYear: 2015, cited: 9, counts: map[int]int{2024: 3}, fwci: fptr(1.5)})}, db, velJournal, 0, 0, 1, nil); err != nil {
			t.Fatalf("Refresh on migrated store #%d: %v", i+1, err)
		}
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM lancet_work_year_counts WHERE work_id='W1'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("yearly rows for W1 = %d (err %v), want 1", n, err)
	}
}

// A mirror created by the previous release already has the word-match index and
// its triggers; adding the columns and the yearly table must leave them working.
// The previous schema is rebuilt by dropping what this release added.
func TestEnsureSchemaMigratesStoreThatAlreadyHasFTS(t *testing.T) {
	db := wordDB(t, w("1", "Sepsis bundle", "ICU"))
	ctx := context.Background()
	for _, stmt := range []string{
		`ALTER TABLE lancet_works DROP COLUMN fwci`,
		`ALTER TABLE lancet_works DROP COLUMN citation_normalized_percentile`,
		`DROP TABLE lancet_work_year_counts`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("rebuilding the previous schema: %s: %v", stmt, err)
		}
	}
	for i := 1; i <= 2; i++ {
		if err := EnsureSchema(ctx, db); err != nil {
			t.Fatalf("EnsureSchema #%d on a pre-velocity store with FTS: %v", i, err)
		}
	}
	if got := titles(t, db, "sepsis"); len(got) != 1 || !got["Sepsis bundle"] {
		t.Fatalf("word match after migration = %v, want [Sepsis bundle]", got)
	}
	// The update trigger fires on the migrated table.
	if _, err := Refresh(ctx, stubFetcher{payload: velPayload(t, velSeed{id: "1", title: "Sepsis bundle revised", pubYear: 2020, cited: 3, counts: map[int]int{2024: 2}, fwci: fptr(1.1)})}, db, velJournal, 0, 0, 1, nil); err != nil {
		t.Fatalf("Refresh after migration: %v", err)
	}
	if got := titles(t, db, "revised"); len(got) != 1 {
		t.Fatalf("index did not follow the update after migration: %v", got)
	}
}

// f) refresh parses the real OpenAlex fixture and replaces yearly rows
type yearRow struct{ Year, Cited int }

func yearRows(t *testing.T, db *sql.DB, workID string) []yearRow {
	t.Helper()
	rs, err := db.Query(`SELECT year, cited_by_count FROM lancet_work_year_counts WHERE work_id = ? ORDER BY year`, workID)
	if err != nil {
		t.Fatalf("query yearly rows: %v", err)
	}
	defer rs.Close()
	var out []yearRow
	for rs.Next() {
		var r yearRow
		if err := rs.Scan(&r.Year, &r.Cited); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func TestRefreshStoresFixtureYearlyCountsAndReplacesThem(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "openalex_work_W3124058635.json"))
	if err != nil {
		t.Fatal(err)
	}
	db := wordDB(t)
	ctx := context.Background()
	if _, err := Refresh(ctx, stubFetcher{payload: raw}, db, velJournal, 0, 0, 1, nil); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	want := []yearRow{{2020, 1}, {2021, 59}, {2022, 143}, {2023, 201}, {2024, 265}, {2025, 254}, {2026, 161}}
	if got := yearRows(t, db, "W3124058635"); !reflect.DeepEqual(got, want) {
		t.Fatalf("yearly rows = %v, want %v", got, want)
	}
	var fwci, pct sql.NullFloat64
	var year, cited int
	if err := db.QueryRow(`SELECT pub_year, cited_count, fwci, citation_normalized_percentile FROM lancet_works WHERE work_id='W3124058635'`).Scan(&year, &cited, &fwci, &pct); err != nil {
		t.Fatalf("query work: %v", err)
	}
	if year != 2021 || cited != 1084 || !fwci.Valid || math.Abs(fwci.Float64-140.0763) > 1e-9 || !pct.Valid || math.Abs(pct.Float64-0.99992475) > 1e-9 {
		t.Errorf("work = year %d cited %d fwci %v pct %v, want 2021 1084 140.0763 0.99992475", year, cited, fwci, pct)
	}

	// Second refresh: 2022 is gone and 2025 changed. The yearly rows must be
	// replaced (no stale 2022, no old 2025 value, no duplicates).
	var page map[string]any
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	res := page["results"].([]any)[0].(map[string]any)
	var counts []any
	for _, c := range res["counts_by_year"].([]any) {
		m := c.(map[string]any)
		switch int(m["year"].(float64)) {
		case 2022:
			continue
		case 2025:
			m["cited_by_count"] = 300.0
		}
		counts = append(counts, m)
	}
	res["counts_by_year"] = counts
	res["fwci"] = nil
	res["citation_normalized_percentile"] = nil
	changed, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Refresh(ctx, stubFetcher{payload: changed}, db, velJournal, 0, 0, 1, nil); err != nil {
		t.Fatalf("second Refresh: %v", err)
	}
	want2 := []yearRow{{2020, 1}, {2021, 59}, {2023, 201}, {2024, 265}, {2025, 300}, {2026, 161}}
	if got := yearRows(t, db, "W3124058635"); !reflect.DeepEqual(got, want2) {
		t.Fatalf("yearly rows after second refresh = %v, want %v", got, want2)
	}
	if err := db.QueryRow(`SELECT fwci, citation_normalized_percentile FROM lancet_works WHERE work_id='W3124058635'`).Scan(&fwci, &pct); err != nil || fwci.Valid || pct.Valid {
		t.Errorf("fwci/pct after a refresh that has none = %v %v (err %v), want NULL NULL", fwci, pct, err)
	}
}

func TestRefreshSelectsYearlyCountsAndScores(t *testing.T) {
	db := wordDB(t)
	f := &recordingFetcher{}
	if _, err := Refresh(context.Background(), f, db, velJournal, 0, 0, 1, nil); err != nil {
		t.Fatal(err)
	}
	if len(f.params) != 1 {
		t.Fatalf("calls = %d, want 1", len(f.params))
	}
	sel := strings.Split(f.params[0]["select"], ",")
	for _, field := range []string{"counts_by_year", "fwci", "citation_normalized_percentile"} {
		if !contains(sel, field) {
			t.Errorf("refresh select %v lacks %s", sel, field)
		}
	}
}

// The curate fixture date used by the real store test must not be fabricated:
// the stored yearly rows reach the ranking through Curate on the same store.
func TestVelocityOnRealFixtureShiftedToCurrentYear(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "openalex_work_W3124058635.json"))
	if err != nil {
		t.Fatal(err)
	}
	sameYearOrSkip(t, func(cy int) {
		delta := cy - 2026
		var page map[string]any
		if err := json.Unmarshal(raw, &page); err != nil {
			t.Fatal(err)
		}
		res := page["results"].([]any)[0].(map[string]any)
		res["publication_year"] = res["publication_year"].(float64) + float64(delta)
		res["title"] = "Fixture work"
		res["primary_topic"] = map[string]any{"display_name": "Rate"}
		for _, c := range res["counts_by_year"].([]any) {
			m := c.(map[string]any)
			m["year"] = m["year"].(float64) + float64(delta)
		}
		shifted, err := json.Marshal(page)
		if err != nil {
			t.Fatal(err)
		}
		db := wordDB(t)
		if _, err := Refresh(context.Background(), stubFetcher{payload: shifted}, db, velJournal, 0, 0, 1, nil); err != nil {
			t.Fatal(err)
		}
		rows := mustVelCurate(t, db, "velocity", 5)
		if len(rows) != 1 {
			t.Fatalf("rows = %d, want 1", len(rows))
		}
		// 0.5*254 + 0.3*265 + 0.2*201 = 246.7 (2025, 2024, 2023 in fixture years)
		wantField(t, "fixture", rows[0], "velocity", fptr(246.7))
		wantField(t, "fixture", rows[0], "citations_last_year", fptr(254))
		wantField(t, "fixture", rows[0], "acceleration", fptr(254-(265+201)/2.0))
		wantField(t, "fixture", rows[0], "fwci", fptr(140.0763))
		wantField(t, "fixture", rows[0], "citation_normalized_percentile", fptr(0.99992475))
	})
}

// h) matches but no yearly rows at all: an old mirror, never silently all-null
func TestVelocityOldMirrorWithoutYearlyRowsErrors(t *testing.T) {
	db := velDB(t,
		velSeed{id: "1", title: "Old mirror work", pubYear: 2015, cited: 100},
		velSeed{id: "2", title: "Another one", pubYear: 2012, cited: 200},
	)
	rows, err := velCurate(db, "velocity", 10)
	if err == nil {
		t.Fatalf("velocity on a mirror without yearly rows returned %d rows and no error", len(rows))
	}
	if !strings.Contains(err.Error(), "refresh") {
		t.Errorf("err = %q, want it to name the refresh command", err)
	}
	// Other sorts keep working on the same store.
	if got := mustVelCurate(t, db, "citations", 10); len(got) != 2 {
		t.Errorf("citations rows = %d, want 2", len(got))
	}
}

func TestVelocityNoMatchesIsNotAnError(t *testing.T) {
	db := velDB(t, velSeed{id: "1", title: "Present", pubYear: 2015, cited: 1, counts: map[int]int{2024: 1}})
	rows, err := Curate(context.Background(), db, "nothingmatchesthis", "", "velocity", false, 10)
	if err != nil || len(rows) != 0 {
		t.Fatalf("no-match velocity = %v, %v; want empty and nil error", rows, err)
	}
}

// g) the live path
func TestCurateLiveVelocityErrors(t *testing.T) {
	_, err := CurateLive(context.Background(), stubFetcher{payload: json.RawMessage(`{"results":[]}`)}, "ai", "", "velocity", false, 5)
	if err == nil || !strings.Contains(err.Error(), "local store") {
		t.Fatalf("CurateLive velocity err = %v, want a local-store error", err)
	}
}
