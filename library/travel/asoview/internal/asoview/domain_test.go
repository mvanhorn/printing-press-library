package asoview

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSourceIdentityAndReadOnlyPaths(t *testing.T) {
	for _, s := range []string{"ticket0000049223", "pln3000044589", Origin + "/item/ticket/ticket0000049223/"} {
		if _, _, err := ParseID(s); err != nil {
			t.Fatal(s, err)
		}
	}
	for _, s := range []string{"ticket1", Origin + "/item/activity/ticket0000049223/", "https://evil.example/item/ticket/ticket0000049223/", Origin + "/item/ticket/ticket0000049223/?x=1"} {
		if _, _, err := ParseID(s); err == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	for _, s := range []string{"https://www.asoview.com.evil.example/search/", Origin + "/purchase/", Origin + "/book/ticket/ticket0000049223/select", Origin + "/api/member", Origin + "/item/ticket/../../purchase/", "http://www.asoview.com/search/"} {
		u, _ := url.Parse(s)
		if err := validateURL(u); err == nil {
			t.Fatalf("unsafe source path %s", s)
		}
	}
}
func TestAgeEvidence(t *testing.T) {
	cases := []struct {
		label    string
		min, max any
	}{{"参加者 10〜85歳", 10, 85}, {"10歳〜80歳", 10, 80}, {"65歳以上 ※証明必要", 65, nil}, {"4歳から", 4, nil}, {"3歳以下", nil, 3}, {"中学生 ※都内在学は無料", nil, nil}, {"", nil, nil}}
	for _, c := range cases {
		a := AgeBand(c.label)
		if a["minimum_years"] != c.min || a["maximum_years"] != c.max {
			t.Errorf("%s: %v", c.label, a)
		}
	}
}
func TestCalendarAndSlotStates(t *testing.T) {
	if s := calendarStatus(Object{"isRemaining": true, "stockStatus": "NOT_OPEN"}); s != "available" {
		t.Fatal("calendar boolean must override legacy raw status", s)
	}
	if s := calendarStatus(Object{"isRemaining": true, "isOutOfPeriod": true}); s != "outside_sales_or_operation_period" {
		t.Fatal(s)
	}
	cases := []struct {
		s    Object
		q    int
		want string
	}{{Object{"isClosed": true, "remainReserveNumber": json.Number("100")}, 1, "closed"}, {Object{"remainReserveNumber": json.Number("0"), "isRequest": true}, 1, "request_only"}, {Object{"remainReserveNumber": json.Number("0")}, 1, "sold_out"}, {Object{"remainReserveNumber": json.Number("1")}, 2, "insufficient_quantity"}, {Object{"remainReserveNumber": json.Number("50"), "maximumReservableQuantity": json.Number("5")}, 6, "party_outside_limits"}, {Object{"remainReserveNumber": json.Number("0"), "canRequestAfterFull": true}, 2, "request_only"}, {Object{}, 1, "unknown"}}
	for _, c := range cases {
		if s := slotStatus(c.s, c.q); s != c.want {
			t.Errorf("%v: got %s want %s", c.s, s, c.want)
		}
	}
	p := Object{"kind": "ticket", "name_ja": "日付指定WEBチケット"}
	if timeMeaning(p, Object{}) != "admission_window_on_selected_date" {
		t.Fatal("opening hours became reserved entry time")
	}
	p["name_ja"] = "日時指定チケット"
	if timeMeaning(p, Object{"timeTicketScheduleId": json.Number("10")}) != "reserved_entry_window" {
		t.Fatal("timed ticket classification lost")
	}
}
func TestDatedSubtotalNotQuote(t *testing.T) {
	rows := []any{Object{"basicFeeNumber": json.Number("12"), "feeLabel": "一般", "sellingFee": json.Number("700"), "unit": "枚", "allocation": json.Number("1")}, Object{"basicFeeNumber": json.Number("13"), "feeLabel": "中学生", "sellingFee": json.Number("250"), "unit": "枚", "allocation": json.Number("1")}}
	bands, err := optionsFrom(rows, true, "2026-10-02")
	if err != nil {
		t.Fatal(err)
	}
	out, err := PartySubtotal(bands, "12:2,13:1")
	if err != nil || out["amount"] != int64(1650) || out["quote_confirmed"] != false {
		t.Fatal(out, err)
	}
	for _, bad := range []string{"12:2,12:1", "999:1", "12:0", "12:51", "12:50,13:1"} {
		if _, err = PartySubtotal(bands, bad); err == nil {
			t.Fatalf("accepted invalid party %s", bad)
		}
	}
	bands[0]["allocation"] = nil
	object(bands[0]["price"])["unit"] = nil
	if _, err = PartySubtotal(bands, "12:1"); err == nil {
		t.Fatal("guessed missing unit")
	}
}

// These responses are deterministic test doubles, never live source proof.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fakeResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}
func TestCacheTTLAndNoStaleFallback(t *testing.T) {
	calls := 0
	c := NewClient(t.TempDir(), time.Second, false, false, false)
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) { calls++; return fakeResponse(200, `{"ok":true}`), nil })
	ctx := context.Background()
	if _, err := c.Get(ctx, "/search/", nil, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(ctx, "/search/", nil, time.Hour); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || c.Stats.CacheHits != 1 {
		t.Fatal(calls, c.Stats)
	}
	c.Offline = true
	if _, err := c.Get(ctx, "/search/", nil, -time.Second); err == nil {
		t.Fatal("stale cache served without warning")
	}
	if calls != 1 {
		t.Fatal("offline requested network")
	}
}
func TestTypedRateLimitAndBoundedRetry(t *testing.T) {
	calls := 0
	c := NewClient("", time.Second, true, false, false)
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		response := fakeResponse(429, `{"error":"slow"}`)
		response.Header.Set("Retry-After", "0")
		return response, nil
	})
	_, err := c.Get(context.Background(), "/search/", nil, 0)
	var e *Error
	if !errors.As(err, &e) || e.Code != 7 || calls != 2 {
		t.Fatal(calls, err)
	}
}
func TestProjectionSourceParsing(t *testing.T) {
	raw := []byte(`<html><script>var ASOVIEW_DATASOURCE = {"ticketType":{"code":"ticket0000049223","title":"日付指定"}};</script></html>`)
	ds, _, err := datasource(raw)
	if err != nil || text(object(ds["ticketType"])["code"]) != "ticket0000049223" {
		t.Fatal(ds, err)
	}
	if _, _, err = datasource([]byte(`<html>Login challenge</html>`)); err == nil {
		t.Fatal("shell accepted as data")
	}
}

