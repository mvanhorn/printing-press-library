package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eplus/internal/cliutil"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDateAndOvernight(t *testing.T) {
	for _, tc := range []struct{ date, clock, want string }{{"2026-11-07", "25:30", "2026-11-08T01:30:00+09:00"}, {"2026-11-07", "2400", "2026-11-08T00:00:00+09:00"}, {"2026-11-07", "9:30", "2026-11-07T09:30:00+09:00"}, {"2026-11-07", "23:59", "2026-11-07T23:59:00+09:00"}, {"2026-11-07", "12:-1", ""}, {"2026-02-30", "1200", ""}, {"2026-11-07", "48:00", ""}} {
		if got := str(at(tc.date, tc.clock)); got != tc.want {
			t.Errorf("at(%s,%s)=%s want %s", tc.date, tc.clock, got, tc.want)
		}
	}
	for _, tc := range []struct{ in, want string }{{"2026/11/7(土)", "2026-11-07"}, {"November 7, 2026 (Sat.)", "2026-11-07"}, {"20260230", ""}} {
		if parseDate(tc.in) != tc.want {
			t.Error(tc)
		}
	}
	a, b := window("受付期間:2026/9/28(月)10:00～2026/10/12(月・祝)23:59")
	if a != "2026-09-28T10:00:00+09:00" || b != "2026-10-12T23:59:00+09:00" {
		t.Fatal(a, b)
	}
	a, b = window("Sales Period: April 27, 2026 (Mon.) 20:00(JST) - December 12, 2026 (Sat.) 21:00(JST)")
	if a != "2026-04-27T20:00:00+09:00" || b != "2026-12-12T21:00:00+09:00" {
		t.Fatal(a, b)
	}
	for _, s := range []string{"2026-02-30", "20261107", "2026/11/07"} {
		if ValidateDate(s) == nil {
			t.Fatalf("accepted invalid flag date %s", s)
		}
	}
	if ValidateDate("2026-11-07") != nil {
		t.Fatal("valid date rejected")
	}
}
func TestSaleRoundSemantics(t *testing.T) {
	for _, tc := range []struct{ label, code, kind, status, inventory string }{{"", "0", "lottery", "accepting", "unknown"}, {"予定枚数終了", "", "lottery", "accepting", "unknown"}, {"", "0", "first_come", "accepting", "available"}, {"受付中", "", "request", "accepting", "unknown"}, {"", "1", "first_come", "sold_out", "unavailable"}, {"", "5", "lottery", "closed", "unknown"}, {"", "3", "first_come", "upcoming", "unknown"}, {"", "99", "unknown", "unknown", "unknown"}} {
		s, i := saleState(tc.label, tc.code, tc.kind, false, true)
		if s != tc.status || i != tc.inventory {
			t.Error(tc, s, i)
		}
	}
	for _, tc := range []struct{ label, code, want string }{{"プレオーダー", "", "lottery"}, {"", "000", "lottery"}, {"先着", "", "first_come"}, {"リクエスト", "", "request"}, {"", "XYZ", "unknown"}} {
		if saleKind(tc.label, tc.code) != tc.want {
			t.Error(tc)
		}
	}
	s := newSale("one", "オフィシャル先行", "lottery", "accepting", "unknown", nil, "2026-10-12T23:59:00+09:00", nil)
	if s["phase"] != "presale" || s["lottery_deadline"] != s["ends_at"] {
		t.Fatal(s)
	}
}
func TestCanonicalURLs(t *testing.T) {
	for _, tc := range []struct {
		in string
		ok bool
	}{{"4592490001-P0030001P021003", true}, {"https://eplus.jp/sf/detail/4592490001", true}, {"https://eplus.jp/sf/detail/1234567890?bad=1", false}, {"https://evil.example/sf/detail/1234567890", false}, {"../1234567890", false}, {"x", false}} {
		_, e := DomesticURL(tc.in)
		if (e == nil) != tc.ok {
			t.Error(tc, e)
		}
	}
	for _, tc := range []struct {
		in string
		ok bool
	}{{"7078", true}, {"tokinosora-live-st", true}, {"https://ib.eplus.jp/index.php?dispatch=products.view&product_id=7078&date=0#scrollToPanel", true}, {"https://ib.eplus.jp/index.php?dispatch=checkout.clear", false}, {"checkout", false}, {"https://ib.eplus.jp.evil.example/thing", false}, {"https://user@ib.eplus.jp/tour", false}} {
		_, e := InternationalURL(tc.in)
		if (e == nil) != tc.ok {
			t.Error(tc, e)
		}
	}
}
func TestSearchValidationAndFilters(t *testing.T) {
	base := SearchOptions{Limit: 2, Pages: 1, Page: 1, From: "2026-11-01", To: "2026-11-30", Region: "kanto", Category: "theatre"}
	if ValidateInternationalSearch(SearchOptions{Limit: 1, Category: "concert"}, 5) != nil {
		t.Fatal("valid international search rejected")
	}
	if ValidateInternationalSearch(SearchOptions{Limit: 1, Category: "bad"}, 5) == nil {
		t.Fatal("invalid international category accepted")
	}
	if ValidateSearch(base) != nil {
		t.Fatal("valid search rejected")
	}
	q := domesticParams(base)
	if q.Get("p_genre_filter") != "400" || q.Get("chiho_filter") != "05" || q.Get("koen_to_filter") != "20261130" {
		t.Fatal(q)
	}
	for _, mut := range []func(*SearchOptions){func(o *SearchOptions) { o.Limit = 101 }, func(o *SearchOptions) { o.Pages = 6 }, func(o *SearchOptions) { o.From = "2026-12-01" }, func(o *SearchOptions) { o.Category = "made-up" }} {
		o := base
		mut(&o)
		if ValidateSearch(o) == nil {
			t.Fatal(o)
		}
	}
	row := Row{"date": "2026-10-31", "date_end": "2026-11-02", "venue": Row{"name": "東京劇場"}, "region": "東京都"}
	if !matches(row, base) {
		t.Fatal("overlap must match service-date range")
	}
	base.Venue = "大阪"
	if matches(row, base) {
		t.Fatal("wrong venue matched")
	}
}

