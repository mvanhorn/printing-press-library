package tenki

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const chiyoda = "https://tenki.jp/forecast/3/16/4410/13101/"
const fuji = "https://tenki.jp/mountain/famous100/5/25/150.html"
const murodo = "https://tenki.jp/kouyou/4/19/30314.html"
const ueno = "https://tenki.jp/sakura/3/16/54401.html"

var snapshotNow = time.Date(2026, 9, 27, 23, 30, 0, 0, JST)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(req *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}
}
func fixtureClient(t *testing.T, fixtures map[string]string) *Client {
	t.Helper()
	c := NewClient(Config{Now: func() time.Time { return snapshotNow }, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		name, ok := fixtures[r.URL.String()]
		if !ok {
			return nil, errors.New("unexpected request " + r.URL.String())
		}
		return response(r, 200, fixture(t, name)), nil
	})})
	c.limiter = nil
	return c
}
func mustValue(t *testing.T, v *float64, want float64) {
	t.Helper()
	if v == nil || *v != want {
		t.Fatalf("numeric evidence = %v, want %g", v, want)
	}
}

func TestDailyCurrentSnapshot(t *testing.T) {
	c := fixtureClient(t, map[string]string{chiyoda + "10days.html": "daily-20260927.html"})
	r, e := c.Daily(context.Background(), chiyoda)
	if e != nil {
		t.Fatal(e)
	}
	if r.Place.Name != "千代田区" || r.Place.ForecastReferenceURL != chiyoda {
		t.Fatalf("identity %+v", r.Place)
	}
	if len(r.Periods) != 14 || r.Periods[0].Date != "2026-09-27" || r.Periods[13].Date != "2026-10-10" {
		t.Fatalf("actual horizon %+v", r.Periods)
	}
	if r.Source.IssueAt != "2026-09-27T23:00:00+09:00" || r.Source.Freshness != "fresh" {
		t.Fatalf("contextual issue %+v", r.Source)
	}
	if !r.Periods[0].Partial || r.Periods[0].Start != "2026-09-27T00:00:00+09:00" || r.Periods[0].WeatherProbabilityFrom != r.Source.IssueAt || r.Periods[0].Kind != "mixed" || r.Periods[0].TemperatureKind != "forecast_or_estimated_actual" || r.Periods[0].WeatherProbabilityKind != "forecast" {
		t.Fatalf("day zero semantics %+v", r.Periods[0])
	}
	if r.Periods[1].Kind != "forecast" || r.Periods[1].TemperatureKind != "forecast" || r.Periods[1].WeatherProbabilityFrom != "" {
		t.Fatal("future daily provenance changed")
	}
	if r.Periods[13].Confidence != "E" || r.Periods[11].Confidence != "D" || r.Periods[1].Confidence != "" {
		t.Fatal("confidence was not preserved")
	}
	mustValue(t, r.Periods[13].MinTemperatureC, 17)
	mustValue(t, r.Periods[13].MaxTemperatureC, 22)
	if r.Periods[13].PrecipAmountMM != nil {
		t.Fatal("late precipitation amount invented")
	}
	for _, p := range r.Periods {
		if p.WindSpeedMS != nil || p.TemperatureC != nil {
			t.Fatal("daily instantaneous wind/temperature fabricated")
		}
	}
	if len(r.Intervals) != 40 || len(r.Instants) != 50 {
		t.Fatalf("4 vs 5 detail alignment: %d intervals %d instants", len(r.Intervals), len(r.Instants))
	}
	mustValue(t, r.Intervals[1].PrecipAmountMM, 10)
	mustValue(t, r.Instants[0].TemperatureC, 22)
	mustValue(t, r.Instants[4].WindSpeedMS, 1)
	if r.Intervals[0].End != "2026-09-28T06:00:00+09:00" || r.Instants[4].ValidAt != "2026-09-29T00:00:00+09:00" {
		t.Fatal("daily interval/instant timing aligned incorrectly")
	}
	if c.Metrics().HTTPRequests != 1 {
		t.Fatal("canonical municipal daily fetch made redundant identity request")
	}
}

