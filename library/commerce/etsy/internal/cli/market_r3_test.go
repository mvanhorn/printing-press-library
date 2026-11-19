package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
)

func TestMarketTrendResumeHint(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprintf("Partial_%t", partial), func(t *testing.T) {
			mktTestEnv(t)
			db := mktOpenFixtureStore(t)
			defer db.Close()
			mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "complete", mktFxProg{})
			var newest int64
			if partial {
				mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "partial", mktFxProg{})
				newest = mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "partial", mktFxProg{})
			}
			out, err := mktRunCLI(t, "market", "trend")
			if err != nil {
				t.Fatalf("TREND_HINT: %v", err)
			}
			note, _ := mktDecode(t, out)["note"].(string)
			if partial {
				expected := fmt.Sprintf("resume the partial snapshot with etsy-pp-cli market snapshot --resume %d", newest)
				if !strings.HasSuffix(note, expected) || strings.Contains(note, "take another snapshot") {
					t.Fatalf("TREND_HINT: %q expected %q", note, expected)
				}
			} else if !strings.Contains(note, `take another snapshot with etsy-pp-cli market snapshot --term "SEARCH TERM"`) || strings.Contains(note, "resume") {
				t.Fatalf("TREND_HINT: %q", note)
			}
		})
	}
}

func TestMarketTermAlternativeEligibility(t *testing.T) {
	for _, doneOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("DoneOnly_%t", doneOnly), func(t *testing.T) {
			mktTestEnv(t)
			db := mktOpenFixtureStore(t)
			defer db.Close()
			terms := []string{"linen apron", "ceramic mug"}
			prog := mktFxProg{}
			if doneOnly {
				prog.reviewsDone = []int64{1000000002}
			}
			alt := mktFxSnapshot(t, db.DB(), time.Now().Add(-time.Hour), terms, 1, "complete", prog)
			selected := mktFxSnapshot(t, db.DB(), time.Now(), terms, 1, "complete", mktFxProg{})
			for _, snap := range []int64{alt, selected} {
				for i, term := range terms {
					mktFxResult(t, db.DB(), snap, mktFxListing{term: term, id: 1000000001 + int64(i), shop: 9000001, pos: 1, title: term})
				}
			}
			if !doneOnly {
				mktFxReviews(t, db.DB(), alt, 1000000002, 1, time.Now())
			}
			out, err := mktRunCLI(t, "market", "demand", "--snapshot", fmt.Sprint(selected))
			if err != nil {
				t.Fatalf("TERM_ALT_ELIGIBILITY: %v", err)
			}
			results := mktDecode(t, out)["results"].([]any)
			if len(results) != 2 {
				t.Fatalf("TERM_ALT_ELIGIBILITY: %s", out)
			}
			for _, raw := range results {
				row := raw.(map[string]any)
				note, _ := row["note"].(string)
				if row["term"] == "linen apron" {
					if strings.Contains(note, "has review data") || !strings.Contains(note, "re-run market snapshot") {
						t.Fatalf("TERM_ALT_ELIGIBILITY: %q", note)
					}
				} else if !strings.Contains(note, fmt.Sprintf("snapshot %d has review data; use --snapshot %d", alt, alt)) {
					t.Fatalf("TERM_ALT_ELIGIBILITY: %q", note)
				}
			}
		})
	}
}

