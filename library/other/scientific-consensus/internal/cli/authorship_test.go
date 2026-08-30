// Hand-authored tests pinning author/journal carry-through: OpenAlex
// authorships and primary_location.source must survive parsing into scWork,
// the workBrief study struct, the emitted JSON, and the BibTeX export. The
// downstream LLM synthesis invented first authors when these were dropped.
// Not generated.
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/scengine"
)

// fakeAuthorshipClient serves one canned /works envelope verbatim.
type fakeAuthorshipClient struct {
	payload string
}

func (f *fakeAuthorshipClient) Get(_ context.Context, _ string, _ map[string]string) (json.RawMessage, error) {
	return json.RawMessage(f.payload), nil
}

// multiAuthorPayload has three authorships in a deliberate non-alphabetical
// order and a primary_location.source, so order preservation is observable.
const multiAuthorPayload = `{"meta":{"count":1},"results":[{
  "id":"https://openalex.org/W1",
  "display_name":"Vitamin D supplementation and respiratory infection: a systematic review",
  "publication_year":2011,
  "cited_by_count":312,
  "type":"article",
  "primary_location":{"source":{"display_name":"The Lancet"}},
  "authorships":[
    {"author":{"display_name":"Zoë Q. Vandermeer"}},
    {"author":{"display_name":"A. Baker"}},
    {"author":{"display_name":"Miguel Ángel Ruiz-Ortega"}}
  ]}]}`

// noAuthorshipsPayload is a work with an empty authorships array — normal for
// some OpenAlex records, not an error.
const noAuthorshipsPayload = `{"meta":{"count":1},"results":[{
  "id":"https://openalex.org/W2",
  "display_name":"An anonymous editorial on vitamin D",
  "publication_year":2015,
  "cited_by_count":4,
  "type":"article",
  "primary_location":{"source":{"display_name":"Nature"}},
  "authorships":[]}]}`

// noSourcePayload has authorships but no primary_location.source.
const noSourcePayload = `{"meta":{"count":1},"results":[{
  "id":"https://openalex.org/W3",
  "display_name":"A preprint on vitamin D",
  "publication_year":2020,
  "cited_by_count":1,
  "type":"preprint",
  "primary_location":{},
  "authorships":[{"author":{"display_name":"Solo Author"}}]}]}`

func fetchOne(t *testing.T, payload string) scWork {
	t.Helper()
	works, _, err := fetchWorks(context.Background(), &fakeAuthorshipClient{payload: payload}, "vitamin d", "", "", 10)
	if err != nil {
		t.Fatalf("fetchWorks: %v", err)
	}
	if len(works) != 1 {
		t.Fatalf("got %d works, want 1", len(works))
	}
	return works[0]
}

func TestFetchWorksCarriesAuthorsAndJournal(t *testing.T) {
	w := fetchOne(t, multiAuthorPayload)
	want := []string{"Zoë Q. Vandermeer", "A. Baker", "Miguel Ángel Ruiz-Ortega"}
	if len(w.Authors) != len(want) {
		t.Fatalf("got %d authors %v, want %d", len(w.Authors), w.Authors, len(want))
	}
	for i := range want {
		if w.Authors[i] != want[i] {
			// Verbatim pass-through: no "et al.", no initials conversion.
			t.Errorf("author %d = %q, want %q", i, w.Authors[i], want[i])
		}
	}
	if w.Venue != "The Lancet" {
		t.Errorf("Venue = %q, want %q", w.Venue, "The Lancet")
	}
}

func TestFetchWorksNoAuthorships(t *testing.T) {
	w := fetchOne(t, noAuthorshipsPayload)
	if len(w.Authors) != 0 {
		t.Errorf("Authors = %v, want empty", w.Authors)
	}
	if w.FirstAuthor != "" {
		t.Errorf("FirstAuthor = %q, want empty", w.FirstAuthor)
	}
	if w.Venue != "Nature" {
		t.Errorf("Venue = %q, want %q", w.Venue, "Nature")
	}
}

