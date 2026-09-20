// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.

package cli

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures are two determination pages verbatim, gzipped. ipp-short-term
// is the smallest surface with a known published row count; disco-gepco is the
// page that carries every awkward row shape NEPRA emits.
func eventsFixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "events-"+name+".php.gz"))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip %s: %v", name, err)
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompress %s: %v", name, err)
	}
	return string(out)
}

// TestEventsExtractShortTerm pins the smallest surface against the published
// row count and date range measured on the live site.
func TestEventsExtractShortTerm(t *testing.T) {
	rows, skipped, err := eventsExtract("ipp-short-term", eventsFixture(t, "ipp-short-term"),
		"https://nepra.org.pk/tariff/Generation%20IPPs%20Short%20Term.php")
	if err != nil {
		t.Fatalf("eventsExtract: %v", err)
	}
	if len(rows) != 43 {
		t.Errorf("rows = %d, want 43 as published", len(rows))
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0 on this page", skipped)
	}

	groups := map[string]int{}
	var withDoc, withDocket, withISO int
	first, last := "9999-99-99", "0000-00-00"
	for _, r := range rows {
		groups[r.Group]++
		if r.DocumentURL != "" {
			withDoc++
		}
		if r.Docket != "" {
			withDocket++
		}
		if r.DateISO != "" {
			withISO++
			if r.DateISO < first {
				first = r.DateISO
			}
			if r.DateISO > last {
				last = r.DateISO
			}
		}
	}
	// Two named company accordions, as surveyed.
	if len(groups) != 2 {
		t.Errorf("accordion groups = %d (%v), want 2", len(groups), groups)
	}
	if withDoc != 43 {
		t.Errorf("rows with a document URL = %d, want all 43: the PDF is the payload", withDoc)
	}
	if withISO != 43 {
		t.Errorf("rows with a parsed date = %d, want 43", withISO)
	}
	// The measured range on the live page.
	if first != "2016-02-04" || last != "2020-02-26" {
		t.Errorf("date range = %s .. %s, want 2016-02-04 .. 2020-02-26", first, last)
	}
	// Dockets are the join keys, so their extraction is pinned.
	if withDocket != 42 {
		t.Errorf("rows naming a TRF docket = %d, want 42", withDocket)
	}
	// Every document URL must be absolute and keep NEPRA's own encoding.
	for _, r := range rows {
		if !strings.HasPrefix(r.DocumentURL, "https://nepra.org.pk/") {
			t.Errorf("document URL is not absolute: %q", r.DocumentURL)
			break
		}
	}
}

// TestEventsExtractAwkwardRowShapes is the row-shape regression guard. The
// GEPCO page carries all three shapes that a strict "first cell is a date"
// reader drops or mangles, and each of them is a real determination.
func TestEventsExtractAwkwardRowShapes(t *testing.T) {
	rows, skipped, err := eventsExtract("disco-gepco", eventsFixture(t, "disco-gepco"),
		"https://nepra.org.pk/tariff/Distribution%20GEPCO.php")
	if err != nil {
		t.Fatalf("eventsExtract: %v", err)
	}
	if len(rows) < 366 {
		t.Errorf("rows = %d, want at least 366; a stricter reader returned 362 and lost four", len(rows))
	}
	// Only genuine furniture may be skipped.
	if skipped > 4 {
		t.Errorf("skipped = %d, want at most 4 furniture rows", skipped)
	}

	// 1. A date carrying stray internal whitespace: GEPCO publishes
	//    "09-12- 2009", which ^dd-mm-yyyy$ rejects.
	var wsDate int
	for _, r := range rows {
		if strings.Contains(r.Date, " ") {
			wsDate++
			if r.DateISO == "" {
				t.Errorf("date %q has internal whitespace and did not parse", r.Date)
			}
		}
	}
	if wsDate == 0 {
		t.Error("no whitespace-bearing date found; this fixture is supposed to contain them")
	}

	// 2. Undated determinations are KEPT, flagged, and never given a guessed
	//    date — but they must carry real text, not the link's own label.
	var undated int
	for _, r := range rows {
		if !r.Undated {
			continue
		}
		undated++
		if r.DateISO != "" || r.Date != "" {
			t.Errorf("an undated row was given a date: %+v", r)
		}
		if r.DocumentURL == "" {
			t.Error("an undated row was kept without a document; it is furniture, not a determination")
		}
		if strings.EqualFold(strings.TrimSpace(r.Description), "view") {
			t.Error(`an undated row reported "View" as its description; the link label was read as the determination text`)
		}
		if len(r.Description) < 20 {
			t.Errorf("undated row description looks like furniture: %q", r.Description)
		}
	}
	if undated != 2 {
		t.Errorf("undated determinations = %d, want 2 on this page", undated)
	}

	// 3. No row anywhere may report the link label as its text.
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Description), "view") {
			t.Errorf("a row reported %q as its description", r.Description)
			break
		}
	}
	// A date that IS present must always parse, or it is a shape we do not
	// yet handle and must not be silently kept as unparsed.
	for _, r := range rows {
		if r.Date != "" && r.DateISO == "" {
			t.Errorf("date %q was kept but not parsed", r.Date)
			break
		}
	}
}

