package jma

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fixtureTransport map[string]string

func (f fixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, ok := f[req.URL.Path]
	code := 200
	if !ok {
		code = 404
		body = `{"error":"fixture missing"}`
	}
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: req}, nil
}
func testClient(t *testing.T, f fixtureTransport) *Client {
	t.Helper()
	c := New(t.TempDir(), true, false, false, time.Second)
	c.HTTP.Transport = f
	c.now = func() time.Time { return time.Date(2026, 10, 1, 2, 0, 0, 0, JST) }
	return c
}
func resultMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}
func warningFixture(status, code string, omit string) string {
	products := []map[string]any{}
	for _, p := range []string{"VPWW55", "VPWW56", "VPWW58", "VPWW59", "VPWW61"} {
		if p == omit {
			continue
		}
		k := map[string]any{"status": "発表警報・注意報はなし"}
		if p == "VPWW55" {
			k = map[string]any{"status": status}
			if code != "" {
				k["code"] = code
			}
		}
		products = append(products, map[string]any{"dataTypeCode": p, "reportDatetime": "2026-10-01T01:00:00+09:00", "publishingOffice": "気象庁", "infoType": "発表", "warning": map[string]any{"class20Items": []any{map[string]any{"areaCode": "1310100", "kinds": []any{k}}}}})
	}
	b, _ := json.Marshal(products)
	return string(b)
}
func TestWarningNoWarningRequiresCompleteKnownRecords(t *testing.T) {
	for _, tt := range []struct{ name, status, code, omit, want string }{
		{"none", "発表警報・注意報はなし", "", "", "none_reported"}, {"lifted", "解除", "10", "", "none_reported"}, {"active", "発表", "43", "", "active"}, {"downgrade", "警報から注意報", "10", "", "active"}, {"unknown status", "weird", "10", "", "incomplete"}, {"unknown code", "継続", "99", "", "incomplete"}, {"missing product", "発表警報・注意報はなし", "", "VPWW56", "incomplete"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := testClient(t, fixtureTransport{"/bosai/warning/data/r8/130000.json": warningFixture(tt.status, tt.code, tt.omit)})
			v, e := c.Warnings(context.Background(), "1310100", true, 0, 20)
			if (tt.want == "incomplete") != (e != nil) {
				t.Fatalf("state %s error %v", tt.want, e)
			}
			r := resultMap(t, v.Results)
			if r["state"] != tt.want {
				t.Fatalf("state %v != %s", r["state"], tt.want)
			}
		})
	}
	c := testClient(t, fixtureTransport{"/bosai/warning/data/r8/130000.json": `[]`})
	v, e := c.Warnings(context.Background(), "1310100", false, 0, 20)
	if e == nil {
		t.Fatal("incomplete source should return exit-5 error")
	}
	if resultMap(t, v.Results)["state"] != "incomplete" {
		t.Fatal("empty source must not clear warnings")
	}
}
func TestNewWarningCodeSemantics(t *testing.T) {
	for _, tt := range []struct {
		code, name, severity string
		level                int
	}{{"49", "レベル４土砂災害危険警報", "urgent_warning", 4}, {"39", "レベル５土砂災害特別警報", "emergency_warning", 5}, {"29", "レベル２土砂災害注意報", "advisory", 2}, {"15", "強風注意報", "advisory", 0}} {
		h, ok := hazard(tt.code)
		if !ok || h.JA != tt.name || h.Severity != tt.severity {
			t.Fatalf("%s: %+v", tt.code, h)
		}
		if tt.level > 0 && h.AlertLevel != tt.level {
			t.Fatal(h)
		}
	}
}
func TestResolutionAndReferenceStations(t *testing.T) {
	i, e := embeddedInventory()
	if e != nil {
		t.Fatal(e)
	}
	p, e := i.Resolve("1310100")
	if e != nil || p.OfficeID != "130000" || len(p.ForecastDistrictIDs) != 1 || p.ForecastDistrictIDs[0] != "130010" {
		t.Fatal(p, e)
	}
	p, e = i.Resolve("270000")
	if e != nil || p.Kind != "office" {
		t.Fatal(p, e)
	}
	if _, e = i.Resolve("not a source place"); e == nil {
		t.Fatal("unknown name must fail")
	}
	x := i.SearchStations("44132")
	if len(x) != 1 || x[0].NameJA != "東京" {
		t.Fatal(x)
	}
	if forecastPath("014030") != "014100" || forecastPath("460040") != "460100" {
		t.Fatal("office alias missing")
	}
}
func TestParallelForecastArraysMustAlign(t *testing.T) {
	if arrLength(2, []string{"1"}) == nil {
		t.Fatal("misaligned values accepted")
	}
	if arrLength(2, nil, []string{"", "3"}) != nil {
		t.Fatal("explicit missing string rejected")
	}
	if v, e := numericAt([]string{""}, 0); e != nil || v != nil {
		t.Fatal(v, e)
	}
	if _, e := numericAt([]string{"nonsense"}, 0); e == nil {
		t.Fatal("invalid number silently became missing")
	}
}
func TestTyphoonIssueJoinRefusesMixedBulletins(t *testing.T) {
	index := `[{"tropicalCyclone":"TC2601","typhoonNumber":"2601","category":"TS","issue":"2026-10-01T01:00:00+09:00"}]`
	track := `[{"part":"title","typhoonNumber":"2601","issue":{"JST":"2026-10-01T01:00:00+09:00"}},{"part":{"en":"Analysis"},"advancedHours":0,"validtime":{"JST":"2026-10-01T00:00:00+09:00"},"center":[30,140]}]`
	spec := strings.ReplaceAll(track, "01:00:00", "02:00:00")
	c := testClient(t, fixtureTransport{"/bosai/typhoon/data/targetTc.json": index, "/bosai/typhoon/data/TC2601/forecast.json": track, "/bosai/typhoon/data/TC2601/specifications.json": spec})
	_, e := c.Typhoon(context.Background(), "TC2601", false, 120)
	if e == nil || !strings.Contains(e.Error(), "different issued times") {
		t.Fatal(e)
	}
}
func TestCacheExpiryAndNoStaleFallback(t *testing.T) {
	c := testClient(t, fixtureTransport{"/bosai/x.json": `{"ok":true}`})
	c.NoCache = false
	var v map[string]any
	if e := c.Get(context.Background(), "/x.json", time.Minute, &v); e != nil {
		t.Fatal(e)
	}
	c.Offline = true
	if e := c.Get(context.Background(), "/x.json", time.Minute, &v); e != nil {
		t.Fatal(e)
	}
	now := c.now()
	c.now = func() time.Time { return now.Add(2 * time.Minute) }
	if e := c.Get(context.Background(), "/x.json", time.Minute, &v); e == nil {
		t.Fatal("stale cache used as current")
	}
	if c.requests != 1 {
		t.Fatalf("offline must not fetch: %d", c.requests)
	}
}
func TestNullTyphoonIndexCannotMeanNone(t *testing.T) {
	c := testClient(t, fixtureTransport{"/bosai/typhoon/data/targetTc.json": `null`})
	if _, e := c.TyphoonList(context.Background(), 0, 20); e == nil {
		t.Fatal("null became no cyclones")
	}
}

