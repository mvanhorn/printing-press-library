// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelMarketDemandHelpWires smoke-tests that the market demand command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMarketDemandHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "demand", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("market demand --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "demand"} {
		if !strings.Contains(help, want) {
			t.Fatalf("market demand --help missing %q in output:\n%s", want, help)
		}
	}
}

// mktDemandFixture builds one snapshot with a "linen apron" term of 4 listings
// whose pooled recent reviews are split as given, and a review-less term.
func mktDemandFixture(t *testing.T, perListing [4]int, collected bool) {
	t.Helper()
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	defer db.Close()
	d := db.DB()
	taken := time.Now()
	prog := mktFxProg{}
	snap := mktFxSnapshot(t, d, taken, []string{"linen apron", "ceramic mug"}, 1, "complete", prog)
	for i := 0; i < 4; i++ {
		id := int64(1000000001 + i)
		mktFxResult(t, d, snap, mktFxListing{term: "linen apron", pos: i + 1, id: id, shop: int64(5001 + i), title: fmt.Sprintf("Linen Apron %d", i), price: 2400})
		mktFxShop(t, d, snap, int64(5001+i), 500+i, 20, 1600000000)
		if collected {
			mktFxReviews(t, d, snap, id, perListing[i], taken.Add(-24*time.Hour))
		}
	}
	if _, err := d.Exec(`INSERT INTO market_term_pages VALUES (?, 'linen apron', 1, 4, 4), (?, 'ceramic mug', 1, 1, 1)`, snap, snap); err != nil {
		t.Fatal(err)
	}
	// ceramic mug: listings exist but no review data was ever collected.
	mktFxResult(t, d, snap, mktFxListing{term: "ceramic mug", pos: 1, id: 1000000020, shop: 5020, title: "Ceramic Mug", price: 1800})
	if collected {
		// Mark all linen apron listings as collected (a listing may have 0 reviews).
		prog, _ := json.Marshal(mktProg{Pages: []string{}, ReviewsDone: []int64{1000000001, 1000000002, 1000000003, 1000000004}})
		_, _ = d.Exec(`UPDATE market_snapshots SET progress_json = ? WHERE id = ?`, string(prog), snap)
	}
}

func mktDemandTermByName(t *testing.T, out, name string) map[string]any {
	t.Helper()
	for _, x := range mktDecode(t, out)["results"].([]any) {
		if x.(map[string]any)["term"] == name {
			return x.(map[string]any)
		}
	}
	t.Fatalf("term %q not in output:\n%s", name, out)
	return nil
}

func TestMarketDemandCantTellFloorNineVersusTen(t *testing.T) {
	// 9 pooled recent reviews over 3 listings: below the floor of 10.
	mktDemandFixture(t, [4]int{3, 3, 3, 0}, true)
	out, err := mktRunCLI(t, "market", "demand")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("9 reviews:\n%s", out)
	if got := mktDemandTermByName(t, out, "linen apron"); got["state"] != "cant_tell" || got["reviews_in_window"].(float64) != 9 {
		t.Errorf("9 reviews: %v, want cant_tell", got)
	}
}

func TestMarketDemandShownAtTen(t *testing.T) {
	mktDemandFixture(t, [4]int{4, 3, 3, 0}, true)
	out, err := mktRunCLI(t, "market", "demand")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("10 reviews:\n%s", out)
	got := mktDemandTermByName(t, out, "linen apron")
	if got["state"] != "shown" || got["reviews_in_window"].(float64) != 10 {
		t.Errorf("10 reviews: %v, want shown", got)
	}
	// Ranking: the shown term sorts ahead of the absent one.
	first := mktDecode(t, out)["results"].([]any)[0].(map[string]any)
	if first["term"] != "linen apron" {
		t.Errorf("first ranked term = %v, want linen apron", first["term"])
	}
	// The listing with a collected 0 is "shown" with 0, not absent.
	var zero map[string]any
	for _, l := range got["listings"].([]any) {
		if l.(map[string]any)["listing_id"].(float64) == 1000000004 {
			zero = l.(map[string]any)
		}
	}
	if zero == nil || zero["state"] != "shown" || zero["reviews_in_window"].(float64) != 0 {
		t.Errorf("collected-zero listing = %v", zero)
	}
}

func TestMarketDemandCantTellWhenFewerThanThreeListingsHaveReviews(t *testing.T) {
	// 12 reviews but concentrated in 2 listings: still can't tell.
	mktDemandFixture(t, [4]int{6, 6, 0, 0}, true)
	out, err := mktRunCLI(t, "market", "demand")
	if err != nil {
		t.Fatal(err)
	}
	if got := mktDemandTermByName(t, out, "linen apron"); got["state"] != "cant_tell" {
		t.Errorf("2 listings with reviews: %v, want cant_tell", got)
	}
}

