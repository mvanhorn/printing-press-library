package repark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/cliutil"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func when(s string) time.Time {
	t, e := ParseJST(s)
	if e != nil {
		panic(e)
	}
	return t
}
func floatp(v float64) *float64 { return &v }

func TestIdentityAndURLs(t *testing.T) {
	for _, tc := range []struct {
		input, id string
		ok        bool
	}{{"REP0022209", "REP0022209", true}, {DetailURL("REP0022209"), "REP0022209", true}, {"REP22", "", false}, {"https://evil.example/?park=REP0022209", "", false}, {"REP0022209&payment=1", "", false}, {"https://www.repark.jp/payment/input/?park=REP0022209", "", false}} {
		id, e := CanonicalID(tc.input)
		if (e == nil) != tc.ok || id != tc.id {
			t.Errorf("%s -> %s %v", tc.input, id, e)
		}
	}
	for _, id := range []string{"REP0022209", "REP0029431"} {
		if !strings.HasSuffix(DetailURL(id), "?park="+id) {
			t.Fatal("canonical URL loses identity")
		}
	}
}

func TestCoordinateBoundsAndDistances(t *testing.T) {
	for _, tc := range []struct {
		p     Coordinates
		valid bool
	}{{Coordinates{34.663534, 135.516310}, true}, {Coordinates{35.6812996, 139.7670658}, true}, {Coordinates{0, 0}, false}, {Coordinates{91, 135}, false}, {Coordinates{35, 181}, false}} {
		if (ValidateCoordinates(tc.p) == nil) != tc.valid {
			t.Errorf("coordinate %+v validity", tc.p)
		}
	}
	a := Coordinates{34.663534, 135.516310}
	for _, tc := range []struct {
		b        Coordinates
		min, max int
	}{{a, 0, 0}, {Coordinates{34.664534, 135.516310}, 110, 112}, {Coordinates{34.663534, 135.517310}, 90, 92}} {
		d := DistanceM(a, tc.b)
		if d < tc.min || d > tc.max {
			t.Errorf("distance %d outside %d..%d", d, tc.min, tc.max)
		}
	}
}

func TestFitKeepsVacancyAndUnknownsSeparate(t *testing.T) {
	l := Limits{HeightM: floatp(2), WidthM: floatp(1.9), PerBay: boolp(true)}
	for _, tc := range []struct {
		v                   Vehicle
		status              string
		conflicts, unknowns int
	}{{Vehicle{}, "not_assessed", 0, 0}, {Vehicle{HeightM: floatp(2)}, "within_supplied_published_limits", 0, 0}, {Vehicle{HeightM: floatp(2.1)}, "exceeds_published_limits", 1, 0}, {Vehicle{WeightT: floatp(2)}, "unknown", 0, 1}, {Vehicle{HeightM: floatp(2.1), WeightT: floatp(2)}, "exceeds_published_limits", 1, 1}} {
		f := AssessFit(l, tc.v)
		if f.Status != tc.status || len(f.Conflicts) != tc.conflicts || len(f.UnknownFields) != tc.unknowns || f.Guaranteed || f.RemainingBayFit != "unknown" {
			t.Errorf("fit %+v", f)
		}
	}
	for _, v := range []Vehicle{{HeightM: floatp(2)}, {WeightT: floatp(-1)}} {
		valid := v.HeightM != nil
		if (ValidateVehicle(v) == nil) != valid {
			t.Fatal("vehicle validation")
		}
	}
}