func TestMissingCycloneScaleStaysNull(t *testing.T) {
	if missingString(nil) != nil || missingString("-") != nil || missingString("強い") != "強い" {
		t.Fatal("missing scale/intensity must remain null")
	}
}

func TestWeeklyBroaderRegionSelectionAndTemperatureBuckets(t *testing.T) {
	inv, e := embeddedInventory()
	if e != nil {
		t.Fatal(e)
	}
	if len(inv.WeekEligible["130020"]) != 2 || inv.WeekEligible["130020"][0] != "130100" {
		t.Fatal("north Izu must authorize broader Izu weekly region")
	}
	raw := `[{"publishingOffice":"気象庁","reportDatetime":"2026-10-01T05:00:00+09:00","timeSeries":[{"timeDefines":["2026-10-01T05:00:00+09:00","2026-10-02T00:00:00+09:00"],"areas":[{"area":{"name":"伊豆諸島北部","code":"130020"},"weatherCodes":["100","200"]}]},{"timeDefines":["2026-10-01T09:00:00+09:00","2026-10-01T09:00:00+09:00","2026-10-02T00:00:00+09:00","2026-10-02T09:00:00+09:00"],"areas":[{"area":{"name":"大島","code":"44172"},"temps":["25","25","18","26"]}]}]},{"publishingOffice":"気象庁","reportDatetime":"2026-10-01T05:00:00+09:00","timeSeries":[{"timeDefines":["2026-10-02T00:00:00+09:00"],"areas":[{"area":{"name":"伊豆諸島","code":"130100"},"weatherCodes":["100"]}]},{"timeDefines":["2026-10-02T00:00:00+09:00"],"areas":[{"area":{"name":"八丈島","code":"44263"},"tempsMin":["20"],"tempsMax":["28"]}]}]}]`
	c := testClient(t, fixtureTransport{"/bosai/forecast/data/forecast/130000.json": raw})
	v, e := c.Forecast(context.Background(), "130020", "all", "", 3)
	if e != nil {
		t.Fatal(e)
	}
	m := resultMap(t, v.Results)
	series := m["series"].([]any)
	var weekly, temp bool
	for _, a := range series {
		row := a.(map[string]any)
		if row["kind"] == "weekly_weather" && row["source_id"] == "130100" {
			weekly = true
		}
		if row["kind"] == "short_temperature" {
			temp = true
			ps := row["points"].([]any)
			if len(ps) != 3 || ps[1].(map[string]any)["temperature_type"] != "minimum" || ps[1].(map[string]any)["temperature_c"] != float64(18) {
				t.Fatal(ps)
			}
		}
	}
	if !weekly || !temp {
		t.Fatal(series)
	}
	for _, hour := range []int{0, 17, 18, 23} {
		if temperatureRole(hour, 0) != "minimum" || temperatureRole(hour, 1) != "maximum" {
			t.Fatalf("incorrect evening bucket %d", hour)
		}
	}
	for _, hour := range []int{5, 10, 11, 16} {
		if temperatureRole(hour, 1) != "unused_source_marker" || temperatureRole(hour, 2) != "minimum" || temperatureRole(hour, 3) != "maximum" {
			t.Fatalf("incorrect daytime bucket %d", hour)
		}
	}
}

