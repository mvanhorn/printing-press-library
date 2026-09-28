package planning

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"
)

const testMunicipality = "https://tenki.jp/forecast/3/16/4410/13101/"
const testSpot = "https://tenki.jp/kouyou/3/13/30139.html"

func number(value float64) *float64 { return &value }
func options() Options {
	return Options{From: "2026-09-28", Days: 1, Limit: 70, Criteria: Criteria{MaxPOP: number(30)}}
}
func dailyInput() Input {
	place := tenki.Place{URL: testMunicipality, Kind: "municipality", Name: "千代田区", Scope: "municipal", ForecastReferenceURL: testMunicipality}
	return Input{Place: place, Weather: tenki.ForecastResult{Place: place, Status: "ok", Source: tenki.Source{URL: testMunicipality + "10days.html", IssueAt: "2026-09-27T22:00:00+09:00", Freshness: "fresh"}, CoverageStart: "2026-09-27", CoverageEnd: "2026-10-10", Periods: []tenki.Period{{Date: "2026-09-28", Kind: "forecast", PrecipProbabilityPct: number(20), PrecipAmountMM: number(0), MinTemperatureC: number(15), MaxTemperatureC: number(25), Confidence: "B"}}}}
}
func cell(t *testing.T, input Input, opts Options) Cell {
	t.Helper()
	result, err := Evaluate([]Input{input}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cells) != opts.Days {
		t.Fatalf("missing cells: %#v", result)
	}
	return result.Cells[0]
}

func TestDailyCoverageAndOutcomes(t *testing.T) {
	cases := []struct {
		name            string
		change          func(*Input)
		criteria        Criteria
		status, verdict string
		available       int
	}{
		{"supplied forecast passes", func(*Input) {}, Criteria{MaxPOP: number(30)}, "meets_criteria", "pass", 1},
		{"missing POP remains unknown", func(i *Input) { i.Weather.Periods[0].PrecipProbabilityPct = nil }, Criteria{MaxPOP: number(30)}, "insufficient_data", "unknown", 0},
		{"explicit zero is real", func(i *Input) { i.Weather.Periods[0].PrecipProbabilityPct = number(0) }, Criteria{MaxPOP: number(0)}, "meets_criteria", "pass", 1},
		{"failing POP with unknown wind", func(i *Input) { i.Weather.Periods[0].PrecipProbabilityPct = number(60) }, Criteria{MaxPOP: number(30), MaxWind: number(5)}, "fails_criteria", "fail", 1},
		{"partial source cannot pass", func(i *Input) { i.Weather.Periods[0].Partial = true }, Criteria{MaxPOP: number(30)}, "insufficient_data", "unknown", 0},
		{"explicit partial failure remains failure", func(i *Input) {
			i.Weather.Periods[0].Partial = true
			i.Weather.Periods[0].PrecipProbabilityPct = number(60)
		}, Criteria{MaxPOP: number(30)}, "fails_criteria", "fail", 0},
		{"estimated data cannot pass", func(i *Input) { i.Weather.Periods[0].Kind = "estimated_actual" }, Criteria{MaxPOP: number(30)}, "insufficient_data", "unknown", 0},
		{"mixed currentday kind cannot pass", func(i *Input) { i.Weather.Periods[0].Kind = "mixed" }, Criteria{MaxPOP: number(30)}, "insufficient_data", "unknown", 0},
		{"source stale blocks positive", func(i *Input) { i.Weather.Source.SourceStale = true; i.Weather.Source.Freshness = "stale" }, Criteria{MaxPOP: number(30)}, "insufficient_data", "unknown", 1},
		{"cache stale blocks positive", func(i *Input) { i.Weather.Source.CacheStale = true }, Criteria{MaxPOP: number(30)}, "insufficient_data", "unknown", 1},
		{"unknown issuance blocks positive", func(i *Input) { i.Weather.Source.Freshness = "unknown"; i.Weather.Source.IssueAt = "" }, Criteria{MaxPOP: number(30)}, "insufficient_data", "unknown", 1},
		{"outofhorizon no phantom row", func(i *Input) { i.Weather.Periods = []tenki.Period{} }, Criteria{MaxPOP: number(30)}, "insufficient_data", "unknown", 0},
		{"acquisition failure no phantom zero", func(i *Input) { i.Weather = tenki.ForecastResult{}; i.WeatherError = "source denied" }, Criteria{MaxPOP: number(30)}, "insufficient_data", "unknown", 0},
		{"wrong forecast municipality", func(i *Input) { i.Weather.Place.ForecastReferenceURL = "https://tenki.jp/forecast/6/29/6110/26103/" }, Criteria{MaxPOP: number(30)}, "insufficient_data", "unknown", 1},
		{"duplicate daily evidence cannot pass", func(i *Input) { i.Weather.Periods = append(i.Weather.Periods, i.Weather.Periods[0]) }, Criteria{MaxPOP: number(30)}, "insufficient_data", "unknown", 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := dailyInput()
			test.change(&input)
			opts := options()
			opts.Criteria = test.criteria
			result := cell(t, input, opts)
			criterion := result.Criteria["max-pop"]
			if result.Status != test.status || criterion.Verdict != test.verdict || criterion.Coverage.Available != test.available {
				t.Fatalf("outcome %s/%s coverage %#v, want %s/%s available%d", result.Status, criterion.Verdict, criterion.Coverage, test.status, test.verdict, test.available)
			}
			if test.criteria.MaxWind != nil && result.Criteria["max-wind"].Verdict != "unknown" {
				t.Fatal("missing wind should remain unknown beside failure")
			}
			if criterion.Coverage.Ratio > 1 {
				t.Fatal("coverage above1")
			}
		})
	}
}

