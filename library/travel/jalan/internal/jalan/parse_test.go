package jalan

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const parserSourceURL = "https://www.jalan.net/uw/uwp3200/uww3201init.do?yadNo=385995&stayYear=2026&stayMonth=11&stayDay=10&stayCount=1&adultNum=2&roomCount=1&planCd=03912759&roomTypeCd=0576806"

func parserFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".html")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func assertBool(t *testing.T, name string, got *bool, want bool) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
func assertAmount(t *testing.T, got Price, want int64) {
	t.Helper()
	if got.Amount == nil || *got.Amount != want {
		t.Fatalf("base quote = %v, want %d", got.Amount, want)
	}
}

func TestParseSearchOrganicPagination(t *testing.T) {
	page, err := ParseSearch(parserFixture(t, "search-pagination"), "https://www.jalan.net/140000/LRG_141600/?idx=0")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 30 || page.Total == nil || *page.Total != 142 || !page.HasNext || page.NoResults {
		t.Fatalf("pagination: items=%d total=%v next=%v empty=%v", len(page.Items), page.Total, page.HasNext, page.NoResults)
	}
	if page.Items[0].ID != "385995" || page.Items[1].ID != "371898" {
		t.Fatalf("organic order shifted by sponsored cards: %s, %s", page.Items[0].ID, page.Items[1].ID)
	}
	seen := map[string]bool{}
	for _, p := range page.Items {
		if seen[p.ID] {
			t.Fatalf("duplicate property %s", p.ID)
		}
		seen[p.ID] = true
	}
	if !seen["348397"] {
		t.Fatal("sponsored duplicate must not remove organic appearance")
	}
	p := page.Items[0]
	if p.NameJa != "箱根湯本温泉　ホテル南風荘" {
		t.Fatalf("Japanese name = %q", p.NameJa)
	}
	assertAmount(t, p.Price, 50600)
	if p.Price.Basis != "whole_stay" || !strings.Contains(p.Price.QuoteText, "～") || !strings.Contains(p.Price.OccupancyText, "大人2名") {
		t.Fatalf("search minimum lost scope: %+v", p.Price)
	}
	if p.RoomBaths.HotSpring != nil || p.Baths.HotSpring != nil {
		t.Fatal("marketing description established unscoped bath facts")
	}
}

func TestParseOffersRoomPlanRelationships(t *testing.T) {
	doc := parserFixture(t, "offers")
	// A repeated exact tuple does not duplicate an offer. The same room in
	// other plans remains a distinct, selectable relationship.
	start := strings.Index(doc, "<tbody>")
	end := strings.Index(doc[start:], "</tbody>") + start + len("</tbody>")
	doc = strings.Replace(doc, "</table>", doc[start:end]+"</table>", 1)
	page, err := ParseOffers(doc, parserSourceURL, "385995")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 54 || page.Total != nil || page.PlanTotal == nil || *page.PlanTotal != 12 {
		t.Fatalf("plan/room counts: offers=%d total=%v plan_total=%v", len(page.Items), page.Total, page.PlanTotal)
	}
	o := page.Items[0]
	if o.PlanID != "03806855" || o.RoomID != "0380486" || !strings.Contains(o.RoomName, "和モダンスーペリアツイン") {
		t.Fatalf("room identity: %+v", o)
	}
	assertAmount(t, o.Price, 66000)
	if o.Price.Basis != "whole_stay" || o.Price.Points != "1,320 ポイント" || !strings.Contains(o.Price.ConditionalDiscount, "6,000円") {
		t.Fatalf("separate quote/coupon/points: %+v", o.Price)
	}
	assertBool(t, "room attached bath", o.Baths.InRoom, true)
	assertBool(t, "outdoor", o.Baths.Outdoor, true)
	assertBool(t, "indoor qualifier", o.Baths.Indoor, false)
	assertBool(t, "room hot spring", o.Baths.HotSpring, true)
	if o.Baths.Private != nil || o.Baths.PrivateReservable != nil {
		t.Fatal("room outdoor bath inferred shared reservable bath")
	}
	negative := false
	for _, e := range o.Baths.Evidence {
		negative = negative || strings.Contains(e.Text, "バスなし")
	}
	if !negative {
		t.Fatal("negative indoor qualifier was discarded")
	}
	if o.Meals != "breakfast_dinner" || o.Smoking != "non_smoking" {
		t.Fatalf("room labels = meals %s smoking %s", o.Meals, o.Smoking)
	}
	plans := map[string]bool{}
	for _, item := range page.Items {
		if item.RoomID == "0546600" {
			plans[item.PlanID] = true
		}
	}
	if len(plans) < 2 {
		t.Fatal("room reused in distinct plans was de-duplicated")
	}
}