func TestJSTAndQuoteBoundaries(t *testing.T) {
	for _, tc := range []struct {
		query string
		valid bool
	}{{"東京駅", true}, {"大阪市天王寺区上汐", true}, {"", false}, {strings.Repeat("長", 73), false}, {"東京\n駅", false}} {
		if (ValidateQuery(tc.query) == nil) != tc.valid {
			t.Errorf("query %q validation", tc.query)
		}
	}
	for _, tc := range []struct {
		s, want string
		ok      bool
	}{{"2026-10-03T00:15", "2026-10-03T00:15:00+09:00", true}, {"2026-10-02T15:15:00Z", "2026-10-03T00:15:00+09:00", true}, {"2026-12-31 23:59", "2026-12-31T23:59:00+09:00", true}, {"2026-02-30T08:00", "", false}, {"2026-10-03T24:00", "", false}, {"2026-10-03T00:00:12+09:00", "", false}} {
		d, e := ParseJST(tc.s)
		if (e == nil) != tc.ok {
			t.Errorf("%s: %v", tc.s, e)
		} else if tc.ok && stamp(d) != tc.want {
			t.Errorf("%s -> %s", tc.s, stamp(d))
		}
	}
	start := when("2026-10-03T23:30")
	now := when("2026-10-02T12:00")
	for _, tc := range []struct {
		bay int
		end time.Time
		ok  bool
	}{{1, start.Add(30 * time.Minute), true}, {1, start.Add(48 * time.Hour), true}, {1, start.Add(48*time.Hour + time.Minute), false}, {0, start.Add(time.Hour), false}, {1000, start.Add(time.Hour), false}, {1, start, false}, {1, when("2028-10-02T12:00"), false}} {
		if (ValidateQuote(tc.bay, start, tc.end, now) == nil) != tc.ok {
			t.Errorf("quote validation %+v", tc)
		}
	}
	for _, tc := range []struct {
		o  Options
		ok bool
	}{{Options{RadiusM: 1000, Limit: 10, MaxScanRecords: 500}, true}, {Options{RadiusM: 2001, Limit: 10, MaxScanRecords: 500}, false}, {Options{RadiusM: 1000, Limit: 10, MaxScanRecords: 500, WithinLimitsOnly: true}, false}} {
		if (ValidateOptions(tc.o) == nil) != tc.ok {
			t.Errorf("options %+v", tc)
		}
	}
}

func TestRatesMaximumsAndRestrictions(t *testing.T) {
	for _, tc := range []struct {
		day, window, rate string
		minutes, amount   int
		overnight         bool
	}{{"全日", "08:00-20:00", "30分/300円", 30, 300, false}, {"土日祝", "20:00-08:00", "60分/100円", 60, 100, true}, {"月～金", "00:00-24:00", "15分/1,100円", 15, 1100, false}} {
		r := parseRate(tc.day, tc.window, tc.rate, "")
		if r.AmountJPY == nil || *r.AmountJPY != tc.amount || r.IntervalMinutes == nil || *r.IntervalMinutes != tc.minutes || r.Overnight != tc.overnight || r.DayType != tc.day {
			t.Errorf("rate %+v", r)
		}
	}
	for _, tc := range []struct {
		source, application, kind string
		amount                    int
		overnight                 bool
	}{{"最大料金<br>【全日】20:00～8:00以内 最大料金600円<br>※最大料金は繰り返し適用となります。", "repeating", "time_window", 600, true}, {"【全日】最大料金入庫後24時間以内1900円<br>※最大料金は１回限り適用。", "one_time", "elapsed_after_entry", 1900, false}, {"入庫当日24時まで最大料金2000円", "unspecified", "calendar_day", 2000, false}, {"イベント開催日 最大料金3,000円", "unspecified", "unparsed", 3000, false}} {
		ms, app := maximums(tc.source)
		if len(ms) != 1 || app != tc.application || ms[0].Kind != tc.kind || ms[0].AmountJPY == nil || *ms[0].AmountJPY != tc.amount || ms[0].Overnight != tc.overnight {
			t.Errorf("maximum %s -> %+v %s", tc.source, ms, app)
		}
	}
	l := parseLimits("高さ1.5m、長さ4.7m、幅1.8m、重量2t、車室ごとに車両制限が異なる")
	if l.HeightM == nil || *l.HeightM != 1.5 || l.PerBay == nil || !*l.PerBay || l.Note == "" {
		t.Fatal(l)
	}
	if numeric("NaN") != nil || numeric("Inf") != nil || numeric("0") != nil {
		t.Fatal("invalid source numeric became a valid limit")
	}
}