const domesticFixture = `<h1>公演日本語</h1><article class="block-ticket-article"><span class="block-ticket-article__date">2026/11/7</span><span class="block-ticket-article__time">開演：25:30 (開場 24:00)</span><a class="block-ticket-article__place" href="/sf/venue/123"><span class="block-ticket-article__venue">東京劇場</span></a><section class="block-ticket"><span class="label-ticket">抽選</span><h3 class="block-ticket__title">先行受付</h3><span class="ticket-status">受付中</span><div class="block-ticket__time">2026/9/28(月)10:00～2026/10/12(月)23:59</div><button onclick="go('https://sp.atom.eplus.jp/sys/main.jsp?prm=P3=0001:P21=003:P7=2:P6=001')"></button></section><section class="block-ticket"><span class="label-ticket">先着</span><h3 class="block-ticket__title">一般発売</h3><span class="ticket-status">空席あり 受付中</span><button onclick="go('https://sp.atom.eplus.jp/sys/main.jsp?prm=P3=0001:P21=003:P7=4:P6=001')"></button></section></article>`

func TestDomesticDetailIdentity(t *testing.T) {
	n, _ := document([]byte(domesticFixture))
	rows, e := parseDomesticDetail(n, "https://eplus.jp/sf/detail/4592490001")
	if e != nil {
		t.Fatal(e)
	}
	r := rows[0]
	if r["id"] != "4592490001/0001/003" || r["date"] != "2026-11-07" || r["start_at"] != "2026-11-08T01:30:00+09:00" {
		t.Fatal(r)
	}
	sales := r["sales"].([]Row)
	if len(sales) != 2 || sales[0]["id"] == sales[1]["id"] || sales[0]["inventory"] != "unknown" || sales[1]["inventory"] != "available" {
		t.Fatal(sales)
	}
	if sales[0]["lottery_deadline"] != "2026-10-12T23:59:00+09:00" {
		t.Fatal(sales)
	}
	reversed := strings.Replace(domesticFixture, "P7=2", "P7=99", 1)
	n, _ = document([]byte(reversed))
	rows, _ = parseDomesticDetail(n, "https://eplus.jp/sf/detail/4592490001")
	if rows[0]["id"] != r["id"] {
		t.Fatal("sale identity changed performance identity")
	}
}
func TestDomesticRecordIdentity(t *testing.T) {
	r := domesticRecord(Row{"kogyo_code": "459249", "kogyo_sub_code": "0001", "koen_code": "003", "koen_detail_url_pc": "/sf/detail/4592490001-P0030001P021003", "koenbi_term": "20261107", "kaien_time": "2530", "kanren_kogyo_sub": Row{"kogyo_name_1": "日本語"}, "kanren_uketsuke_koen_list": []any{Row{"uketsuke_info_code": "022", "hambai_hoho_kubun": "000", "uketsuke_status": "0", "eplus_toriatsukai_ari_flag": true, "uketsuke_end_datetime": "20261012235900"}}})
	if r["id"] != "4592490001/0001/003" || r["start_at"] != "2026-11-08T01:30:00+09:00" {
		t.Fatal(r)
	}
	if r["sales"].([]Row)[0]["inventory"] != "unknown" {
		t.Fatal(r)
	}
}

