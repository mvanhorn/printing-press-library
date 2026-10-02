package yamato

import (
	"context"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/yamato/internal/cliutil"
	"golang.org/x/text/encoding/japanese"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParcelConsequentialBoundaries(t *testing.T) {
	for _, tt := range []struct {
		l, w, h, kg float64
		upright     bool
		size        int
		supported   bool
	}{{20, 20, 20, 2, false, 60, true}, {20, 20, 20, 2.01, false, 80, true}, {70, 45, 30, 23, false, 160, true}, {170, 15, 15, 30, false, 200, true}, {170.01, 10, 10, 30, false, 0, false}, {100.01, 20, 20, 20, true, 0, false}, {80, 70, 50, 30.01, false, 0, false}, {80, 70, 50, 30, false, 200, true}} {
		p, e := Classify(tt.l, tt.w, tt.h, tt.kg, tt.upright)
		if e != nil || p.ChargeableSize != tt.size || p.Supported != tt.supported {
			t.Fatalf("%+v → %+v %v", tt, p, e)
		}
	}
	if _, e := Classify(0, 10, 10, 1, false); e == nil {
		t.Fatal("zero measurement accepted")
	}
}
func TestSourceQuoteProductsAndFailClosed(t *testing.T) {
	for _, tt := range []struct {
		file, service, kind, date string
		cash                      int
	}{{"domestic-quote.html", "takkyubin", "ship", "2026-10-03", 3160}, {"airport-quote.html", "airport", "boarding", "2026-10-06", 3680}, {"airport-quote.html", "airport-roundtrip", "boarding", "2026-10-06", 6500}, {"roundtrip-quote.html", "roundtrip", "use", "2026-10-06", 6120}} {
		b, e := os.ReadFile("testdata/" + tt.file)
		if e != nil {
			t.Fatal(e)
		}
		b, e = japanese.ShiftJIS.NewDecoder().Bytes(b)
		if e != nil {
			t.Fatal(e)
		}
		in := QuoteInput{Service: tt.service, DateKind: tt.kind, Size: 160}
		q, e := ParseQuote(b, in, Quote{}, true)
		if e != nil || q.CalendarDate != tt.date || q.SelectedRate.CashJPY != tt.cash || len(q.Rates) != 8 {
			t.Fatalf("%s %+v %v", tt.service, q, e)
		}
		bad := []byte(strings.ReplaceAll(string(b), "160サイズ", "bogus"))
		if _, e := ParseQuote(bad, in, Quote{}, false); e == nil {
			t.Fatal("ambiguous category accepted")
		}
	}
}
func TestCalendarValidation(t *testing.T) {
	now := time.Date(2026, 10, 1, 16, 0, 0, 0, time.UTC)
	base := QuoteInput{Service: "takkyubin", Origin: "1000005", Destination: "6008216", Date: "2026-10-02", DateKind: "ship", Size: 160}
	if e := base.Validate(now); e != nil {
		t.Fatal(e)
	}
	for _, date := range []string{"2026-10-01", "2026-02-30", "2027-12-01"} {
		in := base
		in.Date = date
		if e := in.Validate(now); e == nil {
			t.Fatalf("invalid %s accepted", date)
		}
	}
	base.DateKind = "boarding"
	if e := base.Validate(now); e == nil {
		t.Fatal("product/date-kind mismatch accepted")
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(r *http.Request, status int, body []byte, contentType string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}
}
func TestThrottleAndBounds(t *testing.T) {
	c := NewClient(time.Second)
	c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		res := response(r, 429, nil, "text/plain")
		res.Header.Set("Retry-After", "5")
		return res, nil
	})
	_, e := c.Fetch(context.Background(), Main+"/test", nil, 1024)
	var throttle *cliutil.RateLimitError
	if !errors.As(e, &throttle) || throttle.RetryAfter != 5*time.Second {
		t.Fatalf("not typed throttle: %v", e)
	}
	if _, e := c.Fetch(context.Background(), "https://evil.invalid/test", nil, 1024); e == nil {
		t.Fatal("non-source URL accepted")
	}
	c = NewClient(time.Second)
	c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return response(r, 200, []byte("12345"), "text/plain"), nil
	})
	if _, e := c.Fetch(context.Background(), Main+"/test", nil, 4); e == nil {
		t.Fatal("oversize response accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := c.Fetch(ctx, Main+"/test", nil, 4); !errors.Is(e, context.Canceled) {
		t.Fatalf("cancellation lost: %v", e)
	}
}
func TestDirectoryRowspan(t *testing.T) {
	doc, e := Parse([]byte(`<table><tr><th>Prefecture</th><th>Airport</th><th>Service</th><th>Floor</th><th>Map</th></tr><tr><td rowspan="2">Chiba</td><td rowspan="2">Narita</td><td>Pickup</td><td>3F</td><td>map1</td></tr><tr><td>Send</td><td>1F</td><td>map2</td></tr></table>`))
	if e != nil {
		t.Fatal(e)
	}
	rows := expandedRows(Nodes(doc, "table")[0])
	if len(rows) != 3 || len(rows[2]) != 5 || Text(rows[2][0]) != "Chiba" || Text(rows[2][2]) != "Send" {
		t.Fatalf("rowspan attribution incorrect: %v", rows)
	}
}

