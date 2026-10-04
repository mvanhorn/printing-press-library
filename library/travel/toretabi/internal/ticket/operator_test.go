// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ticket

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Minimal source-shape excerpts preserve the decision boundaries, rather than
// copying complete operator documents.
const centralOperatorFixture = `<title>ＪＲ東海＆17私鉄 乗り鉄☆たびきっぷ｜ＪＲ東海</title><section class="main"><header class="page-title"><h1>ＪＲ東海＆17私鉄 乗り鉄☆たびきっぷ</h1></header>
<dl><dt>発売条件</dt><dd>土休日を利用開始日として連続する２日間に限り発売します。</dd>
<dt>利用期間</dt><dd>2026年4月4日(土)より通年</dd>
<dt>発売期間</dt><dd>2026年4月1日(水)より通年（利用開始の１ヶ月前から初日まで）</dd>
<dt>有効期間</dt><dd>２日間</dd></dl>
<table><tr><th>種別</th><th>発売額</th></tr><tr><td>おとな</td><td>9,200円</td></tr><tr><td>こども</td><td>4,330円</td></tr></table>
<h2>ＪＲ西日本ネット予約「e5489」</h2><p>甲府、国府津、辰野、塩尻、猪谷、新宮駅では受取できません。</p>
<li>大井川鐵道のＥＬ急行は別途料金が必要（きかんしゃトーマス号等は利用不可）。</li>
<li>別に料金が必要な観光列車・イベント列車は利用いただけません。</li><a href="_pdf/areamap.pdf">フリー区間</a></section>`

const hokkaidoOperatorFixture = `<title>ANA／FDA／Peachきた北海道フリーパス（2026年度設定）｜JR北海道</title><section class="detail-ticketSec"><div class="detail-ticketSec-hWrap"><h2>ANA／FDA／Peachきた北海道フリーパス（2026年度設定）</h2></div>
<section><section><h2>ご利用期間</h2><p>2026年４月１日～2027年４月３日（2027年４月１日利用開始分まで）</p>
<dl><dt>利用制限期間</dt><dd>４月27日～５月６日、８月10日～８月19日、12月28日～１月６日</dd></dl></section>
<section><h2>発売期間</h2><p>2026年４月１日～2027年３月31日</p></section>
<section><h2>前売り期間</h2><p>利用開始当日と前日に限り発売します。</p></section></section>
<table><tr><th>商品名</th><th>発売額</th><th>有効 期間</th></tr><tr><th>大人</th></tr>
<tr><td>ANAきた北海道フリーパス</td><td rowspan="3">16,670</td><td rowspan="3">3日</td></tr>
<tr><td>FDAきた北海道フリーパス</td></tr><tr><td>Peachきた北海道フリーパス</td></tr>
<tr><td>ANAきた北海道フリーパス（U25）</td><td rowspan="3">13,990</td><td rowspan="3">3日</td></tr></table>
<p>Ｕ25はパスの購入日時点で25歳以下、公的証明書をご用意ください。</p><p>話せる券売機で発売します。</p>
<table><tr><th>大人 ご利用券</th><td>1,000円分</td></tr></table></section>`