func TestHourlyCurrentSnapshot(t *testing.T) {
	c := fixtureClient(t, map[string]string{chiyoda + "1hour.html": "hourly-20260927.html"})
	r, e := c.Hourly(context.Background(), chiyoda)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Periods) != 72 || r.Place.Name != "千代田区" {
		t.Fatalf("hourly horizon or identity: %d %+v", len(r.Periods), r.Place)
	}
	if r.Periods[0].Kind != "estimated_actual" || r.Periods[22].Kind != "estimated_actual" || r.Periods[23].Kind != "forecast" {
		t.Fatal("grey elapsed estimates treated as future forecasts")
	}
	if r.Periods[0].PrecipProbabilityPct != nil {
		t.Fatal("missing past probability became numeric")
	}
	last := r.Periods[71]
	if last.Date != "2026-09-29" || last.Start != "2026-09-29T23:00:00+09:00" || last.End != "2026-09-30T00:00:00+09:00" || last.ValidAt != last.End {
		t.Fatalf("24-hour interval rollover %+v", last)
	}
	if r.Periods[0].Weather != "曇り" || r.Periods[2].Weather != "小雨" {
		t.Fatal("weather content not parsed")
	}
	if c.Metrics().HTTPRequests != 1 {
		t.Fatal("canonical municipal hourly fetch made redundant identity request")
	}
}

func TestMountainCurrentSnapshot(t *testing.T) {
	c := fixtureClient(t, map[string]string{fuji: "fuji-20260927.html"})
	r, e := c.Mountain(context.Background(), fuji)
	if e != nil {
		t.Fatal(e)
	}
	if r.Place.Name != "富士山" || r.Place.Scope != "foothill" || r.Place.ForecastReferenceName != "富士宮市" || r.Place.ForecastReferenceURL != "https://tenki.jp/forecast/5/25/5030/22207/" {
		t.Fatalf("foothill identity %+v", r.Place)
	}
	mustValue(t, r.Place.ElevationM, 3776)
	if r.ModelInitialAt != "2026-09-27T15:00:00+09:00" || r.ModelKind != "nearby_model_guidance" || r.SummitForecastAvailable {
		t.Fatalf("model semantics %+v", r)
	}
	if r.ModelSource.IssueAt != "" || r.ModelSource.FreshnessReference != "model_initial_at" || r.ModelSource.FreshnessAt != r.ModelInitialAt {
		t.Fatal("initialization copied into publication issue timestamp")
	}
	if len(r.Levels) != 64 || r.Levels[0].ElevationM != 4400 || r.Levels[63].ElevationM != 300 {
		t.Fatalf("actual model levels %d", len(r.Levels))
	}
	if r.Levels[0].TemperatureC != nil || r.Levels[0].WindSpeedMS != nil {
		t.Fatal("missing model fields not null")
	}
	mustValue(t, r.Levels[1].TemperatureC, 0.9)
	mustValue(t, r.Levels[1].WindSpeedMS, 26.6)
	if r.Levels[1].ValidAt != "2026-09-27T15:00:00+09:00" || r.Levels[1].WindDirection != "西南西" {
		t.Fatal("model instant evidence incorrect")
	}
}

func TestSeasonalCurrentAndEndedSnapshots(t *testing.T) {
	c := fixtureClient(t, map[string]string{murodo: "foliage-murodo-20260927.html", ueno: "sakura-ueno-ended-20260927.html"})
	r, e := c.Seasonal(context.Background(), "kouyou", murodo, 2026)
	if e != nil {
		t.Fatal(e)
	}
	if r.Year != 2026 || r.Status != "ok" || r.UpdateState != "active" || r.Spot.Condition != "紅葉見頃" {
		t.Fatalf("2025 sidebar tainted active 2026 report %+v", r)
	}
	if r.Source.IssueAt != "2026-09-27T15:00:00+09:00" || r.Spot.ReportAt != "" || r.Spot.ReportDate != "2026-09-27" {
		t.Fatal("day-only report laundered through publication/weather time")
	}
	if r.Spot.NormalPeriod != "9月中旬 ～ 10月上旬" || len(r.Spot.Species) != 4 || r.Spot.PredictedBestPeriod != "" {
		t.Fatalf("normal/species/prediction distinction %+v", r.Spot)
	}
	if r.Place.ForecastReferenceURL != "https://tenki.jp/forecast/4/19/5510/16323/" || r.Place.ForecastReferenceName != "立山町" {
		t.Fatalf("seasonal weather reference %+v", r.Place)
	}
	s, e := c.Seasonal(context.Background(), "sakura", ueno, 2026)
	if e != nil {
		t.Fatal(e)
	}
	if s.Status != "season_ended" || s.UpdateState != "ended" || s.Source.IssueAt != "" || s.Source.Freshness != "unknown" {
		t.Fatalf("ended season laundered with fresh weather %+v", s)
	}
	if s.Spot.NormalPeriod != "3月下旬～4月上旬" || s.Spot.PredictedFullBloomDate != "" || s.Place.ForecastReferenceName != "台東区" {
		t.Fatalf("retained spot facts %+v", s.Spot)
	}
	future, e := c.Seasonal(context.Background(), "sakura", ueno, 2027)
	if e != nil {
		t.Fatal(e)
	}
	if future.Status != "year_unavailable" || future.RequestedYear != 2027 || future.Year != 2026 {
		t.Fatalf("future season invented %+v", future)
	}
}