func TestParsePropertyBathScopesAndJapaneseReviews(t *testing.T) {
	p, err := ParseProperty(parserFixture(t, "property-371898"), "https://www.jalan.net/yad371898/", "371898")
	if err != nil {
		t.Fatal(err)
	}
	if p.NameJa != "季の湯　雪月花（共立リゾート）" || p.LodgingType != "旅館" || !strings.Contains(p.Address, "強羅１３００") {
		t.Fatalf("property facts: name=%q type=%q address=%q", p.NameJa, p.LodgingType, p.Address)
	}
	assertBool(t, "shared onsen", p.Baths.HotSpring, true)
	assertBool(t, "shared outdoor", p.Baths.Outdoor, true)
	assertBool(t, "private use", p.Baths.Private, true)
	assertBool(t, "reservable", p.Baths.PrivateReservable, false)
	assertBool(t, "room attached", p.RoomBaths.InRoom, true)
	assertBool(t, "room outdoor", p.RoomBaths.Outdoor, true)
	assertBool(t, "room hot spring", p.RoomBaths.HotSpring, false)
	qualified := false
	for _, e := range p.RoomBaths.Evidence {
		qualified = qualified || strings.Contains(e.Text, "温泉ではございません")
	}
	if !qualified {
		t.Fatal("room negative qualification lost")
	}
	for _, label := range []string{"総合", "部屋", "風呂", "料理（朝食）", "料理（夕食）", "接客・サービス", "清潔感"} {
		if _, ok := p.ReviewCategories[label]; !ok {
			t.Errorf("source review category %q missing", label)
		}
	}
	if p.Price.Amount != nil {
		t.Fatal("property reference inferred dated inventory price")
	}
	p, err = ParseProperty(parserFixture(t, "property-385995"), "https://www.jalan.net/yad385995/", "385995")
	if err != nil {
		t.Fatal(err)
	}
	if p.Baths.Private != nil || p.Baths.PrivateReservable != nil {
		t.Fatal("outdoor 貸切不可 wrongly proves absence of every private bath")
	}
	if !strings.Contains(strings.Join(p.Amenities, " "), "× パジャマ") {
		t.Fatal("amenity absence marker discarded")
	}
}

func TestParsePlanExactQuoteAndTerms(t *testing.T) {
	p, err := ParsePlan(parserFixture(t, "plan"), parserSourceURL, "385995", "03912759", "0576806")
	if err != nil {
		t.Fatal(err)
	}
	assertAmount(t, p.Price, 74800)
	if p.Price.Basis != "whole_stay" || p.Price.TaxInclusion != "included" || p.Price.ServiceChargeInclusion != "included" || p.Price.FinalPayable != nil {
		t.Fatalf("quote scope: %+v", p.Price)
	}
	if !strings.Contains(p.Price.ConditionalDiscount, "68,800円") || !strings.Contains(p.Price.ConditionalDiscount, "6,000円") || p.Price.Points != "1,496 ポイントたまる" {
		t.Fatalf("conditional quote fields: %+v", p.Price)
	}
	if !strings.Contains(p.Fees, "入湯税大人150円") || p.Price.ExtraFees != p.Fees || !strings.Contains(p.Cancellation, "7日～5日前") || !strings.Contains(p.Cancellation, "無連絡キャンセル 宿泊料金の100%") {
		t.Fatalf("fee/cancellation terms lost: %q / %q", p.Fees, p.Cancellation)
	}
	if strings.Contains(p.Cancellation, "クーポン") || strings.Contains(p.Fees, "クーポン") {
		t.Fatal("coupon advertising polluted fee/cancellation")
	}
	if p.CheckIn != "15:00～19:00" || p.CheckOut != "～11:00" || !strings.Contains(p.BookingDeadline, "17時30分") {
		t.Fatalf("check-in/deadline: %q %q %q", p.CheckIn, p.CheckOut, p.BookingDeadline)
	}
	if !strings.Contains(p.PriceBreakdown, "37,400円") || !strings.Contains(p.OccupancyText, "× 2名") || len(p.Restrictions) == 0 || !strings.Contains(p.Description, "アレルギー対応") {
		t.Fatal("occupancy or raw Japanese restrictions lost")
	}
	assertBool(t, "room onsen", p.Baths.HotSpring, true)
	assertBool(t, "room outdoor", p.Baths.Outdoor, true)
}

