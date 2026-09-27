package travel

import (
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fixtureRoom struct {
	plan, room                                                          string
	total                                                               int64
	unavailable, inactive, missingQuote, missingExclusive, missingMeals bool
}

func sampleQuery() OfferQuery {
	return OfferQuery{HotelID: "51870", Checkin: "2026-11-08", Checkout: "2026-11-10", Rooms: 1, AdultsPerRoom: 2, Page: 1, Limit: 5}
}
func doc(body []byte, url string) document {
	return document{Body: body, Source: SourceInfo{Name: "Rakuten Travel", URL: url, FetchedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}}
}
func offerFixture(q OfferQuery, rooms []fixtureRoom, next bool) []byte {
	values := offerValues(q)
	conditions := map[string]any{"isDated": true, "isDayuse": false, "isHotelFixed": true, "f_no": []string{q.HotelID}}
	for k, v := range values {
		conditions[k] = v
	}
	conditionJSON, _ := json.Marshal(conditions)
	plans := map[string]map[string]any{}
	for _, r := range rooms {
		if plans[r.plan] == nil {
			plans[r.plan] = map[string]any{"rooms": map[string]any{}}
		}
		if !r.missingQuote {
			price := map[string]any{"sumTotalChargeTaxInclusive": r.total}
			if !r.missingExclusive {
				price["sumTotalChargeTaxExclusive"] = r.total * 10 / 11
			}
			plans[r.plan]["rooms"].(map[string]any)[r.room] = price
		}
	}
	hotels, _ := json.Marshal(map[string]any{q.HotelID: map[string]any{"plans": plans}})
	var b strings.Builder
	fmt.Fprintf(&b, "<html><title>Hotel plan page</title><script>hinfo.conditions = %s; hinfo.hotels = %s;</script><span class=\"plan-pagination__page--current\">%d</span><span class=\"plan-number__total-count\">12</span>", conditionJSON, hotels, q.Page)
	in, _ := time.Parse("2006-01-02", q.Checkin)
	out, _ := time.Parse("2006-01-02", q.Checkout)
	nights := int(out.Sub(in) / (24 * time.Hour))
	for i, r := range rooms {
		fmt.Fprintf(&b, "<li id=\"%s\" class=\"planThumb\"><h4>Plan %s</h4><p class=\"htlPlnDtlPrv\">Focused plan description</p><ul><li class=\"rm-type-wrapper\" id=\"%s-%s\"><h4>Room %s</h4><p data-locate=\"roomType-Remark\">A brochure mentions 朝食あり; this is not a meal label.</p>", r.plan, r.plan, r.plan, r.room, r.room)
		if !r.missingMeals {
			b.WriteString("<div class=\"htlPlnTypTxt\"><span data-locate=\"roomType-option-meal\">食事 朝食なし 夕食あり</span></div>")
		}
		name := fmt.Sprintf("reserve%d", i)
		fmt.Fprintf(&b, "<form method=\"post\" action=\"https://aps1.travel.rakuten.co.jp/portal/my/ry_kensaku.k4\" name=\"%s\">", name)
		inputs := map[string]string{"f_no": q.HotelID, "f_camp_id": r.plan, "f_syu": r.room, "f_hi1": q.Checkin, "f_hi2": q.Checkout, "f_heya_su": integer(q.Rooms), "f_otona_su": integer(q.AdultsPerRoom)}
		for j, key := range []string{"f_s1", "f_s2", "f_y1", "f_y2", "f_y3", "f_y4"} {
			inputs[key] = integer(q.Children.values()[j])
		}
		for k, v := range inputs {
			fmt.Fprintf(&b, "<input name=\"%s\" value=\"%s\">", k, v)
		}
		b.WriteString("</form>")
		if r.unavailable {
			b.WriteString("<p>空室なし ご希望の日程に該当する空室が見つかりませんでした。</p>")
		} else {
			fmt.Fprintf(&b, "<a data-locate=\"plan-price-detail\">大人%d人 ／%d泊の料金 (税込)</a><dd class=\"cmn_rbAndNvrWrap\"><span class=\"ndPrice\">合計 %d 円</span><span>1人あたり%d円</span>", q.AdultsPerRoom, nights, r.total, r.total/int64(q.AdultsPerRoom))
			if !r.inactive {
				fmt.Fprintf(&b, "<a data-locate=\"plan-reservation-button\" href=\"javascript:document['%s'].submit();\">予約</a>", name)
			}
			b.WriteString("</dd>")
		}
		b.WriteString("</li></ul></li>")
	}
	if next {
		v := offerValues(q)
		v.Set("f_page_no", integer(q.Page+1))
		fmt.Fprintf(&b, "<a href=\"%s\">next</a>", stdhtml.EscapeString("https://hotel.travel.rakuten.co.jp/hotelinfo/plan/"+q.HotelID+"?"+v.Encode()))
	}
	b.WriteString("</html>")
	return []byte(b.String())
}
func assertKind(t *testing.T, err error, kind string) {
	t.Helper()
	var e *SourceError
	if !errors.As(err, &e) || e.Kind != kind {
		t.Fatalf("want %s SourceError; got %v", kind, err)
	}
}
func TestOfferValuesUnitsAndIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, checkout string
		rooms          int
		children       Children
		total          int64
	}{{"one night", "2026-11-09", 1, Children{}, 10160}, {"two nights", "2026-11-10", 1, Children{}, 20720}, {"uniform two rooms", "2026-11-10", 2, Children{}, 20720}, {"all six child categories", "2026-11-10", 2, Children{1, 2, 3, 4, 5, 6}, 20720}} {
		t.Run(tc.name, func(t *testing.T) {
			q := sampleQuery()
			q.Checkout = tc.checkout
			q.Rooms = tc.rooms
			q.Children = tc.children
			r, e := parseOffers(doc(offerFixture(q, []fixtureRoom{{plan: "3951989", room: "s-double-", total: tc.total}, {plan: "1337211", room: "ns-semi-db", total: 25900}}, false), offerURL(q)), q)
			if e != nil {
				t.Fatal(e)
			}
			if r.Status != StatusOK || len(r.Offers) != 2 {
				t.Fatalf("result %#v", r)
			}
			o := r.Offers[0]
			if o.PlanID != "3951989" || o.RoomID != "s-double-" || o.HotelID != "51870" || o.Query.Rooms != tc.rooms || o.Query.Children != tc.children {
				t.Fatalf("identity/party lost %#v", o)
			}
			if o.Price.PerRoomStayJPY != tc.total || o.Price.Currency != "JPY" || o.Price.PerPersonStayJPY == nil || *o.Price.PerPersonStayJPY != tc.total/2 {
				t.Fatalf("price units/value %#v", o.Price)
			}
			if o.Price.ConsumptionTax != "included" || o.Price.AccommodationTax != "unknown" || o.Price.OtherTaxes != "unknown" || o.Price.OptionalFees != "unknown" {
				t.Fatalf("tax certainty %#v", o.Price)
			}
			if o.Meals.Breakfast == nil || *o.Meals.Breakfast || o.Meals.Dinner == nil || !*o.Meals.Dinner {
				t.Fatalf("meal labels %#v", o.Meals)
			}
			if !strings.HasSuffix(o.BookingURL, "#3951989-s-double-") || strings.Contains(o.BookingURL, "aps1") || o.Description == nil || o.PlanName == nil || o.CancellationPolicy != nil {
				t.Fatalf("source handoff/details %#v", o)
			}
		})
	}
}
func TestOfferFailureAndEmptyStates(t *testing.T) {
	base := fixtureRoom{plan: "3951989", room: "s-double-", total: 20720}
	q := sampleQuery()
	cases := []struct {
		name         string
		room         fixtureRoom
		change       func(string) string
		kind, status string
	}{
		{name: "retained form on unavailable row", room: fixtureRoom{plan: "3951989", room: "s-double-", unavailable: true}, status: StatusNoAvailability},
		{name: "actionable missing quote", room: fixtureRoom{plan: "3951989", room: "s-double-", total: 20720, missingQuote: true}, kind: "parse_error"},
		{name: "actionable zero is not free", room: fixtureRoom{plan: "3951989", room: "s-double-", total: 0}, kind: "parse_error"},
		{name: "positive quote missing control", room: fixtureRoom{plan: "3951989", room: "s-double-", total: 20720, inactive: true}, kind: "parse_error"},
		{name: "missing exclusive stays unknown", room: fixtureRoom{plan: "3951989", room: "s-double-", total: 20720, missingExclusive: true}, status: StatusOK},
		{name: "meal prose cannot establish meals", room: fixtureRoom{plan: "3951989", room: "s-double-", total: 20720, missingMeals: true}, status: StatusOK},
		{name: "mismatched room count echo", room: base, change: func(s string) string { return strings.Replace(s, `"f_heya_su":["1"]`, `"f_heya_su":["2"]`, 1) }, kind: "query_mismatch"},
		{name: "mismatched child echo", room: base, change: func(s string) string { return strings.Replace(s, `"f_y4":["0"]`, `"f_y4":["1"]`, 1) }, kind: "query_mismatch"},
		{name: "undated catalogue", room: base, change: func(s string) string { return strings.Replace(s, `"isDated":true`, `"isDated":false`, 1) }, kind: "query_mismatch"},
		{name: "visible quote mismatch", room: base, change: func(s string) string { return strings.Replace(s, "合計 20720 円", "合計 20000 円", 1) }, kind: "parse_error"},
		{name: "wrong stay label", room: base, change: func(s string) string { return strings.Replace(s, "／2泊", "／1泊", 1) }, kind: "parse_error"},
		{name: "wrong reserve tuple", room: base, change: func(s string) string {
			return strings.Replace(s, `name="f_syu" value="s-double-"`, `name="f_syu" value="substitute"`, 1)
		}, kind: "query_mismatch"},
		{name: "malformed embedded JSON", room: base, change: func(s string) string { return strings.Replace(s, "hinfo.hotels = {", "hinfo.hotels = {BAD", 1) }, kind: "parse_error"},
		{name: "fractional JPY", room: base, change: func(s string) string {
			return strings.Replace(s, `"sumTotalChargeTaxInclusive":20720`, `"sumTotalChargeTaxInclusive":20720.5`, 1)
		}, kind: "parse_error"},
		{name: "contradictory exclusive", room: base, change: func(s string) string {
			return strings.Replace(s, `"sumTotalChargeTaxExclusive":18836`, `"sumTotalChargeTaxExclusive":30000`, 1)
		}, kind: "parse_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := string(offerFixture(q, []fixtureRoom{tc.room}, false))
			if tc.change != nil {
				body = tc.change(body)
			}
			r, e := parseOffers(doc([]byte(body), offerURL(q)), q)
			if tc.kind != "" {
				assertKind(t, e, tc.kind)
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if r.Status != tc.status {
				t.Fatalf("status %s", r.Status)
			}
			if tc.status == StatusNoAvailability && len(r.Offers) != 0 {
				t.Fatal("unavailable leaked as offer")
			}
			if tc.room.missingMeals && (r.Offers[0].Meals.Breakfast != nil || r.Offers[0].Meals.Dinner != nil) {
				t.Fatal("meal inferred from brochure")
			}
		})
	}
	for _, body := range []string{"<html>empty layout</html>", "<html>空室なし</html>"} {
		_, e := parseOffers(doc([]byte(body), offerURL(q)), q)
		assertKind(t, e, "parse_error")
	}
}
func TestOfferPageTraversal(t *testing.T) {
	q := sampleQuery()
	q.Limit = 1
	body := offerFixture(q, []fixtureRoom{{plan: "1", room: "a-", total: 20000}, {plan: "2", room: "b-", total: 22000}}, true)
	first, e := parseOffers(doc(body, offerURL(q)), q)
	if e != nil {
		t.Fatal(e)
	}
	if first.Page.NextPage == nil || *first.Page.NextPage != 1 || *first.Page.NextOffset != 1 || first.Page.SourceUnit != "plans" || first.Page.RowsScanned != 2 || first.Page.SourceTotal == nil || *first.Page.SourceTotal != 12 {
		t.Fatalf("first page %#v", first.Page)
	}
	q.Offset = 1
	second, e := parseOffers(doc(body, offerURL(q)), q)
	if e != nil {
		t.Fatal(e)
	}
	if second.Offers[0].PlanID != "2" || *second.Page.NextPage != 2 || *second.Page.NextOffset != 0 {
		t.Fatalf("second page %#v", second)
	}
	q.Offset = 20
	empty, e := parseOffers(doc(body, offerURL(q)), q)
	if e != nil || len(empty.Offers) != 0 || empty.Status != StatusOK || !empty.Page.HasMore {
		t.Fatalf("offset past page %#v %v", empty, e)
	}
}
func TestMetadataExtraction(t *testing.T) {
	t.Run("areas preserve path IDs", func(t *testing.T) {
		d := doc([]byte(`<a href="/yado/tokyo/map.html">東京都</a><a href="/yado/osaka/map.html">大阪府</a><a href="https://evil.example/yado/fake/map.html">bad</a>`), "https://travel.rakuten.co.jp/yado/japan.html")
		r, e := parseAreas(d, "")
		if e != nil || len(r.Areas) != 2 || r.Areas[0].ID != "tokyo" || r.Areas[0].Parent != nil {
			t.Fatalf("areas %#v %v", r, e)
		}
		d = doc([]byte(`<a href="/yado/tokyo/E.html">品川</a><a href="/yado/tokyo/map_s.html">map</a>`), "https://travel.rakuten.co.jp/yado/tokyo/map.html")
		r, e = parseAreas(d, "tokyo")
		if e != nil || len(r.Areas) != 1 || r.Areas[0].ID != "tokyo/E" || *r.Areas[0].Parent != "tokyo" {
			t.Fatalf("child areas %#v %v", r, e)
		}
	})
	t.Run("area page2", func(t *testing.T) {
		q := HotelQuery{Area: "tokyo/E", Page: 2, Limit: 1}
		d := doc([]byte(`<span>322件中31～60件表示</span><div class="hotelBox"><h2><a href="https://travel.rakuten.co.jp/HOTEL/72056/72056.html">Hotel B</a></h2><span class="rating">4.25</span><span>12件</span></div><a href="https://search.travel.rakuten.co.jp/ds/yado/tokyo/E-p3">next</a>`), "https://search.travel.rakuten.co.jp/ds/yado/tokyo/E-p2")
		r, e := parseHotelSearch(d, q)
		if e != nil || len(r.Hotels) != 1 || r.Hotels[0].ID != "72056" || r.Hotels[0].Rating == nil || r.Hotels[0].Rating.Score != 4.25 || *r.Page.NextPage != 3 || *r.Page.SourceTotal != 322 {
			t.Fatalf("page2 %#v %v", r, e)
		}
	})
	t.Run("missing property scalars null", func(t *testing.T) {
		h, e := parseHotel(doc([]byte(`<link rel="canonical" href="https://travel.rakuten.co.jp/HOTEL/1/1.html"><div id="RthNameArea">Property</div>`), "https://travel.rakuten.co.jp/HOTEL/1/1.html"), "1", false)
		if e != nil || h.Name == nil || h.Rating != nil || h.Coordinates != nil || h.Address != nil || h.CoordinatesReason == "" || h.RoomAmenities == nil {
			t.Fatalf("hotel %#v %v", h, e)
		}
	})
	t.Run("facilities and distinct property policies", func(t *testing.T) {
		body := `<link rel="canonical" href="https://travel.rakuten.co.jp/HOTEL/1/1_std.html"><li data-locate="hotel-access"><dl><dt>交通アクセス</dt><dd>駅から徒歩1分</dd></dl></li><li><dl><dt>館内設備</dt><dd><ul><li>レストラン</li><li>駐車場あり</li></ul></dd></dl></li><li><dl><dt>部屋設備・備品</dt><dd><ul><li>Wi-Fi</li></ul></dd></dl></li><li data-locate="hotel-attention"><dl><dt>条件・注意事項</dt><dd><ul><li>宿泊税を含んでいません。</li></ul></dd></dl></li><li data-locate="hotel-cancelPolicy"><dl><dt>キャンセルポリシー</dt><dd>当日80%<div data-locate="hotel-planCancelPolicy">予約画面で必ず確認</div></dd></dl></li><script>latitude=123456; longitude=234567;</script>`
		h, e := parseHotel(doc([]byte(body), "https://travel.rakuten.co.jp/HOTEL/1/1_std.html"), "1", true)
		if e != nil || len(h.HotelAmenities) != 2 || len(h.RoomAmenities) != 1 || len(h.Access) != 1 || len(h.Notes) != 1 || len(h.CancellationPolicy) == 0 || h.PolicyCaveat == nil || h.Coordinates != nil {
			t.Fatalf("details %#v %v", h, e)
		}
	})
	for _, body := range []string{`<html><input name="f_query" value="x">normal unknown layout</html>`, `<html><input name="f_query" value="x">error message</html>`} {
		_, e := parseHotelSearch(doc([]byte(body), "https://kw.travel.rakuten.co.jp/keyword/Search.do"), HotelQuery{Query: "x", Page: 1, Limit: 5})
		assertKind(t, e, "parse_error")
	}
	r, e := parseHotelSearch(doc([]byte(`<html><input name="f_query" value="x"><div id="result">該当する宿泊施設が見つかりませんでした</div></html>`), "https://kw.travel.rakuten.co.jp/keyword/Search.do"), HotelQuery{Query: "x", Page: 1, Limit: 5})
	if e != nil || r.Status != StatusNoMatches || r.Hotels == nil {
		t.Fatalf("empty hotels %#v %v", r, e)
	}
}
func TestCapturedConditionsAreComplete(t *testing.T) {
	for _, name := range []string{"two-night", "infant"} {
		data, e := os.ReadFile(filepath.Join("testdata", "source-conditions-"+name+".json"))
		if e != nil {
			t.Fatal(e)
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(data, &m) != nil || len(m) < 40 {
			t.Fatalf("source metadata lost %s", name)
		}
		value, ok := conditionValue(m["f_y4"])
		want := "0"
		if name == "infant" {
			want = "1"
		}
		if !ok || value != want || !strings.Contains(string(m["f_service"]), "null") {
			t.Fatalf("child/nullable echo lost %s", name)
		}
	}
}

func TestPropertyRelatedLinkCannotValidateWrongID(t *testing.T) {
	body := `<link rel="canonical" href="https://travel.rakuten.co.jp/HOTEL/2/2.html"><div id="RthNameArea"><a href="/HOTEL/2/2.html">Wrong hotel</a></div><a href="/HOTEL/1/1.html">Related requested hotel</a>`
	_, e := parseHotel(doc([]byte(body), "https://travel.rakuten.co.jp/HOTEL/1/1.html"), "1", false)
	assertKind(t, e, "query_mismatch")
	body = `<div id="RthNameArea">Unknown identity</div><a href="/HOTEL/1/1.html">Related hotel</a>`
	_, e = parseHotel(doc([]byte(body), "https://travel.rakuten.co.jp/HOTEL/1/1.html"), "1", false)
	assertKind(t, e, "parse_error")
	for _, body := range []string{`<input name="f_query" value="x"><div id="result">該当する施設をご覧ください。クチコミ0件</div>`, `<input name="f_query" value="x"><div id="result">unexpected broken card 0件</div>`} {
		_, e := parseHotelSearch(doc([]byte(body), "https://kw.travel.rakuten.co.jp/keyword/Search.do"), HotelQuery{Query: "x", Page: 1, Limit: 5})
		assertKind(t, e, "parse_error")
	}
}

func TestPositivePlanPhraseIsNotNoAvailability(t *testing.T) {
	q := sampleQuery()
	body := string(offerFixture(q, nil, false))
	body = strings.Replace(body, "</html>", `<div class="planList">条件に該当するプランをご紹介します。構造が不明です。</div></html>`, 1)
	_, e := parseOffers(doc([]byte(body), offerURL(q)), q)
	assertKind(t, e, "parse_error")
}

func TestUndatedNoplanCatalogueExcluded(t *testing.T) {
	q := sampleQuery()
	body := string(offerFixture(q, []fixtureRoom{{plan: "1", room: "r-", total: 20720}}, false))
	body = strings.Replace(body, "</html>", `<li class="rm-type-wrapper" id="noplan-s-double-">Room catalog with no quote</li></html>`, 1)
	r, e := parseOffers(doc([]byte(body), offerURL(q)), q)
	if e != nil || len(r.Offers) != 1 || r.Page.RowsScanned != 2 {
		t.Fatal("catalog entered inventory", e, r)
	}
}

func TestObservedKeywordNoMatchesMarker(t *testing.T) {
	q := HotelQuery{Query: "CodexNoSuchRakutenProperty202609272323", Page: 1, Limit: 5}
	body, e := os.ReadFile(filepath.Join("testdata", "keyword-no-matches.html"))
	if e != nil {
		t.Fatal(e)
	}
	r, e := parseHotelSearch(doc(body, "https://kw.travel.rakuten.co.jp/keyword/Search.do"), q)
	if e != nil || r.Status != StatusNoMatches || r.Hotels == nil || len(r.Hotels) != 0 || r.Page.HasMore {
		t.Fatalf("observed no-matches lost %#v %v", r, e)
	}
	malformed := strings.Replace(string(body), "宿泊施設が見つかりませんでした。", "宿泊施設を紹介します。", 1)
	_, e = parseHotelSearch(doc([]byte(malformed), "https://kw.travel.rakuten.co.jp/keyword/Search.do"), q)
	assertKind(t, e, "parse_error")
	_, e = parseHotelSearch(doc(body, "https://kw.travel.rakuten.co.jp/keyword/Search.do"), HotelQuery{Query: "different", Page: 1, Limit: 5})
	assertKind(t, e, "query_mismatch")
}