func TestBoundedSearchContentAndNegativeQueries(t *testing.T) {
	c := fixtureClient(t, map[string]string{"https://tenki.jp/search/?keyword=%E4%BA%AC%E9%83%BD": "municipality-search-20260927.html", "https://tenki.jp/search/?keyword=NoSuchPlace987654": "municipality-search-20260927.html", "https://tenki.jp/search/?keyword=100-0001": "municipality-search-20260927.html", "https://tenki.jp/mountain/": "mountain-index-20260927.html", "https://tenki.jp/leisure/6/29/": "leisure-kyoto-directory-20260927.html"})
	r, e := c.Search(context.Background(), "京都", "municipality", 10, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Places) < 2 || !r.Ambiguous || !r.Truncated || r.Places[0].Name != "千代田区" {
		t.Fatalf("substring ambiguity/dedup %+v", r)
	}
	seen := map[string]bool{}
	for _, p := range r.Places {
		if seen[p.URL] {
			t.Fatal("municipality address rows not deduplicated")
		}
		seen[p.URL] = true
	}
	n, e := c.Search(context.Background(), "NoSuchPlace987654", "municipality", 10, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(n.Places) != 0 || n.Status != "no_results" {
		t.Fatalf("negative search returned unrelated navigation %+v", n)
	}
	b, _ := json.Marshal(n)
	if !strings.Contains(string(b), `"places":[]`) {
		t.Fatal("empty search array encoded null")
	}
	zip, e := c.Search(context.Background(), "100-0001", "municipality", 10, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(zip.Places) != 1 || zip.Places[0].Name != "千代田区" {
		t.Fatalf("postcode resolution %+v", zip)
	}
	m, e := c.Search(context.Background(), "富士山", "mountain", 10, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Places) != 1 || m.Places[0].URL != fuji {
		t.Fatalf("mountain directory content %+v", m)
	}
	c.cfg.SearchDirectory = "https://tenki.jp/leisure/6/29/"
	l, e := c.Search(context.Background(), "金閣寺", "leisure", 10, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(l.Places) != 1 || l.Places[0].Name != "金閣寺" || l.Places[0].URL != "https://tenki.jp/leisure/6/29/189/7327/" || l.SearchScope != "selected_leisure_directory" || l.Scanned != 10 {
		t.Fatalf("bounded leisure directory %+v", l)
	}
	ln, e := c.Search(context.Background(), "ImpossiblePlace987654", "leisure", 10, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(ln.Places) != 0 || ln.Status != "no_results" || len(ln.Warnings) == 0 {
		t.Fatalf("negative leisure scope %+v", ln)
	}
}

func TestSeasonalListContentAndBounds(t *testing.T) {
	c := fixtureClient(t, map[string]string{"https://tenki.jp/kouyou/": "foliage-index-20260927.html", "https://tenki.jp/kouyou/search/?keyword=%E7%AB%8B%E5%B1%B1&search_type=venue": "foliage-search-20260927.html", "https://tenki.jp/kouyou/search/?keyword=ImpossiblePlace987654&search_type=venue": "foliage-search-20260927.html", "https://tenki.jp/sakura/": "sakura-index-ended-20260927.html"})
	r, e := c.SeasonalList(context.Background(), "kouyou", "立山", 2026, 10, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Spots) != 3 || r.Year != 2026 || r.Scanned != 3 || r.Spots[2].Place.Name != "立山・室堂平" || r.Spots[2].Condition != "紅葉見頃" {
		t.Fatalf("venue search fields %+v", r)
	}
	if r.Source.IssueAt != "" || r.Source.Freshness != "unknown" {
		t.Fatal("undated search assigned sidebar weather/report time")
	}
	n, e := c.SeasonalList(context.Background(), "kouyou", "ImpossiblePlace987654", 2026, 10, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(n.Spots) != 0 || n.Status != "no_results" {
		t.Fatalf("negative seasonal search %+v", n)
	}
	f, e := c.SeasonalList(context.Background(), "kouyou", "", 2027, 10, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(f.Spots) != 0 || f.Status != "year_unavailable" {
		t.Fatal("future list invented")
	}
	s, e := c.SeasonalList(context.Background(), "sakura", "", 2026, 10, 1)
	if e != nil {
		t.Fatal(e)
	}
	if s.Status != "season_ended" || s.UpdateState != "ended" {
		t.Fatal("ended Sakura list mislabeled")
	}
}

func TestTemporalAndNumericBoundaries(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want *float64
	}{{"---", nil}, {"情報なし", nil}, {"", nil}, {"NaN", nil}, {"20%曇", nil}, {"-2.5℃", number("-2.5")}, {"0mm/h", number("0")}} {
		got := number(test.raw)
		if (got == nil) != (test.want == nil) || (got != nil && *got != *test.want) {
			t.Errorf("number(%q)=%v", test.raw, got)
		}
	}
	for _, test := range []struct {
		raw    string
		anchor time.Time
		want   string
	}{{"1日00:00", time.Date(2026, 12, 31, 23, 0, 0, 0, JST), "2027-01-01T00:00:00+09:00"}, {"31日15:00", time.Date(2026, 1, 31, 23, 0, 0, 0, JST), "2026-01-31T15:00:00+09:00"}, {"1日00:00", time.Date(2026, 1, 31, 23, 0, 0, 0, JST), "2026-02-01T00:00:00+09:00"}, {"27日25:00", snapshotNow, ""}, {"27日23:99", snapshotNow, ""}} {
		if got := stamp(dayClock(test.raw, test.anchor)); got != test.want {
			t.Errorf("dayClock(%q)=%s want %s", test.raw, got, test.want)
		}
	}
	if d := dateFromText("01月01日", time.Date(2026, 12, 31, 23, 0, 0, 0, JST)); d.Year() != 2027 {
		t.Fatal("daily year rollover incorrect")
	}
}

// This constructed active-season variant tests supported optional semantics;
// it is not a claim that September's ended Sakura page has these values.
func TestConstructedActiveSeasonAndWrongYearSidebar(t *testing.T) {
	body := `<div id="main-column"><h2>公園の桜 2026<time datetime="2026-03-20T15:00:00+09:00"></time></h2><div id="section-info"><span class="rank-telop">つぼみ</span><dl><dt>開花予想日</dt><dd>3月24日</dd><dt>満開予想日</dt><dd>3月30日</dd><dt>例年の見ごろ時期</dt><dd>3月下旬～4月上旬</dd><dt>見頃予想</dt><dd>3月末～4月初旬</dd></dl></div></div><aside><h3>2025年予想</h3><time datetime="2025-03-01T00:00:00+09:00"></time></aside>`
	c := NewClient(Config{Now: func() time.Time { return time.Date(2026, 3, 20, 16, 0, 0, 0, JST) }})
	r := c.parseSeasonal("sakura", body, Source{Timezone: Timezone}, ueno, 2026)
	if r.Status != "ok" || r.Year != 2026 || r.Spot.PredictedFloweringDate != "2026-03-24" || r.Spot.PredictedFullBloomDate != "2026-03-30" || r.Spot.PredictedBestPeriod != "3月末～4月初旬" || r.Source.IssueAt != "2026-03-20T15:00:00+09:00" {
		t.Fatalf("constructed in-season semantics %+v", r)
	}
	out := strings.Replace(body, `<div id="section-info">`, `<div class="off-season-box">準備中</div><div id="section-info">`, 1)
	if got := c.parseSeasonal("sakura", out, Source{}, ueno, 2026); got.Status != "out_of_season" {
		t.Fatal("explicit out-of-season status missing")
	}
}

func TestUnknownInputsAndMarkupFailClosed(t *testing.T) {
	c := NewClient(Config{Now: func() time.Time { return snapshotNow }})
	for _, raw := range []string{"http://tenki.jp/forecast/3/16/4410/13101/", "https://tenki.jp.evil.test/forecast/3/16/4410/13101/", "https://tenki.jp:443/forecast/3/16/4410/13101/", "https://user@tenki.jp/forecast/3/16/4410/13101/", "https://tenki.jp/docs/rule/", "https://tenki.jp/forecast/3/16/4410/13101/?evil=1", "https://tenki.jp/forecast/%2e%2e/docs/"} {
		if _, e := validateURL(raw); e == nil {
			t.Errorf("accepted invalid target %s", raw)
		}
	}
	if _, e := c.Search(context.Background(), "京都", "unknown", 1, 1); e == nil {
		t.Fatal("unknown kind accepted")
	}
	if _, e := c.Search(context.Background(), "京都", "municipality", 51, 1); e == nil {
		t.Fatal("oversized bound accepted")
	}
	if _, e := c.parseHourly("<html><h2>unrelated</h2></html>", Source{}, Place{}); e == nil {
		t.Fatal("changed hourly markup silently empty")
	}
	if _, e := c.parseDaily("<html><h2>unrelated</h2></html>", Source{}, Place{}); e == nil {
		t.Fatal("changed daily markup silently empty")
	}
}

func TestLeisureIdentityAndLinkedMunicipalRequest(t *testing.T) {
	const destination = "https://tenki.jp/leisure/6/29/189/7327/"
	const reference = "https://tenki.jp/forecast/6/29/6110/26101/"
	// The destination is the unmodified real source. The municipal response
	// reuses the dated Chiyoda layout solely to assert the linked request and
	// destination/reference separation; it is not a Kyoto forecast snapshot.
	c := fixtureClient(t, map[string]string{destination: "leisure-kinkaku-20260927.html", reference + "10days.html": "daily-20260927.html"})
	resolved, e := c.Resolve(context.Background(), destination)
	if e != nil {
		t.Fatal(e)
	}
	if resolved.Place.Name != "鹿苑寺 金閣寺" || resolved.Place.ForecastReferenceURL != reference || resolved.Place.ForecastReferenceName != "京都市北区" || resolved.Place.Scope != "municipal" || resolved.Place.ElevationM != nil {
		t.Fatalf("actual leisure identity/reference %+v", resolved.Place)
	}
	forecast, e := c.Daily(context.Background(), destination)
	if e != nil {
		t.Fatal(e)
	}
	if forecast.Place.Name != resolved.Place.Name || forecast.Place.URL != destination || forecast.Source.URL != reference+"10days.html" || c.Metrics().HTTPRequests != 2 {
		t.Fatalf("destination/reference separation %+v metrics=%+v", forecast.Place, c.Metrics())
	}
}

func TestConstructedForecastMissingValuesAndCalendarRollover(t *testing.T) {
	dec31 := time.Date(2026, 12, 31, 23, 30, 0, 0, JST)
	c := NewClient(Config{Now: func() time.Time { return dec31 }})
	daily := `<!-- forecast/forecast-days/day.html name:千代田区 jiscode:13101 announce_datetime:2026-12-31 23:00:00 --><dl><dd class="forecast10days-actab"><div class="days">12月31日</div><div class="forecast"><img alt="曇"></div><span class="high-temp">0℃</span><span class="low-temp">---</span><div class="prob-precip">---</div><div class="precip">---</div></dd><dd class="forecast10days-actab"><div class="days">01月01日</div><div class="forecast"><img alt="晴"></div><span class="high-temp">-2℃</span><span class="low-temp">-5℃</span></dd></dl>`
	r, e := c.parseDaily(daily, Source{}, Place{URL: chiyoda, ForecastReferenceURL: chiyoda, Kind: "municipality"})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Periods) != 2 || r.Periods[0].MinTemperatureC != nil || r.Periods[0].PrecipProbabilityPct != nil || r.Periods[0].PrecipAmountMM != nil || r.Periods[1].Date != "2027-01-01" {
		t.Fatalf("missing/calendar daily evidence %+v", r.Periods)
	}
	mustValue(t, r.Periods[0].MaxTemperatureC, 0)
	mustValue(t, r.Periods[1].MaxTemperatureC, -2)
	hourly := fixture(t, "hourly-20260927.html")
	for _, replacement := range [][2]string{{"2026-09-27 23:00:00", "2026-12-31 23:00:00"}, {"2026年09月27日", "2026年12月31日"}, {"2026年09月28日", "2027年01月01日"}, {"2026年09月29日", "2027年01月02日"}} {
		hourly = strings.ReplaceAll(hourly, replacement[0], replacement[1])
	}
	h, e := c.parseHourly(hourly, Source{}, Place{URL: chiyoda, ForecastReferenceURL: chiyoda, Kind: "municipality"})
	if e != nil {
		t.Fatal(e)
	}
	if h.Periods[23].Date != "2026-12-31" || h.Periods[23].ValidAt != "2027-01-01T00:00:00+09:00" || h.Periods[71].End != "2027-01-03T00:00:00+09:00" {
		t.Fatal("hour24/year rollover failed")
	}
	unknown := strings.Replace(daily, "announce_datetime:2026-12-31 23:00:00", "announce_datetime:", 1)
	u, e := c.parseDaily(unknown, Source{}, Place{})
	if e != nil {
		t.Fatal(e)
	}
	if u.Source.Freshness != "unknown" || !strings.Contains(strings.Join(u.Warnings, " "), "explicit assumption") {
		t.Fatal("unknown issue produced unlabelled calendar-year assumption")
	}
}

func TestSourceFreshnessPolicyBoundaries(t *testing.T) {
	for _, age := range []time.Duration{2 * time.Hour, 36 * time.Hour, 12 * time.Hour} {
		var s Source
		applyIssue(&s, snapshotNow.Add(-age), "dated source", age, snapshotNow)
		if s.Freshness != "fresh" {
			t.Fatalf("exact age boundary %s %+v", age, s)
		}
		applyIssue(&s, snapshotNow.Add(-age-time.Second), "dated source", age, snapshotNow)
		if s.Freshness != "stale" || !s.SourceStale {
			t.Fatalf("stale age boundary %s %+v", age, s)
		}
	}
	var unknown Source
	applyIssue(&unknown, time.Time{}, "", 2*time.Hour, snapshotNow)
	if unknown.Freshness != "unknown" || unknown.IssueAt != "" {
		t.Fatal("unknown timestamp fabricated freshness")
	}
}

func TestConstructedMunicipalityWardSearchLabels(t *testing.T) {
	// Constructed search-address rows test ward disambiguation without
	// pretending that the broad 京都/Tokyo source fixture covers Kyoto wards.
	body := `<div class="search-entry-data-wrap"><p class="search-entry-data"><a href="/forecast/6/29/6110/26103/"><span class="zipcode">606-0000</span><span class="address">京都府京都市左京区以下に掲載がない場合</span></a></p><p class="search-entry-data"><a href="/forecast/6/29/6110/26101/"><span class="zipcode">603-0000</span><span class="address">京都府京都市北区以下に掲載がない場合</span></a></p></div>`
	places, scanned := municipalityCandidates(parseHTML(body), "京都市")
	if scanned != 2 || len(places) != 2 || places[0].Name != "京都市左京区" || places[1].Name != "京都市北区" {
		t.Fatalf("ward identities collapsed %+v", places)
	}
	if places[0].Address != "京都府京都市左京区以下に掲載がない場合" || places[0].PostalCode != "606-0000" || places[0].NameSource != "search_address" || places[0].ForecastReferenceName != "" {
		t.Fatalf("unverified address label treated as exact municipality identity %+v", places[0])
	}
}

func TestKyotoRawSearchRetainsWardAndAddressEvidence(t *testing.T) {
	c := fixtureClient(t, map[string]string{"https://tenki.jp/search/?keyword=%E4%BA%AC%E9%83%BD%E5%B8%82": "municipality-kyoto-city-search-20260927.html"})
	r, e := c.Search(context.Background(), "京都市", "municipality", 10, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Places) < 2 || r.Places[0].Name != "京都市左京区" || r.Places[1].Name != "京都市下京区" || r.Places[0].Address != "京都府京都市左京区久多川合町" || r.Places[0].PostalCode != "520-0461" || r.Places[0].NameSource != "search_address" {
		t.Fatalf("raw Kyoto ward/address evidence %+v", r)
	}
	if !r.Ambiguous || !r.Truncated || r.Pages != 1 || r.Scanned != 30 || c.Metrics().HTTPRequests != 1 {
		t.Fatalf("bounded city coverage %+v metrics=%+v", r, c.Metrics())
	}
	if r.Places[0].ForecastReferenceName != "" {
		t.Fatal("address label asserted an unverified exact forecast name")
	}
}
