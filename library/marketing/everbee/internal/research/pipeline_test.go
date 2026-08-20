// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package research

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// fakeAPI replays canned EverBee payloads so the pipeline can be tested without
// spending research quota.
type fakeAPI struct {
	keywordBody string
	productBody string
	keywordErr  error
	productErr  error
	keywordCall int
	productCall int
}

func (f *fakeAPI) Get(_ context.Context, path string, _ map[string]string) (json.RawMessage, error) {
	if strings.HasPrefix(path, PathProductSearch) {
		f.productCall++
		if f.productErr != nil {
			return json.RawMessage(f.productBody), f.productErr
		}
		return json.RawMessage(f.productBody), nil
	}
	return json.RawMessage(`{}`), nil
}

func (f *fakeAPI) PostQueryWithParams(_ context.Context, _ string, _ map[string]string, _ any) (json.RawMessage, int, error) {
	f.keywordCall++
	if f.keywordErr != nil {
		return json.RawMessage(f.keywordBody), 0, f.keywordErr
	}
	return json.RawMessage(f.keywordBody), 200, nil
}

// Payloads shaped like the real API responses captured on 2026-07-11.
const seededKeywordBody = `{
  "total_count": 53938,
  "searched_keyword": {"keyword": "dad shirt", "vol": 2747, "competition": 698000},
  "results": [
    {"keyword": "dadfully shirt", "new_volume": 2747, "competition": 167, "score": 16450},
    {"keyword": "dad shirts",     "new_volume": 2598, "competition": 699472, "score": 0},
    {"keyword": "shirts dad",     "new_volume": 2598, "competition": 700993, "score": 0}
  ]
}`

const seededProductBody = `{
  "total_count": 1153877,
  "results": [
    {"listing_id": 4492750007, "title": "Funny Dad Shirt, Just Resting My Eyes", "price": "24.50", "listing_type": "physical", "tags": ["funny dad shirt"], "cached_est_mo_revenue": 5000, "review_count": 120},
    {"listing_id": 4492750008, "title": "Custom Dad Shirt with Photo",          "price": "30.00", "listing_type": "physical", "tags": ["dad shirt"],       "cached_est_mo_revenue": 3000, "review_count": 40},
    {"listing_id": 4492750009, "title": "Dad SVG Bundle Cricut Cut File",       "price": "3.00",  "listing_type": "download", "tags": ["svg", "cricut"],   "cached_est_mo_revenue": 900,  "review_count": 5}
  ]
}`

// The default feed the published CLI used: the same shape, but the content
// ignores the seed entirely. This is the #1492 bug reproduced as data.
const defaultFeedKeywordBody = `{
  "total_count": 68457368,
  "results": [
    {"keyword": "gifte fully",        "new_volume": 26511, "competition": 110,      "score": 0},
    {"keyword": "gift gifte gifting", "new_volume": 26508, "competition": 21640,    "score": 0},
    {"keyword": "gifttings",          "new_volume": 26492, "competition": 45426668, "score": 0}
  ]
}`

const defaultFeedProductBody = `{
  "total_count": 171547937,
  "results": [
    {"listing_id": 619010577,  "title": "Calf Hair Clutch Purse | Maasai Leather Wrist Wallet", "price": "89.00", "listing_type": "physical", "cached_est_mo_revenue": 100},
    {"listing_id": 1704766411, "title": "Down to Earth DVD",                                    "price": "12.00", "listing_type": "physical", "cached_est_mo_revenue": 50}
  ]
}`

func TestNiche_SeededEvidenceProducesConfidentVerdict(t *testing.T) {
	api := &fakeAPI{keywordBody: seededKeywordBody, productBody: seededProductBody}
	v, err := Niche(context.Background(), api, Options{Seed: "dad shirt"})
	if err != nil {
		t.Fatalf("Niche() error = %v", err)
	}

	if v.Evidence.KeywordsRelevant != 3 {
		t.Errorf("KeywordsRelevant = %d, want 3 (all seeded keywords are on-target)", v.Evidence.KeywordsRelevant)
	}
	if v.Evidence.ProductsRelevant != 3 {
		t.Errorf("ProductsRelevant = %d, want 3", v.Evidence.ProductsRelevant)
	}
	if v.Demand != 2747 || v.Competition != 698000 {
		t.Errorf("seed metrics not carried through: demand=%d competition=%d", v.Demand, v.Competition)
	}
	if !v.SeedMetrics.Present {
		t.Error("SeedMetrics.Present = false, want true — the searched_keyword block was supplied")
	}
	if v.Confidence <= 0 {
		t.Errorf("Confidence = %v, want > 0 with real evidence", v.Confidence)
	}
	if len(v.Provenance) != 2 {
		t.Errorf("want provenance for both fetches, got %d entries", len(v.Provenance))
	}
	for _, p := range v.Provenance {
		if p.QueryScope != "dad shirt" || p.FetchedAt.IsZero() {
			t.Errorf("provenance incomplete: %+v", p)
		}
	}
}