func hourlyInput(date string, start, end int) Input {
	input := dailyInput()
	input.Weather.Source.URL = testMunicipality + "1hour.html"
	input.Weather.Periods = []tenki.Period{}
	input.Weather.Intervals = nil
	input.Weather.Instants = nil
	day, _ := time.ParseInLocation("2006-01-02", date, jst)
	for hour := start; hour <= end; hour++ {
		instant := day.Add(time.Duration(hour) * time.Hour)
		input.Weather.Periods = append(input.Weather.Periods, tenki.Period{Date: date, Start: instant.Add(-time.Hour).Format(time.RFC3339), End: instant.Format(time.RFC3339), ValidAt: instant.Format(time.RFC3339), Kind: "forecast", PrecipProbabilityPct: number(20), PrecipRateMMH: number(0), TemperatureC: number(20), WindSpeedMS: number(2)})
	}
	return input
}

func TestHourlyIntervalAndInstantEndpoints(t *testing.T) {
	opts := options()
	opts.Hours = &Window{Start: 9, End: 17}
	opts.Criteria = Criteria{MaxPOP: number(30), MaxRain: number(0), MinTemp: number(10), MaxTemp: number(30), MaxWind: number(5)}
	input := hourlyInput("2026-09-28", 8, 18)
	// The interval ending09 is outside09–17 even though its instant is used.
	input.Weather.Periods[1].PrecipRateMMH = number(99)
	input.Weather.Periods[1].PrecipProbabilityPct = number(100)
	result := cell(t, input, opts)
	if result.Status != "meets_criteria" {
		t.Fatalf("outside rain incorrectly used: %#v", result)
	}
	for _, key := range []string{"max-pop", "max-rain"} {
		c := result.Criteria[key]
		if c.Coverage.Expected != 8 || c.Coverage.Available != 8 || len(c.Evidence) != 8 || c.Evidence[0].End != "2026-09-28T10:00:00+09:00" || c.Evidence[7].End != "2026-09-28T17:00:00+09:00" {
			t.Fatalf("interval endpoints %s: %#v", key, c)
		}
	}
	for _, key := range []string{"min-temp", "max-temp", "max-wind"} {
		c := result.Criteria[key]
		if c.Coverage.Expected != 9 || len(c.Evidence) != 9 || c.Evidence[0].ValidAt != "2026-09-28T09:00:00+09:00" || c.Evidence[8].ValidAt != "2026-09-28T17:00:00+09:00" {
			t.Fatalf("instant endpoints %s: %#v", key, c)
		}
	}
	input.Weather.Periods[1].TemperatureC = number(5)
	result = cell(t, input, opts)
	if result.Criteria["min-temp"].Verdict != "fail" {
		t.Fatal("09 instant omitted")
	}
	input = hourlyInput("2026-09-28", 9, 17)
	input.Weather.Periods[8].WindSpeedMS = number(7)
	result = cell(t, input, opts)
	if result.Criteria["max-wind"].Verdict != "fail" {
		t.Fatal("17 instant omitted")
	}
}