func TestInlandWaveTideIsExplicitlyNotApplicable(t *testing.T) {
	raw := strings.ReplaceAll(warningFixture("発表警報・注意報はなし", "", "VPWW59"), "1310100", "1920100")
	c := testClient(t, fixtureTransport{"/bosai/warning/data/r8/190000.json": raw})
	v, e := c.Warnings(context.Background(), "1920100", false, 0, 20)
	if e != nil {
		t.Fatal(e)
	}
	m := resultMap(t, v.Results)
	if m["state"] != "none_reported" {
		t.Fatal(m)
	}
	a := m["municipalities"].([]any)[0].(map[string]any)
	if len(a["not_applicable_product_ids"].([]any)) != 1 {
		t.Fatal(a)
	}
	c = testClient(t, fixtureTransport{"/bosai/warning/data/r8/130000.json": strings.ReplaceAll(warningFixture("発表警報・注意報はなし", "", "VPWW59"), "1310100", "1340100")})
	v, e = c.Warnings(context.Background(), "1340100", false, 0, 20)
	if e == nil {
		t.Fatal("incomplete source should return exit-5 error")
	}
	if resultMap(t, v.Results)["state"] != "incomplete" {
		t.Fatal("missing applicable wave/tide product must remain incomplete")
	}
	if _, _, known := lifecycle("未知から注意報"); known {
		t.Fatal("unknown downgrade must stay unknown")
	}
}
