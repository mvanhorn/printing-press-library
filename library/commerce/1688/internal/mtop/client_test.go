package mtop

import (
	"encoding/json"
	"testing"
)

func TestParsePercent(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"11%", 11},
		{"41%", 41},
		{"0%", 0},
		{"", 0},
		{"abc", 0},
		{"7.5%", 7.5},
	}
	for _, c := range cases {
		if got := parsePercent(c.in); got != c.want {
			t.Errorf("parsePercent(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseFloatInt(t *testing.T) {
	if got := parseFloat("7.99"); got != 7.99 {
		t.Errorf("parseFloat = %v", got)
	}
	if got := parseFloat("bad"); got != 0 {
		t.Errorf("parseFloat(bad) = %v, want 0", got)
	}
	if got := parseInt("3163"); got != 3163 {
		t.Errorf("parseInt = %v", got)
	}
	if got := parseInt(""); got != 0 {
		t.Errorf("parseInt(empty) = %v, want 0", got)
	}
}

func TestFirstImage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a.jpg,b.jpg,c.jpg", "a.jpg"},
		{"only.jpg", "only.jpg"},
		{"", ""},
	}
	for _, c := range cases {
		if got := firstImage(c.in); got != c.want {
			t.Errorf("firstImage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSortType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"price-asc", "price-asc"},
		{"price-desc", "price-desc"},
		{"booked", "booked"},
		{"sales", "booked"},
		{"newest", "newOffer"},
		{"", ""},
		{"garbage", ""},
	}
	for _, c := range cases {
		if got := sortType(c.in); got != c.want {
			t.Errorf("sortType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHasServiceTag(t *testing.T) {
	tags := []string{"深度验厂", "24小时发货"}
	if !hasServiceTag(tags, "深度验厂") {
		t.Error("expected to find 深度验厂")
	}
	if hasServiceTag(tags, "缺失") {
		t.Error("did not expect to find 缺失")
	}
}

func TestDecodeOffer(t *testing.T) {
	raw := json.RawMessage(`{
		"offerId": "927875250705",
		"title": "<font color=red>手机壳</font>磁吸",
		"priceInfo": {"price": "7.99", "priceDescription": "限时价"},
		"afterPrice": {"text": "全网10万+件"},
		"bookedCount": "3163",
		"offerRepurchaseRate": "11%",
		"factoryInspection": "true",
		"superFactory": "false",
		"businessInspection": "false",
		"isP4P": "false",
		"province": "广东",
		"city": "佛山市",
		"memberId": "b2b-2850655109d72ea",
		"offerTags": {"serviceTags": ["深度验厂"]},
		"shop": {"text": "佛山市南海区三丰手机配件有限公司", "loginIdOfUtf8": "fssf06"},
		"shopAddition": {"shopLinkUrl": "http://shop.1688.com", "tradeService": {"compositeNewScore": "5.0", "logisticsScore": "4.57", "disputeScore": "4.0", "consultationScore": "4.5"}},
		"list": {"cover": {"pic": "https://img/a.jpg,https://img/b.jpg"}}
	}`)
	o, ok := decodeOffer(raw)
	if !ok {
		t.Fatal("decodeOffer returned ok=false")
	}
	if o.OfferID != "927875250705" {
		t.Errorf("OfferID = %q", o.OfferID)
	}
	if o.Title != "手机壳磁吸" {
		t.Errorf("Title font tags not stripped: %q", o.Title)
	}
	if o.PriceCNY != 7.99 {
		t.Errorf("PriceCNY = %v", o.PriceCNY)
	}
	if o.TransactionCount != 3163 {
		t.Errorf("TransactionCount = %v", o.TransactionCount)
	}
	if o.RepurchasePct != 11 {
		t.Errorf("RepurchasePct = %v", o.RepurchasePct)
	}
	if !o.FactoryInspection || o.SuperFactory {
		t.Errorf("factory flags: inspection=%v super=%v", o.FactoryInspection, o.SuperFactory)
	}
	if !o.VerifiedFactory {
		t.Error("expected VerifiedFactory true (factory_inspection || 深度验厂)")
	}
	if o.TradeComposite != 5.0 {
		t.Errorf("TradeComposite = %v", o.TradeComposite)
	}
	if o.Image != "https://img/a.jpg" {
		t.Errorf("Image = %q", o.Image)
	}
	if o.SupplierLocation != "广东 佛山市" {
		t.Errorf("SupplierLocation = %q", o.SupplierLocation)
	}
	if o.URL != "https://detail.1688.com/offer/927875250705.html" {
		t.Errorf("URL = %q", o.URL)
	}
}

func TestDecodeOfferEmptyID(t *testing.T) {
	if _, ok := decodeOffer(json.RawMessage(`{"title":"no id"}`)); ok {
		t.Error("expected ok=false for offer with no offerId")
	}
}