const catalogFixture = `<div class="top-product-list"><div class="col-product"><a href="/tour"><h2>日本語 artist</h2></a><div class="ga-product-click" data-id="2837" data-name="日本語 artist" data-category="Concert"></div></div></div>`
const tourFixture = `<h1>日本語 tour</h1><div class="group-schedule-title">STREAMING</div><div class="group-schedule-row"><div class="group-schedule-date">Sunlight Stage</div><div class="group-schedule-name">Streaming+</div><div class="group-schedule-mute">from November 7, 2026 - to December 12, 2026</div><a class="group-schedule-btn" href="/index.php?dispatch=products.view&amp;product_id=7078&amp;date=0">BUY NOW</a></div>`
const productFixture = `<h1 class="ty-product-block-title">Sunlight Stage 日本語</h1><div class="ty-price">JPY 0</div><div class="ty-product-block__note">Sales Period: April 27, 2026 (Mon.) 20:00(JST) - December 12, 2026 (Sat.) 21:00(JST)</div><div id="content_description">Start Time: November 7, 2026 (Sat.) 14:00 * Only specified countries supported. Handling charge included.</div><div id="agreement-content-description">Overseas residents must bring passport.</div><input id="only_seat_type" data-combination-id="23046_810422_23047_810423">`

func TestInternationalParsing(t *testing.T) {
	n, _ := document([]byte(catalogFixture))
	rows, e := parseCatalog(n, "https://ib.eplus.jp/concert")
	if e != nil || len(rows) != 1 || rows[0]["name"] != "日本語 artist" || rows[0]["date"] != nil {
		t.Fatal(rows, e)
	}
	n, _ = document([]byte(tourFixture))
	rows, e = parseTour(n, "https://ib.eplus.jp/tour")
	if e != nil || rows[0]["id"] != "ib:product:7078/date/0" || rows[0]["date_end"] != "2026-12-12" || rows[0]["inventory"] != "unknown" {
		t.Fatal(rows, e)
	}
	n, _ = document([]byte(productFixture))
	r, comb, e := parseProduct(n, productURL("7078", "0"))
	if e != nil || comb == "" || r["start_at"] != "2026-11-07T14:00:00+09:00" {
		t.Fatal(r, comb, e)
	}
	if len(r["tickets"].([]Row)) != 0 {
		t.Fatal("static zero placeholder became price")
	}
	if r["overseas_bookability"] != "conditional" || len(r["eligibility"].([]string)) != 1 {
		t.Fatal(r)
	}
	old := timeNow
	timeNow = func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }
	defer func() { timeNow = old }()
	if e = applyTicketData(r, []byte(`{"data":{"status":true,"price":"7,300","currency_code":"JPY","amount":1,"start_selling":"2026/04/27 20:00:00","end_selling":"2026/12/12 21:00:00"}}`), comb); e != nil {
		t.Fatal(e)
	}
	if r["tickets"].([]Row)[0]["price"] != float64(7300) || r["inventory"] != "available" {
		t.Fatal(r)
	}
	r["sales"].([]Row)[0]["status"] = "closed"
	r["sales"].([]Row)[0]["inventory"] = "unknown"
	if e = applyTicketData(r, []byte(`{"data":{"status":true,"price":"7,300","currency_code":"JPY","amount":1}}`), comb); e != nil || r["inventory"] != "unknown" {
		t.Fatal("closed sales must override variant amount", r, e)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fakeClient(t *testing.T, fn func(*http.Request) (string, int)) *Client {
	t.Helper()
	c := NewClient(t.TempDir(), false, time.Second)
	c.limiter = nil
	c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" {
			t.Fatal("mutation", r.Method)
		}
		body, status := fn(r)
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	return c
}