func TestParsePlanWholeStayAndFamilyBreakdowns(t *testing.T) {
	p, err := ParsePlan(parserFixture(t, "two-night-plan"), parserSourceURL+"&stayCount=2", "385995", "03912759", "0576806")
	if err != nil {
		t.Fatal(err)
	}
	assertAmount(t, p.Price, 149600)
	if p.Price.Basis != "whole_stay" || !strings.Contains(p.OccupancyText, "2 泊") || !strings.Contains(p.PriceBreakdown, "2泊目") {
		t.Fatalf("two-night total mistaken for nightly amount: %+v", p.Price)
	}
	p, err = ParsePlan(parserFixture(t, "family-plan"), parserSourceURL, "385995", "03806855", "0546600")
	if err != nil {
		t.Fatal(err)
	}
	assertAmount(t, p.Price, 124740)
	if p.Price.Basis != "whole_stay" || !strings.Contains(p.OccupancyText, "2部屋目") || strings.Count(p.OccupancyText, "（小学生）") != 2 || !strings.Contains(p.OccupancyText, "16,170円") {
		t.Fatalf("family quote must retain both rooms and child lines: %q", p.OccupancyText)
	}
}

func TestUndatedFallbackIsNotADatedBaseQuote(t *testing.T) {
	doc := parserFixture(t, "newyear-plan")
	p, err := ParsePlan(doc, strings.Replace(parserSourceURL, "stayMonth=11&stayDay=10", "stayMonth=12&stayDay=31", 1), "385995", "03912759", "0576806")
	if err != nil {
		t.Fatal(err)
	}
	if p.Availability != "unavailable" || p.Price.Amount != nil || p.Price.Basis != "unknown" || p.Price.Points != "" || !strings.Contains(p.Price.ReferenceQuote, "68,200円～") {
		t.Fatalf("reference price misreported for unavailable date: availability=%s price=%+v", p.Availability, p.Price)
	}
	withoutRejection := strings.ReplaceAll(doc, "ご指定された条件では以下のいずれかの理由でご利用いただけません。", "条件について")
	if _, err = ParsePlan(withoutRejection, parserSourceURL, "385995", "03912759", "0576806"); err == nil {
		t.Fatal("unexplained undated fallback succeeded")
	}
}

