package cli

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/lexis-plus/internal/cliutil"
)

// A pin to one page must not pass a quotation whose words also sit on another page.
func TestPinMustCoverWholeQuotation(t *testing.T) {
	d, err := parseLexisDocx(writeTestDocx(t))
	if err != nil {
		t.Fatal(err)
	}
	q := "[A] widget is not a service . . . The buyer may reject a late widget"
	r := d.checkQuote("", q, "101")
	if r.Pin == nil || r.Pin.Matches == nil || *r.Pin.Matches {
		t.Fatalf("pin 101 for a quotation on pages 101-102 should not match: %+v", r.Pin)
	}
	if !strings.Contains(r.Pin.Note, "only part") {
		t.Errorf("note should say the pin covers only part of the quotation: %q", r.Pin.Note)
	}
	if r := d.checkQuote("", q, "101-102"); r.Pin == nil || r.Pin.Matches == nil || !*r.Pin.Matches {
		t.Errorf("pin 101-102 should match: %+v", r.Pin)
	}
}

// A near match must stay inside one section, so the source text and speaker come from one place.
func TestNearMatchStaysInOneSection(t *testing.T) {
	d, err := parseLexisDocx(writeTestDocx(t))
	if err != nil {
		t.Fatal(err)
	}
	// Words from the end of the majority run straight into words from the dissent.
	q := "without giving notice to anyone at all I would hold that a widget is always a service"
	frags, _, _ := parseQuote(q)
	for _, h := range d.nearHits(frags[0]) {
		if d.toks[h.start].seg != d.toks[h.end-1].seg {
			t.Fatalf("near hit spans sections %d and %d", d.toks[h.start].seg, d.toks[h.end-1].seg)
		}
	}
	_ = d.checkQuote("", q, "102") // must not panic
}

func TestStatuteListsKeepEverySection(t *testing.T) {
	got := func(text string) []string {
		var out []string
		for _, c := range extractCites(text) {
			out = append(out, c.Cite)
		}
		return out
	}
	cases := []struct {
		text string
		want []string
	}{
		{"Lab. Code §§ 1102.5, 1194, and 1198.5", []string{"Cal Lab Code 1102.5", "Cal Lab Code 1194", "Cal Lab Code 1198.5"}},
		{"Lab. Code, §§ 1102.5-1102.6", []string{"Cal Lab Code 1102.5", "Cal Lab Code 1102.6"}},
		{"Gov. Code sections 12940 and 12945", []string{"Cal Gov Code 12940", "Cal Gov Code 12945"}},
		{"Lab. Code § 1102.5, 98 Cal.App.4th 1", []string{"98 Cal.App.4th 1", "Cal Lab Code 1102.5"}},
		{"Lab. Code §§ 1102.5, 98 Cal.App.4th 1", []string{"98 Cal.App.4th 1", "Cal Lab Code 1102.5"}},
	}
	for _, c := range cases {
		if g := got(c.text); fmt.Sprint(g) != fmt.Sprint(c.want) {
			t.Errorf("%q: got %v, want %v", c.text, g, c.want)
		}
	}
}

// brief cites/check/gaps must see citations that appear only in footnotes.
func TestBriefCitesReadsFootnotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "brief.docx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	files := map[string]string{
		"word/document.xml":  `<w:document xmlns:w="x"><w:body><w:p><w:r><w:t>See Tameny v. Atlantic Richfield Co. (1980) 27 Cal.3d 167.</w:t></w:r></w:p></w:body></w:document>`,
		"word/footnotes.xml": `<w:footnotes xmlns:w="x"><w:footnote w:id="2"><w:p><w:r><w:t>1 Accord, Foley v. Interactive Data Corp. (1988) 47 Cal.3d 654; Lab. Code, § 1102.5.</w:t></w:r></w:p></w:footnote></w:footnotes>`,
	}
	for name, body := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	cs, err := briefCites(path)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, c := range cs {
		have[c.Cite] = true
	}
	for _, want := range []string{"27 Cal.3d 167", "47 Cal.3d 654", "Cal Lab Code 1102.5"} {
		if !have[want] {
			t.Errorf("missing %s in %v", want, cs)
		}
	}
}

func TestUniquePullName(t *testing.T) {
	dir := t.TempDir()
	if n := uniquePullName(dir, "x 2026-11-24 101500", ".docx"); n != "x 2026-11-24 101500" {
		t.Fatalf("got %q", n)
	}
	_ = os.WriteFile(filepath.Join(dir, "x 2026-11-24 101500.docx"), nil, 0o600)
	if n := uniquePullName(dir, "x 2026-11-24 101500", ".docx"); n != "x 2026-11-24 101500 (2)" {
		t.Fatalf("got %q", n)
	}
}