func TestMarketDemandAbsentIsNotZero(t *testing.T) {
	mktDemandFixture(t, [4]int{}, false)
	out, err := mktRunCLI(t, "market", "demand")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", out)
	got := mktDemandTermByName(t, out, "linen apron")
	if got["state"] != "absent" {
		t.Errorf("state = %v, want absent", got["state"])
	}
	if v, present := got["reviews_in_window"]; present && v != nil {
		t.Errorf("absent term reported reviews_in_window = %v; absent must not be 0", v)
	}
	for _, l := range got["listings"].([]any) {
		lm := l.(map[string]any)
		if lm["state"] != "absent" || lm["reviews_in_window"] != nil {
			t.Errorf("listing %v should be absent with null reviews", lm)
		}
	}
	// Shop sales still shown even without review data.
	if got["shop_transaction_sold_count_total"] == nil {
		t.Error("shop sales should still be reported")
	}
}

func TestMarketDemandEmptyStoreExitsZero(t *testing.T) {
	mktTestEnv(t)
	out, err := mktRunCLI(t, "market", "demand")
	if err != nil {
		t.Fatalf("empty store: %v", err)
	}
	m := mktDecode(t, out)
	if len(m["results"].([]any)) != 0 || m["note"] == nil {
		t.Errorf("got %v", m)
	}
}

func TestMarketDemandWindowExcludesOldReviews(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	d := db.DB()
	taken := time.Now()
	snap := mktFxSnapshot(t, d, taken, []string{"linen apron"}, 1, "complete", mktFxProg{reviewsDone: []int64{1000000001}})
	mktFxResult(t, d, snap, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 5001, title: "Linen Apron", price: 2400})
	mktFxReviews(t, d, snap, 1000000001, 5, taken.Add(-30*24*time.Hour))
	mktFxReviews(t, d, snap, 1000000001, 7, taken.Add(-400*24*time.Hour))
	db.Close()
	out, err := mktRunCLI(t, "market", "demand", "--window", "12mo", "--min-reviews", "1")
	if err != nil {
		t.Fatal(err)
	}
	got := mktDemandTermByName(t, out, "linen apron")
	if got["reviews_in_window"].(float64) != 5 {
		t.Errorf("reviews_in_window = %v, want 5 (the 400-day-old reviews are outside 12mo)", got["reviews_in_window"])
	}
}

func TestMarketDemandOrderBasis(t *testing.T) {
	for _, collected := range []bool{false, true} {
		t.Run(fmt.Sprint(collected), func(t *testing.T) {
			mktDemandFixture(t, [4]int{0, 0, 0, 0}, collected)
			out, err := mktRunCLI(t, "market", "demand")
			if err != nil {
				t.Fatal(err)
			}
			result := mktDecode(t, out)
			for _, value := range result["results"].([]any) {
				term := value.(map[string]any)
				want := "search_position"
				if collected && term["term"] == "linen apron" {
					want = "reviews_in_window"
				}
				if term["order_basis"] != want {
					t.Errorf("term = %v, want basis %s", term, want)
				}
			}
			const note = "no review data collected; listings are ordered by search position, not demand — re-run market snapshot with --with-reviews"
			if collected {
				if _, ok := result["note"]; ok {
					t.Errorf("unexpected note: %v", result["note"])
				}
			} else if result["note"] != note {
				t.Errorf("note = %v", result["note"])
			}
		})
	}
}

func TestMarketDemandShowsNoResultsTerm(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	snap := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron", "ceramic mug"}, 1, "complete", mktFxProg{reviewsDone: []int64{1000000001}})
	mktFxResult(t, db.DB(), snap, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 9000001})
	if _, err := db.DB().Exec(`INSERT INTO market_term_pages VALUES (?, 'linen apron', 1, 1, 1), (?, 'ceramic mug', 1, 0, 0)`, snap, snap); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, err := mktRunCLI(t, "market", "demand")
	if err != nil {
		t.Fatal(err)
	}
	term := mktDemandTermByName(t, out, "ceramic mug")
	if term["state"] != "no_results" || term["listings_total"] != float64(0) || term["listings_with_review_data"] != float64(0) || term["order_basis"] != "search_position" || term["note"] != "Etsy's search returned no results for this term" {
		t.Fatalf("term: %v", term)
	}
	if listings, ok := term["listings"].([]any); !ok || len(listings) != 0 {
		t.Fatalf("listings: %v", term["listings"])
	}
	if got := mktDecode(t, out)["note"]; got != "1 term(s) returned no results from Etsy's search API" {
		t.Fatalf("note: %v", got)
	}
	if first := mktDecode(t, out)["results"].([]any)[0].(map[string]any); first["term"] != "linen apron" {
		t.Fatalf("order: %s", out)
	}
}