func TestVariantDatesPreserveKnownPageWindow(t *testing.T) {
	start, end := "2026-09-01T10:00:00+09:00", "2026-10-12T23:59:00+09:00"
	for _, tc := range []struct{ name, fields, wantStart, wantEnd string }{
		{"missing", "", start, end},
		{"invalid", `,"start_selling":"unknown","end_selling":""`, start, end},
		{"one replacement", `,"end_selling":"2026/10/13 23:59:00"`, start, "2026-10-13T23:59:00+09:00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sale := newSale("product/sale", "sale", "first_come", "unknown", "unknown", start, end, nil)
			r := Row{"sales": []Row{sale}, "tickets": []Row{}}
			body := `{"data":{"status":true,"price":"7300","currency_code":"JPY","amount":1` + tc.fields + `}}`
			if err := applyTicketData(r, []byte(body), "1_2"); err != nil {
				t.Fatal(err)
			}
			if sale["starts_at"] != tc.wantStart || sale["ends_at"] != tc.wantEnd {
				t.Fatalf("known sale window lost: %v", sale)
			}
		})
	}
}

func TestInternationalPartialFailuresWarn(t *testing.T) {
	const catalog = `<div class="top-product-list"><div class="col-product"><a class="ga-product-click" data-name="First" href="https://ib.eplus.jp/first-tour">First</a></div><div class="col-product"><a class="ga-product-click" data-name="Second" href="https://ib.eplus.jp/second-tour">Second</a></div></div>`
	c := fakeClient(t, func(req *http.Request) (string, int) {
		switch req.URL.Path {
		case "/":
			return catalog, 200
		case "/first-tour":
			return "unavailable", 403
		default:
			return tourFixture, 200
		}
	})
	r, err := c.InternationalSearch(context.Background(), SearchOptions{Limit: 10, From: "2026-01-01"}, 5)
	if err != nil || len(r.Data) == 0 || r.Meta["partial"] != true || len(r.Meta["fetch_failures"].([]Row)) != 1 {
		t.Fatalf("partial results/failures lost: %v, %v", r, err)
	}
	if !strings.Contains(strings.Join(r.Meta["warnings"].([]string), " "), "1 of 2 international detail reads failed") {
		t.Fatalf("partial failure diagnostic missing: %v", r.Meta)
	}
}
func TestFetchCacheRetryAndSafety(t *testing.T) {
	calls := 0
	c := fakeClient(t, func(*http.Request) (string, int) { calls++; return "<h1>safe</h1>", 200 })
	for i := 0; i < 2; i++ {
		_, o, e := c.Fetch(context.Background(), "https://ib.eplus.jp/concert")
		if e != nil || o.Cached != (i == 1) || o.FetchedAt == "" {
			t.Fatal(o, e)
		}
	}
	if calls != 1 || c.Stats().Requests != 1 || c.Stats().CacheHits != 1 {
		t.Fatal(c.Stats(), calls)
	}
	c.Fresh = true
	_, _, e := c.Fetch(context.Background(), "https://ib.eplus.jp/concert")
	if e != nil || calls != 2 {
		t.Fatal(e, calls)
	}
	c.SetRateLimit(1)
	if c.limiter == nil {
		t.Fatal("limiter absent")
	}
	c.limiter = nil
	if _, _, e = c.Fetch(context.Background(), "http://ib.eplus.jp/concert"); e == nil {
		t.Fatal("unsafe URL accepted")
	}
	calls = 0
	c = fakeClient(t, func(*http.Request) (string, int) { calls++; return "", 429 })
	_, _, e = c.Fetch(context.Background(), "https://ib.eplus.jp/concert")
	var rate *cliutil.RateLimitError
	if !errors.As(e, &rate) || calls != 3 {
		t.Fatal(e, calls)
	}
	c = fakeClient(t, func(*http.Request) (string, int) { return strings.Repeat("a", MaxBody+1), 200 })
	if _, _, e = c.Fetch(context.Background(), "https://ib.eplus.jp/concert"); e == nil {
		t.Fatal("oversize body accepted")
	}
	c = fakeClient(t, func(*http.Request) (string, int) {
		return "var APIV3_TOKEN = {'X-APIToken':'private'};_.security_hash = 'csrf';", 200
	})
	_, _, _ = c.Fetch(context.Background(), "https://ib.eplus.jp/concert")
	fs, _ := os.ReadDir(c.CacheDir)
	b, _ := os.ReadFile(filepath.Join(c.CacheDir, fs[0].Name()))
	if strings.Contains(string(b), "private") || strings.Contains(string(b), "csrf") {
		t.Fatal("ephemeral token persisted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if pause(ctx, time.Second) == nil {
		t.Fatal("cancel ignored")
	}
}
func TestClientsUseRealReadContracts(t *testing.T) {
	record := Row{"kogyo_code": "459249", "kogyo_sub_code": "0001", "koen_code": "003", "koen_detail_url_pc": "/sf/detail/4592490001-P0030001P021003", "koenbi_term": "20261107"}
	search, _ := json.Marshal(Row{"data": Row{"so_kensu": 1, "record_list": []Row{record}}})
	for _, tc := range []struct {
		name string
		run  func(*Client) (Result, error)
		body string
		want int
	}{{"domestic search", func(c *Client) (Result, error) {
		return c.DomesticSearch(context.Background(), SearchOptions{Limit: 2, Pages: 1, Page: 1})
	}, `<script id="json">` + string(search) + `</script>`, 1}, {"domestic detail", func(c *Client) (Result, error) { return c.DomesticDetail(context.Background(), "4592490001") }, domesticFixture, 1}, {"international search", func(c *Client) (Result, error) {
		return c.InternationalSearch(context.Background(), SearchOptions{Limit: 2}, 5)
	}, catalogFixture, 1}, {"international detail", func(c *Client) (Result, error) { return c.InternationalDetail(context.Background(), "tour") }, tourFixture, 1}, {"policies", func(c *Client) (Result, error) { return c.Policies(context.Background()) }, `<div class="ty-wysiwyg-content">Q: What payment methods? A: VISA for overseas residents Q: I am Japanese? A: overseas residency Q: Any fee other than price? A: included Q: How to enter the venue? A: confirmation email</div>`, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			c := fakeClient(t, func(*http.Request) (string, int) { return tc.body, 200 })
			r, e := tc.run(c)
			if e != nil || len(r.Data) != tc.want {
				t.Fatal(r, e)
			}
		})
	}
}

