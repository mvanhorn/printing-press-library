package driveplaza

import (
	"fmt"
	"os"
	"testing"
)

func TestRoutePanelsIgnoreNestedTollTabs(t *testing.T) {
	row := `<tr><td>Route</td><td><span class="cell"><em>10</em></span><span class="cell"><em>20</em></span><span class="cell"><em>30</em></span></td><td><span class="cell">1h</span><span class="cell">2h</span><span class="cell">100km</span></td></tr>`
	panel := func(id int, nested string) string {
		return fmt.Sprintf(`<div class="ui-tabbox">%s<div class="c-blockSkin03">Restriction %d</div><div class="js_sapa_icon_area"><span class="txt-sa">Stop %d</span><a href="javascript:goSapaBlog('/sapa/1040/1040021/%d/')">details</a></div><a href="https://www.drivetraffic.jp/map.html?fctime=202610100800&amp;route=%d">forecast</a></div>`, nested, id, id, id, id)
	}
	body := `<table class="table-route">` + row + row + `</table><div class="js-doubleTabContent">` + panel(1, `<div class="ui-tabbox"></div>`) + panel(2, "") + `</div>`
	routes, err := parseRoutes([]byte(body), true, English)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected two alternatives, got %d", len(routes))
	}
	for i, route := range routes {
		id := i + 1
		wantStop := fmt.Sprintf("1040/1040021/%d", id)
		wantForecast := fmt.Sprintf("https://www.drivetraffic.jp/map.html?fctime=202610100800&route=%d", id)
		if len(route.Stops) != 1 || route.Stops[0].ID != wantStop || len(route.ForecastURLs) != 1 || route.ForecastURLs[0] != wantForecast || len(route.Warnings) != 1 || route.Warnings[0] != fmt.Sprintf("Restriction %d", id) {
			t.Fatalf("%s received another panel's details: %+v", route.ID, route)
		}
	}
}

func TestRouteDetailsStayWithEverySourceAlternative(t *testing.T) {
	body, err := os.ReadFile("testdata/route.html")
	if err != nil {
		t.Fatal(err)
	}
	routes, err := parseRoutes(body, true, English)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 3 {
		t.Fatalf("expected three source alternatives, got %d", len(routes))
	}
	for _, route := range routes {
		t.Run(route.ID, func(t *testing.T) {
			if len(route.Stops) < 5 || len(route.ForecastURLs) == 0 || len(route.Warnings) == 0 {
				t.Fatalf("route details were lost or paired with a nested toll tab: stops=%d forecasts=%d warnings=%d", len(route.Stops), len(route.ForecastURLs), len(route.Warnings))
			}
		})
	}
}