func operatorTicket(name, raw string) Ticket {
	return Ticket{Summary: Summary{ID: "tokai_043", NameJA: name, SourceURL: Origin + "/ticket/tokai_043.html", ObservedAt: "2026-10-04T11:00:00Z"}, OperatorURLs: []string{raw}, Sales: period(""), Use: period(""), Price: Price{Status: "unknown", Amounts: []Money{}}}
}
func TestOperatorRulesCurrentPricesAndConflicts(t *testing.T) {
	x := operatorTicket("ＪＲ東海＆17私鉄 乗り鉄☆たびきっぷ", "https://railway.jr-central.co.jp/tickets/noritetsu-tabikippu-17/")
	x.Sales = period("2025年4月1日より通年")
	x.Use = period("2026年4月4日より通年")
	o, e := parseOperator([]byte(centralOperatorFixture), x, "2026-10-04")
	if e != nil || o.Status != "partial_rule_evidence" || o.AllRulesVerified || o.NameMatch != "matched" || o.Use.Start == nil || *o.Use.Start != "2026-04-04" || o.Sales.Start == nil || *o.Sales.Start != "2026-04-01" || !o.Use.Conditional {
		t.Fatalf("operator %+v %v", o, e)
	}
	if len(o.Price.Amounts) != 2 || o.Price.Amounts[0].Amount != "9200" || o.Price.Amounts[1].Amount != "4330" || o.ValidityDays == nil || *o.ValidityDays != 2 || len(o.PurchaseChannels) < 2 || len(o.Exceptions) < 2 || len(o.Supplements) < 2 || len(o.PDFReferences) != 1 {
		t.Fatalf("source rules %+v", o)
	}
	checks := map[string]string{}
	for _, c := range o.RuleChecks {
		checks[c.Field] = c.State
	}
	if checks["sales_start"] != "conflict" || checks["use_start"] != "corroborated" || checks["ticket_fares"] != "operator_only" {
		t.Fatalf("checks %+v", checks)
	}
	x.Operator = &o
	c := Compare(x, "2026-10-10", "2026-10-04")
	if c.OperatorUseCheck.State != "conditional" || c.ConfirmedEligibility != "unknown" || c.Ticket.Price.Status != "unknown" {
		t.Fatalf("comparison %+v", c)
	}
}
func TestOperatorFullwidthU25FareAndArchive(t *testing.T) {
	x := operatorTicket("ANA／FDA／Peachきた北海道フリーパス", "https://www.jrhokkaido.co.jp/CM/Otoku/007246/")
	o, e := parseOperator([]byte(hokkaidoOperatorFixture), x, "2026-10-04")
	if e != nil || o.Status != "partial_rule_evidence" || o.Use.Start == nil || *o.Use.Start != "2026-04-01" || o.Use.End == nil || *o.Use.End != "2027-04-03" || o.Sales.End == nil || *o.Sales.End != "2027-03-31" || len(o.BlackoutSpans) != 3 {
		t.Fatalf("fullwidth %+v %v", o, e)
	}
	if len(o.Use.Intervals) != 1 || len(o.Price.Amounts) != 2 || o.Price.Amounts[0].Amount != "16670" || o.Price.Amounts[1].Amount != "13990" || o.Price.Amounts[1].CategoryJA == nil || !strings.Contains(*o.Price.Amounts[1].CategoryJA, "U25") || o.Price.Amounts[1].CurrencyBasis == nil || o.ValidityDays == nil || *o.ValidityDays != 3 {
		t.Fatalf("fare/edition %+v", o)
	}
	if len(o.Eligibility) == 0 || o.Eligibility[0].Scope == nil || *o.Eligibility[0].Scope != "purchase_age" || o.BlackoutSpans[2].Year != nil || o.AllRulesVerified {
		t.Fatalf("eligibility %+v", o)
	}
	if operatorEdition(o, "2028-01-01") != "archived_use_window" || CheckPeriod(o.Use, "2027-04-04").State != "closed" {
		t.Fatal("archived edition not distinct")
	}
}
func TestOperatorIdentityAndAmbiguityNeverVerify(t *testing.T) {
	x := operatorTicket("試験きっぷ", "https://www.jrhokkaido.co.jp/CM/Otoku/007246/")
	for _, tc := range []struct{ body, want string }{{centralOperatorFixture, "identity_unverified"}, {`<title>試験きっぷ</title><p>参考</p>`, "ambiguous_scope"}} {
		o, e := parseOperator([]byte(tc.body), x, "2026-10-04")
		if e != nil || o.Status != tc.want || o.AllRulesVerified || len(o.Price.Amounts) != 0 {
			t.Fatalf("%+v %v", o, e)
		}
	}
	for _, raw := range []string{"https://www.jrhokkaido.co.jp.evil.example/", "http://www.jrhokkaido.co.jp/", "https://a@www.jrhokkaido.co.jp/", "https://www.jrhokkaido.co.jp:443/"} {
		if safeOperatorURL(raw) {
			t.Fatalf("unsafe supported link %s", raw)
		}
	}
}

type operatorTransport func(*http.Request) (*http.Response, error)

