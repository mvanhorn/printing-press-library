package cli

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