// TestEventsDateISONeverGuesses checks the date normaliser refuses rather than
// repairs. NEPRA publishes at least one impossible month.
func TestEventsDateISONeverGuesses(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"07-09-2026", "2026-09-07"},
		{"7-9-2026", "2026-09-07"},
		{"09-12- 2009", "2009-12-09"},
		{"11-09- 2009", "2009-09-11"},
		{"27-03-1999", "1999-03-27"},
		// Impossible dates must come back empty, never rolled over.
		{"13-13-2026", ""},
		{"32-01-2026", ""},
		{"00-00-2026", ""},
		{"", ""},
		{"not a date", ""},
		{"2026-09-07", ""}, // ISO input is not the published shape
	} {
		if got := eventDateISO(tc.in); got != tc.want {
			t.Errorf("eventDateISO(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestEventsFilterAndDedup covers the selectors and the duplicate policy.
func TestEventsFilterAndDedup(t *testing.T) {
	rows := []EventRow{
		{Surface: "s", Group: "Nishat Power", Date: "01-09-2026", DateISO: "2026-09-01", Description: "FPA", Docket: "TRF-71", DocumentURL: "u1"},
		{Surface: "s", Group: "Nishat Power", Date: "01-09-2026", DateISO: "2026-09-01", Description: "FPA", Docket: "TRF-71", DocumentURL: "u1"},
		{Surface: "s", Group: "Kot Addu", Date: "05-08-2026", DateISO: "2026-08-05", Description: "Quarterly", Docket: "TRF-600", DocumentURL: "u2"},
		{Surface: "s", Group: "Undated Co", Description: "no date here at all", DocumentURL: "u3", Undated: true},
	}

	if got := eventsCountDuplicates(rows); got != 1 {
		t.Errorf("duplicates = %d, want 1", got)
	}
	if got := len(eventsDedup(rows)); got != 3 {
		t.Errorf("deduped rows = %d, want 3", got)
	}
	if got := len(eventsFilter(rows, "nishat", "", "", "")); got != 2 {
		t.Errorf("--company nishat matched %d, want 2 (case-insensitive)", got)
	}
	if got := len(eventsFilter(rows, "", "", "", "TRF-600")); got != 1 {
		t.Errorf("--docket TRF-600 matched %d, want 1", got)
	}
	// A date window must EXCLUDE undated rows rather than guessing them in.
	win := eventsFilter(rows, "", "2026-01-01", "2026-12-31", "")
	for _, r := range win {
		if r.Undated {
			t.Error("an undated row passed a date-window filter; it cannot be placed in time")
		}
	}
	if len(win) != 3 {
		t.Errorf("date window matched %d, want 3 dated rows", len(win))
	}
	// With no window, the undated row is retained.
	if got := len(eventsFilter(rows, "", "", "", "")); got != 4 {
		t.Errorf("unfiltered rows = %d, want all 4 including the undated one", got)
	}
}

// TestEventsCatalogueIntegrity guards the surface list itself.
func TestEventsCatalogueIntegrity(t *testing.T) {
	if len(eventSurfaces) != 28 {
		t.Errorf("catalogued surfaces = %d, want 28", len(eventSurfaces))
	}
	ids := map[string]bool{}
	for _, s := range eventSurfaces {
		if s.ID == "" || s.Label == "" || s.Path == "" {
			t.Errorf("incomplete catalogue entry: %+v", s)
		}
		if ids[s.ID] {
			t.Errorf("duplicate surface id %q", s.ID)
		}
		ids[s.ID] = true
		if !strings.HasPrefix(s.Path, "/") {
			t.Errorf("%s path is not rooted: %q", s.ID, s.Path)
		}
		if s.RowsAsOf < 0 {
			t.Errorf("%s has a negative floor", s.ID)
		}
	}
	// The eleven ex-WAPDA DISCOs plus K-Electric must all be selectable.
	for _, d := range []string{"FESCO", "GEPCO", "HAZECO", "HESCO", "IESCO", "LESCO", "MEPCO", "PESCO", "QESCO", "SEPCO", "TESCO", "KE"} {
		got, err := eventsSelectSurfaces("", d, false)
		if err != nil || len(got) == 0 {
			t.Errorf("--disco %s selected nothing: %v", d, err)
		}
	}
	if _, err := eventsSelectSurfaces("", "NOPE", false); err == nil {
		t.Error("an unknown disco was accepted")
	}
	if _, err := eventsSelectSurfaces("nope", "", false); err == nil {
		t.Error("an unknown surface was accepted")
	}
	if all, err := eventsSelectSurfaces("", "", true); err != nil || len(all) != len(eventSurfaces) {
		t.Errorf("--all selected %d surfaces, want %d", len(all), len(eventSurfaces))
	}

	// A shortfall against a recorded floor must be reported; a zero floor
	// asserts nothing.
	short := eventsShortfalls([]eventsFetchStat{
		{Surface: "ipp-wind", Rows: 1028, FloorAsOf: 2440},
		{Surface: "ipp-wind-ok", Rows: 2440, FloorAsOf: 2440},
		{Surface: "ipp-waste", Rows: 0, FloorAsOf: 0},
	})
	if len(short) != 1 || short[0].Surface != "ipp-wind" {
		t.Errorf("shortfalls = %+v, want exactly the truncated wind page", short)
	}
}