func TestMarketCrossPageOwnership(t *testing.T) {
	for _, scenario := range []string{"better_page_takes_ownership", "in_response_duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			mktTestEnv(t)
			page1 := []int64{1000000002, 1000000003, 1000000001}
			page2 := []int64{1000000004, 1000000005, 1000000001}
			if scenario == "in_response_duplicate" {
				page1 = []int64{1000000001, 1000000002, 1000000001}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ids := page1
				if r.URL.Query().Get("offset") == "25" {
					ids = page2
				}
				results := []map[string]any{}
				for _, listing := range ids {
					results = append(results, map[string]any{"listing_id": listing, "shop_id": 9000001, "title": "linen apron"})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"count": 50, "results": results})
			}))
			defer server.Close()
			db := mktOpenFixtureStore(t)
			defer db.Close()
			id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 2, "partial", mktFxProg{})
			c := client.New(&config.Config{BaseURL: server.URL}, 5*time.Second, 0)
			c.NoCache = true
			run := &mktRun{ctx: context.Background(), c: c, db: db.DB(), snap: &mktSnapshot{ID: id, SortOn: "score"}, perPage: 25}
			fetch := func(page int) {
				t.Helper()
				if _, err := run.mktFetchPage("linen apron", page); err != nil {
					t.Fatalf("CROSS_PAGE_OWNERSHIP_FETCH: %v", err)
				}
			}
			assertOwner := func(wantPage, wantPos int) {
				t.Helper()
				var count, page, pos int
				err := db.DB().QueryRow(`SELECT count(*),min(page),min(position) FROM market_results WHERE snapshot_id=? AND term='linen apron' AND listing_id=1000000001`, id).Scan(&count, &page, &pos)
				if err != nil || count != 1 || page != wantPage || pos != wantPos {
					t.Fatalf("CROSS_PAGE_OWNERSHIP_BEST_POSITION: count=%d page=%d position=%d want=%d/%d err=%v", count, page, pos, wantPage, wantPos, err)
				}
			}
			if scenario == "in_response_duplicate" {
				fetch(1)
				assertOwner(1, 1)
				return
			}
			fetch(2)
			assertOwner(2, 28)
			fetch(1)
			assertOwner(1, 3)
			page2 = page2[:2]
			fetch(2)
			assertOwner(1, 3)
		})
	}
	mktTestEnv(t)
	page1 := []int64{1000000002, 1000000003, 1000000001}
	page2 := []int64{1000000004, 1000000005, 1000000001}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids := page1
		if r.URL.Query().Get("offset") == "25" {
			ids = page2
		}
		results := []map[string]any{}
		for _, id := range ids {
			results = append(results, map[string]any{"listing_id": id, "shop_id": 9000001, "title": "linen apron"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"count": 50, "results": results})
	}))
	defer server.Close()
	db := mktOpenFixtureStore(t)
	defer db.Close()
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 2, "partial", mktFxProg{})
	c := client.New(&config.Config{BaseURL: server.URL}, 5*time.Second, 0)
	c.NoCache = true
	run := &mktRun{ctx: context.Background(), c: c, db: db.DB(), snap: &mktSnapshot{ID: id, SortOn: "score"}, perPage: 25}
	fetch := func(page int) {
		t.Helper()
		if _, err := run.mktFetchPage("linen apron", page); err != nil {
			t.Fatalf("CROSS_PAGE_OWNERSHIP: %v", err)
		}
	}
	assertOwner := func() {
		t.Helper()
		var page, pos int
		if err := db.DB().QueryRow(`SELECT page,position FROM market_results WHERE snapshot_id=? AND listing_id=1000000001`, id).Scan(&page, &pos); err != nil || page != 1 || pos != 3 {
			t.Fatalf("CROSS_PAGE_OWNERSHIP: page=%d position=%d err=%v", page, pos, err)
		}
	}
	fetch(1)
	fetch(2)
	assertOwner()
	page2 = page2[:2]
	fetch(2)
	assertOwner()
	var count, total int
	if err := db.DB().QueryRow(`SELECT count(*) FROM market_results WHERE snapshot_id=? AND term='linen apron'`, id).Scan(&count); err != nil {
		t.Fatalf("CROSS_PAGE_OWNERSHIP: %v", err)
	}
	if err := db.DB().QueryRow(`SELECT sum(listings) FROM market_term_pages WHERE snapshot_id=? AND term='linen apron'`, id).Scan(&total); err != nil || count != total {
		t.Fatalf("CROSS_PAGE_OWNERSHIP: count=%d total=%d err=%v", count, total, err)
	}
	// Restore the duplicate, then remove it from its owning page. No shadow copy
	// is stored for page 2: it reappears only when that page is fetched again.
	page2 = append(page2, 1000000001)
	fetch(2)
	assertOwner()
	page1 = page1[:2]
	fetch(1)
	if err := db.DB().QueryRow(`SELECT count(*) FROM market_results WHERE snapshot_id=? AND listing_id=1000000001`, id).Scan(&count); err != nil || count != 0 {
		t.Fatalf("CROSS_PAGE_OWNERSHIP: owner removal count=%d err=%v", count, err)
	}
	fetch(2)
	var page, pos int
	if err := db.DB().QueryRow(`SELECT page,position FROM market_results WHERE snapshot_id=? AND listing_id=1000000001`, id).Scan(&page, &pos); err != nil || page != 2 || pos != 28 {
		t.Fatalf("CROSS_PAGE_OWNERSHIP: refetch page=%d position=%d err=%v", page, pos, err)
	}
}
