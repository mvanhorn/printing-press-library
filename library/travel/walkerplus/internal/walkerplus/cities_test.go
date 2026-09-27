package walkerplus

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func cityFixture(t *testing.T, locality string) []byte {
	t.Helper()
	data, err := json.Marshal([]map[string]any{{"@type": "Event", "name": "city event", "startDate": "2026-09-27", "endDate": "2026-09-27", "location": map[string]any{"name": "venue", "address": map[string]any{"addressRegion": "東京都", "addressLocality": locality}}}})
	if err != nil {
		t.Fatal(err)
	}
	card := "<li class=\"m-mainlist__item\"><a class=\"m-mainlist-item__ttl\" href=\"/event/ar0313e12345/\">city event</a><p class=\"m-mainlist-item-event__period\">2026年9月27日</p><p class=\"m-mainlist-item-event__place\">venue</p><p class=\"m-mainlist-item__map\"><a href=\"/event_list/ar0313/\">東京都</a></p></li>"
	return []byte("<html><ul>" + card + "</ul><script type=\"application/ld+json\">" + string(data) + "</script></html>")
}

func TestDynamicCityCodeAliasAndJapaneseName(t *testing.T) {
	for _, input := range []string{"ar0313104", "shinjuku", "新宿区"} {
		t.Run(input, func(t *testing.T) {
			requested := []string{}
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				requested = append(requested, r.URL.Path)
				if r.URL.Path == "/event_list/ar0313/" {
					w.Write([]byte("<nav><a href=\"/event_list/ar0313104/\">新宿区</a><a href=\"/event_list/ar0313104/shinjuku/\">新宿区</a></nav>"))
					return
				}
				if r.URL.Path != "/event_list/09/ar0313104/shinjuku/" {
					t.Errorf("city slug guessed or omitted: %s", r.URL.Path)
					w.WriteHeader(404)
					return
				}
				w.Write(cityFixture(t, "新宿区"))
			}, Options{})
			result, err := c.Search(context.Background(), Query{Prefecture: "tokyo", City: input, From: "2026-09-27", To: "2026-09-27", MaxPages: 1})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Events) != 1 || result.Query.City != "ar0313104" || value(result.Events[0].Location.CityCode) != "ar0313104" {
				t.Fatalf("dynamic city lost: %+v", result)
			}
			if result.Coverage.CatalogRequests != 1 || len(result.Coverage.CatalogRoutes) != 1 || result.Coverage.RequestCount != 2 || result.Coverage.ScannedPages != 1 || len(requested) != 2 {
				t.Fatalf("catalog/event budgets conflated: %+v", result.Coverage)
			}
		})
	}
}

func TestCityLocalityAndValidationStayTruthful(t *testing.T) {
	if _, err := NormalizeQuery(Query{City: "shinjuku"}); err == nil {
		t.Fatal("unknown global alias needs prefecture")
	}
	if _, err := NormalizeQuery(Query{Prefecture: "kyoto", City: "ar0313104"}); err == nil {
		t.Fatal("wrong-prefix city accepted")
	}
	if q, err := NormalizeQuery(Query{Prefecture: "osaka", City: "港区"}); err != nil || q.City != "港区" || q.cityPath != "" {
		t.Fatalf("foreign known ward prevented scoped live resolution: %+v %v", q, err)
	}
	if _, err := NormalizeQuery(Query{Prefecture: "tokyo", City: "ar03131"}); err == nil {
		t.Fatal("malformed code entered live alias resolution")
	}
	for _, bad := range []string{"../shinjuku", "x?y", "foo/bar", "foo bar"} {
		if _, err := NormalizeQuery(Query{Prefecture: "tokyo", City: bad}); err == nil {
			t.Fatalf("unsafe city accepted %s", bad)
		}
	}
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/event_list/ar0313/" {
			w.Write([]byte("<nav><a href=\"/event_list/ar0313104/shinjuku/\">新宿区</a></nav>"))
			return
		}
		w.Write(cityFixture(t, "渋谷区"))
	}, Options{})
	result, err := c.Search(context.Background(), Query{Prefecture: "tokyo", City: "shinjuku", MaxPages: 1})
	if err != nil || len(result.Events) != 0 {
		t.Fatalf("route provenance manufactured city membership: err=%v result=%+v", err, result)
	}
	_, err = c.Search(context.Background(), Query{Prefecture: "tokyo", City: "unknown-city", MaxPages: 1})
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("missing catalog city not an input error: %v", err)
	}
	q := Query{City: "ar0101100", cityName: "札幌市"}
	for _, tc := range []struct {
		actual  string
		matches bool
	}{{"札幌市中央区", true}, {"札幌市", true}, {"札幌市外", false}, {"小樽市", false}} {
		e := emptyEvent("ar0101e1", "event", "source")
		e.Location.CityJA = strptr(tc.actual)
		applyQueryCity(&e, q)
		if (e.Location.CityCode != nil) != tc.matches {
			t.Fatalf("parent city/ward mismatch: %+v", tc)
		}
	}
}

func TestCatalogCacheAccountingAndCanonicalDetailLinks(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/event_list/ar0313/" {
			w.Write([]byte("<a href=\"/event_list/ar0313104/shinjuku/\">新宿区</a>"))
			return
		}
		w.Write(cityFixture(t, "新宿区"))
	}, Options{CacheDir: t.TempDir()})
	q := Query{Prefecture: "tokyo", City: "shinjuku", MaxPages: 1}
	if _, err := c.Search(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	second, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if second.Coverage.CatalogRequests != 0 || second.Coverage.RequestCount != 0 || second.Coverage.CacheHits != 2 || len(second.Coverage.CatalogRoutes) != 1 {
		t.Fatalf("warm catalog accounting incorrect: %+v", second.Coverage)
	}
	e := emptyEvent("ar0313e1", "event", "source")
	body := "<h1>event</h1><table><tr class=\"m-infotable__row\"><th>開催日</th><td>2026年9月27日</td></tr></table><a href=\"/event/ar0313e1/data.html\">data</a><a href=\"/event/ar0313e1/price.html\">price</a><a href=\"/event/ar0313e1/nested/data.html\">unexpected</a>"
	links, err := parseDetail(page{body: []byte(body), source: Source{URL: "source"}}, &e)
	if err != nil || len(links) != 2 || strings.Contains(strings.Join(links, ","), "nested") {
		t.Fatalf("detail fanout unbounded: links=%v err=%v", links, err)
	}
}