func TestQuoteSessionAndProductSelection(t *testing.T) {
	for _, tt := range []struct {
		service, link, file, kind string
		cash                      int
	}{{"takkyubin", "TK", "domestic-quote.html", "ship", 3160}, {"roundtrip", "LT", "roundtrip-quote.html", "use", 6120}, {"airport", "KT", "airport-quote.html", "boarding", 3680}, {"airport-roundtrip", "KT", "airport-quote.html", "boarding", 6500}} {
		t.Run(tt.service, func(t *testing.T) {
			b, e := os.ReadFile("testdata/" + tt.file)
			if e != nil {
				t.Fatal(e)
			}
			c := NewClient(time.Second * 5)
			initial := false
			c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodGet {
					if r.URL.Query().Get("LINK") != tt.link {
						t.Errorf("wrong product initializer: %s", r.URL)
					}
					initial = true
					res := response(r, 200, []byte(`<select name="PARA_END"><option value="006">成田空港第2ターミナル</option></select>`), "text/html; charset=utf-8")
					res.Header.Add("Set-Cookie", "public_session=fixture-only; Path=/date/; Secure; HttpOnly")
					return res, nil
				}
				if !initial {
					t.Error("POST preceded initializer")
				}
				if cookie, e := r.Cookie("public_session"); e != nil || cookie.Value != "fixture-only" {
					t.Error("public session did not persist")
				}
				if e := r.ParseForm(); e != nil {
					t.Fatal(e)
				}
				if r.Form.Get("PARA_STA") != "1000005" {
					t.Error("origin lost")
				}
				if r.Form.Get("BTN_EXEC_SLEVEL") != string([]byte{0x8c, 0x9f, 0x8d, 0xf5}) {
					t.Error("submit must use source CP932 bytes")
				}
				return response(r, 200, b, "text/html; charset=Windows-31J"), nil
			})
			in := QuoteInput{Service: tt.service, Origin: "1000005", Destination: "6008216", Airport: "006", Date: "2026-10-08", DateKind: tt.kind, Size: 160}
			q, e := c.Quote(context.Background(), in, false)
			if e != nil || q.SelectedRate.CashJPY != tt.cash || c.Meta().Requests != 2 {
				t.Fatalf("%+v requests%d error%v", q, c.Meta().Requests, e)
			}
		})
	}
}
func TestPolicyTablesAndFreshnessFailures(t *testing.T) {
	for _, id := range []string{"takkyubin", "airport", "roundtrip", "same-day"} {
		b, e := os.ReadFile("testdata/" + id + "-policy.html")
		if e != nil {
			t.Fatal(e)
		}
		c := NewClient(time.Second)
		c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) { return response(r, 200, b, "text/html"), nil })
		p, e := c.Product(context.Background(), id)
		if e != nil || p.Title == "" || p.Acceptance == "" {
			t.Fatalf("%s: %+v %v", id, p, e)
		}
		if id != "same-day" && len(p.SizeLimits) != 8 {
			t.Fatalf("%s sizes: %+v", id, p.SizeLimits)
		}
	}
	pdf, e := os.ReadFile("testdata/same-day_delivery.pdf")
	if e != nil {
		t.Fatal(e)
	}
	page, e := os.ReadFile("testdata/same-day-policy.html")
	if e != nil {
		t.Fatal(e)
	}
	for _, changed := range []bool{false, true} {
		c := NewClient(time.Second * 3)
		c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == SameDayPDF {
				b := pdf
				if changed {
					b = []byte("%PDF-changed-source")
				}
				return response(r, 200, b, "application/pdf"), nil
			}
			return response(r, 200, page, "text/html"), nil
		})
		d, e := c.SameDay(context.Background(), "Narita", 10)
		if e != nil || len(d.FeeExamples) != 2 {
			t.Fatalf("same-day %+v %v", d, e)
		}
		if (!changed && len(d.Schedules) != 4) || (changed && len(d.Schedules) != 0) {
			t.Fatalf("freshness not enforced: %+v", d)
		}
	}
}
func TestCounterFreshnessAndEmptyCollections(t *testing.T) {
	body := []byte(`<table><tr><th>Prefecture</th><th>Airport</th><th>Service</th><th>Floor</th><th>Map</th></tr><tr><td>Chiba</td><td>Narita</td><td>Send</td><td>1F</td><td><a href="/ytc/en/send/services/airport/narita2_send.html">Map</a></td></tr></table>`)
	for _, query := range []string{"Narita", "no-such-counter"} {
		c := NewClient(time.Second * 3)
		c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == CounterList {
				return response(r, 200, body, "text/html"), nil
			}
			return response(r, 200, []byte("GIF89a-changed"), "image/gif"), nil
		})
		rows, total, e := c.Counters(context.Background(), query, 10, 0)
		if e != nil || rows == nil {
			t.Fatalf("empty must be []: %+v %v", rows, e)
		}
		if query == "Narita" && (total != 1 || rows[0].Hours != nil || !strings.Contains(rows[0].HoursStatus, "changed")) {
			t.Fatalf("stale image hours claimed: %+v", rows)
		}
	}
	c := NewClient(time.Second)
	c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return response(r, 200, []byte(`<html>source changed</html>`), "text/html"), nil
	})
	if _, _, e := c.Counters(context.Background(), "filtered", 10, 0); e == nil {
		t.Fatal("changed filtered directory mistaken for no matches")
	}
}
func TestPartialFailureAndRateLimitPropagation(t *testing.T) {
	pdf, e := os.ReadFile("testdata/same-day_delivery.pdf")
	if e != nil {
		t.Fatal(e)
	}
	for _, status := range []int{500, 429} {
		c := NewClient(time.Second * 3)
		c.HTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() == SameDayPDF {
				return response(r, 200, pdf, "application/pdf"), nil
			}
			return response(r, status, nil, "text/plain"), nil
		})
		d, e := c.SameDay(context.Background(), "Narita", 10)
		if status == 429 {
			var rate *cliutil.RateLimitError
			if !errors.As(e, &rate) {
				t.Fatalf("typed throttle swallowed: %v", e)
			}
		} else if e != nil || len(d.Schedules) != 4 || len(c.Meta().FetchFailures) != 1 {
			t.Fatalf("partial failure lost: %+v %v", d, e)
		}
	}
}
func TestNonFiniteMeasurementsAndRateColumnDrift(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.MaxFloat64} {
		if _, e := Classify(v, v, v, 1, false); e == nil {
			t.Fatalf("invalid measurement %v accepted", v)
		}
	}
	b, e := os.ReadFile("testdata/airport-quote.html")
	if e != nil {
		t.Fatal(e)
	}
	b, e = japanese.ShiftJIS.NewDecoder().Bytes(b)
	if e != nil {
		t.Fatal(e)
	}
	b = []byte(strings.ReplaceAll(string(b), "キャッシュレス決済", "changed-column"))
	if _, e := ParseQuote(b, QuoteInput{Service: "airport", DateKind: "boarding", Size: 160}, Quote{}, false); e == nil {
		t.Fatal(fmt.Sprint("changed price columns accepted"))
	}
}