func TestParserZeroRequiresPositiveSourceEvidence(t *testing.T) {
	search, err := ParseSearch(`<div class="jlnpc-planListCnt-header">0軒ありました。</div>`, parserSourceURL)
	if err != nil || !search.NoResults || len(search.Items) != 0 {
		t.Fatalf("explicit search zero: %+v %v", search, err)
	}
	offers, err := ParseOffers(parserFixture(t, "no-offers"), parserSourceURL, "385995")
	if err != nil || !offers.NoResults || len(offers.Items) != 0 || offers.Total == nil || *offers.Total != 0 {
		t.Fatalf("native no offers: %+v %v", offers, err)
	}
	if !strings.Contains(strings.Join(offers.Warnings, " "), "予約受付を停止中") {
		t.Fatal("no-inventory/reservation-stop ambiguity discarded")
	}
	wrong := `<html><h1>アクセスエラー</h1><p>現在ご利用いただけません。</p></html>`
	if _, err = ParseSearch(wrong, parserSourceURL); err == nil {
		t.Fatal("wrong search page became zero")
	}
	if _, err = ParseOffers(wrong, parserSourceURL, "385995"); err == nil {
		t.Fatal("wrong offers page became zero")
	}
	if _, err = ParseProperty(wrong, parserSourceURL, "385995"); err == nil {
		t.Fatal("wrong property page succeeded")
	}
	if _, err = ParsePlan(wrong, parserSourceURL, "385995", "03912759", "0576806"); err == nil {
		t.Fatal("wrong plan page succeeded")
	}
	if _, err = ParsePlan(parserFixture(t, "plan"), parserSourceURL, "385995", "03806855", "0576806"); err == nil {
		t.Fatal("different plan identity succeeded")
	}
}

func TestPriceUnitsAndSmokingAmbiguity(t *testing.T) {
	for _, tc := range []struct{ text, basis string }{
		{"1泊1部屋の大人1名分(税込・サービス料込)", "per_person_per_night"},
		{"1泊1部屋(税込)", "per_room_per_night"},
		{"2泊 大人2名 合計(税込)", "whole_stay"},
		{"税込", "unknown"},
	} {
		if p := priceQuote("16,700円", tc.text, parserSourceURL); p.Basis != tc.basis {
			t.Errorf("unit %q = %s want %s", tc.text, p.Basis, tc.basis)
		}
	}
	if smoking([]string{"禁煙ではありません"}) != "unknown" {
		t.Fatal("negation became non-smoking")
	}
	doc := parserFixture(t, "plan")
	doc = strings.ReplaceAll(doc, "禁煙ルーム", "喫煙ルーム")
	doc = strings.ReplaceAll(doc, "■禁煙室", "禁煙ルームをご希望の場合は別タイプをご検討ください")
	p, err := ParsePlan(doc, parserSourceURL, "385995", "03912759", "0576806")
	if err != nil {
		t.Fatal(err)
	}
	if p.Smoking != "smoking" {
		t.Fatalf("alternative-room mention overrode current-room label: %s", p.Smoking)
	}
	if smoking([]string{"禁煙ルーム", "喫煙ルーム"}) != "unknown" {
		t.Fatal("contradictory source labels were silently resolved")
	}
	contradictory := strings.Replace(parserFixture(t, "plan"), "禁煙ルーム</li>", "禁煙ルーム</li><li class=\"c-label\">喫煙ルーム</li>", 1)
	p, err = ParsePlan(contradictory, parserSourceURL, "385995", "03912759", "0576806")
	if err != nil {
		t.Fatal(err)
	}
	if p.Smoking != "unknown" {
		t.Fatal("description resolved contradictory current-room labels")
	}

}

func TestParserJSONKeepsUnknownsAndPlanFields(t *testing.T) {
	p, err := ParsePlan(parserFixture(t, "plan"), parserSourceURL, "385995", "03912759", "0576806")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err = json.Unmarshal(b, &item); err != nil {
		t.Fatal(err)
	}
	if item["cancellation"] == nil || item["price_breakdown"] == nil || item["restrictions"] == nil {
		t.Fatal("embedded offer JSON discarded plan fields")
	}
	property := emptyProperty("385995", parserSourceURL)
	b, err = json.Marshal(property)
	if err != nil {
		t.Fatal(err)
	}
	item = nil
	if err = json.Unmarshal(b, &item); err != nil {
		t.Fatal(err)
	}
	if item["address"] != nil || item["lodging_type"] != nil || item["amenities"] == nil || item["review_categories"] == nil {
		t.Fatalf("unknowns/collections: %s", b)
	}
	price := item["price"].(map[string]any)
	if price["amount"] != nil || price["extra_fees"] != nil || price["tax_inclusion"] != "unknown" {
		t.Fatalf("unknown price: %v", price)
	}
}