// THE #1492 REGRESSION. Feed the pipeline exactly what the default feed returns
// and assert it cannot produce a confident verdict.
func TestNiche_DefaultFeedContentYieldsZeroConfidence(t *testing.T) {
	api := &fakeAPI{keywordBody: defaultFeedKeywordBody, productBody: defaultFeedProductBody}
	v, err := Niche(context.Background(), api, Options{Seed: "dad shirt"})
	if err != nil {
		t.Fatalf("Niche() error = %v", err)
	}

	if v.Evidence.TotalEvidence() != 0 {
		t.Errorf("TotalEvidence = %d, want 0 — none of the default-feed rows are about dad shirts", v.Evidence.TotalEvidence())
	}
	if v.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0. The published CLI reported 1.0 here; that is the bug.", v.Confidence)
	}
	if v.Opportunity != 0 {
		t.Errorf("Opportunity = %v, want 0 with no relevant evidence", v.Opportunity)
	}
	// The rows are still returned (annotate-only), just not counted as evidence.
	if len(v.Keywords) != 3 || len(v.Products) != 2 {
		t.Errorf("rows must be returned even when irrelevant: got %d keywords, %d products", len(v.Keywords), len(v.Products))
	}
	if len(v.Warnings) == 0 || !strings.Contains(strings.Join(v.Warnings, " "), "no evidence") {
		t.Errorf("want an explicit no-evidence warning, got %v", v.Warnings)
	}
}

func TestNiche_ProductTypeConstraintExcludesDigital(t *testing.T) {
	api := &fakeAPI{keywordBody: seededKeywordBody, productBody: seededProductBody}
	v, err := Niche(context.Background(), api, Options{Seed: "dad shirt", ProductType: "apparel", ExcludeSVGPNG: true})
	if err != nil {
		t.Fatalf("Niche() error = %v", err)
	}
	// 3 relevant listings, but the SVG cut file is not apparel.
	if v.Evidence.ProductsRelevant != 3 {
		t.Errorf("ProductsRelevant = %d, want 3", v.Evidence.ProductsRelevant)
	}
	if v.Evidence.ProductsInType != 2 {
		t.Errorf("ProductsInType = %d, want 2 (the SVG cut file must be excluded)", v.Evidence.ProductsInType)
	}
	// The price band must be computed from apparel only — the $3 SVG would drag
	// the median down and misprice the niche.
	if v.PriceBand.Min < 20 {
		t.Errorf("price band min = %v; the $3 digital listing leaked into the band", v.PriceBand.Min)
	}
}

func TestNiche_EmptySeedRejected(t *testing.T) {
	api := &fakeAPI{keywordBody: seededKeywordBody, productBody: seededProductBody}
	if _, err := Niche(context.Background(), api, Options{Seed: "   "}); err == nil {
		t.Error("want an error for an empty seed")
	}
}

func TestNiche_PlanCapIsTypedNotEmpty(t *testing.T) {
	api := &fakeAPI{
		keywordBody: `{"message":"Please upgrade your plan to continue keyword research"}`,
		keywordErr:  errors.New("HTTP 402"),
		productBody: seededProductBody,
	}
	_, err := Niche(context.Background(), api, Options{Seed: "dad shirt"})
	if !errors.Is(err, ErrPlanCapReached) {
		t.Errorf("want ErrPlanCapReached so a quota refusal is never mistaken for 'no results'; got %v", err)
	}
}

func TestNiche_KeywordFailureStillReturnsProductEvidence(t *testing.T) {
	api := &fakeAPI{
		keywordBody: `{}`,
		keywordErr:  errors.New("transport blew up"),
		productBody: seededProductBody,
	}
	v, err := Niche(context.Background(), api, Options{Seed: "dad shirt"})
	if err != nil {
		t.Fatalf("a keyword failure must degrade, not abort: %v", err)
	}
	if v.Evidence.KeywordsRelevant != 0 || v.Evidence.ProductsRelevant == 0 {
		t.Errorf("want product-only evidence, got %+v", v.Evidence)
	}
	// Confidence is capped because keyword evidence is missing.
	if v.Confidence > MaxConfidenceWithoutKeywordEvidence {
		t.Errorf("Confidence = %v, must be capped at %v without keyword evidence", v.Confidence, MaxConfidenceWithoutKeywordEvidence)
	}
	if v.Provenance[0].Fallback == "" {
		t.Error("a degraded path must record a fallback reason in provenance")
	}
}

