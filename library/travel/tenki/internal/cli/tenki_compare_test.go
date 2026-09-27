package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"
)

func compareSource() *tenkiFake {
	pop := 20.0
	place := tenki.Place{Name: "千代田区", URL: tenkiTokyo, Kind: "municipality", Scope: "municipal", ForecastReferenceURL: tenkiTokyo}
	source := tenki.Source{URL: tenkiTokyo, Freshness: "fresh", IssueAt: "2026-09-27T22:00:00+09:00"}
	return &tenkiFake{place: tenki.PlaceResult{Place: place, Source: source, Status: "ok"}, daily: tenki.ForecastResult{Place: place, Source: source, Status: "ok", Periods: []tenki.Period{{Date: "2026-09-28", Kind: "forecast", PrecipProbabilityPct: &pop}}, CoverageStart: "2026-09-27", CoverageEnd: "2026-10-10"}}
}

func comparison(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	result := tenkiResult(t, value)
	matrix, ok := result["comparison"].(map[string]any)
	if !ok {
		t.Fatalf("missing comparison: %#v", result)
	}
	return matrix
}

func TestTenkiCompareCriterionRequiredAndNumericBounds(t *testing.T) {
	cases := [][]string{
		{"--place", tenkiTokyo},
		{"--max-pop", "30"},
		{"--place", tenkiTokyo, "--max-pop", "101"},
		{"--place", tenkiTokyo, "--max-pop", "NaN"},
		{"--place", tenkiTokyo, "--max-rain", "-1"},
		{"--place", tenkiTokyo, "--max-wind", "Inf"},
		{"--place", tenkiTokyo, "--min-temp", "30", "--max-temp", "20"},
		{"--place", tenkiTokyo, "--max-pop", "30", "--sort", "max-wind"},
		{"--place", tenkiTokyo, "--max-pop", "30", "--limit", "71"},
		{"--place", tenkiTokyo, "--max-pop", "30", "--days", "15"},
		{"--place", tenkiTokyo, "--max-pop", "30", "--hours", "9:00-17:00"},
		{"--place", tenkiTokyo, "--max-pop", "30", "--season-condition", "見頃"},
		{"--place", tenkiTokyo, "--max-pop", "30", "--year", "2026"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			f := compareSource()
			args = append([]string{"compare", "--json"}, args...)
			_, stdout, _, err, _ := tenkiRun(t, f, args...)
			if err == nil || ExitCode(err) != 2 || len(f.calls) > 0 || stdout != "" {
				t.Fatalf("bad comparison consumed source or exited incorrectly: %v %q %v", err, stdout, f.calls)
			}
		})
	}
}

func TestTenkiCompareMatrixMetadataProjectionAndAbsentCriterion(t *testing.T) {
	f := compareSource()
	value, stdout, stderr, err, _ := tenkiRun(t, f, "compare", "--place", tenkiTokyo, "--from", "2026-09-28", "--days", "2", "--max-pop", "30", "--sort", "max-pop", "--agent")
	if err != nil {
		t.Fatal(err)
	}
	matrix := comparison(t, value)
	cells := matrix["cells"].([]any)
	if len(cells) != 2 || cells[0].(map[string]any)["status"] != "meets_criteria" || cells[1].(map[string]any)["status"] != "insufficient_data" {
		t.Fatalf("matrix/horizon: %s", stdout)
	}
	meta := value["meta"].(map[string]any)
	if meta["source"] != "live" || meta["data_origin"] != "computed" || meta["metrics"] == nil {
		t.Fatalf("computed versus source transport: %#v", meta)
	}
	if strings.Count(stdout, "\n") != 1 || stderr != "" || !f.deadline {
		t.Fatalf("stdout/stderr/timeout contract: %q %q", stdout, stderr)
	}
	value, stdout, _, err, _ = tenkiRun(t, compareSource(), "compare", "--place", tenkiTokyo, "--from", "2026-09-28", "--days", "1", "--max-pop", "30", "--agent", "--select", "results.comparison.cells.date,results.comparison.cells.criteria.max-pop.value")
	if err != nil {
		t.Fatal(err)
	}
	if len(value) != 1 || strings.Contains(stdout, "weather_source") {
		t.Fatalf("projection leaked fields: %s", stdout)
	}
	_, _, _, err, _ = tenkiRun(t, compareSource(), "compare", "--place", tenkiTokyo, "--from", "2026-09-28", "--days", "1", "--max-pop", "30", "--json", "--select", "results.comparison.cells.criteria.max-wind.value")
	if err == nil {
		t.Fatal("unsupplied criterion selector must fail")
	}
}

