package cli

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestHistoryAPIURLIncludesPageNumber(t *testing.T) {
	got := historyAPIURL("USER/ID", "LSAC140", 3, 25)
	want := "https://app.lawhub.org/api/request/v2/api/user/USER%2FID/history/LSAC140?PageNumber=3&SortOrder=desc&SortField=startDate&PageSize=25"
	if got != want {
		t.Fatalf("historyAPIURL()=%q want %q", got, want)
	}
}

func TestShouldDisableChromeSandboxEnvDetection(t *testing.T) {
	if shouldDisableChromeSandbox() != shouldDisableChromeSandbox() {
		t.Fatal("sandbox detection should be stable within one process")
	}
}

func TestParseDuration(t *testing.T) {
	cases := map[string]int{
		"1m 30s":      90,
		"2m":          120,
		"45s":         45,
		"1h 2m 3s":    3723,
		"Time 3m 05s": 185,
	}
	for in, want := range cases {
		if got := parseDuration(in); got != want {
			t.Fatalf("parseDuration(%q)=%d want %d", in, got, want)
		}
	}
}

func TestParseReportRowsJSONNormalizesRows(t *testing.T) {
	raw := `[
		{"section_index":1,"question_number":1,"flagged_text":"Not Flagged","chosen_answer":"C","answer_status":"incorrect","correct_answer":"B","question_type":"Matching Flaws","difficulty":4,"time_text":"2m 5s"},
		{"section_index":1,"question_number":2,"flagged_text":"Flagged","chosen_answer":"A","answer_status":"correct","correct_answer":"A","question_type":"Main Point","difficulty":2,"time_text":"45s"}
	]`
	rows, err := ParseReportRowsJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].IsCorrect != 0 || rows[0].Flagged != 0 || rows[0].TimeSpentSeconds != 125 {
		t.Fatalf("bad normalized first row: %+v", rows[0])
	}
	if rows[1].IsCorrect != 1 || rows[1].Flagged != 1 || rows[1].TimeSpentSeconds != 45 {
		t.Fatalf("bad normalized second row: %+v", rows[1])
	}
}

func TestParseReportRowsJSONDefaultsUnexpectedFlagTextToNotFlagged(t *testing.T) {
	raw := `[
		{"section_index":1,"question_number":1,"flagged_text":"","chosen_answer":"A","answer_status":"correct","correct_answer":"A","question_type":"Strengthen","difficulty":2,"time_text":"30s"},
		{"section_index":1,"question_number":2,"flagged_text":"Yes","chosen_answer":"B","answer_status":"correct","correct_answer":"B","question_type":"Flaw","difficulty":3,"time_text":"45s"}
	]`
	rows, err := ParseReportRowsJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Flagged != 0 {
		t.Fatalf("empty flagged_text should default to not flagged, got %+v", rows[0])
	}
	if rows[1].Flagged != 1 {
		t.Fatalf("positive flagged_text should mark flagged, got %+v", rows[1])
	}
}

func TestParseReportRowsJSONSameLetterIncorrectBlanksCorrectAnswer(t *testing.T) {
	raw := `[{"section_index":1,"question_number":1,"flagged_text":"Not Flagged","chosen_answer":"C","answer_status":"incorrect","correct_answer":"C","question_type":"Inference","difficulty":3,"time_text":"1m"}]`
	rows, err := ParseReportRowsJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].IsCorrect != 0 || rows[0].CorrectAnswer != "" {
		t.Fatalf("expected authoritative incorrect + blank answer, got %+v", rows[0])
	}
}

func TestParseReportRowsJSONRejectsBadPayloads(t *testing.T) {
	bad := []string{
		`[]`,
		`[{"section_index":0,"question_number":1}]`,
		`[{"section_index":1,"question_number":1,"answer_status":"weird"}]`,
		`[{"section_index":1,"question_number":1},{"section_index":1,"question_number":1}]`,
	}
	for _, raw := range bad {
		if _, err := ParseReportRowsJSON(raw); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
}

func TestUpsertReportQuestionsPreservesExistingTimeOnZeroParse(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE questions (
		id TEXT PRIMARY KEY,
		attempt_id TEXT,
		section_id TEXT,
		section_index INTEGER,
		question_number INTEGER,
		question_type TEXT,
		chosen_answer TEXT,
		correct_answer TEXT,
		is_correct INTEGER,
		time_spent_seconds INTEGER,
		flagged INTEGER,
		source_url TEXT,
		answered INTEGER,
		difficulty INTEGER
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO questions(id,attempt_id,section_id,section_index,question_number,time_spent_seconds) VALUES('existing','attempt1','sectionA',1,1,95)`)
	if err != nil {
		t.Fatal(err)
	}
	rows := []reportRow{{SectionIndex: 1, QuestionNumber: 1, QuestionType: "Strengthen", ChosenAnswer: "B", CorrectAnswer: "B", IsCorrect: 1, TimeSpentSeconds: 0}}
	updated := upsertReportQuestions(db, "attempt1", rows)
	if updated != 1 {
		t.Fatalf("updated=%d", updated)
	}
	var seconds int
	if err := db.QueryRow(`SELECT time_spent_seconds FROM questions WHERE id='existing'`).Scan(&seconds); err != nil {
		t.Fatal(err)
	}
	if seconds != 95 {
		t.Fatalf("time_spent_seconds=%d want 95", seconds)
	}
}

func TestUpsertReportQuestionsDoesNotDuplicateExistingRows(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE questions (
		id TEXT PRIMARY KEY,
		attempt_id TEXT,
		section_id TEXT,
		section_index INTEGER,
		question_number INTEGER,
		question_type TEXT,
		chosen_answer TEXT,
		correct_answer TEXT,
		is_correct INTEGER,
		time_spent_seconds INTEGER,
		flagged INTEGER,
		source_url TEXT,
		answered INTEGER,
		difficulty INTEGER
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO questions(id,attempt_id,section_id,section_index,question_number) VALUES('existing','attempt1','sectionA',1,1)`)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ParseReportRowsJSON(`[{"section_index":1,"question_number":1,"flagged_text":"Not Flagged","chosen_answer":"B","answer_status":"correct","correct_answer":"B","question_type":"Strengthen","difficulty":3,"time_text":"1m"}]`)
	if err != nil {
		t.Fatal(err)
	}
	updated := upsertReportQuestions(db, "attempt1", rows)
	if updated != 1 {
		t.Fatalf("updated=%d", updated)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM questions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected no duplicate rows, got %d", count)
	}
	var qtype string
	if err := db.QueryRow(`SELECT question_type FROM questions WHERE id='existing'`).Scan(&qtype); err != nil {
		t.Fatal(err)
	}
	if qtype != "Strengthen" {
		t.Fatalf("question_type=%q", qtype)
	}
}