func TestShippingDateCalendarsAndHelperContracts(t *testing.T) {
	for _, tt := range []struct{ file, service, kind string }{{"airport-ship-quote.html", "airport", "earliest_boarding_date"}, {"roundtrip-ship-quote.html", "roundtrip", "earliest_use_date"}} {
		b, e := os.ReadFile("testdata/" + tt.file)
		if e != nil {
			t.Fatal(e)
		}
		b, e = japanese.ShiftJIS.NewDecoder().Bytes(b)
		if e != nil {
			t.Fatal(e)
		}
		q, e := ParseQuote(b, QuoteInput{Service: tt.service, DateKind: "ship", Size: 160}, Quote{}, false)
		if e != nil || q.CalendarDate != "2026-10-04" || q.CalendarKind != tt.kind {
			t.Fatalf("%s: %+v %v", tt.file, q, e)
		}
	}
	for _, tt := range []struct {
		size  int
		valid bool
	}{{60, true}, {160, true}, {200, true}, {180, true}, {0, false}, {59, false}, {161, false}} {
		if ValidSize(tt.size) != tt.valid {
			t.Fatalf("size%d", tt.size)
		}
	}
	for _, tt := range []struct {
		url   string
		valid bool
	}{{Main + "/test", true}, {Date + "MainSmp?LINK=KT", true}, {"http://www.kuronekoyamato.co.jp/", false}, {"https://www.kuronekoyamato.co.jp.evil.test/", false}, {"https://evil.test/", false}} {
		if AllowedURL(tt.url) != tt.valid {
			t.Fatalf("URL %s", tt.url)
		}
	}
	for _, tt := range []struct {
		q, value string
		matched  bool
	}{{"NARITA", "Narita terminal2", true}, {"成田", "成田空港", true}, {"absent", "Narita", false}, {"", "Anything", true}} {
		if Matches(tt.q, tt.value) != tt.matched {
			t.Fatalf("match %+v", tt)
		}
	}
	doc, e := Parse([]byte(`<table id="rates"><tr><th>Size</th><th>Cash</th></tr><tr><td>160</td><td>3680<script>ignore me</script></td></tr></table>`))
	if e != nil {
		t.Fatal(e)
	}
	tables := Nodes(doc, "table")
	if len(tables) != 1 || Attr(tables[0], "id") != "rates" || len(Rows(tables[0])) != 2 || Text(tables[0]) != "Size Cash 160 3680" {
		t.Fatal("HTML helper contract changed")
	}
}

func TestDecimalDimensionBoundaries(t *testing.T) {
	for _, tt := range []struct {
		l, w, h   float64
		size      int
		supported bool
	}{{63.2, 64.9, 31.9, 160, true}, {63.2, 64.9, 31.90000000000001, 180, true}, {63.2, 64.9, 71.9, 200, true}, {63.2, 64.9, 71.90000000000002, 0, false}, {20.1, 20.2, 19.7, 60, true}, {100, 99.99999999999997, 0.00000000000004, 0, false}} {
		p, e := Classify(tt.l, tt.w, tt.h, 1, false)
		if e != nil || p.ChargeableSize != tt.size || p.Supported != tt.supported {
			t.Fatalf("%+v → %+v %v", tt, p, e)
		}
	}
}
