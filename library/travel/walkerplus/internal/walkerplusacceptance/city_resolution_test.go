package walkerplusacceptance_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/walkerplus"
)

func shinjukuEvent(id, title, date string) sourceEvent {
	e := eventFixture(id, title, date, date, "2026年10月11日(日)")
	e.cityCode, e.cityJA, e.citySlug = "ar0313104", "新宿区", "shinjuku"
	return e
}

func withTokyoCityCatalog(t *testing.T, s *sourceTransport) {
	t.Helper()
	s.pages["/event_list/ar0313/"] = renderFixture(t, "tokyo-city-catalog", nil)
}

func TestCatalogCityResolutionUsesSourcePathAndSharedPageBudget(t *testing.T) {
	for i, city := range []string{"ar0313104", "shinjuku", "新宿区"} {
		t.Run(city, func(t *testing.T) {
			positive := shinjukuEvent(fmt.Sprintf("ar0313e%d", 1060+i*10), "新宿区の体験会", "2026-10-11")
			second := positive
			second.id, second.title = fmt.Sprintf("ar0313e%d", 1061+i*10), "新宿区の別の体験会"
			wrongCity := eventFixture(fmt.Sprintf("ar0313e%d", 1062+i*10), "渋谷区の体験会", "2026-10-11", "2026-10-11", "2026年10月11日(日)")
			wrongDate := shinjukuEvent(fmt.Sprintf("ar0313e%d", 1063+i*10), "翌日の新宿区の体験会", "2026-10-12")
			wrongDate.period = "2026年10月12日(月)"
			s := fixtureSource(t, []sourceEvent{positive, second, wrongCity, wrongDate})
			withTokyoCityCatalog(t, s)
			base := "/event_list/10/ar0313104/shinjuku/eg0120/"
			setListing(s, listingFixture(t, []sourceEvent{positive, wrongCity, wrongDate}, base+"2.html", ""), base)
			setListing(s, listingFixture(t, []sourceEvent{second, positive}, base+"3.html", ""), base+"2.html")
			q := trip("2026-10-11", "2026-10-11")
			q.City, q.MaxPages = city, 2
			if city == "ar0313104" {
				q.Prefecture = ""
			} // Canonical codes carry their parent prefecture.
			r, err := fixtureClient(t, s).Search(context.Background(), q)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Events) != 2 {
				t.Fatalf("source city route lost positive events or leaked wrong city/date: IDs=%v coverage=%+v", eventIDs(r.Events), r.Coverage)
			}
			assertStrings(t, "source city results", eventIDs(r.Events), []string{positive.id, second.id})
			if r.Query.City != "ar0313104" || r.Query.Prefecture != "ar0313" {
				t.Errorf("city query was not canonicalized using source catalog: %+v", r.Query)
			}
			for _, got := range r.Events {
				if got.Location.CityCode == nil || *got.Location.CityCode != "ar0313104" || got.Location.CityJA == nil || *got.Location.CityJA != "新宿区" {
					t.Errorf("resolved code/name missing actual locality evidence: %+v", got.Location)
				}
				if got.StartDate == nil || *got.StartDate != "2026-10-11" {
					t.Errorf("wrong date leaked into city query: %+v", got)
				}
			}
			requests, _ := s.snapshot()
			assertStrings(t, "catalog then actual city listing paths", requests, []string{"/event_list/ar0313/", base, base + "2.html"})
			if r.Coverage.RequestCount != 3 || r.Coverage.CatalogRequests != 1 || r.Coverage.ScannedPages != 2 || r.Coverage.RequestedPages != 2 || r.Coverage.DetailCount != 0 {
				t.Errorf("one catalog fetch must be additional to the shared listing page budget: %+v", r.Coverage)
			}
			assertStrings(t, "catalog routes", r.Coverage.CatalogRoutes, []string{"https://www.walkerplus.com/event_list/ar0313/"})
			assertStrings(t, "listing routes", r.Coverage.Routes, []string{"https://www.walkerplus.com" + base, "https://www.walkerplus.com" + base + "2.html"})
			if !r.Coverage.Truncated || r.Coverage.NextPage == nil || *r.Coverage.NextPage != "https://www.walkerplus.com"+base+"3.html" {
				t.Errorf("unscanned third listing page lost budget coverage: %+v", r.Coverage)
			}
		})
	}
}

func TestCatalogCityResolutionRejectsUnknownAndForeignCities(t *testing.T) {
	tests := []struct {
		name, prefecture, city string
		requests               int
	}{
		{"unknown_catalog_code", "tokyo", "ar0313999", 1},
		{"unknown_catalog_alias", "tokyo", "no-such-city", 1},
		{"foreign_catalog_alias", "tokyo", "kyoto", 1},
		{"wrong_requested_prefecture", "kyoto", "ar0313104", 0},
		{"invalid_code_parent", "tokyo", "ar9999104", 0},
		{"alias_without_prefecture", "", "shinjuku", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := fixtureSource(t, nil)
			withTokyoCityCatalog(t, s)
			q := trip("2026-10-11", "2026-10-11")
			q.Prefecture, q.City = tc.prefecture, tc.city
			r, err := fixtureClient(t, s).Search(context.Background(), q)
			if err == nil {
				t.Fatalf("invalid city yielded empty success instead of clear error: %+v", r)
			}
			if !strings.Contains(strings.ToLower(err.Error()), "city") && !strings.Contains(strings.ToLower(err.Error()), "prefecture") {
				t.Errorf("city validation error lacks clear context: %v", err)
			}
			requests, _ := s.snapshot()
			if len(requests) != tc.requests {
				t.Errorf("city validation exceeded catalog lookup bound: requests=%v want_count=%d error=%v", requests, tc.requests, err)
			}
			for _, path := range requests {
				if path != "/event_list/ar0313/" {
					t.Errorf("invalid city fetched guessed/foreign listing route: %v", requests)
				}
			}
		})
	}
}