func TestChildPerformanceCanonicalParent(t *testing.T) {
	r := domesticRecord(Row{"kogyo_code": "051089", "kogyo_sub_code": "1669", "koen_code": "001", "koen_detail_url_pc": "/sf/detail/0510890001-P0031669P021001"})
	if r["event_id"] != "0510890001" || r["id"] != "0510890001/1669/001" || r["source_sub_code"] != "1669" {
		t.Fatal(r)
	}
}
func TestRealInternationalOptionMarkup(t *testing.T) {
	body := productFixture + `<div class="ty-product-options__item"><label class="ty-control-group__label">Venue:</label>Streaming+<input type="hidden" value="810422"></div><div class="ty-product-options__item"><label class="ty-control-group__label">Ticket type:</label><span>Virtual Ticket</span><input type="hidden" value="810423"></div>`
	n, _ := document([]byte(body))
	r, _, e := parseProduct(n, productURL("7078", "0"))
	if e != nil || obj(r["venue"])["name"] != "Streaming+" || ticketName(r) != "Virtual Ticket" {
		t.Fatal(r, e)
	}
}
func TestRedirectsCountAndPace(t *testing.T) {
	c := NewClient("", true, 3*time.Second)
	c.SetRateLimit(2)
	times := []time.Time{}
	c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		times = append(times, time.Now())
		h := make(http.Header)
		status := 200
		if len(times) == 1 {
			status = 302
			h.Set("Location", "https://ib.eplus.jp/concert")
		}
		return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader("public")), Request: r}, nil
	})
	_, _, e := c.Fetch(context.Background(), "https://ib.eplus.jp/")
	if e != nil || c.Stats().Requests != 2 || len(times) != 2 || times[1].Sub(times[0]) < 450*time.Millisecond {
		t.Fatal(c.Stats(), times, e)
	}
	c.limiter = nil
	c.stats.Requests = 31
	times = nil
	_, _, e = c.Fetch(context.Background(), "https://ib.eplus.jp/")
	if e == nil || len(times) != 1 || c.Stats().Requests != 32 {
		t.Fatal("redirect exceeded budget", c.Stats(), times, e)
	}
}
func TestInternationalPriceThrottleRemainsTyped(t *testing.T) {
	c := fakeClient(t, func(r *http.Request) (string, int) {
		if r.URL.Query().Get("dispatch") == "products.get_ticket_type" {
			return "", 429
		}
		return productFixture, 200
	})
	_, e := c.InternationalDetail(context.Background(), "7078")
	var rate *cliutil.RateLimitError
	if !errors.As(e, &rate) {
		t.Fatal(e)
	}
}

