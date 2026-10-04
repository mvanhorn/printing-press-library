// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ticket

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/toretabi/internal/cliutil"
)

const detailFixture = `<h1 class="title-01">試験きっぷ</h1><div class="content-cell" data-area="3"><p>普通列車の乗り降りが自由です。利用券（1,000円分）が付きます。</p><table><tr><th>発売期間</th><td>2026年10月1日～2027年3月27日<br>利用開始日の1カ月前から発売。</td></tr><tr><th>利用期間</th><td>2027年3月28日までの金・土曜・休日、2026年12月25日～2027年1月3日</td></tr><tr><th>有効期間</th><td>連続する2日間</td></tr></table><p>＊別途特急券が必要です。<br>＊観光列車は利用になれません。<br>＊4月27日～5月6日、12月28日～1月6日は利用できません。<br>うりば●指定席券売機<br>＊25歳以下は公的証明書が必要です。</p><p class="btn-01"><a href="https://operator.example/ticket/">詳細を確認する</a></p><a href="https://social.example/">share</a></div>`

func listingFixture(name, next string) string {
	return `<select name="area"><option value="1">北海道</option></select><select name="class"><option value="4">フリー</option></select><ul class="nav-index-04"><li><a href="https://www.toretabi.jp/ticket/east_027.html"><b>` + name + `</b><ul><li class="type1">フリー</li><li class="type2">東北</li><li class="type3">期間限定</li><li>特急券別購入</li></ul></a></li></ul>` + next
}
func sample(t *testing.T, id, at string) Ticket {
	t.Helper()
	x, e := ParseDetail([]byte(detailFixture), id, at)
	if e != nil {
		t.Fatal(e)
	}
	return x
}