func TestFetchWorksNoPrimaryLocationSource(t *testing.T) {
	w := fetchOne(t, noSourcePayload)
	if w.Venue != "" {
		t.Errorf("Venue = %q, want empty", w.Venue)
	}
	if len(w.Authors) != 1 || w.Authors[0] != "Solo Author" {
		t.Errorf("Authors = %v, want [Solo Author]", w.Authors)
	}
}

// TestWorkBriefJSONOmitsEmptyAuthorship pins the omitempty contract: a study
// with no authors and no journal emits neither key, rather than an empty
// string or a placeholder the LLM could mistake for a real author.
func TestWorkBriefJSONOmitsEmptyAuthorship(t *testing.T) {
	b, err := json.Marshal(workBrief{Title: "Untitled", CitedBy: 1})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if strings.Contains(got, `"authors"`) {
		t.Errorf("JSON contains authors key for an author-less study: %s", got)
	}
	if strings.Contains(got, `"journal"`) {
		t.Errorf("JSON contains journal key for a source-less study: %s", got)
	}
}

// TestStudyBriefsCarryAuthorsAndJournal pins the carry-through from the parsed
// work into both study-list builders used by consensus, compare, and
// controversies.
func TestStudyBriefsCarryAuthorsAndJournal(t *testing.T) {
	work := scWork{
		Title:   "Statins reduced mortality in a randomized controlled trial",
		Authors: []string{"Jane Roe", "John Doe"},
		Venue:   "New England Journal of Medicine",
		Year:    2011,
		CitedBy: 99,
	}
	stances := []workStance{{
		Work:       work,
		Stance:     scengine.StanceSupporting,
		Confidence: 0.9,
		Design:     scengine.DesignRCT,
	}}

	for _, tc := range []struct {
		name   string
		briefs []workBrief
	}{
		{"allStudyBriefs", allStudyBriefs(stances)},
		{"topByStance", topByStance(stances, scengine.StanceSupporting, 2)},
	} {
		if len(tc.briefs) != 1 {
			t.Fatalf("%s: got %d briefs, want 1", tc.name, len(tc.briefs))
		}
		b := tc.briefs[0]
		if len(b.Authors) != 2 || b.Authors[0] != "Jane Roe" || b.Authors[1] != "John Doe" {
			t.Errorf("%s: Authors = %v, want [Jane Roe John Doe]", tc.name, b.Authors)
		}
		if b.Journal != "New England Journal of Medicine" {
			t.Errorf("%s: Journal = %q, want the source display name", tc.name, b.Journal)
		}
	}
}

// TestRenderCurateBibtexMultiAuthor pins the BibTeX rendering: multiple
// authors join with " and ", and an author-less entry omits the field.
func TestRenderCurateBibtexMultiAuthor(t *testing.T) {
	var sb strings.Builder
	renderCurateBibtex(&sb, []curateItem{
		{
			Rank:    1,
			Title:   "Vitamin D and respiratory infection",
			Authors: []string{"Zoë Q. Vandermeer", "A. Baker", "Miguel Ángel Ruiz-Ortega"},
			Year:    2011,
			DOI:     "10.1016/s0140-6736(11)60000-0",
			Venue:   "The Lancet",
		},
		{Rank: 2, Title: "An anonymous editorial", Year: 2015},
	})
	got := sb.String()

	wantAuthor := "  author = {Zoë Q. Vandermeer and A. Baker and Miguel Ángel Ruiz-Ortega},"
	if !strings.Contains(got, wantAuthor) {
		t.Errorf("BibTeX missing joined author line %q:\n%s", wantAuthor, got)
	}
	if !strings.Contains(got, "  journal = {The Lancet},") {
		t.Errorf("BibTeX missing journal line:\n%s", got)
	}

	// The second entry has no authors, so exactly one author line overall.
	if n := strings.Count(got, "  author = {"); n != 1 {
		t.Errorf("got %d author lines, want 1 (author-less entry must omit it):\n%s", n, got)
	}
}
