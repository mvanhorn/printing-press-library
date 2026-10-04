package lancet

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func wordDB(t *testing.T, works ...decodedWork) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if len(works) > 0 {
		if _, err := StoreWorks(ctx, db, works, "0140-6736", "The Lancet"); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func w(id, title, topic string) decodedWork {
	return decodedWork{ID: id, Title: title, Topic: topic, Year: 2024, Date: "2024-01-01", Cited: 1}
}

func titles(t *testing.T, db *sql.DB, topic string) map[string]bool {
	t.Helper()
	rows, err := Curate(context.Background(), db, topic, "", "citations", false, 100)
	if err != nil {
		t.Fatalf("Curate(%q): %v", topic, err)
	}
	out := map[string]bool{}
	for _, r := range rows {
		out[r.Title] = true
	}
	return out
}

func seedWords(t *testing.T) *sql.DB {
	return wordDB(t,
		w("1", "AI in diagnosis", "Medicine"),
		w("2", "Air pollution and health", "Environment"),
		w("3", "Trials against malaria", "Infection"),
		w("4", "Heart failure outcomes", "Cardiology"),
		w("5", "Badger culling", "Ecology"),
		w("6", "Machine learning", "AI"),
		w("7", "Early detection", "Diagnosis"),
	)
}

func TestCurateWholeWordNotSubstring(t *testing.T) {
	got := titles(t, seedWords(t), "ai")
	if !got["AI in diagnosis"] || !got["Machine learning"] {
		t.Errorf("ai should match title and topic words, got %v", got)
	}
	for _, bad := range []string{"Air pollution and health", "Trials against malaria", "Heart failure outcomes"} {
		if got[bad] {
			t.Errorf("ai must not match %q", bad)
		}
	}
	if got := titles(t, seedWords(t), "badge"); len(got) != 0 {
		t.Errorf("badge must not match badger, got %v", got)
	}
}

func TestCurateStemmingAndCase(t *testing.T) {
	db := seedWords(t)
	// porter does NOT unify diagnoses/diagnosis (stems "diagnos" vs "diagnosi");
	// it does unify regular plurals, which is what we can rely on.
	if got := titles(t, db, "outcome"); !got["Heart failure outcomes"] {
		t.Errorf("outcome should stem-match outcomes, got %v", got)
	}
	if got := titles(t, db, "diagnoses"); got["AI in diagnosis"] {
		t.Logf("note: diagnoses now matches diagnosis (tokenizer changed?)")
	}
	lo, up := titles(t, db, "ai"), titles(t, db, "AI")
	if len(lo) != len(up) || len(lo) == 0 {
		t.Errorf("case-insensitive mismatch: %v vs %v", lo, up)
	}
}

func TestCurateMultiWordAND(t *testing.T) {
	db := seedWords(t)
	// "ai" in title of #1 and "diagnosis" in title of #1; #6 has ai only (topic), #7 diagnosis only.
	got := titles(t, db, "ai diagnosis")
	if len(got) != 1 || !got["AI in diagnosis"] {
		t.Errorf("want only 'AI in diagnosis', got %v", got)
	}
	// one word in title, the other in topic
	db2 := wordDB(t, w("a", "Neural networks", "Oncology"))
	if got := titles(t, db2, "neural oncology"); len(got) != 1 {
		t.Errorf("title+topic words should match, got %v", got)
	}
	if got := titles(t, db2, "neural cardiology"); len(got) != 0 {
		t.Errorf("only one of two words present must not match, got %v", got)
	}
}

func TestCurateHostileInputsDoNotError(t *testing.T) {
	db := seedWords(t)
	for _, in := range []string{`"`, `AND`, `OR`, `NOT`, `*`, `covid-19`, `o'brien`, `NEAR(`, `a"b`, `)`, ``, `   `} {
		if _, err := Curate(context.Background(), db, in, "", "citations", false, 10); err != nil {
			t.Errorf("Curate(%q) errored: %v", in, err)
		}
	}
	if got := titles(t, wordDB(t, w("x", "AND other words", "")), "AND"); len(got) != 1 {
		t.Errorf("AND must be a literal word, got %v", got)
	}
}

func TestCurateEmptyTopicReturnsAll(t *testing.T) {
	if got := titles(t, seedWords(t), "  "); len(got) != 7 {
		t.Errorf("empty topic returns every work, got %d", len(got))
	}
}

func TestEnsureSchemaBackfillsLegacyDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.Exec(`CREATE TABLE lancet_works (work_id TEXT PRIMARY KEY, doi TEXT, title TEXT, journal_issn TEXT,
		journal_name TEXT, pub_year INTEGER, pub_date TEXT, cited_count INTEGER, is_oa INTEGER, topic TEXT, synced_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO lancet_works(work_id,title,topic,cited_count,pub_year,pub_date,is_oa) VALUES ('1','AI in diagnosis','Medicine',1,2024,'2024-01-01',0),('2','Air quality','Env',1,2024,'2024-01-01',0)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // second open must be idempotent
		if err := EnsureSchema(ctx, db); err != nil {
			t.Fatalf("EnsureSchema #%d: %v", i+1, err)
		}
		got := titles(t, db, "ai")
		if len(got) != 1 || !got["AI in diagnosis"] {
			t.Fatalf("open #%d: got %v", i+1, got)
		}
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM lancet_works_fts WHERE lancet_works_fts MATCH '"ai"'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("index rows for ai = %d (err %v), want 1 (no duplicate backfill)", n, err)
	}
}

func TestCurateIndexFollowsInsertUpdateDelete(t *testing.T) {
	db := wordDB(t)
	ctx := context.Background()
	if _, err := StoreWorks(ctx, db, []decodedWork{w("1", "Sepsis bundle", "ICU")}, "0140-6736", "The Lancet"); err != nil {
		t.Fatal(err)
	}
	if got := titles(t, db, "sepsis"); len(got) != 1 {
		t.Fatalf("insert not indexed: %v", got)
	}
	if _, err := db.Exec(`UPDATE lancet_works SET title='Stroke pathway' WHERE work_id='1'`); err != nil {
		t.Fatal(err)
	}
	if got := titles(t, db, "sepsis"); len(got) != 0 {
		t.Errorf("old title still matches after update: %v", got)
	}
	if got := titles(t, db, "stroke"); len(got) != 1 {
		t.Errorf("new title not matched after update: %v", got)
	}
	if _, err := db.Exec(`DELETE FROM lancet_works WHERE work_id='1'`); err != nil {
		t.Fatal(err)
	}
	if got := titles(t, db, "stroke"); len(got) != 0 {
		t.Errorf("deleted row still matches: %v", got)
	}
}

func TestCuratePunctuationSplitsWords(t *testing.T) {
	db := seedWords(t)
	for _, in := range []string{"ai,diagnosis", "ai/diagnosis"} {
		if got := titles(t, db, in); len(got) != 1 || !got["AI in diagnosis"] {
			t.Errorf("%q should behave like 'ai diagnosis', got %v", in, got)
		}
	}
	if got := titles(t, db, "ai,cardiology"); len(got) != 0 {
		t.Errorf("only one word present must not match, got %v", got)
	}
	if got := titles(t, wordDB(t, w("c", "COVID-19 vaccines", "")), "covid-19"); len(got) != 1 {
		t.Errorf("covid-19 should match COVID-19, got %v", got)
	}
}

func TestEnsureWorksFTSInterruptedBackfillRollsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interrupted.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.Exec(`CREATE TABLE lancet_works (work_id TEXT PRIMARY KEY, doi TEXT, title TEXT, journal_issn TEXT,
		journal_name TEXT, pub_year INTEGER, pub_date TEXT, cited_count INTEGER, is_oa INTEGER, topic TEXT, synced_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO lancet_works(work_id,title,topic,cited_count,pub_year,pub_date,is_oa) VALUES ('1','AI in diagnosis','Medicine',1,2024,'2024-01-01',0)`); err != nil {
		t.Fatal(err)
	}
	ftsRebuildHook = func() error { return errors.New("injected rebuild failure") }
	err = EnsureSchema(ctx, db)
	ftsRebuildHook = nil
	if err == nil {
		t.Fatal("EnsureSchema must fail when the rebuild fails")
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'lancet_works_fts%'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("FTS objects left behind after failed open: %d (err %v)", n, err)
	}
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("normal open: %v", err)
	}
	if got := titles(t, db, "ai"); len(got) != 1 || !got["AI in diagnosis"] {
		t.Errorf("pre-existing work not found after recovery: %v", got)
	}
}

func TestCurateCombiningMarksStayInWord(t *testing.T) {
	pre, dec := "Naïve trial", "Naïve trial"
	db := wordDB(t, w("p", pre, ""), w("d", dec, ""))
	matrix := map[string]map[string]bool{}
	for name, q := range map[string]string{"precomposed": "naïve", "decomposed": "naïve"} {
		matrix[name] = titles(t, db, q)
		t.Logf("query %-11s -> precomposed title: %v, decomposed title: %v", name, matrix[name][pre], matrix[name][dec])
	}
	if !matrix["decomposed"][dec] {
		t.Errorf("decomposed query must match the decomposed title, got %v", matrix["decomposed"])
	}
	for title := range matrix["precomposed"] {
		if !matrix["decomposed"][title] {
			t.Errorf("decomposed query must match everything the precomposed query matches; missing %q", title)
		}
	}
}

func TestEnsureWorksFTSRestoresBusyTimeout(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "busy.db"))
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
	var before, after int
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := ensureWorksFTS(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Errorf("busy_timeout after setup = %d, want original %d", after, before)
	}
}

func TestCurateNumberAndPrivateUseRunesStayInWord(t *testing.T) {
	forms := map[string]string{
		"superscript-No": "a²",
		"roman-Nl":       "Ⅻ",
		"subscript-No":   "x₂",
		"private-use-Co": "pq",
	}
	for name, form := range forms {
		db := wordDB(t, w("1", "Study of "+form+" markers", ""))
		got := titles(t, db, form)
		t.Logf("%-15s query %q -> match: %v", name, form, len(got) == 1)
		if len(got) != 1 {
			t.Errorf("%s: query %q must match its own title, got %v", name, form, got)
		}
	}
}