func TestSourceFixturesRetainTariffAndUnknowns(t *testing.T) {
	tm := when("2026-10-02T23:45")
	l, e := parseDetail(fixture(t, "detail-osaka.html"), "REP0022209", tm)
	if e != nil {
		t.Fatal(e)
	}
	if l.Name != "上汐４丁目第３" || l.Capacity == nil || *l.Capacity != 4 || len(l.Rates) != 2 || len(l.Maximums) != 2 || l.MaximumApplication != "repeating" || l.Limits.HeightM == nil || l.Hours.Open24H == nil || !*l.Hours.Open24H || l.Coordinates == nil || l.CalculatorURL == "" {
		t.Fatalf("detail content %+v", l)
	}
	if l.Occupancy.ExactAvailableSpaces != nil || l.Occupancy.MeasurementTime != nil || l.TaxIncluded != nil || l.Fit.Guaranteed {
		t.Fatal("unknown source facts invented")
	}
	if _, e = parseDetail(fixture(t, "detail-osaka.html"), "REP9999999", tm); e == nil {
		t.Fatal("wrong identity accepted")
	}
	rows, e := parseMarkers(fixture(t, "markers-osaka.json"), tm)
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 3 || rows[0].ID != l.ID || len(rows[0].Rates) != 2 || rows[0].Rates[0].AmountJPY == nil || *rows[0].Rates[0].AmountJPY != 300 || rows[0].SourceImportDate == "" || rows[0].MaximumApplication != "repeating" {
		t.Fatalf("marker fixture %+v", rows)
	}
	for _, b := range []string{"[]", "null", "{}", "[{\"park_code\":\"REP9999999\"}]"} {
		rs, e := parseMarkers([]byte(b), tm)
		if b == "[]" {
			if e != nil || len(rs) != 0 {
				t.Fatal("empty array rejected")
			}
		} else if e == nil {
			t.Fatalf("malformed markers %s accepted", b)
		}
	}
	q, e := parseQuote(fixture(t, "quote-osaka.html"), l.ID, l.Name, 1, when("2026-10-03T08:00"), when("2026-10-03T12:00"), tm)
	if e != nil {
		t.Fatal(e)
	}
	if q.AmountJPY != 1800 || len(q.SourceCautions) != 4 || q.DiscountsIncluded || q.FinalBilledChargeGuaranteed || q.TaxIncluded != nil {
		t.Fatalf("quote %+v", q)
	}
	if _, e = parseQuote(fixture(t, "quote-osaka.html"), l.ID, l.Name, 1, when("2026-10-03T08:00"), when("2026-10-03T12:01"), tm); e == nil {
		t.Fatal("mismatching echoed source interval accepted")
	}
}