func TestRecommendationsNeverBecomeFilteredMatches(t *testing.T) {
	raw := []byte(`<html><script>var ASOVIEW_DATASOURCE = {"displayFilterTotalCount":"1","caughtBasePlanPriceList":[{"goodsId":"ticket0000049223","sellingPrice":700}],"raiseBasePlanPriceList":[{"goodsId":"ticket0000012233","sellingPrice":900}]};</script><ul class="search-result-list"><li class="search-result-list__item"><a class="search-result-list__plan-link" href="/item/ticket/ticket0000049223/"><b class="search-result-list__plan-name">Match</b></a></li></ul><ul class="search-result-list"><li class="search-result-list__item"><a class="search-result-list__plan-link" href="/item/ticket/ticket0000012233/"><b class="search-result-list__plan-name">Unrelated recommendation</b></a></li></ul><a href="/search/?page=2">2</a></html>`)
	ds, doc, err := datasource(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows, next := parseCards(doc, ds, 1)
	if len(rows) != 1 || rows[0]["id"] != "ticket0000049223" || next {
		t.Fatal("recommendations leaked or source count fabricated continuation", rows, next)
	}
}

func TestRedirectWireBudgetAndPacing(t *testing.T) {
	c := NewClient("", 2*time.Second, true, false, false)
	attempts := 0
	c.HTTP.Transport = ReadOnlyTransport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		resp := fakeResponse(200, `{"ok":true}`)
		if r.URL.Query().Get("hop") == "" {
			resp.StatusCode = 302
			resp.Header.Set("Location", Origin+"/search/?hop=1")
		}
		return resp, nil
	}), BeforeRequest: c.recordAttempt}
	start := time.Now()
	if _, err := c.Get(context.Background(), "/search/", nil, 0); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || c.Stats.Requests != 2 || time.Since(start) < 280*time.Millisecond {
		t.Fatal("wire attempts or pacing bypassed", attempts, c.Stats, time.Since(start))
	}
	c.Stats.Requests = 20
	if _, err := c.Get(context.Background(), "/search/", nil, 0); err == nil {
		t.Fatal("wire budget bypassed")
	}
}
func TestAllSourceTransportsBoundBodies(t *testing.T) {
	tr := ReadOnlyTransport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return fakeResponse(200, strings.Repeat("x", maxBody+1)), nil
	})}
	req, _ := http.NewRequest(http.MethodGet, Origin+"/search/", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if _, err = io.ReadAll(resp.Body); err == nil || !strings.Contains(err.Error(), "4 MiB") {
		t.Fatal("body cap absent", err)
	}
}
func TestMalformedBandsFailClosed(t *testing.T) {
	for _, row := range []Object{{}, {"basicFeeNumber": json.Number("1")}, {"basicFeeNumber": json.Number("1"), "feeLabel": "Adult", "sellingFee": "unknown"}} {
		if _, err := optionsFrom([]any{row}, true, "2026-10-02"); err == nil {
			t.Fatal("malformed band accepted", row)
		}
	}
}
func TestRefreshedInventoryCarriesProvenance(t *testing.T) {
	c := NewClient(t.TempDir(), time.Second, false, false, true)
	inv := Object{"categories": []any{Object{"id": "192", "name_ja": "水族館", "type": "category"}}, "fetched_at": "2026-09-30T12:00:00Z"}
	if err := c.persistInventory("categories", inv); err != nil {
		t.Fatal(err)
	}
	out, err := c.Inventory(context.Background(), "categories", "水族館", 20, false)
	if err != nil {
		t.Fatal(err)
	}
	if object(out["inventory"])["basis"] != "locally_refreshed_first_party_inventory" || len(c.Sources) != 1 || c.Sources[0].URL != Origin+"/leisure/" || c.Sources[0].FetchedAt != "2026-09-30T12:00:00Z" {
		t.Fatal(out, c.Sources)
	}
}

func TestCompressedAndStackedBodiesHaveDecodedCap(t *testing.T) {
	var gz bytes.Buffer
	g := gzip.NewWriter(&gz)
	_, _ = g.Write(bytes.Repeat([]byte("x"), maxBody+1))
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	var stack bytes.Buffer
	z := zlib.NewWriter(&stack)
	_, _ = z.Write(gz.Bytes())
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		encoding string
		body     []byte
	}{{"gzip", gz.Bytes()}, {"gzip, deflate", stack.Bytes()}} {
		tr := ReadOnlyTransport{Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			resp := fakeResponse(200, "")
			resp.Body = io.NopCloser(bytes.NewReader(tc.body))
			resp.Header.Set("Content-Encoding", tc.encoding)
			return resp, nil
		})}
		req, _ := http.NewRequest(http.MethodGet, Origin+"/search/", nil)
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.ReadAll(resp.Body); err == nil || !strings.Contains(err.Error(), "4 MiB") {
			t.Fatal(tc.encoding, "decoded cap bypass", err)
		}
		_ = resp.Body.Close()
		if resp.Header.Get("Content-Encoding") != "" {
			t.Fatal("generated client would decode twice")
		}
	}
}