// A broken research path (e.g. an expired token 401ing both endpoints) must be
// an error, not a zero-evidence verdict. A caller would read "no opportunity
// here" from what is really "the API refused us".
func TestNiche_BothLegsFailedIsAnErrorNotAnEmptyVerdict(t *testing.T) {
	api := &fakeAPI{
		keywordBody: `{"message":"Could not authenticate with the provided credentials."}`,
		keywordErr:  errors.New("HTTP 401"),
		productBody: `{"message":"Could not authenticate with the provided credentials."}`,
		productErr:  errors.New("HTTP 401"),
	}
	v, err := Niche(context.Background(), api, Options{Seed: "dad shirt"})
	if err == nil {
		t.Fatalf("want an error when neither leg could be fetched; got verdict %+v", v)
	}
	if v != nil {
		t.Errorf("want a nil verdict on a failed research path, got %+v", v)
	}
	if !strings.Contains(err.Error(), "research path failed") {
		t.Errorf("error should name the failed research path, got %v", err)
	}
}

// A zero-demand seed makes saturation undefined (+Inf). json.Marshal cannot
// encode Inf, so before this was fixed the command crashed at output time. The
// undefined value must marshal as null — NOT as 0, which would read as
// "uncrowded" and is the opposite of the truth.
func TestNiche_ZeroDemandSaturationMarshalsAsNull(t *testing.T) {
	const zeroDemandBody = `{
	  "searched_keyword": {"keyword": "dad shirt", "vol": 0, "competition": 5000},
	  "results": [{"keyword": "dad shirt", "new_volume": 0, "competition": 5000, "score": 0}]
	}`
	api := &fakeAPI{keywordBody: zeroDemandBody, productBody: seededProductBody}

	v, err := Niche(context.Background(), api, Options{Seed: "dad shirt"})
	if err != nil {
		t.Fatalf("Niche() error = %v", err)
	}
	if v.Saturation != nil {
		t.Errorf("Saturation = %v, want nil (undefined at zero demand)", *v.Saturation)
	}

	// The real regression: this used to fail with "json: unsupported value: +Inf".
	blob, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling a zero-demand verdict must not fail: %v", err)
	}
	if !strings.Contains(string(blob), `"saturation":null`) {
		t.Errorf("want saturation null in JSON, got: %s", truncate(string(blob), 200))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func TestNormalize(t *testing.T) {
	subs := []SubNiche{
		{Seed: "a", Opportunity: 40},
		{Seed: "b", Opportunity: 80},
		{Seed: "c", Opportunity: 0, Error: "boom"},
	}
	Normalize(subs)
	if subs[0].Seed != "b" || subs[0].NormalizedScore != 100 {
		t.Errorf("best child should sort first at 100: %+v", subs[0])
	}
	if subs[1].Seed != "a" || subs[1].NormalizedScore != 50 {
		t.Errorf("second child should normalize to 50: %+v", subs[1])
	}
	for _, s := range subs {
		if s.Error != "" && s.NormalizedScore != 0 {
			t.Errorf("errored child must score 0, got %v", s.NormalizedScore)
		}
	}
}

func TestChildSeeds(t *testing.T) {
	rows := []KeywordRow{
		{Keyword: "dad shirt funny", Relevance: Relevance("dad", "dad shirt funny")},
		{Keyword: "dad mug", Relevance: Relevance("dad", "dad mug")},
		{Keyword: "gifttings", Relevance: Relevance("dad", "gifttings")},
		{Keyword: "dad shirt vintage", Relevance: Relevance("dad", "dad shirt vintage")},
	}
	// Constrain to shirts: the mug and the junk keyword must not become children.
	got := ChildSeeds(rows, "dad", "shirt", DefaultMinRelevance, 10)
	want := []string{"dad shirt funny", "dad shirt vintage"}
	if len(got) != len(want) {
		t.Fatalf("ChildSeeds() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ChildSeeds()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Limit is honored.
	if len(ChildSeeds(rows, "dad", "", DefaultMinRelevance, 1)) != 1 {
		t.Error("ChildSeeds must honor its limit")
	}
}

func TestParseSeedMetrics(t *testing.T) {
	// Object form (single seed).
	got := parseSeedMetrics(json.RawMessage(`{"keyword":"dad shirt","vol":2747,"competition":698000}`), "dad shirt")
	if !got.Present || got.Volume != 2747 || got.Competition != 698000 {
		t.Errorf("object form = %+v", got)
	}
	// Array form (comma-joined seeds).
	got = parseSeedMetrics(json.RawMessage(`[{"keyword":"dad shirt","new_volume":2600,"competition":5}]`), "dad shirt")
	if !got.Present || got.Volume != 2600 {
		t.Errorf("array form = %+v", got)
	}
	// Absent: must report Present=false rather than a fabricated zero.
	got = parseSeedMetrics(nil, "dad shirt")
	if got.Present {
		t.Errorf("absent block must yield Present=false, got %+v", got)
	}
}
