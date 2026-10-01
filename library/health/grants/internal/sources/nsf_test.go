package sources

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type nsfRoundTripFunc func(*http.Request) (*http.Response, error)

func (f nsfRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNSFStem(t *testing.T) {
	cases := []struct{ in, want string }{
		{"resilience", "resil"},
		{"resilient", "resil"},
		{"resiliency", "resil"},
		{"computing", "comput"},
		{"therapies", "therap"},
		{"therapy", "therap"},
		{"published", "publish"},
		{"genes", "gene"},
		{"gene", "gene"},   // four letters left is the floor, so nothing is trimmed
		{"aging", "aging"}, // trimming "-ing" would leave only two characters
		{"characterization", "character"},
		{"sustainability", "sustain"},
		{"observations", "observ"},
		{"measurements", "measur"},
		{"Climate", "climate"},
	}
	for _, c := range cases {
		if got := nsfStem(c.in); got != c.want {
			t.Errorf("nsfStem(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNSFTerms(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"climate resilience", []string{"climate", "resil"}},
		{"gene therapy", []string{"gene", "therap"}},
		{"the use of AI for imaging", []string{"ai", "imag"}},
		{"AI", []string{"ai"}},
		{"OR", []string{"OR"}},
		{"or no as", nil},
		{"of in to", nil},
		{"cancer", []string{"cancer"}},
		{"", nil},
	}
	for _, c := range cases {
		if got := nsfTerms(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("nsfTerms(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSearchNSFRejectsStopWordOnlyQueryBeforeRequest(t *testing.T) {
	previousClient := client
	t.Cleanup(func() { client = previousClient })
	called := false
	client = &http.Client{Transport: nsfRoundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("unexpected request")
	})}
	awards, _, err := SearchNSF("the use of", 10)
	if !errors.Is(err, ErrNoSearchTerms) || len(awards) != 0 || called {
		t.Fatalf("stop-word-only query: awards=%d err=%v request=%v", len(awards), err, called)
	}
}

func TestSearchNSFUppercaseAcronymReachesAPI(t *testing.T) {
	previousClient := client
	t.Cleanup(func() { client = previousClient })
	requests := 0
	client = &http.Client{Transport: nsfRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if got := req.URL.Query().Get("keyword"); got != "OR" {
			t.Errorf("keyword = %q, want OR", got)
		}
		body := `{"response":{"award":[{"id":"or-1","title":"OR Methods for Cancer Research","abstractText":"Operations research methods"}]}}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})}
	awards, stats, err := SearchNSF("OR", 1)
	if err != nil || requests != 1 || len(awards) != 1 || awards[0].ID != "or-1" || stats.Matched != 1 {
		t.Fatalf("acronym search: awards=%#v stats=%+v requests=%d err=%v", awards, stats, requests, err)
	}
}

func TestNSFScoreMatchesShortAcronymsAsWholeTokens(t *testing.T) {
	matching := nsfAwardRaw{NSFAward: NSFAward{Title: "AI for Scientific Discovery"}}
	if _, _, ok := nsfScore(matching, nsfTerms("AI")); !ok {
		t.Fatal("standalone AI token did not match")
	}
	nonMatching := nsfAwardRaw{NSFAward: NSFAward{Title: "Training Systems"}, Abstract: "Detailed methods"}
	if _, _, ok := nsfScore(nonMatching, nsfTerms("AI")); ok {
		t.Fatal("AI incorrectly matched inside another word")
	}
	ordinaryWord := nsfAwardRaw{NSFAward: NSFAward{Title: "Cancer Research or Treatment"}}
	if _, _, ok := nsfScore(ordinaryWord, nsfTerms("OR cancer")); ok {
		t.Fatal("uppercase OR matched an ordinary lowercase conjunction")
	}
}

// The measured case: "climate resilience" must match an abstract that only ever
// says "resilient".
func TestNSFScoreMatchesByStem(t *testing.T) {
	award := nsfAwardRaw{
		NSFAward: NSFAward{Title: "Coastal Infrastructure Study"},
		Abstract: "This project studies how climate change makes coastal towns more resilient.",
	}
	score, titleHit, ok := nsfScore(award, nsfTerms("climate resilience"))
	if !ok {
		t.Fatal("expected an abstract saying \"resilient\" to match the term \"resilience\"")
	}
	if titleHit {
		t.Error("titleHit = true, want false: neither term is in the title")
	}
	if score != 2 {
		t.Errorf("score = %d, want 2 (two abstract hits)", score)
	}
}

func TestNSFScore(t *testing.T) {
	cases := []struct {
		name     string
		award    nsfAwardRaw
		query    string
		wantOK   bool
		wantHit  bool
		wantScrE int
	}{
		{
			name:     "both terms in title",
			award:    nsfAwardRaw{NSFAward: NSFAward{Title: "A Novel Gene Therapy Platform", ID: "1"}, Abstract: "delivery vectors"},
			query:    "gene therapy",
			wantOK:   true,
			wantHit:  true,
			wantScrE: 20,
		},
		{
			name:     "one term in title, one in abstract",
			award:    nsfAwardRaw{NSFAward: NSFAward{Title: "Gene Editing in Zebrafish", ID: "2"}, Abstract: "a therapeutic approach"},
			query:    "gene therapy",
			wantOK:   true,
			wantHit:  true,
			wantScrE: 11,
		},
		{
			name:    "missing a term is rejected",
			award:   nsfAwardRaw{NSFAward: NSFAward{Title: "Squid Hydrodynamics", ID: "3"}, Abstract: "artificial neural networks are used"},
			query:   "artificial intelligence",
			wantOK:  false,
			wantHit: false,
		},
		{
			name:     "empty query matches nothing",
			award:    nsfAwardRaw{NSFAward: NSFAward{Title: "Anything", ID: "4"}, Abstract: ""},
			query:    "",
			wantOK:   false,
			wantHit:  false,
			wantScrE: 0,
		},
	}
	for _, c := range cases {
		score, hit, ok := nsfScore(c.award, nsfTerms(c.query))
		if ok != c.wantOK || hit != c.wantHit || (ok && score != c.wantScrE) {
			t.Errorf("%s: nsfScore = (%d, %v, %v), want (%d, %v, %v)",
				c.name, score, hit, ok, c.wantScrE, c.wantHit, c.wantOK)
		}
	}
}

func TestNSFRank(t *testing.T) {
	pool := []nsfAwardRaw{
		{NSFAward: NSFAward{ID: "1", Title: "Squid Hydrodynamics", FundsObligated: "900000"},
			Abstract: "artificial neural sampling, no second term here"},
		{NSFAward: NSFAward{ID: "2", Title: "Graduate Fellowship", FundsObligated: "100000"},
			Abstract: "supports the artificial intelligence priority area"},
		{NSFAward: NSFAward{ID: "3", Title: "Artificial Intelligence for Robotics", FundsObligated: "500000"},
			Abstract: "an artificial intelligence system"},
		// Same collaborative award, registered per institution: different IDs,
		// identical titles. The larger fundsObligatedAmt must win.
		{NSFAward: NSFAward{ID: "4a", Title: "Collaborative Research: AI and Intelligence Testing", FundsObligated: "200000"},
			Abstract: "artificial methods"},
		{NSFAward: NSFAward{ID: "4b", Title: "collaborative research:  AI and Intelligence Testing ", FundsObligated: "800000"},
			Abstract: "artificial methods"},
	}

	awards, stats := nsfRank(pool, nsfTerms("artificial intelligence"), 5)

	if stats.Examined != 5 {
		t.Errorf("Examined = %d, want 5", stats.Examined)
	}
	if stats.Matched != 3 {
		t.Errorf("Matched = %d, want 3 (one rejected, one deduped)", stats.Matched)
	}
	if stats.TitleMatched != 2 {
		t.Errorf("TitleMatched = %d, want 2", stats.TitleMatched)
	}
	wantIDs := []string{"3", "4b", "2"}
	var gotIDs []string
	for _, a := range awards {
		gotIDs = append(gotIDs, a.ID)
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("ranked IDs = %v, want %v", gotIDs, wantIDs)
	}
}

func TestNSFRankLimitsRows(t *testing.T) {
	pool := []nsfAwardRaw{
		{NSFAward: NSFAward{ID: "1", Title: "Cancer A"}, Abstract: "cancer"},
		{NSFAward: NSFAward{ID: "2", Title: "Cancer B"}, Abstract: "cancer"},
		{NSFAward: NSFAward{ID: "3", Title: "Cancer C"}, Abstract: "cancer"},
	}
	awards, stats := nsfRank(pool, nsfTerms("cancer"), 2)
	if len(awards) != 2 {
		t.Errorf("len(awards) = %d, want 2", len(awards))
	}
	if stats.Matched != 3 {
		t.Errorf("Matched = %d, want 3 (stats count matches, not shown rows)", stats.Matched)
	}
}

// The PI must reach the normalised output in the NIH path's
// "SURNAME, FIRSTNAME" shape, built from the split pair. The measured hazard is
// a multi-word surname: reading the combined pdPIName instead and taking its
// last word as the surname misfiles "Emma A Elliott Smith" (9 of 236 awards
// measured 2026-09-04).
func TestNSFRankEmitsPI(t *testing.T) {
	pool := []nsfAwardRaw{
		{NSFAward: NSFAward{ID: "1", Title: "Isotope Ecology"}, Abstract: "ecology",
			PIFirstName: "Emma", PILastName: "Elliott Smith"},
	}
	awards, _ := nsfRank(pool, nsfTerms("ecology"), 5)
	if len(awards) != 1 {
		t.Fatalf("len(awards) = %d, want 1", len(awards))
	}
	if got, want := awards[0].PI, "Elliott Smith, Emma"; got != want {
		t.Errorf("PI = %q, want %q", got, want)
	}
}

func TestNSFPIName(t *testing.T) {
	cases := []struct{ first, last, want string }{
		{"Jenny", "Ouyang", "Ouyang, Jenny"},
		{"Emma", "Elliott Smith", "Elliott Smith, Emma"},
		{" Emma ", " Elliott Smith ", "Elliott Smith, Emma"},
		{"", "Ouyang", "Ouyang"}, // no stray leading comma
		{"Jenny", "", "Jenny"},   // no stray trailing comma
		{"", "", ""},
	}
	for _, c := range cases {
		if got := nsfPIName(c.first, c.last); got != c.want {
			t.Errorf("nsfPIName(%q, %q) = %q, want %q", c.first, c.last, got, c.want)
		}
	}
}

// nsfLiveBody is a verbatim trimmed response from the NSF Awards API, measured
// 2026-09-04 for the keyword "isotope ecology" with the printFields this
// package requests. It exists because TestNSFRankEmitsPI sets PIFirstName and
// PILastName by hand and therefore proves only the formatting: a wrong JSON tag
// would leave that test green while the app emitted an empty contact_pi_name.
// Decoding this body through the real nsfResp/nsfAwardRaw structs is what pins
// the tags. The second award carries a multi-word surname on purpose.
//
// What this fixture cannot cover: the request is never made, so a printFields
// typo that stops the API returning piFirstName/piLastName at all is invisible
// here. That gap is real and is not claimed to be covered.
const nsfLiveBody = `{
  "response": {
    "award": [
      {
        "id": "2628024",
        "title": "RAPID: Reproductive Ecology of Antarctic Krill",
        "fundsObligatedAmt": "168531",
        "awardeeName": "Oregon State University",
        "startDate": "09/01/2026",
        "expDate": "08/31/2027",
        "abstractText": "Stable isotope work on krill ecology.",
        "piFirstName": "Kim",
        "piLastName": "Bernard"
      },
      {
        "id": "2606140",
        "title": "Anaerobic Fungi in Digestion",
        "fundsObligatedAmt": "419981",
        "awardeeName": "University of Texas at Austin",
        "startDate": "09/01/2026",
        "expDate": "08/31/2029",
        "abstractText": "Isotope tracing informs the ecology of the digester community.",
        "piFirstName": "Xavier",
        "piLastName": "Fonoll Almansa"
      }
    ]
  }
}`

// TestNSFDecodeEmitsPI runs a real API response body through the actual decode
// path and out to JSON, so the input tags (piFirstName, piLastName), the
// struct wiring and the output key (contact_pi_name) are all pinned by one
// test. Asserting on the marshalled output rather than the Go field is
// deliberate: the field name is ours, but contact_pi_name is the contract the
// NIH path already publishes and the web app reads.
func TestNSFDecodeEmitsPI(t *testing.T) {
	var resp nsfResp
	if err := json.Unmarshal([]byte(nsfLiveBody), &resp); err != nil {
		t.Fatalf("decoding the measured NSF body: %v", err)
	}
	if len(resp.Response.Award) != 2 {
		t.Fatalf("decoded %d awards, want 2", len(resp.Response.Award))
	}
	if got := resp.Response.Award[0].PIFirstName; got != "Kim" {
		t.Errorf("PIFirstName = %q, want %q — check the json tag", got, "Kim")
	}
	if got := resp.Response.Award[1].PILastName; got != "Fonoll Almansa" {
		t.Errorf("PILastName = %q, want %q — check the json tag", got, "Fonoll Almansa")
	}

	awards, _ := nsfRank(resp.Response.Award, nsfTerms("isotope ecology"), 5)
	if len(awards) != 2 {
		t.Fatalf("ranked %d awards, want 2", len(awards))
	}

	out, err := json.Marshal(awards)
	if err != nil {
		t.Fatalf("marshalling awards: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("re-decoding awards: %v", err)
	}
	byID := map[string]string{}
	for _, a := range decoded {
		pi, ok := a["contact_pi_name"].(string)
		if !ok {
			t.Fatalf("award %v has no string contact_pi_name — check the output json tag", a["id"])
		}
		byID[a["id"].(string)] = pi
	}
	for id, want := range map[string]string{
		"2628024": "Bernard, Kim",
		"2606140": "Fonoll Almansa, Xavier",
	} {
		if got := byID[id]; got != want {
			t.Errorf("award %s contact_pi_name = %q, want %q", id, got, want)
		}
	}
}
