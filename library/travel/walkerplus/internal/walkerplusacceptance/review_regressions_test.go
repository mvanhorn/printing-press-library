package walkerplusacceptance_test

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/walkerplus"
)

func TestReviewWeeklyClosuresAndPositiveRecurrence(t *testing.T) {
	// 2026-10-11 is Sunday, 12 is Monday, and 13 is Tuesday. A closure
	// excludes its day; only affirmative recurrence can confirm another day.
	tests := []struct {
		name, clause        string
		confirmed, possible []string
		closed              bool
	}{
		{"weekly_short_closed", "毎週月曜休館", []string{}, []string{"2026-10-11", "2026-10-13"}, true},
		{"weekly_full_closed", "毎週月曜日休館", []string{}, []string{"2026-10-11", "2026-10-13"}, true},
		{"weekly_label_closed", "休館日：毎週月曜日", []string{}, []string{"2026-10-11", "2026-10-13"}, true},
		{"plain_closed", "休館日：月曜", []string{}, []string{"2026-10-11", "2026-10-13"}, true},
		{"daily_weekly_closed", "期間中は毎日開催。毎週月曜日休館", []string{"2026-10-11", "2026-10-13"}, []string{}, true},
		{"daily_label_closed", "期間中は毎日開催。休館日：毎週月曜日", []string{"2026-10-11", "2026-10-13"}, []string{}, true},
		{"positive_monday_only", "毎週月曜日開催", []string{"2026-10-12"}, []string{}, false},
		{"positive_sunday_short", "毎日曜開催", []string{"2026-10-11"}, []string{}, false},
		{"positive_sunday_full", "毎日曜日開催", []string{"2026-10-11"}, []string{}, false},
		{"daily_except_monday", "毎日開催（月曜を除く）", []string{"2026-10-11", "2026-10-13"}, []string{}, true},
		{"daily_except_weekends", "毎日開催（土日を除く）", []string{"2026-10-12", "2026-10-13"}, []string{}, false},
		{"positive_then_closure", "毎週月・火曜日開催。毎週月曜日休館", []string{"2026-10-13"}, []string{}, true},
		{"closure_then_positive", "毎週月曜日休館。毎週火曜日開催", []string{"2026-10-13"}, []string{}, true},
		{"unsupported_monthly_first_sunday", "毎月第1日曜日開催", []string{}, []string{"2026-10-11", "2026-10-12", "2026-10-13"}, false},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := eventFixture(fmt.Sprintf("ar0313e%d", 12000+i), tc.name, "2026-10-10", "2026-10-18", "2026年10月10日(土)～10月18日(日)")
			e.schedule = e.period + " " + tc.clause
			s := fixtureSource(t, []sourceEvent{e})
			setListing(s, listingFixture(t, []sourceEvent{e}, "", ""), "/event_list/10/ar0313/eg0120/")
			r, err := fixtureClient(t, s).Shortlist(context.Background(), trip("2026-10-11", "2026-10-13"))
			if err != nil {
				t.Fatal(err)
			}
			got := onlyEvent(t, r)
			if got.Match == nil {
				t.Fatal("positive recurrence fixture lost trip match")
			}
			assertStrings(t, "confirmed_days", got.Match.ConfirmedDays, tc.confirmed)
			assertStrings(t, "possible_days", got.Match.PossibleDays, tc.possible)
			if tc.closed {
				assertStrings(t, "closed_weekdays", got.Schedule.ClosedWeekdays, []string{"月曜"})
			}
			if tc.name == "unsupported_monthly_first_sunday" && len(got.Schedule.Unresolved) == 0 {
				t.Error("unsupported monthly/nth-week schedule omitted unresolved explanation")
			}
			if got.Schedule.Raw == nil || *got.Schedule.Raw != e.schedule {
				t.Errorf("schedule source text lost: %+v", got.Schedule)
			}
			assertEvidence(t, got, "schedule", tc.clause)
			if r.Coverage.ScannedPages != 1 || r.Coverage.DetailCount != 1 || r.Coverage.RequestCount != 4 {
				t.Errorf("weekly matching changed fetch bounds: %+v", r.Coverage)
			}
		})
	}
}

