package cli

import (
	"strings"
	"testing"
)

func TestComputeScamScore_paymentKeyword(t *testing.T) {
	in := scamScoreInput{
		BodyText: "Please send wire transfer via Western Union ASAP.",
	}
	res := computeScamScore(in)
	if res.Score < 25 {
		t.Errorf("payment keyword should add 25+ points; got %d", res.Score)
	}
	hit := false
	for _, c := range res.Contributions {
		if c.Rule == "payment_keyword" && c.Points == 25 {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected payment_keyword contribution, got %+v", res.Contributions)
	}
}

func TestComputeScamScore_freshAndBelowMedian(t *testing.T) {
	in := scamScoreInput{
		PostedAt: 1_000_000,
		Now:      1_000_000 + 3600, // 1h after
		Price:    50,
		Median:   200,
		BodyText: "great deal",
	}
	res := computeScamScore(in)
	if res.Score < 30 {
		t.Errorf("fresh + below 50%% of median should add 30 points; got %d", res.Score)
	}
}

func TestComputeScamScore_capsAt100(t *testing.T) {
	in := scamScoreInput{
		PostedAt:    1_000_000,
		Now:         1_000_000 + 3600,
		Price:       10,
		Median:      200,
		BodyText:    "wire transfer western union, ship only, out of town. https://evil.example.com",
		ClusterSize: 5,
	}
	res := computeScamScore(in)
	if res.Score != 100 {
		t.Errorf("score should cap at 100; got %d", res.Score)
	}
}

func TestComputeScamScore_dupeClusterRule(t *testing.T) {
	in := scamScoreInput{
		BodyText:    "normal listing",
		ClusterSize: 3,
	}
	res := computeScamScore(in)
	if res.Score != 15 {
		t.Errorf("dupe cluster of 3 should add exactly 15 points; got %d", res.Score)
	}
}

func TestComputeScamScore_externalURL(t *testing.T) {
	res := computeScamScore(scamScoreInput{BodyText: "see https://evil.example.com/listing for details"})
	if res.Score != 10 {
		t.Errorf("external URL should add 10 points; got %d (contribs: %+v)", res.Score, res.Contributions)
	}
	// craigslist.org URLs do not count
	res = computeScamScore(scamScoreInput{BodyText: "see https://images.craigslist.org/foo for photo"})
	if res.Score != 0 {
		t.Errorf("craigslist URL should not add points; got %d", res.Score)
	}
}

func TestHasExternalURL(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"plain text", false},
		{"see https://example.com", true},
		{"see https://images.craigslist.org/foo.jpg", false},
		{"contact me at not.a.url", false},
	}
	for _, tc := range cases {
		got := hasExternalURL(tc.body)
		if got != tc.want {
			t.Errorf("hasExternalURL(%q)=%v want %v", tc.body, got, tc.want)
		}
	}
}

// Sanity guard: rule labels are stable lowercase identifiers, not human prose.
func TestScamScoreRuleLabels(t *testing.T) {
	in := scamScoreInput{BodyText: "wire transfer", ClusterSize: 3}
	res := computeScamScore(in)
	for _, c := range res.Contributions {
		if c.Rule == "" || strings.Contains(c.Rule, " ") {
			t.Errorf("rule label %q should be lowercase identifier", c.Rule)
		}
	}
}
