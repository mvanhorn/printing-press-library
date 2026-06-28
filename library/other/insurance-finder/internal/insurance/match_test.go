package insurance

import (
	"reflect"
	"testing"
)

func tierSet(recs []Recommendation, tier string) map[string]bool {
	out := map[string]bool{}
	for _, r := range recs {
		if r.Tier == tier {
			out[r.Provider.ID] = true
		}
	}
	return out
}

// The central business rule: an importer / private-label / manufacturer is
// routed to specialty markets, and the mainstream instant-quote carriers that
// decline the "deemed manufacturer" class are pushed to the avoid tier.
func TestMatch_ImporterRoutesToSpecialtyMarkets(t *testing.T) {
	reg := mustRegistry(t)
	recs := Match(importerProfile(), reg.Providers)

	if recs[0].Provider.ID != "insurance-canopy" {
		t.Errorf("rank 1 = %q, want insurance-canopy", recs[0].Provider.ID)
	}
	if recs[1].Provider.ID != "xinsurance" {
		t.Errorf("rank 2 = %q, want xinsurance", recs[1].Provider.ID)
	}

	wantRecommended := map[string]bool{
		"insurance-canopy": true,
		"xinsurance":       true,
		"veracity":         true,
		"insureon":         true,
		"tivly":            true,
	}
	gotRecommended := tierSet(recs, TierRecommended)
	if !reflect.DeepEqual(gotRecommended, wantRecommended) {
		t.Errorf("recommended tier = %v, want %v", gotRecommended, wantRecommended)
	}

	// Mainstream instant-quote carriers that decline the imported class must be
	// in the avoid tier with a negative score.
	avoid := tierSet(recs, TierAvoid)
	for _, id := range []string{"the-hartford", "biberk", "next-insurance"} {
		if !avoid[id] {
			t.Errorf("%q should be in the avoid tier for an importer", id)
		}
		if gotRecommended[id] {
			t.Errorf("%q must NOT be recommended for an importer", id)
		}
	}

	byID := recByID(recs)
	if s := byID["biberk"].Score; s >= 0 {
		t.Errorf("biberk score for importer = %d, want negative", s)
	}
}

// The inverse: a low-hazard retailer that does NOT import is routed to the
// mainstream instant-quote carriers, and biBerk/Next are now GOOD options.
func TestMatch_NonImporterRetailRoutesToMainstream(t *testing.T) {
	reg := mustRegistry(t)
	recs := Match(retailProfile(), reg.Providers)

	if recs[0].Provider.ID != "simply-business" {
		t.Errorf("rank 1 = %q, want simply-business", recs[0].Provider.ID)
	}

	wantRecommended := map[string]bool{
		"simply-business": true,
		"coterie":         true,
		"hiscox":          true,
		"biberk":          true,
		"next-insurance":  true,
	}
	gotRecommended := tierSet(recs, TierRecommended)
	if !reflect.DeepEqual(gotRecommended, wantRecommended) {
		t.Errorf("recommended tier = %v, want %v", gotRecommended, wantRecommended)
	}

	// Specialty importer markets are not the right first stop for a low-hazard
	// retailer - they should not be in the recommended tier.
	for _, id := range []string{"xinsurance", "veracity"} {
		if gotRecommended[id] {
			t.Errorf("%q should not be recommended for a low-hazard retailer", id)
		}
	}

	// biBerk/Next flip from avoid (importer) to recommended (retail).
	avoid := tierSet(recs, TierAvoid)
	for _, id := range []string{"biberk", "next-insurance"} {
		if avoid[id] {
			t.Errorf("%q should NOT be avoided for a low-hazard retailer", id)
		}
	}
}

func TestMatch_Deterministic(t *testing.T) {
	reg := mustRegistry(t)
	p := importerProfile()
	a := Match(p, reg.Providers)
	b := Match(p, reg.Providers)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Match is not deterministic")
	}
	// Ranks are 1..N contiguous.
	for i, r := range a {
		if r.Rank != i+1 {
			t.Errorf("rank[%d] = %d, want %d", i, r.Rank, i+1)
		}
	}
}

func TestMatch_ImporterGetsForeignProductsCaution(t *testing.T) {
	reg := mustRegistry(t)
	recs := Match(importerProfile(), reg.Providers)
	byID := recByID(recs)
	// Hiscox is a standard-market partial fit for importers; it must carry the
	// foreign-products-exclusion caution.
	found := false
	for _, c := range byID["hiscox"].Cautions {
		if containsFold(c, "foreign-products") {
			found = true
		}
	}
	if !found {
		t.Errorf("hiscox importer cautions = %v, want a foreign-products-exclusion caution", byID["hiscox"].Cautions)
	}
}

func TestShortlist_DropsAvoidTier(t *testing.T) {
	reg := mustRegistry(t)
	recs := Match(importerProfile(), reg.Providers)
	short := Shortlist(recs)
	for _, r := range short {
		if r.Tier == TierAvoid {
			t.Errorf("shortlist should not contain avoid-tier provider %q", r.Provider.ID)
		}
	}
	if len(short) >= len(recs) {
		t.Errorf("shortlist (%d) should be smaller than full list (%d)", len(short), len(recs))
	}
}

// A business with only SOME of the importer/private-label/manufacturer flags set
// must be judged on the appetites that actually apply - not on the importer
// appetite when it is not an importer.
func TestMatch_PartialClassFlagsUseOnlyApplicableAppetite(t *testing.T) {
	reg := mustRegistry(t)

	// Manufacturer-only: The Hartford declines importers but writes
	// manufacturers (partial), so it must NOT be avoided here.
	mfg := recByID(Match(manufacturerOnlyProfile(), reg.Providers))
	if mfg["the-hartford"].Tier == TierAvoid {
		t.Errorf("manufacturer-only: the-hartford = avoid (score %d); it writes manufacturers and must not be declined", mfg["the-hartford"].Score)
	}
	// biBerk declines manufacturers too, so a real decline still fires.
	if mfg["biberk"].Tier != TierAvoid {
		t.Errorf("manufacturer-only: biberk should still be avoid (it declines manufacturers)")
	}

	// Private-label-only: The Hartford declines private-label, so it stays
	// avoid; Insureon writes private-label (partial) and must not be avoided.
	pl := recByID(Match(privateLabelOnlyProfile(), reg.Providers))
	if pl["the-hartford"].Tier != TierAvoid {
		t.Errorf("private-label-only: the-hartford should be avoid (declines private-label)")
	}
	if pl["insureon"].Tier == TierAvoid {
		t.Errorf("private-label-only: insureon should not be avoid (writes private-label)")
	}
}