func (f operatorTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOperatorAccessStatesAndLimits(t *testing.T) {
	x := operatorTicket("試験きっぷ", "https://www.jrhokkaido.co.jp/CM/Otoku/007246/")
	for _, tc := range []struct {
		code           int
		ct, body, want string
	}{{403, "text/html", "Forbidden", "http_error"}, {200, "application/pdf", "%PDF", "unsupported_pdf"}, {200, "application/json", "{}", "unsupported_format"}, {200, "text/html", strings.Repeat("x", MaxOperatorBody+1), "response_limit"}} {
		c := NewClient()
		c.HTTP.Transport = operatorTransport(func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" || r.URL.String() != x.OperatorURLs[0] {
				t.Error("request changed")
			}
			return &http.Response{StatusCode: tc.code, Header: http.Header{"Content-Type": []string{tc.ct}}, Body: io.NopCloser(strings.NewReader(tc.body)), Request: r}, nil
		})
		o, e := c.ReadOperator(context.Background(), x, "2026-10-04")
		if e != nil || o.Status != tc.want || o.ObservedAt == nil || o.HTTPStatus == nil || *o.HTTPStatus != tc.code || o.AllRulesVerified {
			t.Fatalf("%+v %v", o, e)
		}
	}
	c := NewClient()
	c.HTTP.Transport = operatorTransport(func(r *http.Request) (*http.Response, error) { return nil, errors.New("transport down") })
	o, e := c.ReadOperator(context.Background(), x, "2026-10-04")
	if e != nil || o.Status != "access_error" || o.ObservedAt != nil {
		t.Fatalf("failed access %+v %v", o, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = c.ReadOperator(ctx, x, "2026-10-04"); !errors.Is(e, context.Canceled) {
		t.Fatalf("cancellation %v", e)
	}
	x.OperatorURLs = []string{"https://operator.example/ticket"}
	o, e = c.ReadOperator(context.Background(), x, "2026-10-04")
	if e != nil || o.Status != "unsupported_operator" {
		t.Fatal(o, e)
	}
}
func TestOperatorCacheKeepsClocksAndNewestObservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), CacheFilename)
	x := operatorTicket("新しいきっぷ", "https://www.jrhokkaido.co.jp/CM/Otoku/007246/")
	x.ObservedAt = "2026-10-04T11:00:00.000000001Z"
	op := emptyOperator(x.OperatorURLs[0], "2026-10-04")
	at := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	op.ObservedAt = &at
	op.Status = "partial_rule_evidence"
	x.Operator = &op
	if e := Save(context.Background(), path, x); e != nil {
		t.Fatal(e)
	}
	old := x
	old.ObservedAt = "2026-10-04T11:00:00Z"
	old.NameJA = "古いきっぷ"
	if e := Save(context.Background(), path, old); e != nil {
		t.Fatal(e)
	}
	xs, e := Cached(context.Background(), path, x.ID, 1)
	if e != nil || len(xs) != 1 {
		t.Fatal(xs, e)
	}
	v := xs[0]
	if v.NameJA != x.NameJA || v.Operator == nil || v.Operator.ObservedAt == nil || *v.Operator.ObservedAt != at || v.Operator.Transport != "local" || v.Operator.ObservationAgeSeconds < 3599 {
		t.Fatal(fmt.Sprintf("lost separate clocks: %+v", v))
	}
}

func TestOperatorGenericTitleRequiresSpecificHeading(t *testing.T) {
	x := operatorTicket("えちごツーデーパス", "https://www.jreast.co.jp/tickets/info.aspx?GoodsCd=2993")
	body := `<title>おトクなきっぷ：JR東日本</title><article><h2>えちごツーデーパス</h2><table><tr><th>発売期間</th><td>2026年10月1日～2027年3月27日</td></tr><tr><th>利用期間</th><td>2027年3月28日まで</td></tr></table></article>`
	o, e := parseOperator([]byte(body), x, "2028-01-01")
	if e != nil || o.Status != "partial_rule_evidence" || o.NameMatch != "matched" || o.NameEvidence.TextJA != "えちごツーデーパス" || o.EditionState != "archived_use_window" || o.Sales.End == nil || *o.Sales.End != "2027-03-27" || o.AllRulesVerified {
		t.Fatalf("specific heading %+v %v", o, e)
	}
}

func TestOperatorFormulaAndTicketSetAreNotFlatFares(t *testing.T) {
	x := operatorTicket("6枚回数券", "https://www.jr-eki.com/ticket/brand/2-2BW")
	body := `<title>6枚回数券</title><article><h2>6枚回数券</h2><table><tr><th>発売期間</th><td>通年</td></tr><tr><th>ご利用期間</th><td>通年</td></tr><tr><th>発売額</th><td>大人：片道運賃を６倍して１割引き。例）330円×6×0.9＝1,782円</td></tr></table></article>`
	o, e := parseOperator([]byte(body), x, "2026-10-04")
	if e != nil || o.Status != "partial_rule_evidence" || !o.Sales.YearRound || o.Price.Status != "unknown" || len(o.Price.Amounts) != 0 || o.AllRulesVerified {
		t.Fatalf("formula became fare %+v %v", o, e)
	}
}

