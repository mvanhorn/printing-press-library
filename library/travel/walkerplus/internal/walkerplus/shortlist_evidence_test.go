package walkerplus

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestShortlistEnrichesUnknownFiltersButRejectsKnownConflicts(t *testing.T) {
	for _, tc := range []struct {
		name, listingCity, listingCategory, detailCity, detailCategory string
		details, returned                                              int
	}{
		{"resolved_missing_attributes", "", "", "渋谷区", "eg0107", 1, 1},
		{"unresolved_attributes", "", "", "", "", 1, 0},
		{"conflicting_detail", "", "", "新宿区", "eg0107", 1, 0},
		{"known_city_conflict", "新宿区", "", "渋谷区", "eg0107", 0, 0},
		{"known_category_conflict", "", "eg0102", "渋谷区", "eg0107", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			list := string(cityFixture(t, tc.listingCity))
			if tc.listingCategory != "" {
				list = strings.Replace(list, "</li>", "<a href=\"/event_list/"+tc.listingCategory+"/\">category</a></li>", 1)
			}
			schema, err := json.Marshal(map[string]any{"@type": "Event", "name": "city event", "startDate": "2026-09-27", "endDate": "2026-09-27", "location": map[string]any{"name": "venue", "address": map[string]any{"addressRegion": "東京都", "addressLocality": tc.detailCity}}})
			if err != nil {
				t.Fatal(err)
			}
			detail := "<h1>city event</h1><script type=\"application/ld+json\">" + string(schema) + "</script><table><tr class=\"m-infotable__row\"><th>開催日</th><td>2026年9月27日</td></tr></table>"
			if tc.detailCategory != "" {
				detail += "<li class=\"m-detailtag__tag\"><a href=\"/event_list/" + tc.detailCategory + "/\">category</a></li>"
			}
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/event_list/") {
					w.Write([]byte(list))
					return
				}
				if tc.details == 0 {
					t.Errorf("known conflict consumed detail budget: %s", r.URL.Path)
				}
				w.Write([]byte(detail))
			}, Options{})
			q := Query{Prefecture: "tokyo", City: "shibuya", Category: "museums", From: "2026-09-27", To: "2026-09-27", MaxPages: 1, MaxDetails: 1}
			result, err := c.Shortlist(context.Background(), q)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Events) != tc.returned || result.Coverage.DetailCount != tc.details || result.Coverage.RequestCount != 1+tc.details {
				t.Fatalf("wrong bounded evidence filtering: %+v", result)
			}
			for _, event := range result.Events {
				if value(event.Location.CityCode) != "ar0313113" || !categoryMatches(event.Categories, "eg0107") {
					t.Fatalf("unverified final filters: %+v", event)
				}
			}
			search, err := c.Search(context.Background(), q)
			if err != nil {
				t.Fatal(err)
			}
			if len(search.Events) != 0 || search.Coverage.DetailCount != 0 || search.Coverage.RequestCount != 1 {
				t.Fatalf("Search stopped being strict/listing-only: %+v", search)
			}
		})
	}
}

func TestKnownListingContradictionsCannotBeNormalizedIntoMatches(t *testing.T) {
	for _, tc := range []struct{ name, prefectureName, cityCode string }{
		{"contradictory_prefecture", "京都府", "ar0313113/shibuya"},
		{"foreign_city_code", "東京都", "ar0101100/sapporo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card := "<li class=\"m-mainlist__item\"><a class=\"m-mainlist-item__ttl\" href=\"/event/ar0313e12345/\">event</a><p class=\"m-mainlist-item-event__period\">2026年9月27日</p><a href=\"/event_list/ar0313/\">" + tc.prefectureName + "</a><a href=\"/event_list/" + tc.cityCode + "/\">渋谷区</a><a href=\"/event_list/eg0107/\">museum</a></li>"
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, "/event_list/") {
					t.Errorf("known contradictory listing consumed detail request: %s", r.URL.Path)
					w.WriteHeader(404)
					return
				}
				w.Write([]byte(card))
			}, Options{})
			got, err := c.Shortlist(context.Background(), Query{Prefecture: "tokyo", City: "shibuya", Category: "museums", From: "2026-09-27", To: "2026-09-27", MaxPages: 1, MaxDetails: 1})
			if err != nil || len(got.Events) != 0 || got.Coverage.DetailCount != 0 || got.Coverage.RequestCount != 1 {
				t.Fatalf("known contradiction bypassed compatibility: %+v err=%v", got, err)
			}
		})
	}
}