func TestMarketDemandLegacySnapshotNote(t *testing.T) {
	for _, missingTable := range []bool{false, true} {
		t.Run(fmt.Sprint(missingTable), func(t *testing.T) {
			mktTestEnv(t)
			db := mktOpenFixtureStore(t)
			snap := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron", "ceramic mug"}, 1, "complete", mktFxProg{})
			mktFxResult(t, db.DB(), snap, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 555001})
			if _, err := db.DB().Exec(`UPDATE market_snapshots SET progress_json = ? WHERE id = ?`, `{"pages":["ceramic mug|1"]}`, snap); err != nil {
				t.Fatal(err)
			}
			if missingTable {
				if _, err := db.DB().Exec(`DROP TABLE market_term_pages`); err != nil {
					t.Fatal(err)
				}
			}
			db.Close()
			out, err := mktRunCLI(t, "market", "demand")
			if err != nil {
				t.Fatal(err)
			}
			result := mktDecode(t, out)
			const legacy = "1 term(s) were fetched before per-term outcome recording; any with no results are not shown"
			const noReviews = "no review data collected; listings are ordered by search position, not demand — re-run market snapshot with --with-reviews"
			const rowNote = `"linen apron": no review data was collected for this term (absent is not zero); re-run market snapshot with --with-reviews`
			if result["note"] != noReviews+"; "+legacy+"; "+rowNote {
				t.Fatalf("note: %v", result["note"])
			}
			if len(result["results"].([]any)) != 1 {
				t.Fatalf("results: %s", out)
			}
		})
	}
}

