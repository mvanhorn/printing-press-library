package nepraper

import (
	"errors"
	"math"
	"os"
	"strings"
	"testing"
)

func TestExtractTextRejectsNonPDF(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
	}{
		// NEPRA's own index links to a FY2023-24 PER that 404s with a
		// 9-byte HTML body. Handing that to a PDF reader must fail loudly
		// rather than yield an empty document that looks like "no data".
		{"nepra 404 body", []byte("Not Found")},
		{"empty", nil},
		{"html", []byte("<html><body>Forbidden</body></html>")},
		{"truncated header", []byte("%PD")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := ExtractText(tc.in)
			if err == nil {
				t.Fatalf("want an error, got doc with %d pages", doc.NumPages)
			}
			if !errors.Is(err, ErrNotPDF) {
				t.Fatalf("want ErrNotPDF, got %v", err)
			}
		})
	}
}

// TestExtractTextSyntheticPER runs the extractor over a hand-built PDF that
// reproduces, construct for construct, the layouts the real PERs use. Each
// assertion below corresponds to a way a naive extractor gets these documents
// wrong.
func TestExtractTextSyntheticPER(t *testing.T) {
	b, err := os.ReadFile("testdata/synthetic-per.pdf")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ExtractText(b)
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}

	if doc.NumPages != 2 {
		t.Errorf("NumPages = %d, want 2 (read from the page tree, not from file(1))", doc.NumPages)
	}
	if len(doc.Pages) != 2 {
		t.Fatalf("len(Pages) = %d, want 2", len(doc.Pages))
	}
	if doc.Creator != "Nitro Pro 8" {
		t.Errorf("Creator = %q, want %q", doc.Creator, "Nitro Pro 8")
	}
	if got := doc.LowTextPages(20); len(got) != 0 {
		t.Errorf("LowTextPages(20) = %v, want none: every page has a real text layer", got)
	}
	if doc.CharCount() < 500 {
		t.Errorf("CharCount = %d, implausibly low for a real text layer", doc.CharCount())
	}

	lines := map[int][]string{}
	for _, p := range doc.Pages {
		for _, l := range p.Lines {
			lines[p.Number] = append(lines[p.Number], l.Text())
		}
	}
	cases := []struct {
		name string
		page int
		want string
		why  string
	}{
		{
			name: "cell split into two glyph runs is reassembled",
			page: 1, want: "PESCO 16696.51 17358.60 0.00",
			why: `the breach cell is drawn as "0." then "00"; without geometric ` +
				`gluing it reads as two cells`,
		},
		{
			name: "cell split into three glyph runs is reassembled",
			page: 1, want: "MEPCO 31419.30 9704.00 21,715.3 0",
			why: `"21" ",7" "15.3" must glue into one number, and the trailing 0 ` +
				`is a chart axis label on the same baseline`,
		},
		{
			name: "adjacent numeric cells are NOT glued together",
			page: 1, want: "GEPCO 45.19 14 31.19",
			why: "character-class gluing would fuse 45.19 and 14 into 45.1914",
		},
		{
			name: "weighted-average row survives as its own line",
			page: 1, want: "W. Av: 17.923 16.181 1.742",
			why: "the summary row is drawn as two runs, W. then Av:",
		},
		{
			name: "zero-advance-width cells are read in content-stream order",
			page: 2, want: "MEPCO 39.733 2794 4723.73 3726.61 1182.56",
			why: "these glyphs are all drawn at one pen position with float noise; " +
				"sorting on raw x turns 1182.56 into 5618.21",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !containsLine(lines[tc.page], tc.want) {
				t.Errorf("page %d has no line %q (%s)\ngot:\n  %s",
					tc.page, tc.want, tc.why, strings.Join(lines[tc.page], "\n  "))
			}
		})
	}
}

func containsLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

// TestExtractedCorpusShape asserts the measured shape of the real reports
// against the committed fixtures: the page count comes from the PDF page tree,
// not from file(1), which reports "1 pages" for six of the seven files.
func TestExtractedCorpusShape(t *testing.T) {
	cases := []struct {
		fixture  string
		fy       string
		pages    int
		creator  string
		producer string
	}{
		{"fy2014-15.spans.tsv", "FY2014-15", 27, "Nitro Pro 8", ""},
		{"fy2018-19.spans.tsv", "FY2018-19", 28, "Nitro Pro 8", ""},
		{"fy2024-25.spans.tsv", "FY2024-25", 35, "", "iLovePDF"},
	}
	for _, tc := range cases {
		t.Run(tc.fy, func(t *testing.T) {
			fx := loadFixture(t, tc.fixture)
			if fx.FY != tc.fy {
				t.Errorf("fixture FY = %q, want %q", fx.FY, tc.fy)
			}
			if fx.Doc.NumPages != tc.pages {
				t.Errorf("NumPages = %d, want %d", fx.Doc.NumPages, tc.pages)
			}
			if fx.Doc.Creator != tc.creator {
				t.Errorf("Creator = %q, want %q", fx.Doc.Creator, tc.creator)
			}
			if fx.Doc.Producer != tc.producer {
				t.Errorf("Producer = %q, want %q", fx.Doc.Producer, tc.producer)
			}
			for _, p := range fx.Doc.Pages {
				if p.CharCount() < 20 {
					t.Errorf("page %d has %d chars: the corpus has a real text "+
						"layer on every content page", p.Number, p.CharCount())
				}
			}
		})
	}
}

