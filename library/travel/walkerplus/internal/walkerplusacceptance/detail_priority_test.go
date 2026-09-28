package walkerplusacceptance_test

import (
	"context"
	"strings"
	"testing"
)

func TestReviewShortlistPrioritizesVerifiedListingCandidates(t *testing.T) {
	for _, sort := range []string{"relevance", "start", "source"} {
		t.Run(sort, func(t *testing.T) {
			for _, budget := range []int{1, 2} {
				name := "one_detail_selects_known"
				if budget == 2 {
					name = "remaining_budget_enriches_unknown"
				}
				t.Run(name, func(t *testing.T) {
					unknownDetail := eventFixture("ar0313e13000", "一覧では所在地と分類が不明", "2026-10-10", "2026-10-11", "2026年10月10日(土)～10月11日(日)")
					unknownDetail.schedule = unknownDetail.period + " 期間中は毎日開催"
					unknownListing := unknownDetail
					unknownListing.cityCode, unknownListing.cityJA, unknownListing.categories = "", "", nil
					known := eventFixture("ar0313e13001", "一覧ですべての条件が分かる体験", "2026-10-11", "2026-10-11", "2026年10月11日(日)")
					s := fixtureSource(t, []sourceEvent{unknownDetail, known})
					// The earlier unknown card comes first for all requested sorts.
					// Only event-local detail tags can resolve its missing category.
					for _, page := range []string{"", "data.html", "price.html"} {
						path := "/event/" + unknownDetail.id + "/" + page
						tag := `<span class="m-detailtag__tag"><a href="/event_list/eg0120/">体験イベント・アクティビティ</a></span>`
						s.pages[path] = strings.Replace(s.pages[path], "</body>", tag+"</body>", 1)
					}
					base := "/event_list/10/ar0313113/shibuya/eg0120/"
					setListing(s, listingFixture(t, []sourceEvent{unknownListing, known}, "", ""), base)
					q := trip("2026-10-11", "2026-10-11")
					q.City, q.Sort, q.MaxDetails = "shibuya", sort, budget
					r, err := fixtureClient(t, s).Shortlist(context.Background(), q)
					if err != nil {
						t.Fatal(err)
					}
					if budget == 1 {
						assertStrings(t, "verified candidate takes first detail slot", eventIDs(r.Events), []string{known.id})
					} else {
						assertStrings(t, "final requested sort after bounded enrichment", eventIDs(r.Events), []string{unknownDetail.id, known.id})
					}
					for _, e := range r.Events {
						if e.Match == nil {
							t.Fatalf("returned event %s has no detail match", e.ID)
						}
						assertStrings(t, "detail-confirmed attendance", e.Match.ConfirmedDays, []string{"2026-10-11"})
						if e.Location.CityCode == nil || *e.Location.CityCode != "ar0313113" || len(e.Categories) != 1 || e.Categories[0].Code != "eg0120" {
							t.Errorf("returned event %s lacks verified city/category: city=%+v categories=%+v", e.ID, e.Location, e.Categories)
						}
					}
					requests, maxActive := s.snapshot()
					if len(requests) != 1+budget*3 || r.Coverage.RequestCount != 1+budget*3 || r.Coverage.ScannedPages != 1 || r.Coverage.DetailCount != budget || r.Coverage.CandidateCount != 2 || r.Coverage.ReturnedCount != budget || r.Coverage.CatalogRequests != 0 {
						t.Errorf("candidate priority changed fetch bounds: requests=%v coverage=%+v", requests, r.Coverage)
					}
					if maxActive > 2 || r.Coverage.Truncated != (budget == 1) {
						t.Errorf("candidate priority changed concurrency/truncation: active=%d coverage=%+v", maxActive, r.Coverage)
					}
					for _, path := range requests {
						if budget == 1 && strings.Contains(path, unknownDetail.id) {
							t.Errorf("unknown listing consumed sole detail budget: %s", path)
						}
					}
				})
			}
		})
	}
}
