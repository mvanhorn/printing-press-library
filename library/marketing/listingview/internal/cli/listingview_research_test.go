package cli

import (
	"encoding/json"
	"testing"
)

func TestNicheVerdict(t *testing.T) {
	cases := []struct {
		name string
		view nicheView
		want string
	}{
		{"go", nicheView{SearchVolume: 600000, CompetingListings: 5000, OpportunityRatio: 120, WinnablePct: 60, TopSellerSamples: 20}, "GO"},
		{"caution", nicheView{CompetingListings: 1000, OpportunityRatio: 5, WinnablePct: 25, TopSellerSamples: 20}, "CAUTION"},
		{"avoid-entrenched", nicheView{CompetingListings: 1000, OpportunityRatio: 8, WinnablePct: 5, TopSellerSamples: 20}, "AVOID"},
		{"avoid-low-demand", nicheView{CompetingListings: 100000, OpportunityRatio: 1, WinnablePct: 40, TopSellerSamples: 20}, "AVOID"},
		{"insufficient", nicheView{CompetingListings: 0, TopSellerSamples: 0}, "INSUFFICIENT-DATA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := nicheVerdict(tc.view)
			if got != tc.want {
				t.Fatalf("nicheVerdict(%+v) = %q, want %q", tc.view, got, tc.want)
			}
		})
	}
}

func TestGradeTag(t *testing.T) {
	cases := []struct {
		name string
		g    gradedTag
		want string
	}{
		{"strong", gradedTag{OpportunityScore: 75}, "strong"},
		{"ok", gradedTag{OpportunityScore: 50}, "ok"},
		{"weak", gradedTag{OpportunityScore: 20}, "weak"},
		{"unknown", gradedTag{}, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gradeTag(tc.g); got != tc.want {
				t.Fatalf("gradeTag(%+v) = %q, want %q", tc.g, got, tc.want)
			}
		})
	}
}

func TestRound2(t *testing.T) {
	if got := round2(1.23456); got != 1.23 {
		t.Fatalf("round2(1.23456) = %v, want 1.23", got)
	}
	if got := round2(1.235); got != 1.24 {
		t.Fatalf("round2(1.235) = %v, want 1.24", got)
	}
}

func TestFirstNonZero(t *testing.T) {
	if got := firstNonZero(0, 0, 5, 9); got != 5 {
		t.Fatalf("firstNonZero = %v, want 5", got)
	}
	if got := firstNonZero(0, 0); got != 0 {
		t.Fatalf("firstNonZero(all zero) = %v, want 0", got)
	}
}

func TestListAndFieldHelpers(t *testing.T) {
	raw := json.RawMessage(`{"keywords":[{"keyword":"sticker","searchVolume":"5764996.14","competition":194268}]}`)
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	kws := listOf(data, "keywords")
	if len(kws) != 1 {
		t.Fatalf("listOf got %d, want 1", len(kws))
	}
	if got := strOf(kws[0], "keyword"); got != "sticker" {
		t.Fatalf("strOf keyword = %q, want sticker", got)
	}
	// numOf must tolerate a JSON-string-encoded number.
	if got := numOf(kws[0], "searchVolume"); got != 5764996.14 {
		t.Fatalf("numOf searchVolume = %v, want 5764996.14", got)
	}
	if got := numOf(kws[0], "competition"); got != 194268 {
		t.Fatalf("numOf competition = %v, want 194268", got)
	}
}

func TestStringsOf(t *testing.T) {
	plain := json.RawMessage(`{"tags":["a","b","c"]}`)
	var d1 map[string]json.RawMessage
	_ = json.Unmarshal(plain, &d1)
	if got := stringsOf(d1, "tags"); len(got) != 3 || got[0] != "a" {
		t.Fatalf("stringsOf(plain) = %v", got)
	}
	objs := json.RawMessage(`{"tags":[{"tag":"x"},{"tag":"y"}]}`)
	var d2 map[string]json.RawMessage
	_ = json.Unmarshal(objs, &d2)
	if got := stringsOf(d2, "tags"); len(got) != 2 || got[1] != "y" {
		t.Fatalf("stringsOf(objects) = %v", got)
	}
}