func TestCurrentDayFieldKindsAreSeparate(t *testing.T) {
	input := dailyInput()
	period := &input.Weather.Periods[0]
	period.Kind = "mixed"
	period.Partial = true
	period.TemperatureKind = "forecast_or_estimated_actual"
	period.WeatherProbabilityKind = "forecast"
	period.WeatherProbabilityFrom = "2026-09-28T15:00:00+09:00"
	period.PrecipProbabilityPct = number(80)
	opts := options()
	opts.Criteria = Criteria{MaxPOP: number(30), MaxTemp: number(30)}
	result := cell(t, input, opts)
	if result.Status != "fails_criteria" || result.Criteria["max-pop"].Verdict != "fail" || result.Criteria["max-temp"].Verdict != "unknown" {
		t.Fatalf("mixed field types laundered: %#v", result.Criteria)
	}
	if result.Criteria["max-pop"].Evidence[0].Start != period.WeatherProbabilityFrom || result.Criteria["max-pop"].Evidence[0].Kind != "forecast" || result.Criteria["max-temp"].Evidence[0].Kind != "forecast_or_estimated_actual" {
		t.Fatalf("per-field evidence kind/period absent: %#v", result.Criteria)
	}
}

func TestHourlyMissingPartialDuplicateAndCoarserEvidence(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Input)
	}{
		{"missing interval", func(i *Input) { i.Weather.Periods = append(i.Weather.Periods[:3], i.Weather.Periods[4:]...) }},
		{"estimated interval", func(i *Input) { i.Weather.Periods[3].Kind = "estimated_actual" }},
		{"partial interval", func(i *Input) { i.Weather.Periods[3].Partial = true }},
		{"missing rain numeric", func(i *Input) { i.Weather.Periods[3].PrecipRateMMH = nil }},
		{"duplicate interval", func(i *Input) { i.Weather.Periods = append(i.Weather.Periods, i.Weather.Periods[3]) }},
		{"coarser substituted interval", func(i *Input) { i.Weather.Periods[3].Start = "2026-09-28T06:00:00+09:00" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := hourlyInput("2026-09-28", 9, 17)
			test.change(&input)
			opts := options()
			opts.Hours = &Window{9, 17}
			opts.Criteria = Criteria{MaxRain: number(0)}
			result := cell(t, input, opts)
			criterion := result.Criteria["max-rain"]
			if result.Status != "insufficient_data" || criterion.Verdict != "unknown" || criterion.Coverage.Available != 7 {
				t.Fatalf("gap accepted: %#v", criterion)
			}
		})
	}
	input := dailyInput()
	opts := options()
	opts.Hours = &Window{9, 17}
	result := cell(t, input, opts)
	if result.Status != "insufficient_data" || result.Criteria["max-pop"].Coverage.Available != 0 {
		t.Fatal("daily values substituted for hourly window")
	}
}

func TestHourlyMidnightAndYearRollover(t *testing.T) {
	input := hourlyInput("2026-12-31", 23, 24)
	opts := options()
	opts.From = "2026-12-31"
	opts.Hours = &Window{23, 24}
	opts.Criteria = Criteria{MaxRain: number(0), MaxWind: number(3)}
	result := cell(t, input, opts)
	if result.Status != "meets_criteria" || result.Criteria["max-rain"].Evidence[0].End != "2027-01-01T00:00:00+09:00" || result.Criteria["max-wind"].Coverage.Expected != 2 {
		t.Fatalf("rollover: %#v", result)
	}
}

func TestDailyWindUsesFiveSeparateInstants(t *testing.T) {
	input := dailyInput()
	day, _ := time.ParseInLocation("2006-01-02", "2026-09-28", jst)
	for _, hour := range []int{0, 6, 12, 18, 24} {
		input.Weather.Instants = append(input.Weather.Instants, tenki.Period{Date: "2026-09-28", ValidAt: day.Add(time.Duration(hour) * time.Hour).Format(time.RFC3339), Kind: "forecast", WindSpeedMS: number(3)})
	}
	input.Weather.Intervals = []tenki.Period{{Date: "2026-09-28", WindSpeedMS: number(99)}}
	opts := options()
	opts.Criteria = Criteria{MaxWind: number(5)}
	result := cell(t, input, opts)
	if result.Status != "meets_criteria" || result.Criteria["max-wind"].Coverage.Expected != 5 || *result.Criteria["max-wind"].Value != 3 {
		t.Fatalf("intervals zipped into instants: %#v", result)
	}
	input.Weather.Instants = input.Weather.Instants[:4]
	result = cell(t, input, opts)
	if result.Status != "insufficient_data" {
		t.Fatal("missing24 instant accepted")
	}
}