func TestMarketDemandTermPageTotalsAndOrder(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	defer db.Close()
	if _, err := db.DB().Exec(`INSERT INTO market_term_pages VALUES (1, 'linen apron', 1, 2, 2), (1, 'linen apron', 2, 2, 0), (1, 'ceramic mug', 1, 0, 0), (2, 'Juniper Ceramics', 1, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	pages, err := mktLoadTermPages(db.DB(), 1)
	if err != nil || len(pages) != 2 || pages["linen apron"] != 2 {
		t.Fatalf("pages=%v err=%v", pages, err)
	}
	terms := mktDemandTerms([]string{"Juniper Ceramics", "ceramic mug", "linen apron", "ceramic mug"}, map[string]int{"Juniper Ceramics": 0, "ceramic mug": 0, "linen apron": 2}, []mktRow{{Term: "linen apron", ListingID: 1000000001}}, nil, nil, 10, 10, nil)
	if len(terms) != 3 || terms[0].Term != "linen apron" || terms[1].Term != "Juniper Ceramics" || terms[2].Term != "ceramic mug" {
		t.Fatalf("terms: %+v", terms)
	}
	terms = mktDemandTerms(nil, nil, []mktRow{{Term: "linen apron", ListingID: 1000000001}}, nil, nil, 10, 10, nil)
	if len(terms) != 1 || terms[0].State != "absent" {
		t.Fatalf("defensive rows: %+v", terms)
	}
}

func TestMarketReviewSnapshotAdvice(t *testing.T) {
	for _, source := range []string{"none", "progress", "rows", "both"} {
		t.Run(source, func(t *testing.T) {
			mktTestEnv(t)
			db := mktOpenFixtureStore(t)
			taken := time.Now()
			prog := mktFxProg{}
			if source == "progress" || source == "both" {
				prog.reviewsDone = []int64{1000000001}
			}
			older := mktFxSnapshot(t, db.DB(), taken.Add(-time.Hour), []string{"linen apron"}, 1, "complete", prog)
			// Link review evidence to the term for per-term eligibility.
			mktFxResult(t, db.DB(), older, mktFxListing{term: "linen apron", id: 1000000001, shop: 9000001, pos: 1, title: "linen apron"})
			if source == "rows" || source == "both" {
				mktFxReviews(t, db.DB(), older, 1000000001, 1, taken.Add(-2*time.Hour))
			}
			newer := mktFxSnapshot(t, db.DB(), taken, []string{"linen apron"}, 1, "complete", mktFxProg{})
			mktFxResult(t, db.DB(), newer, mktFxListing{term: "linen apron", id: 1000000002, shop: 9000001, pos: 1, title: "Linen Apron", price: 2400})
			if _, err := db.DB().Exec(`INSERT INTO market_term_pages VALUES (?, 'linen apron', 1, 1, 1)`, newer); err != nil {
				t.Fatal(err)
			}
			// A more recent partial snapshot must never be suggested.
			mktFxSnapshot(t, db.DB(), taken, []string{"linen apron"}, 1, "partial", mktFxProg{reviewsDone: []int64{1000000003}})
			db.Close()
			for _, command := range []string{"demand", "price"} {
				for _, explicit := range []bool{false, true} {
					args := []string{"market", command}
					if explicit {
						args = append(args, "--snapshot", fmt.Sprint(newer))
					}
					out, err := mktRunCLI(t, args...)
					if err != nil {
						t.Fatal(err)
					}
					got := mktDecode(t, out)
					if got["snapshot_id"] != float64(newer) {
						t.Fatalf("selection changed: %v", got)
					}
					hint := fmt.Sprintf("snapshot %d has review data; use --snapshot %d, or re-run market snapshot --with-reviews", older, older)
					want := "no review data collected; price-demand correlation not computed — " + hint
					if command == "demand" {
						want = "no review data collected; listings are ordered by search position, not demand — " + hint
					}
					if source == "none" {
						want = "no review data collected; price-demand correlation not computed — re-run market snapshot with --with-reviews"
						if command == "demand" {
							want = "no review data collected; listings are ordered by search position, not demand — re-run market snapshot with --with-reviews"
						}
					}
					if got["note"] != want {
						t.Errorf("%s note=%q want %q", command, got["note"], want)
					}
					if command == "demand" {
						term := got["results"].([]any)[0].(map[string]any)
						tail := hint
						if source == "none" {
							tail = "re-run market snapshot with --with-reviews"
						}
						if want := "no review data was collected for this term (absent is not zero); " + tail; term["note"] != want {
							t.Errorf("term note=%q want %q", term["note"], want)
						}
					}
				}
			}
		})
	}
}

func TestMarketReviewSnapshotExcludesSelected(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	defer db.Close()
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"ceramic mug"}, 1, "complete", mktFxProg{reviewsDone: []int64{1000000001}})
	selected, err := mktGetSnapshot(db.DB(), id)
	if err != nil {
		t.Fatal(err)
	}
	alt, err := mktOlderReviewSnapshot(db.DB(), selected)
	if err != nil || alt != nil {
		t.Fatalf("alt=%v err=%v", alt, err)
	}
}

func mktDemandPartialFixture(t *testing.T, fetched bool) (int64, string) {
	t.Helper()
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron", "ceramic mug"}, 1, "partial", mktFxProg{})
	if fetched {
		mktFxResult(t, db.DB(), id, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 9000001})
		if _, err := db.DB().Exec(`INSERT INTO market_term_pages VALUES (?, 'linen apron', 1, 1, 1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	out, err := mktRunCLI(t, "market", "demand", "--snapshot", fmt.Sprint(id))
	if err != nil {
		t.Fatal(err)
	}
	return id, out
}

func TestMarketDemandNotFetchedTerm(t *testing.T) {
	id, out := mktDemandPartialFixture(t, true)
	rows := mktDecode(t, out)["results"].([]any)
	var found bool
	for _, row := range rows {
		r := row.(map[string]any)
		if r["term"] == "ceramic mug" {
			found = r["state"] == "not_fetched" && r["listings_total"] == nil && len(r["listings"].([]any)) == 0
		}
	}
	if !found || !strings.Contains(out, fmt.Sprintf("1 term(s) not yet fetched; resume with etsy-pp-cli market snapshot --resume %d", id)) {
		t.Fatalf("NOT_FETCHED_BRANCH_BROKEN: %s", out)
	}
}

func TestMarketDemandBrandNewPartialAllNotFetched(t *testing.T) {
	_, out := mktDemandPartialFixture(t, false)
	rows := mktDecode(t, out)["results"].([]any)
	if len(rows) != 2 || strings.Contains(out, "before per-term outcome recording") {
		t.Fatalf("BRAND_NEW_PARTIAL_BROKEN: %s", out)
	}
	for _, row := range rows {
		r := row.(map[string]any)
		if r["state"] != "not_fetched" || r["listings_total"] != nil || len(r["listings"].([]any)) != 0 {
			t.Fatalf("BRAND_NEW_PARTIAL_BROKEN: %s", out)
		}
	}
}

func TestMarketDemandMixedLegacyOmitted(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron", "ceramic mug"}, 1, "partial", mktFxProg{})
	if _, err := db.DB().Exec(`UPDATE market_snapshots SET progress_json=? WHERE id=?`, `{"pages":["ceramic mug|1"]}`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`INSERT INTO market_term_pages VALUES (?, 'linen apron', 1, 0, 0)`, id); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, err := mktRunCLI(t, "market", "demand", "--snapshot", fmt.Sprint(id))
	if err != nil {
		t.Fatal(err)
	}
	rows := mktDecode(t, out)["results"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["term"] != "linen apron" || !strings.Contains(out, "1 term(s) were fetched before per-term outcome recording; any with no results are not shown") {
		t.Fatalf("MIXED_LEGACY_OMISSION_BROKEN: %s", out)
	}
}
