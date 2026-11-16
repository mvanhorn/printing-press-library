package cli

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestDocx builds a tiny Word file shaped like a Lexis+ case download (synthetic text, no Lexis content).
func writeTestDocx(t *testing.T) string {
	t.Helper()
	p := func(s string) string { return `<w:p><w:r><w:t xml:space="preserve">` + s + `</w:t></w:r></w:p>` }
	body := strings.Join([]string{
		p("CautionAs of: January 1, 2026 1:00 PM Z"),
		p("Alpha v. Beta"), p("Supreme Court of Testland"), p("May 1, 1990"), p("No. 1"),
		p("Reporter"), p("10 Test.3d 100 *; 200 Alt.2d 50 **"),
		p("Case Summary"), p("The court held that widgets are goods under the code."),
		p("Opinion by: SMITH"), p("Opinion"),
		`<w:p><w:r><w:t xml:space="preserve">[*101] We hold that HN1 a widget is not a service under the code</w:t></w:r>` +
			`<w:r><w:footnoteReference w:customMarkFollows="1" w:id="2" /><w:t>1</w:t></w:r>` +
			`<w:r><w:t xml:space="preserve"> and the seller must deliver it promptly.</w:t></w:r></w:p>`,
		p(" [*102] "), // a page marker alone in its own paragraph, as Lexis often writes it
		p("[**51] The buyer may reject a late widget without giving notice to anyone at all."),
		p("Dissent by: JONES"), p("Dissent"),
		p("[*103] I would hold that a widget is always a service and never a good."),
		p("End of Document"),
	}, "")
	doc := `<?xml version="1.0"?><w:document xmlns:w="x"><w:body>` + body + `</w:body></w:document>`
	fns := `<?xml version="1.0"?><w:footnotes xmlns:w="x"><w:footnote w:id="2"><w:p><w:r><w:t xml:space="preserve">1 Gadgets are a different matter entirely.</w:t></w:r></w:p></w:footnote></w:footnotes>`
	path := filepath.Join(t.TempDir(), "case.docx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, content := range map[string]string{"word/document.xml": doc, "word/footnotes.xml": fns} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return path
}

func TestCitecheckQuote(t *testing.T) {
	d, err := parseLexisDocx(writeTestDocx(t))
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != "case" || d.Title != "Alpha v. Beta" || d.FirstPage != "100" || d.Signal != "Caution" || len(d.Reporters) != 2 {
		t.Fatalf("header parsed wrong: %+v", d)
	}
	cases := []struct {
		name, quote, pin, status string
		pinOK                    bool
		flag, dev                string
	}{
		{"verbatim across the footnote call and HN marker", "a widget is not a service under the code and the seller must deliver it promptly", "101", "verbatim", true, "", ""},
		{"wrong pin", "The buyer may reject a late widget", "101", "verbatim", false, "", ""},
		{"dropped not", "We hold that a widget is a service under the code and the seller must deliver it promptly", "101", "differs", true, "", "not"},
		{"signaled", "[A] widget is not a service . . . The buyer may reject a late widget", "101-102", "verbatim_with_signaled_changes", true, "", ""},
		{"dissent", "a widget is always a service and never a good", "103", "verbatim", true, "DISSENT", ""},
		{"editorial", "widgets are goods under the code", "", "verbatim", false, "LEXIS CASE SUMMARY", ""},
		{"footnote", "Gadgets are a different matter entirely", "101, fn. 1", "verbatim", true, "", ""},
		{"fabricated", "sellers enjoy an absolute privilege to delay every delivery forever", "101", "not_found", false, "", ""},
	}
	for _, c := range cases {
		r := d.checkQuote("", c.quote, c.pin)
		if r.Status != c.status {
			t.Errorf("%s: status %q, want %q", c.name, r.Status, c.status)
		}
		if c.pin != "" && r.Pin != nil && (r.Pin.Matches == nil || *r.Pin.Matches != c.pinOK) && r.Status != "not_found" {
			t.Errorf("%s: pin matches=%v, want %v (%+v)", c.name, r.Pin.Matches, c.pinOK, r.Pin)
		}
		if c.flag != "" && !strings.Contains(strings.Join(r.Flags, "|"), c.flag) {
			t.Errorf("%s: flags %v, want one containing %q", c.name, r.Flags, c.flag)
		}
		if c.dev != "" {
			if len(r.Fragments) == 0 || len(r.Fragments[0].Deviations) != 1 || r.Fragments[0].Deviations[0].Source != c.dev {
				t.Errorf("%s: deviations %+v, want source word %q", c.name, r.Fragments, c.dev)
			}
		}
	}
	if got := d.pageText(102, 102); !strings.Contains(got, "The buyer may reject") || strings.Contains(got, "We hold") {
		t.Errorf("pageText(102) = %q", got)
	}
}

func TestBriefAuthorities(t *testing.T) {
	text := `(Tameny v. Atlantic Richfield Co. (1980) 27 Cal.3d 167, 178 [164 Cal.Rptr. 839, 610 P.2d 1330].) See (Tameny, supra, 27 Cal.3d at p. 172.)
Twombly, 550 U.S. 544, 570, 127 S.Ct. 1955 (2007); Foley v. Interactive Data Corp. (1988) 47 Cal.3d 654; Smith v. Jones, 2019 WL 1234567.`
	a, other := briefAuthorities(text)
	if len(a) != 3 {
		t.Fatalf("got %d authorities, want 3: %+v", len(a), a)
	}
	if a[0].Cite != "27 Cal.3d 167" || len(a[0].Parallels) != 2 || len(a[0].Seen) != 2 || a[0].Seen[0].Pin != "178" || a[0].Seen[1].Pin != "172" || !a[0].Seen[1].Short {
		t.Errorf("Tameny parsed wrong: %+v", *a[0])
	}
	if a[1].Cite != "550 U.S. 544" || len(a[1].Parallels) != 1 || a[1].Seen[0].Pin != "570" {
		t.Errorf("Twombly parsed wrong: %+v", *a[1])
	}
	if a[2].Cite != "47 Cal.3d 654" || a[2].Seen[0].Pin != "" {
		t.Errorf("Foley parsed wrong: %+v", *a[2])
	}
	if len(other) != 1 || other[0] != "2019 WL 1234567" {
		t.Errorf("other = %v", other)
	}
}