func TestExplicitClosedAccessLabels(t *testing.T) {
	for _, tc := range []struct{ label, status, inventory string }{{"休演", "cancelled", "unavailable"}, {"公演中止", "cancelled", "unavailable"}, {"扱いなし", "not_handled", "unknown"}} {
		s, i := saleState(tc.label, "", "lottery", false, true)
		if s != tc.status || i != tc.inventory {
			t.Fatal(tc, s, i)
		}
		n, _ := document([]byte(strings.Replace(domesticFixture, "受付中", tc.label, 1)))
		rows, e := parseDomesticDetail(n, "https://eplus.jp/sf/detail/4592490001")
		if e != nil || rows[0]["sales"].([]Row)[0]["status"] != tc.status {
			t.Fatal(rows, e)
		}
	}
}

func TestAmbiguousSameDayJSONLDIsNotDuplicated(t *testing.T) {
	article := strings.Replace(domesticFixture, "開演：25:30 (開場 24:00)", "", 1)
	article = strings.ReplaceAll(article, "https://sp.atom.eplus.jp/sys/main.jsp", "https://other.invalid")
	first := strings.Replace(article, `class="block-ticket-article"`, `class="block-ticket-article 20261107-start1200"`, 1)
	second := strings.Replace(article, `class="block-ticket-article"`, `class="block-ticket-article 20261107-start1300"`, 1)
	ld := `<script type="application/ld+json">{"startDate":"2026-11-07T12:00:00+09:00","url":"https://eplus.jp/sf/detail/4592490001-P0030001P021001","location":{"name":"東京劇場"}}</script><script type="application/ld+json">{"startDate":"2026-11-07T13:00:00+09:00","url":"https://eplus.jp/sf/detail/4592490001-P0030001P021002","location":{"name":"東京劇場"}}</script>`
	n, _ := document([]byte(first + second + ld))
	rows, e := parseDomesticDetail(n, "https://eplus.jp/sf/detail/4592490001")
	if e != nil || len(rows) != 2 || rows[0]["id"] == rows[1]["id"] || rows[0]["identity_ambiguous"] != true {
		t.Fatal(rows, e)
	}
	n, _ = document([]byte(first + first + ld))
	if _, e = parseDomesticDetail(n, "https://eplus.jp/sf/detail/4592490001"); e == nil {
		t.Fatal("indistinguishable sessions were merged")
	}
}