func TestSourceIndexedDayTypeGroups(t *testing.T) {
	lots, err := parseMarkers(fixture(t, "markers-day-types.json"), when("2026-10-02T23:45"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lots) != 1 || len(lots[0].Rates) != 4 || lots[0].Rates[0].DayType != "月～土" || lots[0].Rates[2].DayType != "日祝" || lots[0].Rates[1].AmountJPY == nil || *lots[0].Rates[1].AmountJPY != 110 {
		t.Fatalf("indexed source groups dropped: %+v", lots)
	}
	for _, tc := range []struct {
		raw   string
		count int
		valid bool
	}{{`[{"name":"全日"}]`, 1, true}, {`{"4":{"name":"日祝"},"1":{"name":"月～土"}}`, 2, true}, {`{}`, 0, true}, {`"bad"`, 0, false}} {
		v, e := orderedSourceValues([]byte(tc.raw))
		if (e == nil) != tc.valid || len(v) != tc.count {
			t.Errorf("source values %s: %d %v", tc.raw, len(v), e)
		}
	}
}

func TestBaySpecificCapsRemainUnparsedAndVariationUnknown(t *testing.T) {
	for _, source := range []string{"【全日】8:00～18:00以内 最大料金1600円※12～14番は1400円", "【全日】18:00～8:00以内 最大料金2300円※12～14番は2100円", "【全日】12～14番 最大料金1400円"} {
		ms, _ := maximums(source)
		if len(ms) != 1 || ms[0].AmountJPY != nil || ms[0].Kind != "unparsed" || ms[0].SourceText != source {
			t.Fatalf("bay exception became a universal cap: %+v", ms)
		}
	}
	for _, tc := range []struct {
		source string
		varies bool
	}{{"高さ2m、長さ5m、幅1.9m、重量2t", false}, {"高さ1.5m、車室ごとに車両制限が異なる", true}, {"全車室共通の車両サイズ指定があります", false}, {"軽自動車専用車室があります", false}, {"車室ごとに同じ車両制限です", false}, {"車室によって車両制限が異なります", true}, {"全車室共通です。車室によって車両制限は異なりません。", false}, {"車室ごとに車両制限の違いはありません。", false}, {"車室によって車両制限は異ならない。", false}, {"高さは全車室共通です。幅は車室によって異なります。", true}} {
		l := parseLimits(tc.source)
		if tc.varies {
			if l.PerBay == nil || !*l.PerBay {
				t.Fatal(l)
			}
		} else if l.PerBay != nil {
			t.Fatal("unknown bay variation became false")
		}
		body, err := json.Marshal([]map[string]any{{"park_code": "REP0022209", "park_name": "制限注記の検証", "latitude": "34.663534", "longitude": "135.516310", "limit_note": tc.source}})
		if err != nil {
			t.Fatal(err)
		}
		markers, err := parseMarkers(body, when("2026-10-02T23:45"))
		if err != nil {
			t.Fatal(err)
		}
		if tc.varies {
			if markers[0].Limits.PerBay == nil || !*markers[0].Limits.PerBay {
				t.Fatalf("explicit marker variation missing: %+v", markers[0].Limits)
			}
		} else if markers[0].Limits.PerBay != nil {
			t.Fatalf("generic marker restriction became a bay-variation assertion: %+v", markers[0].Limits)
		}
	}
	rows, err := parseMarkers(fixture(t, "markers-osaka.json"), when("2026-10-02T23:45"))
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Limits.PerBay != nil {
		t.Fatal("absent marker bay note became uniform-bay assertion")
	}
}

func TestClientWorkflowsUseSourceContracts(t *testing.T) {
	for _, tc := range []string{"detail", "nearby", "search", "quote"} {
		t.Run(tc, func(t *testing.T) {
			simulations := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/parking_user/time/result/detail/":
					w.Write(fixture(t, "detail-osaka.html"))
				case "/ajax/time_markers.json":
					if !strings.HasPrefix(r.URL.Query().Get("range"), "C34.66353400,135.51631000N") {
						t.Error("wrong source range grammar")
					}
					w.Write(fixture(t, "markers-osaka.json"))
				case "/parking_user/time/freeword/":
					if r.URL.Query().Get("word") != "上汐" || r.URL.Query().Get("st") != "1" {
						t.Error("wrong freeword contract")
					}
					http.Redirect(w, r, "/parking_user/time/map.html?lat=34.663534&lon=135.516310", http.StatusFound)
				case "/parking_user/time/map.html":
					fmt.Fprint(w, `<input id="freeword" value="大阪市天王寺区上汐">`)
				case "/parking_user/time/result/calculation/":
					if r.Method == "GET" {
						w.Write(fixture(t, "calculator-form.html"))
					} else {
						simulations++
						r.ParseForm()
						for k, v := range map[string]string{"func": "settime", "pkid": "22209", "settime-pksno": "1", "settime-startDate": "2026/10/03", "settime-startHour": "08", "settime-startMinute": "00", "settime-endDate": "2026/10/03", "settime-endHour": "12", "settime-endMinute": "00"} {
							if r.Form.Get(k) != v {
								t.Errorf("calculator wire %s=%s", k, r.Form.Get(k))
							}
						}
						w.Write(fixture(t, "quote-osaka.html"))
					}
				default:
					t.Errorf("unsupported source path %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			c := New(time.Second, 0, 3)
			c.origin = server.URL
			c.now = func() time.Time { return when("2026-10-02T12:00") }
			ctx := context.Background()
			o := Options{RadiusM: 1000, Limit: 1, MaxScanRecords: 500}
			switch tc {
			case "detail":
				_, e := c.Detail(ctx, "REP0022209")
				if e != nil {
					t.Fatal(e)
				}
			case "nearby":
				v, e := c.Nearby(ctx, Coordinates{34.663534, 135.516310}, o)
				if e != nil || len(v.Results) != 1 || !v.OutputTruncated {
					t.Fatalf("nearby %+v %v", v, e)
				}
			case "search":
				v, e := c.Search(ctx, "上汐", o)
				if e != nil || v.ResolvedPlace != "大阪市天王寺区上汐" || c.requests != 3 {
					t.Fatalf("search %+v %v", v, e)
				}
			case "quote":
				q, e := c.Quote(ctx, "REP0022209", 1, when("2026-10-03T08:00"), when("2026-10-03T12:00"))
				if e != nil || q.AmountJPY != 1800 || simulations != 1 {
					t.Fatalf("quote %+v %v", q, e)
				}
			}
			m := c.Meta()
			if m.Requests < 1 || m.Requests > m.RequestBudget || len(m.SourceURLs) != m.Requests || m.Source != "live" {
				t.Fatal(m)
			}
		})
	}
}

func TestClientRejectsThrottleOversizeRedirectAndBudget(t *testing.T) {
	for _, tc := range []string{"throttle", "oversize", "redirect", "budget", "timeout"} {
		t.Run(tc, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch tc {
				case "throttle":
					w.Header().Set("Retry-After", "20")
					w.WriteHeader(429)
				case "oversize":
					w.Header().Set("Content-Length", fmt.Sprint(MaxBodyBytes+1))
					w.WriteHeader(200)
				case "redirect":
					http.Redirect(w, r, "https://evil.example/payment", 302)
				case "timeout":
					<-r.Context().Done()
				default:
					w.Write(fixture(t, "detail-osaka.html"))
				}
			}))
			defer s.Close()
			c := New(time.Second, 0, 1)
			c.origin = s.URL
			ctx := context.Background()
			if tc == "timeout" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 10*time.Millisecond)
				defer cancel()
			}
			_, err := c.Detail(ctx, "REP0022209")
			if tc == "budget" {
				if err != nil {
					t.Fatal(err)
				}
				_, err = c.Detail(ctx, "REP0022209")
			}
			if err == nil {
				t.Fatalf("%s accepted", tc)
			}
			if tc == "throttle" {
				var rate *cliutil.RateLimitError
				if !errors.As(err, &rate) || rate.RetryAfter != 20*time.Second {
					t.Fatalf("throttle not typed: %v", err)
				}
			}
		})
	}
}