func TestSourceRegistry(t *testing.T) {
	// FY2023-24 must be representable as UNAVAILABLE, which is not the same
	// as "the report reports nothing".
	v := AvailabilityFor("FY2023-24")
	if v.Kind != KindUnavailable {
		t.Errorf("AvailabilityFor(FY2023-24).Kind = %s, want %s", v.Kind, KindUnavailable)
	}
	if !strings.Contains(v.Reason, "404") {
		t.Errorf("FY2023-24 reason should say why it is unavailable, got %q", v.Reason)
	}
	if got := AvailabilityFor("FY2018-19"); got.Kind != KindAbsent {
		t.Errorf("AvailabilityFor(FY2018-19).Kind = %s, want %s (it is published)",
			got.Kind, KindAbsent)
	}

	// The FY2020-21 URL really does end in a trailing encoded space. Removing
	// it returns 404, so the path must not be "cleaned".
	s, ok := SourceFor("FY2020-21")
	if !ok {
		t.Fatal("no source record for FY2020-21")
	}
	if !strings.HasSuffix(s.URLPath, "Companies%20.pdf") {
		t.Errorf("FY2020-21 URLPath = %q, want it to keep the trailing %%20 before .pdf", s.URLPath)
	}
	if s.Bytes != 2277838 {
		t.Errorf("FY2020-21 Bytes = %d, want 2277838", s.Bytes)
	}
	if len(s.LowTextPages) != 1 || s.LowTextPages[0] != 1 {
		t.Errorf("FY2020-21 LowTextPages = %v, want [1] (the cover)", s.LowTextPages)
	}

	// Every retrievable report has a real text layer; six of seven were made
	// with Nitro Pro 8.
	nitro, published := 0, 0
	for _, rec := range Sources() {
		if rec.Availability != AvailabilityPublished {
			continue
		}
		published++
		if !rec.HasTextLayer {
			t.Errorf("%s: HasTextLayer is false; no report in this corpus needs OCR", rec.FY)
		}
		if rec.Pages <= 1 {
			t.Errorf("%s: Pages = %d; file(1) reports 1 page for most of these and is wrong",
				rec.FY, rec.Pages)
		}
		if rec.Creator == "Nitro Pro 8" {
			nitro++
		}
	}
	if published != 7 {
		t.Errorf("published reports = %d, want 7", published)
	}
	if nitro != 6 {
		t.Errorf("reports created by Nitro Pro 8 = %d, want 6 of 7", nitro)
	}
}

func TestKnownUnverifiedIsNeverANumber(t *testing.T) {
	got := KnownUnverified()
	if len(got) != 3 {
		t.Fatalf("KnownUnverified() has %d entries, want 3", len(got))
	}
	wantRaw := map[string]bool{"19,535.": true, "28,189.": true, "15,896.": true}
	for _, u := range got {
		if !wantRaw[u.RawLabel] {
			t.Errorf("unexpected raw label %q", u.RawLabel)
		}
		v := u.UnverifiedValue()
		if v.Kind != KindUnverified {
			t.Errorf("%s: Kind = %s, want %s", u.RawLabel, v.Kind, KindUnverified)
		}
		if _, ok := v.Float(); ok {
			t.Errorf("%s: Float() returned a number; a truncated chart label must never "+
				"be parsed as one", u.RawLabel)
		}
		if v.Raw != u.RawLabel {
			t.Errorf("Raw = %q, want the truncated string %q verbatim", v.Raw, u.RawLabel)
		}
		// Independently: the parser must reach the same verdict from the raw
		// string alone.
		if p := ParseFigure(u.RawLabel); p.Kind != KindUnverified {
			t.Errorf("ParseFigure(%q).Kind = %s, want %s", u.RawLabel, p.Kind, KindUnverified)
		}
	}
}

func TestKnownArtifactsRecordDirection(t *testing.T) {
	arts := KnownArtifacts()
	if len(arts) < 4 {
		t.Fatalf("KnownArtifacts() has %d entries, want at least 4", len(arts))
	}
	byKey := map[Key]KnownArtifact{}
	for _, a := range arts {
		byKey[a.Key] = a
	}
	saidi := byKey[Key{PeriodFY: "FY2024-25", Entity: EntityMEPCO, Metric: MetricSAIDI}]
	if saidi.Direction != "unknowable" {
		t.Errorf("FY2024-25 MEPCO SAIDI Direction = %q, want %q: the two figures are each "+
			"triple-attested and the document has no tie-breaker", saidi.Direction, "unknowable")
	}
	if math.Abs(saidi.Ratio-2.9994249) > 1e-6 {
		t.Errorf("FY2024-25 MEPCO SAIDI Ratio = %v, want 2.9994249", saidi.Ratio)
	}
	thousand := byKey[Key{PeriodFY: "FY2020-21", Entity: EntityMEPCO, Metric: MetricSAIDI}]
	if !strings.HasPrefix(thousand.Direction, "inferred") {
		t.Errorf("FY2020-21 MEPCO SAIDI Direction = %q, want it marked as inferred, not proven",
			thousand.Direction)
	}
	if thousand.Ratio != 1000 {
		t.Errorf("FY2020-21 MEPCO SAIDI Ratio = %v, want 1000", thousand.Ratio)
	}
}
