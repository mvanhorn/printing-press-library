package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/store"
)

func TestDigestReviews(t *testing.T) {
	reviews := []store.ReviewRow{
		{ReviewID: "1", Score: 5, Version: "1.0", Reply: true, Text: "great game love it"},
		{ReviewID: "2", Score: 1, Version: "1.0", Text: "crashes constantly crashes again"},
		{ReviewID: "3", Score: 2, Version: "1.1", Text: "too many crashes ads everywhere"},
		{ReviewID: "4", Score: 4, Version: "1.1", Text: "decent"},
	}
	v := digestReviews("com.x", reviews, "")
	if v.Total != 4 {
		t.Errorf("total = %d, want 4", v.Total)
	}
	if v.StarHistogram["5"] != 1 || v.StarHistogram["1"] != 1 || v.StarHistogram["2"] != 1 || v.StarHistogram["4"] != 1 {
		t.Errorf("star histogram = %+v", v.StarHistogram)
	}
	if v.ByVersion["1.0"] != 2 || v.ByVersion["1.1"] != 2 {
		t.Errorf("byVersion = %+v", v.ByVersion)
	}
	if v.ReplyRate != 0.25 {
		t.Errorf("replyRate = %v, want 0.25", v.ReplyRate)
	}
	// "crashes" appears in two low-star reviews -> should be a top term
	found := false
	for _, tc := range v.TopTerms {
		if tc.Term == "crashes" && tc.Count >= 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'crashes' as a top complaint term, got %+v", v.TopTerms)
	}
}

func TestDigestReviewsSinceVersion(t *testing.T) {
	reviews := []store.ReviewRow{
		{ReviewID: "1", Score: 5, Version: "1.0"},
		{ReviewID: "2", Score: 3, Version: "2.0"},
	}
	v := digestReviews("com.x", reviews, "2.0")
	if v.Total != 1 {
		t.Errorf("since-version filter: total = %d, want 1", v.Total)
	}
}

func TestTokenize(t *testing.T) {
	got := tokenize("Crashes, crashes! Too-many ADS.")
	want := map[string]bool{"crashes": true, "too": true, "many": true, "ads": true}
	for _, tok := range got {
		delete(want, tok)
	}
	if len(want) != 0 {
		t.Errorf("tokenize missed %v (got %v)", want, got)
	}
}

func TestRound2(t *testing.T) {
	cases := map[float64]float64{1.0 / 3: 0.33, 0.125: 0.13, 4.0: 4.0}
	for in, want := range cases {
		if got := round2(in); got != want {
			t.Errorf("round2(%v) = %v, want %v", in, got, want)
		}
	}
}
