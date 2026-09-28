package tenki

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestSeasonalRequestedYearBoundsUseSourceFixture(t *testing.T) {
	client := fixtureClient(t, map[string]string{
		murodo:                     "foliage-murodo-20260927.html",
		ueno:                       "sakura-ueno-ended-20260927.html",
		"https://tenki.jp/kouyou/": "foliage-index-20260927.html",
		"https://tenki.jp/sakura/": "sakura-index-ended-20260927.html",
	})
	for _, year := range []int{1900, 1999, 2000, 2100, 2101, 2200} {
		for _, season := range []struct{ kind, target string }{{"kouyou", murodo}, {"sakura", ueno}} {
			t.Run(season.kind+"/"+strconv.Itoa(year), func(t *testing.T) {
				show, err := client.Seasonal(context.Background(), season.kind, season.target, year)
				if err != nil {
					t.Fatalf("accepted year became acquisition error: %v", err)
				}
				if show.Status != "year_unavailable" || show.RequestedYear != year || show.Year != 2026 || show.Spot.Year != 2026 || show.Source.URL != season.target {
					t.Fatalf("requested/source year contract lost: %+v", show)
				}
				list, err := client.SeasonalList(context.Background(), season.kind, "", year, 10, 1)
				if err != nil {
					t.Fatalf("accepted list year became acquisition error: %v", err)
				}
				if list.Status != "year_unavailable" || list.RequestedYear != year || list.Year != 2026 || list.Pages != 1 || list.Spots == nil || len(list.Spots) != 0 {
					t.Fatalf("list invented season or lost source year: %+v", list)
				}
			})
		}
	}
	for _, year := range []int{1899, 2201} {
		t.Run("invalid/"+strconv.Itoa(year), func(t *testing.T) {
			before := client.Metrics().HTTPRequests
			if _, err := client.Seasonal(context.Background(), "kouyou", murodo, year); err == nil || !strings.Contains(err.Error(), "1900") || !strings.Contains(err.Error(), "2200") {
				t.Fatalf("invalid show year error: %v", err)
			}
			if _, err := client.SeasonalList(context.Background(), "kouyou", "", year, 10, 1); err == nil || !strings.Contains(err.Error(), "1900") || !strings.Contains(err.Error(), "2200") {
				t.Fatalf("invalid list year error: %v", err)
			}
			if client.Metrics().HTTPRequests != before {
				t.Fatal("out-of-range year performed acquisition")
			}
		})
	}
}
