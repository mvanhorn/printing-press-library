package poke

import (
	"reflect"
	"testing"
)

func TestParseTeam(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: nil},
		{name: "single", in: "pikachu", want: []string{"pikachu"}},
		{name: "comma-separated", in: "pikachu,charizard,blastoise", want: []string{"pikachu", "charizard", "blastoise"}},
		{name: "space-separated", in: "pikachu charizard blastoise", want: []string{"pikachu", "charizard", "blastoise"}},
		{name: "mixed-trim-uppercase", in: "  Pikachu , CHARIZARD ,blastoise ", want: []string{"pikachu", "charizard", "blastoise"}},
		{name: "skip-empty", in: ",,pikachu,,charizard,", want: []string{"pikachu", "charizard"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTeam(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTeam(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestDefensiveProfile(t *testing.T) {
	// Charizard is fire/flying. Ground attacks fire (1×) but flying is
	// immune to ground (0×). Net product is 0 — flying immunity wins.
	rels := []TypeRelations{
		// fire's defensive relations
		{
			DoubleDamageFrom: []string{"water", "ground", "rock"},
			HalfDamageFrom:   []string{"fire", "grass", "ice", "bug", "steel", "fairy"},
		},
		// flying's defensive relations
		{
			DoubleDamageFrom: []string{"electric", "ice", "rock"},
			HalfDamageFrom:   []string{"grass", "fighting", "bug"},
			NoDamageFrom:     []string{"ground"},
		},
	}
	got := DefensiveProfile(rels)
	if got["ground"] != 0 {
		t.Errorf("expected ground multiplier 0 (flying immunity), got %v", got["ground"])
	}
	if got["rock"] != 4.0 {
		t.Errorf("expected rock multiplier 4 (2x from fire AND flying), got %v", got["rock"])
	}
	if got["water"] != 2.0 {
		t.Errorf("expected water multiplier 2 (only fire), got %v", got["water"])
	}
	if got["fire"] != 0.5 {
		t.Errorf("expected fire multiplier 0.5, got %v", got["fire"])
	}
	if got["bug"] != 0.25 {
		t.Errorf("expected bug multiplier 0.25 (resisted by both), got %v", got["bug"])
	}
}

func TestBucketDefensive(t *testing.T) {
	// Build a profile by hand.
	profile := map[string]float64{
		"normal":   1,
		"fighting": 1,
		"flying":   1,
		"poison":   1,
		"ground":   0,    // immune
		"rock":     4,    // 4×
		"bug":      0.25, // ¼×
		"ghost":    1,
		"steel":    1,
		"fire":     0.5, // ½×
		"water":    2,   // 2×
		"grass":    1,
		"electric": 2, // 2×
		"psychic":  1,
		"ice":      1,
		"dragon":   1,
		"dark":     1,
		"fairy":    1,
	}
	buckets := BucketDefensive(profile)
	// Expect ordered: 4x, 2x, 1/2, 1/4, 0
	want := []struct {
		mult  string
		types []string
	}{
		{"4x", []string{"rock"}},
		{"2x", []string{"electric", "water"}},
		{"1/2", []string{"fire"}},
		{"1/4", []string{"bug"}},
		{"0", []string{"ground"}},
	}
	if len(buckets) != len(want) {
		t.Fatalf("got %d buckets, want %d: %#v", len(buckets), len(want), buckets)
	}
	for i, w := range want {
		if buckets[i].Multiplier != w.mult {
			t.Errorf("bucket %d: multiplier %q, want %q", i, buckets[i].Multiplier, w.mult)
		}
		if !reflect.DeepEqual(buckets[i].Types, w.types) {
			t.Errorf("bucket %d (%s): types %v, want %v", i, w.mult, buckets[i].Types, w.types)
		}
	}
}

func TestOffensiveCoverage(t *testing.T) {
	// Pikachu attacks with electric. electric hits flying + water for 2×.
	rels := []TypeRelations{
		{DoubleDamageTo: []string{"flying", "water"}},
	}
	got := OffensiveCoverage(rels)
	if !got["flying"] || !got["water"] {
		t.Errorf("expected flying+water covered, got %v", got)
	}
	if got["ground"] {
		t.Errorf("ground should not be covered by electric")
	}
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", &stringErr{"connection refused"}, false},
		{"contains 429", &stringErr{"got HTTP 429"}, true},
		{"rate limit phrase", &stringErr{"rate limit exceeded"}, true},
		{"too many requests", &stringErr{"upstream returned Too Many Requests"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRateLimitError(tt.err); got != tt.want {
				t.Errorf("isRateLimitError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

type stringErr struct{ s string }

func (e *stringErr) Error() string { return e.s }
