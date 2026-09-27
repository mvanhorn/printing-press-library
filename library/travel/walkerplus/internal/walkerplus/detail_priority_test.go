package walkerplus

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
)

type priorityFixture struct{ id, start, end, city, category string }

func prioritySchema(t *testing.T, p priorityFixture) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"@type": "Event", "name": p.id, "startDate": p.start, "endDate": p.end, "location": map[string]any{"name": "venue", "address": map[string]any{"addressRegion": "東京都", "addressLocality": p.city}}})
	if err != nil {
		t.Fatal(err)
	}
	return "<script type=\"application/ld+json\">" + string(data) + "</script>"
}

func TestDetailBudgetPrioritizesFullySupportedListingFilters(t *testing.T) {
	rows := []priorityFixture{
		{"ar0313e21001", "2026-09-27", "2026-09-27", "", ""},
		{"ar0313e21002", "2026-09-28", "2026-09-28", "", "eg0107"},
		{"ar0313e21003", "2026-09-29", "2026-09-29", "渋谷区", ""},
		{"ar0313e21004", "2026-09-28", "2026-09-30", "渋谷区", "eg0107"},
		{"ar0313e21005", "2026-09-29", "2026-09-29", "渋谷区", "eg0107"},
	}
	for _, tc := range []struct {
		name, sort string
		budget     int
		want       []string
	}{
		{"start_tier_order", "start", 1, []string{"ar0313e21004"}},
		{"end_tier_order", "end", 1, []string{"ar0313e21005"}},
		{"source_tier_order", "source", 1, []string{"ar0313e21004"}},
		{"relevance_tier_order", "relevance", 1, []string{"ar0313e21004"}},
		{"partial_evidence_stays_unknown", "start", 2, []string{"ar0313e21004", "ar0313e21005"}},
		{"remaining_unknown_budget_start", "start", 3, []string{"ar0313e21001", "ar0313e21004", "ar0313e21005"}},
		{"remaining_unknown_budget_end", "end", 3, []string{"ar0313e21001", "ar0313e21005", "ar0313e21004"}},
		{"final_source_order", "source", 3, []string{"ar0313e21001", "ar0313e21004", "ar0313e21005"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var listing strings.Builder
			details := map[string]string{}
			for _, p := range rows {
				card := "<li class=\"m-mainlist__item\"><a class=\"m-mainlist-item__ttl\" href=\"/event/" + p.id + "/\">" + p.id + "</a><p class=\"m-mainlist-item-event__place\">venue</p><a href=\"/event_list/ar0313/\">東京都</a>"
				if p.category != "" {
					card += "<a href=\"/event_list/" + p.category + "/\">museum</a>"
				}
				card += "</li>"
				listing.WriteString(card + prioritySchema(t, p))
				resolved := p
				resolved.city = "渋谷区"
				resolved.category = "eg0107"
				raw := "2026年9月" + strings.TrimPrefix(p.start, "2026-09-") + "日"
				if p.end != p.start {
					raw += "～9月" + strings.TrimPrefix(p.end, "2026-09-") + "日"
				}
				details["/event/"+p.id+"/"] = "<h1>" + p.id + "</h1>" + prioritySchema(t, resolved) + "<table><tr class=\"m-infotable__row\"><th>開催日</th><td>" + raw + "</td></tr></table><li class=\"m-detailtag__tag\"><a href=\"/event_list/eg0107/\">museum</a></li>"
			}
			var mu sync.Mutex
			requested := map[string]bool{}
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/event_list/") {
					w.Write([]byte(listing.String()))
					return
				}
				mu.Lock()
				requested[eventIDFromPath(r.URL.Path)] = true
				mu.Unlock()
				body, ok := details[r.URL.Path]
				if !ok {
					t.Errorf("unexpected detail path %s", r.URL.Path)
					w.WriteHeader(404)
					return
				}
				w.Write([]byte(body))
			}, Options{})
			q := Query{Prefecture: "tokyo", City: "shibuya", Category: "museums", From: "2026-09-27", To: "2026-09-30", Sort: tc.sort, MaxPages: 1, MaxDetails: tc.budget}
			got, err := c.Shortlist(context.Background(), q)
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for _, e := range got.Events {
				ids = append(ids, e.ID)
			}
			if strings.Join(ids, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("selection/final %s ordering: got %v want %v", tc.sort, ids, tc.want)
			}
			if got.Coverage.DetailCount != tc.budget || got.Coverage.RequestCount != 1+tc.budget || len(requested) != tc.budget {
				t.Fatalf("request budget changed: %+v requested=%v", got.Coverage, requested)
			}
			for _, id := range tc.want {
				if !requested[id] {
					t.Fatalf("selected event %s was not enriched", id)
				}
			}
			if requested["ar0313e21002"] || requested["ar0313e21003"] {
				t.Fatalf("partial evidence preempted fully supported filters: %v", requested)
			}
			if !strings.Contains(strings.Join(got.Coverage.Reasons, " "), "listing-verified location/category") {
				t.Fatalf("selection policy not explained: %v", got.Coverage.Reasons)
			}
		})
	}
}