func TestTenkiCompareDeduplicatesMunicipalityFetchAndPreservesPlaces(t *testing.T) {
	f := compareSource()
	a := "https://tenki.jp/leisure/3/16/90/100001/"
	b := "https://tenki.jp/leisure/3/16/90/100002/"
	f.placesByURL = map[string]tenki.PlaceResult{}
	for _, target := range []string{a, b} {
		place := f.place
		place.Place.URL = target
		place.Place.Kind = "leisure"
		f.placesByURL[target] = place
	}
	value, _, _, err, _ := tenkiRun(t, f, "compare", "--place", a, "--place", b, "--from", "2026-09-28", "--days", "1", "--max-pop", "30", "--refresh")
	if err != nil {
		t.Fatal(err)
	}
	dailyCalls := 0
	for _, call := range f.calls {
		if strings.HasPrefix(call, "daily:") {
			dailyCalls++
		}
	}
	if dailyCalls != 1 {
		t.Fatalf("same source ref fetched %d times: %v", dailyCalls, f.calls)
	}
	cells := comparison(t, value)["cells"].([]any)
	if len(cells) != 2 || cells[0].(map[string]any)["place"].(map[string]any)["url"] == cells[1].(map[string]any)["place"].(map[string]any)["url"] {
		t.Fatalf("destination identity collapsed: %#v", cells)
	}
}

func TestTenkiCompareFetchFailurePreservesUnknownCellsAndStderr(t *testing.T) {
	f := compareSource()
	bad := "https://tenki.jp/forecast/6/29/6110/26103/"
	f.errorsByCall = map[string]error{"daily:" + bad: errors.New("source denied403")}
	value, stdout, stderr, err, _ := tenkiRun(t, f, "compare", "--place", tenkiTokyo, "--place", bad, "--from", "2026-09-28", "--days", "1", "--max-pop", "30", "--json")
	if err != nil {
		t.Fatal(err)
	}
	result := tenkiResult(t, value)
	failures := result["fetch_failures"].([]any)
	if result["partial"] != true || len(failures) != 1 || failures[0].(map[string]any)["place"] != bad || !strings.Contains(stderr, "1 of 2 places had fetch failures") || !strings.Contains(stderr, "1 of 2 places") {
		t.Fatalf("failure accounting: %s stderr%q", stdout, stderr)
	}
	cells := comparison(t, value)["cells"].([]any)
	unknown := cells[1].(map[string]any)
	if unknown["status"] != "insufficient_data" || unknown["criteria"].(map[string]any)["max-pop"].(map[string]any)["value"] != nil {
		t.Fatalf("phantom zero from failed fetch: %#v", unknown)
	}
}

func TestTenkiCompareThrottlePropagatesInsteadOfPartialSuccess(t *testing.T) {
	f := compareSource()
	f.errorsByCall = map[string]error{"daily:" + tenkiTokyo: &cliutil.RateLimitError{URL: tenkiTokyo}}
	_, stdout, _, err, _ := tenkiRun(t, f, "compare", "--place", tenkiTokyo, "--from", "2026-09-28", "--days", "1", "--max-pop", "30", "--json")
	if ExitCode(err) != 7 || stdout != "" {
		t.Fatalf("throttle converted to partial: %v %q", err, stdout)
	}
}