func TestReviewerRelatedTicketRulesAreScoped(t *testing.T) {
	x := operatorTicket("えちごツーデーパス", "https://www.jreast.co.jp/tickets/info.aspx?GoodsCd=2993")
	body := `<title>おトクなきっぷ：JR東日本</title><main><article><h2>えちごツーデーパス</h2><p>次の設定は未発表です。</p></article><aside><article><h2>別の関連きっぷ</h2><table><tr><th>発売期間</th><td>2020年1月1日～2020年12月31日</td></tr><tr><th>利用期間</th><td>2020年1月1日～2020年12月31日</td></tr></table><table><tr><th>種別</th><th>発売額</th></tr><tr><td>大人</td><td>2,000円</td></tr></table></article></aside></main>`
	o, e := parseOperator([]byte(body), x, "2026-10-04")
	if e != nil {
		t.Fatal(e)
	}
	if o.Sales.End != nil || o.Use.End != nil || len(o.Price.Amounts) != 0 || o.Status == "partial_rule_evidence" {
		t.Errorf("related-ticket facts were assigned to requested ticket: status=%s sale=%v use=%v fares=%+v", o.Status, o.Sales.End, o.Use.End, o.Price.Amounts)
	}
}

func TestOperatorKnownScopeRejectsForeignPanelsAndAmbiguity(t *testing.T) {
	x := operatorTicket("えちごツーデーパス", "https://www.jreast.co.jp/tickets/info.aspx?GoodsCd=2993")
	unrelated := `<article><h2>別の関連きっぷ</h2><dl><dt>発売期間</dt><dd>2020年1月1日～2020年12月31日</dd><dt>利用期間</dt><dd>2020年1月1日～2020年12月31日</dd></dl><table><tr><th>種別</th><th>発売額</th></tr><tr><td>大人</td><td>2,000円</td></tr></table><p>25歳以下に発売、e5489、別途特急券が必要です。</p><a href="other.pdf">別の商品</a></article>`
	shell := `<title>おトクなきっぷ：JR東日本</title><div id="mainContents"><div id="mainVisual"><h2>えちごツーデーパス</h2></div><main id="contents"><section class="contentsWrapper"><p>次の設定は未発表です。</p>`
	for _, panel := range []string{unrelated, `<aside>` + unrelated + `</aside>`, `<div class="recommendedTickets">` + unrelated + `</div>`} {
		o, e := parseOperator([]byte(shell+panel+`</section></main></div>`), x, "2026-10-04")
		if e != nil || o.Status != "ambiguous_rules" || o.Sales.End != nil || o.Use.End != nil || len(o.Price.Amounts) != 0 || len(o.PurchaseChannels) != 0 || len(o.Eligibility) != 0 || len(o.Supplements) != 0 || len(o.PDFReferences) != 0 {
			t.Fatalf("borrowed foreign facts %+v %v", o, e)
		}
	}
	for _, body := range []string{`<title>えちごツーデーパス</title>` + unrelated, `<title>えちごツーデーパス</title><article><h2>えちごツーデーパス</h2></article><article><h2>えちごツーデーパス</h2></article>`} {
		o, e := parseOperator([]byte(body), x, "2026-10-04")
		if e != nil || o.Status != "ambiguous_scope" || len(o.Price.Amounts) != 0 || o.Sales.End != nil {
			t.Fatalf("ambiguous target scope %+v %v", o, e)
		}
	}
}
func TestReviewerOperatorSaleLeadTimeRemainsConditional(t *testing.T) {
	x := operatorTicket("えちごツーデーパス", "https://www.jreast.co.jp/tickets/info.aspx?GoodsCd=2993")
	// Real source layout plus the reviewer's exact observed relative-sale wording.
	body := `<title>おトクなきっぷ：JR東日本</title><div id="mainContents"><div id="mainVisual"><h2>えちごツーデーパス</h2></div><main id="contents"><section class="contentsWrapper"><table><tr><th>発売期間</th><td>2027年3月27日まで。※有効期間開始日の1ヶ月前から有効期間開始日までの発売です。</td></tr><tr><th>利用期間</th><td>2027年3月28日までの金・土曜・休日</td></tr></table></section></main></div>`
	o, e := parseOperator([]byte(body), x, "2026-10-04")
	if e != nil {
		t.Fatal(e)
	}
	if o.Status != "partial_rule_evidence" || !o.Sales.Conditional || CheckPeriod(o.Sales, "2026-10-04").State != "conditional" || CheckPeriod(o.Sales, "2027-03-28").State != "closed" || o.ScopeStatus != "verified_product_container" {
		t.Fatalf("sale lead time lost %+v", o)
	}
}
