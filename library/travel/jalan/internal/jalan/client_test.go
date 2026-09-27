package jalan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/cliutil"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func httpResult(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"text/html; charset=UTF-8"}}, Body: io.NopCloser(strings.NewReader(body))}
}
func testClient(t *testing.T, backend transportFunc) *Client {
	t.Helper()
	c := NewClient(Options{CacheDir: t.TempDir()})
	c.HTTP.Transport = backend
	c.limiter = nil
	c.now = func() time.Time { return testNow }
	return c
}
func echoControls(r *http.Request) string {
	var b strings.Builder
	for name, values := range r.URL.Query() {
		if name == "idx" || name == "distCd" {
			continue
		}
		fmt.Fprintf(&b, `<input type="hidden" name="%s" value="%s">`, name, values[0])
	}
	return b.String()
}

// Compact source-shaped cards exercise the actual parser with independent IDs.
func searchHTML(r *http.Request, start, count, total int, hasNext bool) string {
	var b strings.Builder
	b.WriteString(echoControls(r))
	fmt.Fprintf(&b, `<div class="jlnpc-planListCnt-header">%d軒</div>`, total)
	// A sponsored entry must not consume a native organic offset.
	b.WriteString(`<div class="p-yadoCassette p-yadoCassette--pr" id="sa_yad999999"><a class="jlnpc-yadoCassette__link" href="/yad999999/"></a><h2 class="p-searchResultItem__facilityName">広告の宿</h2></div>`)
	for i := 0; i < count; i++ {
		id := 100000 + start + i
		fmt.Fprintf(&b, `<div class="p-yadoCassette" id="yad%d"><a class="jlnpc-yadoCassette__link" href="/yad%d/"></a><h2 class="p-searchResultItem__facilityName">宿%d</h2></div>`, id, id, id)
	}
	if hasNext {
		b.WriteString(`<a class="next" onclick="selectPage('30','2')">次へ</a>`)
	}
	return b.String()
}
func offersHTML(r *http.Request, count int) string {
	var b strings.Builder
	b.WriteString(echoControls(r))
	b.WriteString(`<div id="planlist-header"><span class="volume">1件</span></div><div class="p-planCassette" data-plancode="03912759"><p class="p-searchResultItem__catchPhrase">宿泊プラン</p><p class="p-mealType__value">朝/夕あり</p><table><thead><tr><th class="p-searchResultItem__headCell--total">大人2名　合計</th></tr></thead><tbody>`)
	for i := 0; i < count; i++ {
		fmt.Fprintf(&b, `<tr><td><table><tr class="js-searchYadoRoomPlanCd" id="yd385995pc03912759rc%07d"><td><a class="p-searchResultItem__planName" href="/uw/uwp3200/uww3201init.do?planCd=03912759&amp;roomTypeCd=%07d">部屋%d</a><span class="p-searchResultItem__total">%d円</span><span class="p-searchResultItem__rest">残り1室</span></td></tr></table></td></tr>`, 5700000+i, 5700000+i, i, 20000+i)
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}
func TestSearchLogicalPagesDoNotSkipItems(t *testing.T) {
	var offsets []int
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("idx"))
		offsets = append(offsets, offset)
		return httpResult(200, searchHTML(r, offset, 30, 90, true)), nil
	})
	q := datedQuery()
	q.Destination = "Hakone"
	q.Page = 2
	q.Limit = 5
	response, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 5 || response.Results[0].(Property).ID != "100005" || len(offsets) != 1 || offsets[0] != 0 {
		t.Fatalf("logical second page skipped data: %v %v", response.Results, offsets)
	}
	q.Page = 2
	q.Limit = 20
	response, err = c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 20 || response.Results[0].(Property).ID != "100020" || response.Results[19].(Property).ID != "100039" {
		t.Fatalf("cross-page slice wrong: %v", response.Results)
	}
	if offsets[len(offsets)-2] != 0 || offsets[len(offsets)-1] != 30 {
		t.Fatalf("offsets not aligned: %v", offsets)
	}
	if response.Meta["upstream_requests"] != 2 || response.Pagination["returned_count"] != 20 {
		t.Fatalf("metrics wrong: %v %v", response.Meta, response.Pagination)
	}
}
func TestSearchSecondPageFailureIsPartial(t *testing.T) {
	calls := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Query().Get("idx") == "30" {
			return httpResult(503, "temporarily unavailable"), nil
		}
		return httpResult(200, searchHTML(r, 0, 30, 90, true)), nil
	})
	q := datedQuery()
	q.Destination = "Hakone"
	q.Page = 2
	q.Limit = 20
	response, err := c.Search(context.Background(), q)
	var partial *PartialError
	if !errors.As(err, &partial) || len(response.Results) != 10 || len(response.FetchFailures) != 1 || calls != 3 || response.Meta["status"] != "partial" {
		t.Fatalf("failure lost: response=%+v err=%v calls=%d", response, err, calls)
	}
}
func TestOffersPagesUseTuplesNotPlanCount(t *testing.T) {
	c := testClient(t, func(r *http.Request) (*http.Response, error) { return httpResult(200, offersHTML(r, 12)), nil })
	q := datedQuery()
	q.Page = 2
	q.Limit = 5
	response, err := c.Offers(context.Background(), "385995", q)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 5 || response.Results[0].(Offer).RoomID != "5700005" || response.Pagination["total"] != 12 {
		t.Fatalf("tuple page wrong: %+v", response)
	}
	plans := response.Pagination["source_plan_count"].(*int)
	if plans == nil || *plans != 1 {
		t.Fatalf("plan count lost: %+v", response.Pagination)
	}
}
func TestHTTPFailureKindsAndLimits(t *testing.T) {
	t.Run("typed rate limit retained", func(t *testing.T) {
		calls := 0
		c := testClient(t, func(*http.Request) (*http.Response, error) {
			calls++
			r := httpResult(429, "")
			r.Header.Set("Retry-After", "1")
			return r, nil
		})
		_, err := c.fetch(context.Background(), "https://www.jalan.net/test", c.newSession())
		var rate *cliutil.RateLimitError
		if !errors.As(err, &rate) || calls != 2 {
			t.Fatalf("rate error=%v calls=%d", err, calls)
		}
	})
	t.Run("access not empty", func(t *testing.T) {
		c := testClient(t, func(*http.Request) (*http.Response, error) { return httpResult(403, "denied"), nil })
		_, err := c.Property(context.Background(), "385995")
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != "access_failure" {
			t.Fatal(err)
		}
	})
	t.Run("parse not empty", func(t *testing.T) {
		c := testClient(t, func(*http.Request) (*http.Response, error) { return httpResult(200, "<html>wrong page</html>"), nil })
		_, err := c.Property(context.Background(), "385995")
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != "parse_failure" {
			t.Fatal(err)
		}
	})
	t.Run("body bounded", func(t *testing.T) {
		c := testClient(t, func(*http.Request) (*http.Response, error) {
			return httpResult(200, strings.Repeat("x", maxResponseBytes+1)), nil
		})
		_, err := c.fetch(context.Background(), "https://www.jalan.net/test", c.newSession())
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != "response_too_large" {
			t.Fatal(err)
		}
	})
	t.Run("source ignored occupancy", func(t *testing.T) {
		c := testClient(t, func(r *http.Request) (*http.Response, error) {
			body := searchHTML(r, 0, 1, 1, false)
			body = strings.ReplaceAll(body, `name="adultNum" value="2"`, `name="adultNum" value="1"`)
			return httpResult(200, body), nil
		})
		q := datedQuery()
		q.Destination = "Hakone"
		_, err := c.Search(context.Background(), q)
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != "unsupported" {
			t.Fatalf("ignored occupancy accepted: %v", err)
		}
	})
}
func TestWindows31JDecode(t *testing.T) {
	original := `<html><meta charset="Shift_JIS"><title>箱根 髙橋</title></html>`
	raw, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), []byte(original))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeHTML(raw, "text/html; charset=Windows-31J")
	if err != nil || decoded != original {
		t.Fatalf("CP932 decode=%q %v", decoded, err)
	}
}
func TestCommandTimeoutAndRetryAfterBudget(t *testing.T) {
	c := testClient(t, func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	c.options.Timeout = 15 * time.Millisecond
	_, err := c.Property(context.Background(), "385995")
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != "timeout" {
		t.Fatalf("timeout not typed: %v", err)
	}
	c = testClient(t, func(*http.Request) (*http.Response, error) {
		r := httpResult(429, "")
		r.Header.Set("Retry-After", "10")
		return r, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	session := c.newSession()
	_, err = c.fetch(ctx, "https://www.jalan.net/test", session)
	var rate *cliutil.RateLimitError
	if !errors.As(err, &rate) || session.requests != 1 {
		t.Fatalf("retry exceeded budget: %v requests=%d", err, session.requests)
	}
}