func TestReviewShortlistDefersMissingListingAttributesToDetail(t *testing.T) {
	matching := eventFixture("ar0313e12100", "詳細で所在地と分類が分かる体験", "2026-10-11", "2026-10-11", "2026年10月11日(日)")
	missing := matching
	missing.cityCode, missing.cityJA, missing.categories = "", "", nil
	wrongCity, wrongCategory, unknown := matching, matching, matching
	wrongCity.id, wrongCity.title, wrongCity.cityCode, wrongCity.cityJA, wrongCity.citySlug = "ar0313e12101", "詳細が港区の体験", "ar0313103", "港区", "minato"
	wrongCategory.id, wrongCategory.title, wrongCategory.categories = "ar0313e12102", "詳細が音楽イベント", []categoryFact{{"eg0109", "ライブ・音楽イベント"}}
	unknown.id, unknown.title, unknown.cityCode, unknown.cityJA, unknown.categories = "ar0313e12103", "詳細でも不明のイベント", "", "", nil
	conflictCity, conflictCategory := matching, matching
	// Conflicts sort before eligible candidates. If admitted into the candidate
	// pool, they consume the bounded budget and hit deliberately absent pages.
	conflictCity.id, conflictCity.title, conflictCity.cityCode, conflictCity.cityJA, conflictCity.citySlug = "ar0313e12090", "一覧で港区と分かるイベント", "ar0313103", "港区", "minato"
	conflictCategory.id, conflictCategory.title, conflictCategory.categories = "ar0313e12091", "一覧で音楽と分かるイベント", []categoryFact{{"eg0109", "ライブ・音楽イベント"}}
	details := []sourceEvent{matching, wrongCity, wrongCategory, unknown}
	listings := []sourceEvent{missing}
	for _, detail := range details[1:] {
		listing := detail
		listing.cityCode, listing.cityJA, listing.categories = "", "", nil
		listings = append(listings, listing)
	}
	listings = append(listings, conflictCity, conflictCategory)
	s := fixtureSource(t, details)
	for _, detail := range details {
		for _, page := range []string{"", "data.html", "price.html"} {
			path := "/event/" + detail.id + "/" + page
			var tags strings.Builder
			for _, category := range detail.categories {
				fmt.Fprintf(&tags, `<span class="m-detailtag__tag"><a href="/event_list/%s/">%s</a></span>`, html.EscapeString(category.code), html.EscapeString(category.name))
			}
			s.pages[path] = strings.Replace(s.pages[path], "</body>", tags.String()+"</body>", 1)
		}
	}
	base := "/event_list/10/ar0313113/shibuya/eg0120/"
	setListing(s, listingFixture(t, listings, base+"2.html", ""), base)
	q := trip("2026-10-11", "2026-10-11")
	q.City, q.MaxDetails = "shibuya", 4
	c := fixtureClient(t, s)
	search, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if search.Events == nil || len(search.Events) != 0 {
		t.Fatalf("Search accepted missing or conflicting listing facts: IDs=%v", eventIDs(search.Events))
	}
	before, _ := s.snapshot()
	assertStrings(t, "strict listing-only search requests", before, []string{base})
	if search.Coverage.DetailCount != 0 || search.Coverage.RequestCount != 1 {
		t.Errorf("Search spent a detail budget: %+v", search.Coverage)
	}

	r, err := c.Shortlist(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 1 {
		t.Fatalf("Shortlist must recover exactly the detail-verified positive: IDs=%v coverage=%+v", eventIDs(r.Events), r.Coverage)
	}
	got := r.Events[0]
	if got.ID != matching.id || got.Location.CityJA == nil || *got.Location.CityJA != "渋谷区" || got.Location.CityCode == nil || *got.Location.CityCode != "ar0313113" {
		t.Errorf("shortlist locality lacks authoritative detail facts: %+v", got.Location)
	}
	if len(got.Categories) != 1 || got.Categories[0].Code != "eg0120" {
		t.Errorf("shortlist category lacks event-local detail facts: %+v", got.Categories)
	}
	if got.Match == nil {
		t.Fatal("detail-verified positive missing match")
	}
	assertStrings(t, "confirmed attendance", got.Match.ConfirmedDays, []string{"2026-10-11"})
	requests, maxActive := s.snapshot()
	if len(requests)-len(before) != 13 || r.Coverage.RequestCount != 13 || r.Coverage.ScannedPages != 1 || r.Coverage.DetailCount != 4 || r.Coverage.CandidateCount != 6 || r.Coverage.ExcludedCount != 5 {
		t.Errorf("deferred verification changed page/detail/request bounds: requests=%v coverage=%+v", requests[len(before):], r.Coverage)
	}
	if maxActive > 2 || !r.Coverage.Truncated {
		t.Errorf("deferred verification ignored concurrency/page bounds: active=%d coverage=%+v", maxActive, r.Coverage)
	}
	for _, path := range requests[len(before):] {
		if strings.Contains(path, conflictCity.id) || strings.Contains(path, conflictCategory.id) {
			t.Errorf("explicit listing conflict spent detail budget: %s", path)
		}
	}
}

func TestReviewCoverageJSONCatalogKeys(t *testing.T) {
	coverage := walkerplus.Coverage{CatalogRequests: 1, CatalogRoutes: []string{"https://www.walkerplus.com/event_list/ar0313/"}}
	b, err := json.Marshal(coverage)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["catalog_requests"]) != "1" || string(fields["catalog_routes"]) != `["https://www.walkerplus.com/event_list/ar0313/"]` {
		t.Errorf("catalog JSON contract lost snake_case keys/values: %s", b)
	}
	for _, unexpected := range []string{"CatalogRequests", "CatalogRoutes"} {
		if _, exists := fields[unexpected]; exists {
			t.Errorf("catalog JSON exposes PascalCase %s: %s", unexpected, b)
		}
	}
}