func TestUnresolvedSearchAndScanFilterBounds(t *testing.T) {
	for _, tc := range []string{"unresolved", "scan", "fit", "occupancy"} {
		t.Run(tc, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc == "unresolved" {
					fmt.Fprint(w, `<a href="/parking_user/time/map.html?lat=35&lon=135">京都の候補</a><a href="https://evil.example/">bad</a>`)
				} else {
					w.Write(fixture(t, "markers-osaka.json"))
				}
			}))
			defer s.Close()
			c := New(time.Second, 0, 1)
			c.origin = s.URL
			o := Options{RadiusM: 1000, Limit: 10, MaxScanRecords: 500}
			if tc == "unresolved" {
				v, e := c.Search(context.Background(), "京都", o)
				if e != nil || v.Anchor != nil || v.Status != "needs_refinement" || len(v.Candidates) != 1 || len(v.Results) != 0 {
					t.Fatalf("unresolved %+v %v", v, e)
				}
				return
			}
			if tc == "scan" {
				o.MaxScanRecords = 1
			}
			if tc == "fit" {
				o.Vehicle.HeightM = floatp(99)
				o.WithinLimitsOnly = true
			}
			if tc == "occupancy" {
				o.AvailableOnly = true
			}
			v, e := c.Nearby(context.Background(), Coordinates{34.663534, 135.516310}, o)
			if e != nil {
				t.Fatal(e)
			}
			if tc == "scan" && (v.ScannedRecords != 1 || !v.ScanTruncated) {
				t.Fatal(v)
			}
			if tc == "fit" && (len(v.Results) != 0 || v.Note == "") {
				t.Fatal("mismatching fit filter produced a false match")
			}
			if tc == "occupancy" {
				for _, l := range v.Results {
					if l.Occupancy.Category != "available" && l.Occupancy.Category != "crowded" {
						t.Fatal("occupancy filter leak")
					}
				}
			}
		})
	}
}