func seasonalInput() Input {
	input := dailyInput()
	input.Place = tenki.Place{URL: testSpot, Kind: "kouyou", Scope: "municipal", ForecastReferenceURL: testMunicipality, Name: "尾瀬"}
	input.Season = &tenki.SeasonalResult{Kind: "kouyou", Year: 2026, RequestedYear: 2026, Status: "ok", UpdateState: "active", Place: input.Place, Source: tenki.Source{URL: testSpot, Freshness: "fresh", IssueAt: "2026-09-28T15:00:00+09:00"}, Spot: tenki.SeasonalSpot{Place: input.Place, Year: 2026, Condition: "青葉", ReportDate: "2026-09-28", NormalPeriod: "9月下旬～10月上旬"}}
	return input
}

func TestSeasonalPairingYearFreshnessAndApplicability(t *testing.T) {
	cases := []struct {
		name                  string
		change                func(*Input, *Options)
		status, seasonVerdict string
	}{
		{"same day report no hidden peak criterion", func(*Input, *Options) {}, "meets_criteria", "pass"},
		{"explicit exact condition fails", func(_ *Input, o *Options) { o.SeasonCondition = "見頃" }, "fails_criteria", "fail"},
		{"future date current condition is not prediction", func(i *Input, o *Options) { o.From = "2026-09-29"; i.Weather.Periods[0].Date = "2026-09-29" }, "insufficient_data", "unknown"},
		{"wrong source year", func(i *Input, _ *Options) { i.Season.Year = 2025; i.Season.Spot.Year = 2025 }, "insufficient_data", "unknown"},
		{"requested future year", func(_ *Input, o *Options) { o.Year = 2027 }, "insufficient_data", "unknown"},
		{"ended season with fresh weather", func(i *Input, _ *Options) { i.Season.Status = "season_ended"; i.Season.UpdateState = "ended" }, "insufficient_data", "unknown"},
		{"weather issue does not refresh report", func(i *Input, _ *Options) { i.Season.Source.Freshness = "stale"; i.Season.Source.SourceStale = true }, "insufficient_data", "unknown"},
		{"wrong municipality join", func(i *Input, _ *Options) {
			i.Season.Place.ForecastReferenceURL = "https://tenki.jp/forecast/6/29/6110/26103/"
		}, "insufficient_data", "unknown"},
		{"wrong spot join", func(i *Input, _ *Options) {
			i.Season.Place.URL = "https://tenki.jp/kouyou/3/16/33001.html"
			i.Season.Spot.Place.URL = i.Season.Place.URL
		}, "insufficient_data", "unknown"},
		{"unknown season beside definite weather failure", func(i *Input, _ *Options) {
			i.Season.Status = "season_ended"
			i.Weather.Periods[0].PrecipProbabilityPct = number(80)
		}, "fails_criteria", "unknown"},
		{"date-only report not fabricated from issue", func(i *Input, _ *Options) {
			i.Season.Spot.ReportDate = ""
			i.Season.Spot.ReportAt = i.Season.Source.IssueAt
		}, "insufficient_data", "unknown"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := seasonalInput()
			opts := options()
			opts.Season = "kouyou"
			opts.Year = 2026
			test.change(&input, &opts)
			result := cell(t, input, opts)
			if result.Status != test.status || result.Seasonal.Verdict != test.seasonVerdict {
				t.Fatalf("season laundering: status%s seasonal%#v", result.Status, result.Seasonal)
			}
			if result.Seasonal.Evidence.Spot.Condition != "青葉" {
				t.Fatal("raw condition changed")
			}
			if result.WeatherScope != "municipal" || len(result.Assumptions) == 0 {
				t.Fatal("municipality-level weather scope lost")
			}
		})
	}
}

func TestEndedSakuraAndExactSourcePrediction(t *testing.T) {
	input := seasonalInput()
	input.Place.URL = "https://tenki.jp/sakura/3/16/54401.html"
	input.Place.Kind = "sakura"
	input.Season.Place = input.Place
	input.Season.Spot.Place = input.Place
	input.Season.Kind = "sakura"
	input.Season.Source.URL = input.Place.URL
	input.Season.Status = "season_ended"
	input.Season.UpdateState = "ended"
	opts := options()
	opts.Season = "sakura"
	opts.Year = 2026
	result := cell(t, input, opts)
	if result.Status == "meets_criteria" {
		t.Fatal("ended Sakura became positive")
	}
	input.Season.Status = "ok"
	input.Season.UpdateState = "active"
	input.Season.Spot.ReportDate = "2026-09-27"
	input.Season.Spot.PredictedFullBloomDate = "2026-09-28"
	opts.SeasonCondition = "満開"
	result = cell(t, input, opts)
	if result.Status != "meets_criteria" || result.Seasonal.EvidenceKind != "seasonal_prediction" {
		t.Fatalf("explicit source prediction lost: %#v", result.Seasonal)
	}
}