func TestParseListingAndID(t *testing.T) {
	for _, tc := range []struct {
		body string
		ok   bool
	}{{listingFixture("えちご", ""), true}, {`<h1>challenge</h1>`, false}, {`<ul class="nav-index-04"></ul>`, false}} {
		x, e := ParseListing([]byte(tc.body), "2026-10-04T11:00:00Z")
		if (e == nil) != tc.ok {
			t.Fatalf("ok=%v err=%v", tc.ok, e)
		}
		if tc.ok && (len(x.Tickets) != 1 || x.Tickets[0].NameJA != "えちご" || len(x.Areas) != 1) {
			t.Fatalf("bad listing %+v", x)
		}
	}
	for id, want := range map[string]bool{"tokai_043": true, "east_027": true, "../tokai_043": false, "https://example.com": false, "east_027?x": false} {
		if ValidID(id) != want {
			t.Fatalf("ID %s", id)
		}
	}
}
func TestDetailEvidenceAndPriceBoundary(t *testing.T) {
	x := sample(t, "east_027", "2026-10-04T11:00:00Z")
	if x.Price.Status != "unknown" || len(x.Price.Amounts) != 0 || len(x.Benefits) == 0 || x.Benefits[0].Amount != "1000" {
		t.Fatalf("voucher became fare %+v", x.Price)
	}
	if x.Sales.Start == nil || *x.Sales.Start != "2026-10-01" || x.Sales.End == nil || *x.Sales.End != "2027-03-27" || x.ValidityDays == nil || *x.ValidityDays != 2 {
		t.Fatalf("periods %+v", x)
	}
	if len(x.OperatorURLs) != 1 || len(x.Supplements) == 0 || len(x.Exceptions) == 0 || len(x.PurchaseChannels) == 0 || len(x.Eligibility) == 0 || len(x.BlackoutSpans) != 2 || x.BlackoutSpans[1].Year != nil {
		t.Fatalf("evidence groups %+v", x)
	}
	if x.InventoryStatus != "unknown" || x.OperatorVerification != "required" {
		t.Fatal("invented availability")
	}
	priced := strings.Replace(detailFixture, "</table>", `<tr><th>価格</th><td>おとな9,200円、こども4,330円</td></tr></table>`, 1)
	y, e := ParseDetail([]byte(priced), "tokai_043", x.ObservedAt)
	if e != nil || y.Price.Status != "published" || len(y.Price.Amounts) != 2 || y.Price.Amounts[0].Unit != "source_labelled_ticket_price" {
		t.Fatalf("fare %+v %v", y.Price, e)
	}
}
func TestPeriodConservativeComparison(t *testing.T) {
	tests := []struct{ raw, on, want string }{{"2027年3月28日までの金・土曜・休日", "2027-03-29", "closed"}, {"2027年3月28日までの金・土曜・休日", "2026-10-10", "conditional"}, {"2026年10月1日～2026年10月31日", "2026-10-20", "within_published_window"}, {"2026年10月1日～2026年10月31日", "2026-11-01", "closed"}, {"通年", "2026-10-20", "within_published_window"}, {"2日間", "2026-10-20", "unknown"}, {"", "", "unknown"}}
	for _, tc := range tests {
		if got := CheckPeriod(period(tc.raw), tc.on); got.State != tc.want {
			t.Fatalf("%q %s: %+v", tc.raw, tc.on, got)
		}
	}
	p := period("2026年12月28日～1月6日")
	if len(p.Intervals) != 1 || p.Intervals[0].End != "2027-01-06" {
		t.Fatalf("year crossing %+v", p)
	}
	p = period("12月28日～1月6日")
	if len(p.Intervals) != 0 || len(p.ExplicitDates) != 0 {
		t.Fatal("invented year")
	}
	x := sample(t, "east_027", "2026-10-04T11:00:00Z")
	c := Compare(x, "2027-03-29", "2027-04-01")
	if c.EditionState != "archived_use_window" || c.ConfirmedEligibility != "unknown" || c.SalesCheck.State != "closed" {
		t.Fatalf("archival %+v", c)
	}
}
func TestEvidenceBounds(t *testing.T) {
	long := strings.Repeat("長", EvidenceRunes+1)
	x := clip(long)
	if !x.Truncated || len([]rune(x.TextJA)) != EvidenceRunes {
		t.Fatal("unbounded evidence")
	}
	if _, e := ParseDetail([]byte(`<h1>challenge</h1>`), "east_027", "2026-10-04T11:00:00Z"); e == nil {
		t.Fatal("challenge parsed")
	}
}
func testClient(s *httptest.Server) *Client {
	c := NewClient()
	c.origin = s.URL
	c.HTTP = s.Client()
	c.limiter = cliutil.NewAdaptiveLimiter(100)
	return c
}
func TestClientRealShapeAndListingBounds(t *testing.T) {
	requests := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/html")
		if strings.HasSuffix(r.URL.Path, ".html") {
			fmt.Fprint(w, detailFixture)
			return
		}
		if r.URL.Query().Get("area") != "1" || r.URL.Query().Get("class") != "4" {
			t.Error("native filters absent")
		}
		fmt.Fprint(w, listingFixture("えちご", `<a href="/ticket/page/2/">2</a>`))
	}))
	defer s.Close()
	c := testClient(s)
	x, e := c.List(context.Background(), ListOptions{Area: "1", Type: "4", Query: "never-match", MaxPages: 1, Limit: 2})
	if e != nil || requests != 1 || len(x.Tickets) != 0 || x.Coverage.ScannedTickets != 1 || x.Coverage.Complete || x.Coverage.Continuation == nil || !strings.Contains(x.Coverage.Note, "Zero matches") {
		t.Fatalf("bounded zero %+v %v", x, e)
	}
	y, e := c.Get(context.Background(), "east_027")
	if e != nil || y.NameJA != "試験きっぷ" || y.ObservedAt == "" {
		t.Fatalf("get %+v %v", y, e)
	}
	if _, e = c.List(context.Background(), ListOptions{MaxPages: 6, Limit: 10}); e == nil {
		t.Fatal("unbounded pages")
	}
}
func TestClientSourceErrorsAndCancellation(t *testing.T) {
	for _, status := range []int{404, 429, 500} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		_, e := testClient(s).Get(context.Background(), "east_027")
		s.Close()
		if e == nil {
			t.Fatalf("HTTP%d empty success", status)
		}
		if status == 429 {
			var rate *cliutil.RateLimitError
			if !errors.As(e, &rate) {
				t.Fatal("throttle not typed")
			}
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, strings.Repeat("x", MaxBody+1))
	}))
	defer s.Close()
	if _, e := testClient(s).Get(context.Background(), "east_027"); e == nil {
		t.Fatal("body cap absent")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := testClient(s).Get(ctx, "east_027"); e == nil {
		t.Fatal("context ignored")
	}
}
func TestCacheTransactionsNewestAndOffline(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), CacheFilename)
	xs, e := Cached(ctx, path, "", 3)
	if e != nil || len(xs) != 0 {
		t.Fatal(e)
	}
	if _, e = os.Stat(path); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("empty read created database")
	}
	newer := sample(t, "east_027", "2026-10-04T11:00:00.000000001Z")
	newer.NameJA = "new"
	if e = Save(ctx, path, newer); e != nil {
		t.Fatal(e)
	}
	older := newer
	older.NameJA = "old"
	older.ObservedAt = "2026-10-04T11:00:00Z"
	if e = Save(ctx, path, older); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	xs, e = Cached(ctx, path, "east_027", 3)
	if e != nil || len(xs) != 1 || xs[0].NameJA != "new" || xs[0].Transport != "local" || xs[0].ObservedAt != "2026-10-04T11:00:00.000000001Z" {
		t.Fatalf("cache %+v %v", xs, e)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("read changed database")
	}
	xs, e = Cached(ctx, path, "%", 3)
	if e != nil || len(xs) != 0 {
		t.Fatal("query not literal")
	}
}
func TestCacheWALCurrentRowsAndBounds(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), CacheFilename)
	x := sample(t, "east_027", "2026-10-04T11:00:00Z")
	if e := Save(ctx, path, x); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec(`PRAGMA journal_mode=WAL`); e != nil {
		t.Fatal(e)
	}
	x.NameJA = "committed-WAL"
	x.ObservedAt = "2026-10-04T11:00:01Z"
	b, _ := json.Marshal(x)
	if _, e = db.Exec(`UPDATE ticket_observations SET data=? WHERE id=?`, string(b), x.ID); e != nil {
		t.Fatal(e)
	}
	rows, e := Cached(ctx, path, "east_027", 3)
	if e != nil || len(rows) != 1 || rows[0].NameJA != "committed-WAL" {
		t.Fatalf("WAL read %+v %v", rows, e)
	}
	if _, e = Cached(ctx, path, "", 51); e == nil {
		t.Fatal("cache output cap")
	}
	x.Description.TextJA = strings.Repeat("x", MaxPayload)
	if e = Save(ctx, path, x); e == nil {
		t.Fatal("payload cap")
	}
}
func TestCacheCapacityAndValidation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), CacheFilename)
	x := sample(t, "east_027", "2026-10-04T11:00:00Z")
	for i := 0; i < 101; i++ {
		x.ID = fmt.Sprintf("east_%03d", i)
		x.SourceURL = Origin + "/ticket/" + x.ID + ".html"
		x.ObservedAt = time.Date(2026, 10, 4, 11, 0, i, 0, time.UTC).Format(time.RFC3339Nano)
		if e := Save(ctx, path, x); e != nil {
			t.Fatal(e)
		}
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var count int
	if e = db.QueryRow(`SELECT count(*) FROM ticket_observations`).Scan(&count); e != nil || count != MaxCached {
		t.Fatalf("retention %d %v", count, e)
	}
	x.SourceURL = "https://wrong.example/ticket/" + x.ID + ".html"
	if e = Save(ctx, path, x); e == nil {
		t.Fatal("false source accepted")
	}
}

func TestDetailRegionSelectorVaries(t *testing.T) {
	for _, area := range []string{"1", "2", "3", "5", "10"} {
		body := strings.Replace(detailFixture, `data-area="3"`, `data-area="`+area+`"`, 1)
		x, err := ParseDetail([]byte(body), "east_027", "2026-10-04T11:00:00Z")
		if err != nil || x.Sales.End == nil {
			t.Fatalf("area %s: %v", area, err)
		}
	}
}

func TestLinkedPurchaseTextPreserved(t *testing.T) {
	body := strings.Replace(detailFixture, "指定席券売機", `<a href="https://operator.example/purchase">指定席券売機</a>`, 1)
	x, err := ParseDetail([]byte(body), "east_027", "2026-10-04T11:00:00Z")
	if err != nil || len(x.PurchaseChannels) == 0 || !strings.Contains(x.PurchaseChannels[0].TextJA, "指定席券売機") {
		t.Fatalf("purchase text lost %+v %v", x.PurchaseChannels, err)
	}
}
