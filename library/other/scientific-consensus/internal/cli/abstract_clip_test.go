// Hand-authored tests for the truncation marker on study abstracts. They call
// the production clipAbstract and marshal the production workBrief, so a
// regression in either surfaces here.
package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/scengine"
)

const abstractEllipsis = "…"

// longWordsAbstract builds an abstract of at least n runes made of
// space-separated words, so a word boundary is always near the cap.
func longWordsAbstract(n int) string {
	var b strings.Builder
	for i := 0; utf8.RuneCountInString(b.String()) < n; i++ {
		b.WriteString("disagreement resolved in study group ")
	}
	return b.String()
}

func TestClipAbstractShortIsUnchanged(t *testing.T) {
	in := "A short abstract."
	got, truncated := clipAbstract(in)
	if got != in || truncated {
		t.Fatalf("clipAbstract(short) = %q, %v; want input unchanged, false", got, truncated)
	}
}

func TestClipAbstractExactlyAtCapIsUnchanged(t *testing.T) {
	// The multibyte case is over the cap in bytes but not in runes, so it
	// exercises the rune check rather than the byte-length fast path.
	for _, in := range []string{
		strings.Repeat("a", maxAbstractChars),
		strings.Repeat("é", maxAbstractChars),
	} {
		got, truncated := clipAbstract(in)
		if got != in || truncated {
			t.Fatalf("clipAbstract(%d runes, %d bytes) truncated=%v len=%d; want unchanged, false",
				maxAbstractChars, len(in), truncated, utf8.RuneCountInString(got))
		}
	}
}

func TestClipAbstractCutsAtWordBoundary(t *testing.T) {
	in := longWordsAbstract(maxAbstractChars + 300)
	got, truncated := clipAbstract(in)
	if !truncated {
		t.Fatal("truncated = false; want true for an over-cap abstract")
	}
	if n := utf8.RuneCountInString(got); n > maxAbstractChars {
		t.Fatalf("result has %d runes; want <= %d including the marker", n, maxAbstractChars)
	}
	if !strings.HasSuffix(got, abstractEllipsis) {
		t.Fatalf("result does not end with %q: ...%q", abstractEllipsis, tailRunes(got, 20))
	}
	body := strings.TrimSuffix(got, abstractEllipsis)
	if !strings.HasPrefix(in, body) {
		t.Fatal("text before the marker is not a prefix of the input")
	}
	next, _ := utf8.DecodeRuneInString(in[len(body):])
	if !unicode.IsSpace(next) {
		t.Fatalf("cut is mid-word: next input rune is %q after ...%q", next, tailRunes(body, 20))
	}
}

func TestClipAbstractNoWhitespaceCutsOnRune(t *testing.T) {
	in := strings.Repeat("x", 2000)
	got, truncated := clipAbstract(in)
	if !truncated {
		t.Fatal("truncated = false; want true")
	}
	if n := utf8.RuneCountInString(got); n != maxAbstractChars {
		t.Fatalf("result has %d runes; want exactly %d", n, maxAbstractChars)
	}
	if !strings.HasSuffix(got, abstractEllipsis) {
		t.Fatalf("result does not end with %q", abstractEllipsis)
	}
}

func TestClipAbstractMultibyteStaysValid(t *testing.T) {
	in := strings.Repeat("Élettani vizsgálat 研究結果は有意 ", 120)
	if utf8.RuneCountInString(in) <= maxAbstractChars {
		t.Fatal("fixture is not over the cap")
	}
	got, truncated := clipAbstract(in)
	if !truncated {
		t.Fatal("truncated = false; want true")
	}
	if !utf8.ValidString(got) {
		t.Fatal("result is not valid UTF-8")
	}
	if n := utf8.RuneCountInString(got); n > maxAbstractChars {
		t.Fatalf("result has %d runes; want <= %d", n, maxAbstractChars)
	}
	if !strings.HasSuffix(got, abstractEllipsis) {
		t.Fatalf("result does not end with %q", abstractEllipsis)
	}
}

func TestWorkBriefAbstractTruncatedJSON(t *testing.T) {
	plain, err := json.Marshal(workBrief{Title: "t", Abstract: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "abstract_truncated") {
		t.Fatalf("untruncated brief carries abstract_truncated: %s", plain)
	}
	marked, err := json.Marshal(workBrief{Title: "t", Abstract: "a…", AbstractTruncated: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(marked), `"abstract_truncated":true`) {
		t.Fatalf("truncated brief lacks \"abstract_truncated\":true: %s", marked)
	}
}

func TestBriefBuildersMarkTruncatedAbstract(t *testing.T) {
	long := longWordsAbstract(maxAbstractChars + 300)
	stances := []workStance{
		{Work: scWork{Title: "long", Abstract: long}, Stance: scengine.StanceSupporting},
		{Work: scWork{Title: "short", Abstract: "brief"}, Stance: scengine.StanceSupporting},
	}
	check := func(name string, briefs []workBrief) {
		t.Helper()
		for _, b := range briefs {
			want := b.Title == "long"
			if b.AbstractTruncated != want {
				t.Errorf("%s: %q AbstractTruncated = %v; want %v", name, b.Title, b.AbstractTruncated, want)
			}
			if want && !strings.HasSuffix(b.Abstract, abstractEllipsis) {
				t.Errorf("%s: %q abstract lacks the marker", name, b.Title)
			}
		}
	}
	check("topByStance", topByStance(stances, scengine.StanceSupporting, 5))
	check("allStudyBriefs", allStudyBriefs(stances))
	check("retractedBriefs", retractedBriefs([]scWork{stances[0].Work, stances[1].Work}))
}

func tailRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}
