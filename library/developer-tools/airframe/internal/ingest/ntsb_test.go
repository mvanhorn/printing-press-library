// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package ingest

import (
	"context"
	"database/sql"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/store"
)

// fakeExtractor returns predetermined CSV content per table name. The
// MDBExtractor signature returns a ReadCloser; we wrap strings.NewReader in
// io.NopCloser to satisfy the contract.
func fakeExtractor(tables map[string]string) MDBExtractor {
	return func(_ context.Context, _ string, table string) (io.ReadCloser, error) {
		csv, ok := tables[table]
		if !ok {
			return nil, &fakeExtractError{table: table}
		}
		return io.NopCloser(strings.NewReader(csv)), nil
	}
}

type fakeExtractError struct{ table string }

func (e *fakeExtractError) Error() string { return "fake extractor: no fixture for table " + e.table }

func TestIngestNTSBEvents(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	tx, err := st.DB().Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer tx.Rollback()

	csv := strings.Join([]string{
		`"ev_id","ev_type","ev_date","ev_city","ev_state","ev_country","latitude","longitude","ev_highest_injury","inj_tot_f","inj_tot_s","inj_tot_m","inj_tot_n","wx_cond_basic","light_cond","phase_flt_spec","ntsb_no"`,
		`"ERA22LA001","ACC","2022-01-04 00:00:00","Boca Raton","FL","USA","26.3683","-80.1289","NONE","0","0","0","2","VMC","DAYL","ENRO","ERA22LA001"`,
		`"WPR24FA101","ACC","2024-06-15 00:00:00","Reno","NV","USA","39.5296","-119.8138","FATL","2","0","0","0","","DAYL","TKOF","WPR24FA101"`,
		// row with empty ev_id should be skipped silently
		`"","INC","2023-01-01","","",,,,,,,,,,,,""`,
	}, "\n") + "\n"

	extract := fakeExtractor(map[string]string{"events": csv})
	n, err := ingestNTSBEvents(context.Background(), tx, extract, "/fake.mdb", nil)
	if err != nil {
		t.Fatalf("ingestNTSBEvents: %v", err)
	}
	if n != 2 {
		t.Errorf("inserted = %d, want 2", n)
	}

	var date, state, hi string
	var fatal sql.NullInt64
	row := tx.QueryRow(`SELECT event_date, event_state, highest_injury, total_fatal FROM events WHERE event_id='WPR24FA101'`)
	if err := row.Scan(&date, &state, &hi, &fatal); err != nil {
		t.Fatalf("scan WPR24FA101: %v", err)
	}
	if date != "2024-06-15" || state != "NV" || hi != "FATL" || !fatal.Valid || fatal.Int64 != 2 {
		t.Errorf("WPR24FA101 row mismatch: date=%q state=%q hi=%q fatal=%v", date, state, hi, fatal)
	}
}