// Separate homes keep separate retrieval logs, red-flag reports, and purge targets.
func TestHistoryFollowsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LEXIS_PLUS_HOME", home)
	t.Setenv("LEXIS_PLUS_LOG", "")
	if legacyLexisDir() != "" {
		t.Fatal("legacy location must not be used under a home override")
	}
	if !strings.HasPrefix(logPath(), home) || !strings.HasPrefix(redFlagCachePath("27 Cal.3d 167"), home) {
		t.Fatalf("paths escape the home: %s, %s", logPath(), redFlagCachePath("27 Cal.3d 167"))
	}
	if err := appendLog(logEntry{Cite: "27 Cal.3d 167", Kind: "case", Path: "/tmp/x.docx", Format: "docx", Matter: "A"}); err != nil {
		t.Fatal(err)
	}
	if got := readLog(); len(got) != 1 || got[0].Matter != "A" {
		t.Fatalf("readLog = %+v", got)
	}
	targets, err := collectLocalDataTargets(map[string]bool{localDataKindHistory: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Path != logPath() {
		t.Fatalf("purge targets = %+v", targets)
	}
}

func TestCiteNotFoundIsDistinctFromRequestFailure(t *testing.T) {
	if !isCiteNotFound(fmt.Errorf("wrapped: %w", &citeNotFoundError{"x"})) {
		t.Error("a wrapped not-found error should still be recognized")
	}
	if isCiteNotFound(errors.New("Lexis+ GET /search/ returned HTTP 503")) || isCiteNotFound(authErr(errors.New("expired"))) {
		t.Error("request failures must not read as unrecognized citations")
	}
}

func TestLocalModeNeverContactsLexis(t *testing.T) {
	if _, err := newLexisSession(&rootFlags{dataSource: "local"}); err == nil || !strings.Contains(err.Error(), "--data-source local") {
		t.Fatalf("want a local-mode refusal, got %v", err)
	}
}

// A number after a list that has ended ("30 days") is prose, not another section.
func TestStatuteListStopsAtProse(t *testing.T) {
	var got []string
	for _, c := range extractCites("Under Lab. Code sections 201 and 202, 30 days of penalties are available, and 50 percent more.") {
		got = append(got, c.Cite)
	}
	if fmt.Sprint(got) != "[Cal Lab Code 201 Cal Lab Code 202]" {
		t.Errorf("got %v", got)
	}
	got = nil
	for _, c := range extractCites("Lab. Code §§ 1102.5(a), 1194, and 1198.5(b).") {
		got = append(got, c.Cite)
	}
	if fmt.Sprint(got) != "[Cal Lab Code 1102.5 Cal Lab Code 1194 Cal Lab Code 1198.5]" {
		t.Errorf("subdivisions: got %v", got)
	}
}

// Context for a later section of a list comes from the list itself, not from other uses of the number.
func TestStatuteSpansIgnoreUnrelatedNumbers(t *testing.T) {
	text := strings.Join(strings.Fields("The fee was $1194 and the case is at page 1194. Lab. Code §§ 1102.5, 1194 applies."), " ")
	spans := statuteSpans(text)["Cal Lab Code 1194"]
	if len(spans) != 1 {
		t.Fatalf("want one span, got %v", spans)
	}
	sp := spans[0]
	if text[sp[0]:sp[1]] != "1194" || !strings.Contains(text[sp[0]-12:sp[0]], "1102.5, ") {
		t.Errorf("span points at %q in %q", text[sp[0]:sp[1]], text[sp[0]-12:sp[1]])
	}
}

// Commands that first touch the new history location together must not lose each other's records.
func TestLegacyMigrationDoesNotLoseConcurrentWrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	// Every variable os.UserConfigDir and the path helpers read on any platform points into the
	// test directory, so this test can never touch a real history file (Windows reads APPDATA).
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Setenv("USERPROFILE", home)
	for _, k := range []string{"LEXIS_PLUS_HOME", "LEXIS_PLUS_DATA_DIR", "LEXIS_PLUS_LOG"} {
		t.Setenv(k, "")
	}
	old := legacyLexisDir()
	if old == "" {
		t.Skip("no legacy location on this platform")
	}
	// Refuse to write anywhere outside the test directory, whatever the platform resolved.
	if rel, err := filepath.Rel(home, old); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("legacy location %q is outside the test directory %q; refusing to write", old, home)
	}
	if dd, err := cliutil.DataDir(); err != nil || !strings.HasPrefix(dd, home) {
		t.Fatalf("data directory %q is outside the test directory %q; refusing to write", dd, home)
	}
	if err := os.MkdirAll(old, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "retrievals.jsonl"), []byte(`{"cite":"old 1 X 1","kind":"case","path":"/x","format":"docx"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const n = 24
	done := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			done <- appendLog(logEntry{Cite: fmt.Sprintf("%d Test 1", i), Kind: "case", Path: "/x", Format: "docx"})
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if got := len(readLog()); got != n+1 {
		t.Fatalf("history has %d records, want %d (old record plus every concurrent write)", got, n+1)
	}
}
