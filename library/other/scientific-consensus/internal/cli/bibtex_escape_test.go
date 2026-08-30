// Hand-authored tests for the BibTeX renderer: field-value escaping and
// citation-key normalization. Unescaped OpenAlex values and DOI-derived keys
// carrying illegal characters corrupt the export on import into Zotero.
// Not generated.
package cli

import (
	"strings"
	"testing"
)

func TestBibtexEscapeValue(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"braces dropped", "Jane {Doe}", "Jane Doe"},
		{"backslash dropped", "A\\B Smith", "AB Smith"},
		{"ampersand and percent escaped", "Health & Safety 50% rule", "Health \\& Safety 50\\% rule"},
		{"dollar hash underscore escaped", "a$b#c_d", "a\\$b\\#c\\_d"},
		{"newline becomes space", "Line one\nLine two", "Line one Line two"},
		{"cr and tab become spaces", "A\r\n\tB", "A B"},
		{"whitespace collapsed and trimmed", "  A   B  ", "A B"},
		{"typographic quotes preserved", "\u201cQuoted\u201d \u2018x\u2019", "\u201cQuoted\u201d \u2018x\u2019"},
	} {
		if got := bibtexEscapeValue(tc.in); got != tc.want {
			t.Errorf("%s: bibtexEscapeValue(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// TestBibtexCiteKeyParenthesisedDOI pins the real production DOI whose
// parentheses previously landed verbatim in the citation key.
func TestBibtexCiteKeyParenthesisedDOI(t *testing.T) {
	got := bibtexCiteKey("10.1016/s2213-8587(21)00051-6")
	if want := "10_1016_s2213-8587_21_00051-6"; got != want {
		t.Errorf("bibtexCiteKey = %q, want %q", got, want)
	}
	if strings.ContainsAny(got, "().,/") {
		t.Errorf("key %q still carries characters illegal in a BibTeX key", got)
	}
	if got := bibtexCiteKey("///...///"); got != "" {
		t.Errorf("all-illegal DOI: got %q, want empty so the caller falls back to refN", got)
	}
}

// TestRenderCurateBibtexEscapingAndKeys covers the rendered entry end to end:
// every interpolated value is escaped, the key is legal, colliding keys are
// disambiguated in order, and a DOI-less entry falls back to refN.
func TestRenderCurateBibtexEscapingAndKeys(t *testing.T) {
	var sb strings.Builder
	renderCurateBibtex(&sb, []curateItem{
		{
			Rank:    1,
			Title:   "Cost & benefit of 5% {supplementation}",
			Authors: []string{"Jane {Doe}", "A\\B Smith", "Line\nBreak"},
			Year:    2021,
			DOI:     "10.1016/s2213-8587(21)00051-6",
			Venue:   "Journal of A & B",
		},
		// These three DOIs are distinct but normalize onto the same key.
		{Rank: 2, Title: "First", Year: 2022, DOI: "10.1/a(b"},
		{Rank: 3, Title: "Second", Year: 2022, DOI: "10.1/a.b"},
		{Rank: 4, Title: "Third", Year: 2022, DOI: "10.1/a_b"},
		{Rank: 5, Title: "No DOI", Year: 2023},
	})
	got := sb.String()

	for _, want := range []string{
		"@article{10_1016_s2213-8587_21_00051-6,",
		"  title = {Cost \\& benefit of 5\\% supplementation},",
		"  author = {Jane Doe and AB Smith and Line Break},",
		"  journal = {Journal of A \\& B},",
		"  doi = {10.1016/s2213-8587(21)00051-6},",
		"@article{10_1_a_b,",
		"@article{10_1_a_b_2,",
		"@article{10_1_a_b_3,",
		"@article{ref5,",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("BibTeX output missing %q:\n%s", want, got)
		}
	}

	// No entry key may carry a character that is illegal in a BibTeX key.
	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "@article{") {
			continue
		}
		key := strings.TrimSuffix(strings.TrimPrefix(line, "@article{"), ",")
		for _, r := range key {
			ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_'
			if !ok {
				t.Errorf("key %q contains illegal character %q", key, r)
			}
		}
	}
}