func TestIngestNTSBAircraft(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// Seed parent events rows so the FK is satisfied.
	if _, err := st.DB().Exec(`INSERT INTO events(event_id, event_date) VALUES('ERA22LA001','2022-01-04')`); err != nil {
		t.Fatalf("seed events: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO events(event_id, event_date) VALUES('MULTIAC01','2023-04-15')`); err != nil {
		t.Fatalf("seed multi: %v", err)
	}

	tx, err := st.DB().Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer tx.Rollback()

	csv := strings.Join([]string{
		`"ev_id","Aircraft_Key","regis_no","damage","oper_name","far_part","phase_flt_spec"`,
		`"ERA22LA001","1","12345","SUBS","Private","091","ENRO"`,
		`"MULTIAC01","1","N67890","DEST","Some Airline","121","TKOF"`,
		`"MULTIAC01","2","G-OABC","NONE","UK Operator","","CRSE"`,
	}, "\n") + "\n"

	extract := fakeExtractor(map[string]string{"aircraft": csv})
	n, err := ingestNTSBAircraft(context.Background(), tx, extract, "/fake.mdb", nil)
	if err != nil {
		t.Fatalf("ingestNTSBAircraft: %v", err)
	}
	if n != 3 {
		t.Errorf("inserted = %d, want 3", n)
	}

	// US tail should be normalized with N-prefix, foreign tail passes through.
	var us, foreign string
	if err := tx.QueryRow(`SELECT registration FROM event_aircraft WHERE event_id='ERA22LA001' AND aircraft_idx=1`).Scan(&us); err != nil {
		t.Fatalf("scan us tail: %v", err)
	}
	if us != "N12345" {
		t.Errorf("expected N12345 (prefixed), got %q", us)
	}
	if err := tx.QueryRow(`SELECT registration FROM event_aircraft WHERE event_id='MULTIAC01' AND aircraft_idx=2`).Scan(&foreign); err != nil {
		t.Fatalf("scan foreign tail: %v", err)
	}
	if foreign != "G-OABC" {
		t.Errorf("expected G-OABC (passthrough), got %q", foreign)
	}
}

func TestIngestNTSBNarrativesTruncated(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	if _, err := st.DB().Exec(`INSERT INTO events(event_id, event_date) VALUES('LONG01','2024-01-01')`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tx, err := st.DB().Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer tx.Rollback()

	longFactual := strings.Repeat("a", 2000)
	csv := strings.Join([]string{
		`"ev_id","Aircraft_Key","narr_accp","narr_accf","narr_cause","narr_inc"`,
		`"LONG01","1","","` + longFactual + `","",""`,
	}, "\n") + "\n"

	extract := fakeExtractor(map[string]string{"narratives": csv})
	n, err := ingestNTSBNarratives(context.Background(), tx, extract, "/fake.mdb", false, nil)
	if err != nil {
		t.Fatalf("ingestNTSBNarratives: %v", err)
	}
	if n != 1 {
		t.Errorf("inserted = %d, want 1", n)
	}

	var summary string
	var blob []byte
	if err := tx.QueryRow(`SELECT summary, full_zstd FROM narratives WHERE event_id='LONG01'`).Scan(&summary, &blob); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len([]rune(summary)) != MaxSummaryChars {
		t.Errorf("summary rune-length = %d, want %d", len([]rune(summary)), MaxSummaryChars)
	}
	if blob != nil {
		t.Errorf("expected NULL full_zstd when --full-narratives is false, got %d bytes", len(blob))
	}
}

func TestIngestNTSBNarrativesFullZstd(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	if _, err := st.DB().Exec(`INSERT INTO events(event_id, event_date) VALUES('FULL01','2024-01-01')`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tx, err := st.DB().Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer tx.Rollback()

	factual := "Pilot reported loss of engine power on takeoff roll and aborted. Aircraft sustained substantial damage when it overran the runway."
	csv := strings.Join([]string{
		`"ev_id","Aircraft_Key","narr_accp","narr_accf","narr_cause","narr_inc"`,
		`"FULL01","1","","` + factual + `","The probable cause was a failed fuel pump.",""`,
	}, "\n") + "\n"

	extract := fakeExtractor(map[string]string{"narratives": csv})
	n, err := ingestNTSBNarratives(context.Background(), tx, extract, "/fake.mdb", true, nil)
	if err != nil {
		t.Fatalf("ingestNTSBNarratives: %v", err)
	}
	if n != 1 {
		t.Errorf("inserted = %d, want 1", n)
	}

	var summary string
	var blob []byte
	if err := tx.QueryRow(`SELECT summary, full_zstd FROM narratives WHERE event_id='FULL01'`).Scan(&summary, &blob); err != nil {
		t.Fatalf("scan: %v", err)
	}
	// Summary should be the probable cause (preferred over factual when present).
	if !strings.Contains(summary, "probable cause") {
		t.Errorf("summary should contain probable cause, got %q", summary)
	}
	if len(blob) == 0 {
		t.Errorf("expected non-empty zstd blob with --full-narratives, got empty")
	}
	// zstd magic bytes: 0x28 0xB5 0x2F 0xFD
	if len(blob) < 4 || blob[0] != 0x28 || blob[1] != 0xB5 || blob[2] != 0x2F || blob[3] != 0xFD {
		t.Errorf("blob does not start with zstd magic; first 4 bytes: %x", blob[:4])
	}
}

func TestNTSBDate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2024-06-15 14:30:00", "2024-06-15"},
		{"2022-01-04", "2022-01-04"},
		{"6/15/2024", "2024-06-15"},
		{"06/15/2024 14:30:00", "2024-06-15"},
		{"", ""},
		{"not-a-date", "not-a-date"},
	}
	for _, c := range cases {
		if got := ntsbDate(c.in); got != c.want {
			t.Errorf("ntsbDate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeNNumberMaybe(t *testing.T) {
	cases := []struct{ in, want string }{
		{"12345", "N12345"},
		{"N628TS", "N628TS"},
		{"G-OABC", "G-OABC"},
		{"JA8119", "JA8119"},
		{"VH-XYZ", "VH-XYZ"},
		{"", ""},
		{"  n123  ", "N123"},
	}
	for _, c := range cases {
		if got := normalizeNNumberMaybe(c.in); got != c.want {
			t.Errorf("normalizeNNumberMaybe(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
