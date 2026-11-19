package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func mktReviewAdviceFixture(t *testing.T, selectedTerms, alternativeTerms []string) (int64, int64, int64) {
	t.Helper()
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	defer db.Close()
	now := time.Now()
	// An older matching snapshot ensures the newest eligible alternative wins.
	decoy := mktFxSnapshot(t, db.DB(), now.Add(-2*time.Hour), alternativeTerms, 1, "complete", mktFxProg{reviewsDone: []int64{1000000001}})
	alternative := mktFxSnapshot(t, db.DB(), now.Add(-time.Hour), alternativeTerms, 1, "complete", mktFxProg{reviewsDone: []int64{1000000001}})
	// Per-term advice requires a result linking the completed review fetch to its term.
	for _, term := range alternativeTerms {
		mktFxResult(t, db.DB(), decoy, mktFxListing{term: term, id: 1000000001, shop: 9000001, pos: 1, title: term})
		mktFxResult(t, db.DB(), alternative, mktFxListing{term: term, id: 1000000001, shop: 9000001, pos: 1, title: term})
	}
	selected := mktFxSnapshot(t, db.DB(), now, selectedTerms, 1, "complete", mktFxProg{})
	// A newer partial snapshot must not qualify.
	mktFxSnapshot(t, db.DB(), now.Add(time.Hour), selectedTerms, 1, "partial", mktFxProg{reviewsDone: []int64{1000000001}})
	for i, term := range selectedTerms {
		mktFxResult(t, db.DB(), selected, mktFxListing{term: term, id: int64(1000000010 + i), shop: 9000001, pos: 1, title: term, price: 2400})
		if _, err := db.DB().Exec("INSERT INTO market_term_pages VALUES (?, ?, 1, 1, 1)", selected, term); err != nil {
			t.Fatal(err)
		}
	}
	return selected, alternative, decoy
}

func mktAssertReviewAdvice(t *testing.T, selected, alternative, decoy int64, topMatch bool, termMatches map[string]bool) {
	t.Helper()
	if decoy == alternative {
		t.Fatal("REVIEW_ADVICE_NEWEST_PER_TERM_DISTINCT: identical snapshot ids")
	}
	decoyHint := fmt.Sprintf("--snapshot %d", decoy)
	hint := fmt.Sprintf("snapshot %d has review data; use --snapshot %d", alternative, alternative)
	for _, command := range []string{"demand", "price"} {
		out, err := mktRunCLI(t, "market", command, "--snapshot", fmt.Sprint(selected))
		if err != nil {
			t.Fatal(err)
		}
		got := mktDecode(t, out)
		note, _ := got["note"].(string)
		if strings.Contains(note, decoyHint) {
			t.Fatalf("REVIEW_ADVICE_NEWEST_PER_TERM_TOP: %s note=%q", command, note)
		}
		if strings.Contains(note, hint) != topMatch || (!topMatch && (!strings.Contains(note, "re-run market snapshot") || strings.Contains(note, "has review data"))) {
			t.Fatalf("review_advice_top_term_coverage: %s note=%q match=%v", command, note, topMatch)
		}
		if command == "demand" {
			for _, raw := range got["results"].([]any) {
				row := raw.(map[string]any)
				term := row["term"].(string)
				note := row["note"].(string)
				if strings.Contains(note, decoyHint) {
					t.Fatalf("REVIEW_ADVICE_NEWEST_PER_TERM_ROW: term=%q note=%q", term, note)
				}
				if strings.Contains(note, hint) != termMatches[term] || (!termMatches[term] && strings.Contains(note, "has review data")) {
					t.Fatalf("review_advice_per_term_coverage: term=%q note=%q", term, note)
				}
			}
		}
	}
}

func TestMarketReviewAdviceDisjointTerms(t *testing.T) {
	selected, alternative, decoy := mktReviewAdviceFixture(t, []string{"linen apron"}, []string{"ceramic mug"})
	mktAssertReviewAdvice(t, selected, alternative, decoy, false, nil)
}

func TestMarketReviewAdviceSupersetNamed(t *testing.T) {
	selected, alternative, decoy := mktReviewAdviceFixture(t, []string{"linen apron"}, []string{"linen apron", "ceramic mug"})
	mktAssertReviewAdvice(t, selected, alternative, decoy, true, map[string]bool{"linen apron": true})
}

func TestMarketReviewAdvicePartialOverlap(t *testing.T) {
	selected, alternative, decoy := mktReviewAdviceFixture(t, []string{"linen apron", "ceramic mug"}, []string{"linen apron"})
	mktAssertReviewAdvice(t, selected, alternative, decoy, false, map[string]bool{"linen apron": true})
}

func TestMarketReviewAdvicePricePrefix(t *testing.T) {
	selected, _, _ := mktReviewAdviceFixture(t, []string{"ceramic mug"}, []string{"ceramic mug"})
	out, err := mktRunCLI(t, "market", "price", "--snapshot", fmt.Sprint(selected))
	if err != nil {
		t.Fatal(err)
	}
	note := mktDecode(t, out)["note"].(string)
	if !strings.HasPrefix(note, "no review data collected; price-demand correlation not computed — ") || !strings.Contains(note, "has review data") {
		t.Fatalf("review_advice_price_prefix_lost: %q", note)
	}
}

func TestMarketReviewAdviceExactCase(t *testing.T) {
	selected, alternative, decoy := mktReviewAdviceFixture(t, []string{"ceramic mug"}, []string{"Ceramic mug"})
	mktAssertReviewAdvice(t, selected, alternative, decoy, false, nil)
}