func TestTenkiCompareActivityWindowUsesHourlyOnly(t *testing.T) {
	f := compareSource()
	f.hourly = f.daily
	value, _, _, err, _ := tenkiRun(t, f, "compare", "--place", tenkiTokyo, "--from", "2026-09-28", "--days", "1", "--hours", "09:00-17:00", "--max-rain", "0", "--sort", "max-rain")
	if err != nil {
		t.Fatal(err)
	}
	matrix := comparison(t, value)
	current := matrix["cells"].([]any)[0].(map[string]any)
	if current["status"] != "insufficient_data" || matrix["rain_unit"] != "mm/h per preceding source hour" {
		t.Fatalf("coarser weather accepted or units wrong: %#v", matrix)
	}
	if strings.Contains(strings.Join(f.calls, " "), "daily:") || !strings.Contains(strings.Join(f.calls, " "), "hourly:") {
		t.Fatalf("wrong source product: %v", f.calls)
	}
}

func TestTenkiCompareMunicipalityUsesOneProductCallAndVerifiedIdentity(t *testing.T) {
	f := compareSource()
	value, _, _, err, _ := tenkiRun(t, f, "compare", "--place", tenkiTokyo, "--place", tenkiTokyo+"10days.html", "--from", "2026-09-28", "--days", "1", "--max-pop", "30", "--refresh")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || f.calls[0] != "daily:"+tenkiTokyo {
		t.Fatalf("extra base-page resolution/fetch: %v", f.calls)
	}
	cells := comparison(t, value)["cells"].([]any)
	if len(cells) != 1 || cells[0].(map[string]any)["place"].(map[string]any)["name"] != "千代田区" {
		t.Fatalf("verified product identity or canonical dedup lost: %#v", cells)
	}
	if value["meta"].(map[string]any)["metrics"].(map[string]any)["http_requests"] != float64(1) {
		t.Fatal("product request metrics incorrect")
	}
}

func TestTenkiCompareMountainAndFutureSeasonRemainInsufficient(t *testing.T) {
	f := compareSource()
	f.place.Place.URL = tenkiFuji
	f.place.Place.Kind = "mountain"
	f.place.Place.Scope = "foothill"
	value, _, _, err, _ := tenkiRun(t, f, "compare", "--place", tenkiFuji, "--from", "2026-09-28", "--days", "1", "--max-pop", "30")
	if err != nil {
		t.Fatal(err)
	}
	if comparison(t, value)["cells"].([]any)[0].(map[string]any)["status"] != "insufficient_data" {
		t.Fatal("mountain became summit recommendation")
	}
	f = compareSource()
	f.place.Place.URL = tenkiOze
	f.place.Place.Kind = "kouyou"
	f.season = tenki.SeasonalResult{Kind: "kouyou", Year: 2026, RequestedYear: 2026, Status: "ok", UpdateState: "active", Place: f.place.Place, Source: tenki.Source{URL: tenkiOze, Freshness: "fresh", IssueAt: "2026-09-27T15:00:00+09:00"}, Spot: tenki.SeasonalSpot{Place: f.place.Place, Year: 2026, Condition: "見頃", ReportDate: "2026-09-27"}}
	value, _, _, err, _ = tenkiRun(t, f, "compare", "--place", tenkiOze, "--from", "2026-09-28", "--days", "1", "--max-pop", "30", "--season", "kouyou", "--year", "2026")
	if err != nil {
		t.Fatal(err)
	}
	current := comparison(t, value)["cells"].([]any)[0].(map[string]any)
	if current["status"] != "insufficient_data" || current["criteria"].(map[string]any)["max-pop"].(map[string]any)["verdict"] != "pass" || current["seasonal"].(map[string]any)["verdict"] != "unknown" {
		t.Fatalf("weather/season times mixed: %#v", current)
	}
}