func TestCatalogCityShortlistRechecksDetailLocality(t *testing.T) {
	stable := shinjukuEvent("ar0313e1100", "新宿区で開催される体験会", "2026-10-11")
	moved := shinjukuEvent("ar0313e1101", "詳細で会場変更された体験会", "2026-10-11")
	changed := moved
	changed.cityCode, changed.cityJA, changed.citySlug = "ar0313113", "渋谷区", "shibuya"
	s := fixtureSource(t, []sourceEvent{stable, changed})
	withTokyoCityCatalog(t, s)
	base := "/event_list/10/ar0313104/shinjuku/eg0120/"
	setListing(s, listingFixture(t, []sourceEvent{stable, moved}, "", ""), base)
	q := trip("2026-10-11", "2026-10-11")
	q.City, q.MaxDetails = "shinjuku", 2
	r, err := fixtureClient(t, s).Shortlist(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	got := onlyEvent(t, r)
	if got.ID != stable.id || got.Location.CityCode == nil || *got.Location.CityCode != "ar0313104" || got.Location.CityJA == nil || *got.Location.CityJA != "新宿区" {
		t.Errorf("changed detail locality passed stale listing city code: %+v", got)
	}
	if got.Match == nil {
		t.Fatal("resolved city positive lost derived match")
	}
	assertStrings(t, "resolved city confirmed_days", got.Match.ConfirmedDays, []string{"2026-10-11"})
	requests, _ := s.snapshot()
	if len(requests) != 8 || r.Coverage.RequestCount != 8 || r.Coverage.CatalogRequests != 1 || r.Coverage.ScannedPages != 1 || r.Coverage.CandidateCount != 2 || r.Coverage.DetailCount != 2 || r.Coverage.ExcludedCount != 1 {
		t.Errorf("detail locality recheck/budget coverage incorrect: requests=%v coverage=%+v", requests, r.Coverage)
	}
}

func TestCatalogCityForeignKnownAliasConsultsRequestedPrefecture(t *testing.T) {
	q := trip("2026-10-11", "2026-10-11")
	q.Prefecture, q.City = "kyoto", "shibuya"
	normalized, err := walkerplus.NormalizeQuery(q)
	if err != nil {
		t.Fatalf("ambiguous alias should defer to requested prefecture catalog: %v", err)
	}
	if normalized.Prefecture != "ar0726" || normalized.City != "shibuya" {
		t.Fatalf("offline normalization guessed a foreign city code: %+v", normalized)
	}

	// A nonempty Kyoto source page proves the catalog is valid. The foreign
	// navigation link deliberately tests that a familiar alias from Tokyo cannot
	// be accepted in Kyoto, even when it appears in shared navigation.
	castle := eventFixture("ar0726e612292", "京都南丹園部城祭り2026", "2026-10-03", "2026-10-03", "2026年10月3日(土)")
	castle.prefectureCode, castle.prefectureJA, castle.cityCode, castle.cityJA = "ar0726", "京都府", "", "南丹市"
	castle.venue = "南丹市国際交流会館 園部公園一帯"
	castle.categories = []categoryFact{{"eg0135", "祭り"}, {"eg126", "展示会"}}
	navigation := `<nav><a href="/event_list/ar0726/">京都府</a><a href="/event_list/ar0313113/shibuya/">渋谷区</a></nav>`
	catalog := strings.ReplaceAll(listingFixture(t, []sourceEvent{castle}, "", navigation), "東京都", "京都府")
	s := fixtureSource(t, nil)
	s.pages["/event_list/ar0726/"] = catalog
	r, err := fixtureClient(t, s).Search(context.Background(), q)
	if !errors.Is(err, walkerplus.ErrInvalidQuery) {
		t.Fatalf("foreign alias must fail ErrInvalidQuery after source lookup; result=%+v error=%v", r, err)
	}
	requests, _ := s.snapshot()
	assertStrings(t, "foreign alias catalog lookup only", requests, []string{"/event_list/ar0726/"})
	if r.Coverage.RequestCount != 1 || r.Coverage.CatalogRequests != 1 || r.Coverage.ScannedPages != 0 {
		t.Errorf("foreign alias guessed a listing route or miscounted catalog work: %+v", r.Coverage)
	}
	assertStrings(t, "foreign alias catalog routes", r.Coverage.CatalogRoutes, []string{"https://www.walkerplus.com/event_list/ar0726/"})
	assertStrings(t, "foreign alias event routes", r.Coverage.Routes, []string{})
}

func eventIDs(events []walkerplus.Event) []string {
	ids := []string{}
	for _, e := range events {
		ids = append(ids, e.ID)
	}
	return ids
}