func TestMountainCannotSummitRank(t *testing.T) {
	input := dailyInput()
	input.Place.URL = "https://tenki.jp/mountain/famous100/5/25/150.html"
	input.Place.Kind = "mountain"
	input.Place.Scope = "foothill"
	result := cell(t, input, options())
	if result.Status != "insufficient_data" || !strings.Contains(strings.Join(result.Reasons, " "), "summit") {
		t.Fatalf("foothill falsely summit-ranked: %#v", result)
	}
}

func TestDeterministicSortTiesLimitsAndAbsentValues(t *testing.T) {
	a, b := dailyInput(), dailyInput()
	a.Place.URL = "https://tenki.jp/forecast/3/16/4410/13104/"
	a.Place.ForecastReferenceURL = a.Place.URL
	a.Weather.Place = a.Place
	opts := options()
	result, err := Evaluate([]Input{a, b}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Sort != "max-pop" || result.Cells[0].Place.URL != testMunicipality || !result.Cells[0].Tie || result.Cells[0].TieGroup != result.Cells[1].TieGroup {
		t.Fatalf("tie/sort: %#v", result)
	}
	opts.Limit = 1
	result, err = Evaluate([]Input{a, b}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated || result.RequestedCells != 2 || result.ReturnedCells != 1 || !result.Cells[0].Tie {
		t.Fatal("truncated ties/counts lost")
	}
	opts.Limit = 70
	opts.Criteria = Criteria{MinTemp: number(5)}
	a.Weather.Periods[0].MinTemperatureC = number(20)
	b.Weather.Periods[0].MinTemperatureC = number(10)
	result, err = Evaluate([]Input{b, a}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.SortDirection != "descending" || *result.Cells[0].SortValue != 20 {
		t.Fatal("min-temp sort direction")
	}
	a.Weather.Periods[0].MinTemperatureC = nil
	b.Weather.Periods[0].MinTemperatureC = nil
	result, err = Evaluate([]Input{a, b}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Cells[0].Tie || result.Cells[0].SortValue != nil {
		t.Fatal("unknown values are not numerical ties or zeros")
	}
}

func TestValidationRejectsMismatchedAndNonfiniteCriteria(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Options)
		places int
	}{
		{"no criteria", func(o *Options) { o.Criteria = Criteria{} }, 1},
		{"too many places", func(*Options) {}, 6},
		{"too many days", func(o *Options) { o.Days = 15 }, 1},
		{"too many cells", func(o *Options) { o.Limit = 71 }, 1},
		{"bad date", func(o *Options) { o.From = "2026-02-30" }, 1},
		{"NaN", func(o *Options) { o.Criteria.MaxPOP = number(math.NaN()) }, 1},
		{"infinity", func(o *Options) { o.Criteria.MaxWind = number(math.Inf(1)) }, 1},
		{"POP outside percent", func(o *Options) { o.Criteria.MaxPOP = number(101) }, 1},
		{"negative rain", func(o *Options) { o.Criteria.MaxRain = number(-1) }, 1},
		{"negative wind", func(o *Options) { o.Criteria.MaxWind = number(-1) }, 1},
		{"contradictory temperatures", func(o *Options) { o.Criteria.MinTemp = number(30); o.Criteria.MaxTemp = number(20) }, 1},
		{"sort unsupplied criterion", func(o *Options) { o.Sort = "max-wind" }, 1},
		{"season condition no season", func(o *Options) { o.SeasonCondition = "見頃" }, 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			opts := options()
			test.change(&opts)
			if err := Validate(opts, test.places); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}

func TestFivePlacesFourteenDatesRemainBounded(t *testing.T) {
	inputs := []Input{dailyInput(), dailyInput(), dailyInput(), dailyInput(), dailyInput()}
	opts := options()
	opts.Days = 14
	opts.Limit = 70
	result, err := Evaluate(inputs, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequestedCells != 70 || result.ReturnedCells != 70 || len(result.Cells) != 70 || result.Truncated {
		t.Fatalf("maximum matrix bounds: %#v", result)
	}
	for _, cell := range result.Cells {
		if cell.Date != "2026-09-28" && (cell.Status != "insufficient_data" || cell.Criteria["max-pop"].Value != nil) {
			t.Fatal("unsupported date acquired phantom data")
		}
	}
}
