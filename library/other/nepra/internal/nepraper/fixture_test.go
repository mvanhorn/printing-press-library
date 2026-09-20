package nepraper

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The .spans.tsv fixtures are trimmed text-layer captures of the real NEPRA
// PDFs: a handful of pages each, one line per glyph, with the exact
// coordinates and advance widths the PDF content stream drew them at. The
// multi-megabyte PDFs themselves are not committed.
//
// Loading them through PageFromSpans means the fixtures exercise the same line
// grouping, cell gluing and content-stream ordering that ExtractText uses; only
// the pdf-library call is skipped. Page numbers are the original 1-indexed PDF
// page numbers, so provenance assertions match the real documents.
type fixture struct {
	FY       string
	Pages    int
	Creator  string
	Producer string
	Source   string
	Kept     []int
	Doc      *Doc
}

func loadFixture(t *testing.T, name string) *fixture {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	fx := &fixture{}
	spans := map[int][]Span{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if text == "" {
			continue
		}
		cols := strings.Split(text, "\t")
		if strings.HasPrefix(cols[0], "#") {
			if len(cols) < 2 {
				continue
			}
			switch cols[0] {
			case "#fy":
				fx.FY = cols[1]
			case "#pages":
				fx.Pages = atoiOrFail(t, cols[1])
			case "#creator":
				fx.Creator = cols[1]
			case "#producer":
				fx.Producer = cols[1]
			case "#source":
				fx.Source = cols[1]
			case "#page":
				fx.Kept = append(fx.Kept, atoiOrFail(t, cols[1]))
			}
			continue
		}
		if len(cols) != 5 {
			t.Fatalf("%s:%d: want 5 columns, got %d", name, line, len(cols))
		}
		pg := atoiOrFail(t, cols[0])
		spans[pg] = append(spans[pg], Span{
			X:    atofOrFail(t, cols[1]),
			Y:    atofOrFail(t, cols[2]),
			W:    atofOrFail(t, cols[3]),
			Text: cols[4],
			Page: pg,
		})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	fx.Doc = &Doc{NumPages: fx.Pages, Creator: fx.Creator, Producer: fx.Producer}
	for _, pg := range fx.Kept {
		fx.Doc.Pages = append(fx.Doc.Pages, PageFromSpans(pg, spans[pg]))
	}
	if len(fx.Doc.Pages) == 0 {
		t.Fatalf("%s: fixture declared no pages", name)
	}
	return fx
}

func atoiOrFail(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		t.Fatalf("bad int %q: %v", s, err)
	}
	return n
}

func atofOrFail(t *testing.T, s string) float64 {
	t.Helper()
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		t.Fatalf("bad float %q: %v", s, err)
	}
	return f
}

// report parses the fixture, failing the test on error.
func (fx *fixture) report(t *testing.T) *Report {
	t.Helper()
	r, err := ParseReliability(fx.Doc, fx.FY)
	if err != nil {
		t.Fatalf("ParseReliability(%s): %v", fx.FY, err)
	}
	return r
}

// find returns the observation for a key that came from a given table label,
// which is how a test pins a figure to the table that printed it.
func findObs(t *testing.T, r *Report, k Key, tableLabel string) Observation {
	t.Helper()
	for _, o := range Lookup(r, k) {
		if o.Prov.TableLabel == tableLabel {
			return o
		}
	}
	var have []string
	for _, o := range Lookup(r, k) {
		have = append(have, o.Prov.TableLabel+"="+o.Value.String())
	}
	t.Fatalf("no observation for %s from %s; have [%s]", k, tableLabel, strings.Join(have, " "))
	return Observation{}
}

func mustFloat(t *testing.T, v Value, what string) float64 {
	t.Helper()
	f, ok := v.Float()
	if !ok {
		t.Fatalf("%s: want a numeric value, got %s", what, v)
	}
	return f
}
