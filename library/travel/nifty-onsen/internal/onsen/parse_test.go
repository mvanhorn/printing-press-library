package onsen

import (
	"strings"
	"testing"
)

func TestFacilityID(t *testing.T) {
	for _, tc := range []struct {
		in string
		ok bool
	}{{"onsen012278", true}, {Origin + "/saitamashi-onsen/onsen012278/", true}, {"https://evil.test/onsen012278/", false}, {"onsen123", false}, {Origin + "/x/onsen012278/coupon/", false}, {Origin + "/x/onsen012278/?x=1", false}} {
		t.Run(tc.in, func(t *testing.T) {
			_, e := FacilityID(tc.in)
			if (e == nil) != tc.ok {
				t.Fatalf("got %v", e)
			}
		})
	}
}
func TestCatalog(t *testing.T) {
	for _, tc := range []struct {
		in, out string
		ok      bool
	}{{"東京都", "tokyo", true}, {"hokkaido", "hokkaido", true}, {"japan", "", true}, {"bogus", "", false}} {
		v, e := ResolveRegion(tc.in)
		if v != tc.out || (e == nil) != tc.ok {
			t.Fatalf("%s: %s %v", tc.in, v, e)
		}
	}
	if len(Prefectures) != 47 {
		t.Fatal(len(Prefectures))
	}
	for _, tc := range []struct {
		names []string
		want  string
		ok    bool
	}{{[]string{"day-use", "stay"}, "c3=3072", true}, {[]string{"sauna", "natural"}, "c8=1073750016", true}, {[]string{"bogus"}, "", false}} {
		v, e := FilterParams(tc.names)
		if (e == nil) != tc.ok || e == nil && v.Encode() != tc.want {
			t.Fatalf("%v %v", v, e)
		}
	}
}
func TestSearchURL(t *testing.T) {
	for _, tc := range []struct {
		o    SearchOptions
		want string
		ok   bool
	}{{SearchOptions{Region: "tokyo", Query: "草津", Page: 2, Filters: []string{"day-use"}}, Origin + "/tokyo/search/page-2/?c3=1024&text=%E8%8D%89%E6%B4%A5", true}, {SearchOptions{Page: 0}, "", false}, {SearchOptions{Page: 1, Region: "bad"}, "", false}, {SearchOptions{Page: 1, Query: strings.Repeat("あ", 121)}, "", false}} {
		v, e := SearchURL(tc.o)
		if v != tc.want || (e == nil) != tc.ok {
			t.Fatalf("%s %v", v, e)
		}
	}
}

const searchFixture = `<html><li class="shop pr" data-onsen-id="onsen000001"><a href="/x/onsen000001/"><p class="name">Sponsored</p></a></li><li class="shop" data-onsen-id="onsen012278" data-latitude="35.69" data-longitude="139.7" data-has-coupon="1"><a href="/x/onsen012278/"><p class="name">温泉A</p></a><p class="point"><strong>4.6点</strong> / 1,234件</p><span class="accessMain">東京都 / 新宿区</span><span class="time">10:00～翌8:30</span><span class="price">入浴料金 1,200円～</span><ul class="types"><li class="oneday badgeOn">日帰り</li><li class="stay">宿泊</li></ul></li><p class="newpagerDisp"><span class="max">44</span>件中</p><a href="/tokyo/search/page-2/?c3=1024">次</a></html>`

func TestParseSearch(t *testing.T) {
	for _, tc := range []struct {
		body  string
		count int
		ok    bool
	}{{searchFixture, 1, true}, {`<div id="listResultWrap">該当する施設が見つかりません</div>`, 0, true}, {`<html>Access denied</html>`, 0, false}} {
		v, e := ParseSearch([]byte(tc.body), Origin+"/tokyo/search/", 1)
		if (e == nil) != tc.ok || len(v.Items) != tc.count {
			t.Fatalf("%+v %v", v, e)
		}
		if tc.count == 1 {
			f := v.Items[0]
			if f.ID != "onsen012278" || f.ReviewCount == nil || *f.ReviewCount != 1234 || f.MinPriceJPY == nil || *f.MinPriceJPY != 1200 || f.DayUse == nil || !*f.DayUse || f.Stay != nil || v.NextURL == nil || *v.Total != 44 {
				t.Fatalf("%+v", v)
			}
		}
	}
}
func field(k, v string) string { return `<div><h4>` + k + `</h4></div><div><p>` + v + `</p></div>` }
func TestParseDetail(t *testing.T) {
	for _, tc := range []struct {
		label                  string
		natural, private, room string
	}{{"天然温泉、貸切風呂、個室", "source_claim", "source_claim", "source_claim"}, {"貸切風呂、個室風呂", "unknown", "unknown", "unknown"}, {"家族風呂", "unknown", "unknown", "unknown"}, {"スーパー銭湯", "unknown", "unknown", "unknown"}} {
		labels := ""
		parts := strings.Split(tc.label, "、")
		if tc.label == "貸切風呂、個室風呂" {
			parts = []string{tc.label}
		}
		for _, s := range parts {
			labels += `<span>` + s + `</span>`
		}
		b := `<html><link rel="canonical" href="` + Origin + `/x/onsen012278/"><h1>温泉A</h1><aside>タトゥーOK 天然温泉 貸切風呂</aside>` + field("料金", "平日1,265円 土日祝1,520円<br>タオル330円<br>オムツのお子様は浴場に入れません") + field("営業時間", "9:00～25:00<br>最終受付24:00") + field("交通アクセス", "駅徒歩9分") + field("特徴", labels) + `<a class="badge is-disabled" data-event-action="detailIntro">天然温泉</a><a class="badge" data-event-action="detailIntro" data-event-category="kashikiriIconClick_smp">貸切風呂</a><section>口コミ: 貸切風呂 タトゥー歓迎</section></html>`
		v, e := ParseDetail([]byte(b), Origin+"/x/onsen012278/", "onsen012278")
		if e != nil {
			t.Fatal(e)
		}
		if v.NaturalHotSpring.State != tc.natural || v.PrivateRentableBath.State != tc.private || v.PrivateRoom.State != tc.room {
			t.Fatalf("%s %+v", tc.label, v)
		}
		if v.Policies["tattoo"].State != "unknown" || v.Policies["children"].State != "source_text" || v.ReservableInventory != nil || !strings.Contains(*v.Admission, "土日祝1,520円") || !strings.Contains(*v.Hours, "最終受付24:00") {
			t.Fatalf("%+v", v)
		}
	}
	if _, e := ParseDetail([]byte(`<h1>Wrong</h1>`), Origin+"/x/onsen000001/", "onsen012278"); e == nil {
		t.Fatal("identity accepted")
	}
}
func TestParseCoupons(t *testing.T) {
	for _, tc := range []struct {
		body  string
		count int
		ok    bool
	}{{`<link rel="canonical" href="` + Origin + `/x/onsen012278/coupon/"><li class="couponItem AppCouponItem" id="123_cidx"><h3 class="couponTtl">＜ペアのみ＞アプリ限定</h3><p class="couponDiscount">1人あたり 平日2,000円→1,800円 土日祝2,800円→2,520円</p><span class="couponValid">有効期限：2026年12月31日まで</span><ul><li class="couponDiscountItem">2名同時入館。他割引併用不可。おふろパス登録が必要</li></ul></li>`, 1, true}, {`クーポンはありません`, 0, true}, {`Access denied`, 0, false}} {
		v, e := ParseCoupons([]byte(tc.body), Origin+"/x/onsen012278/coupon/", "onsen012278")
		if (e == nil) != tc.ok || len(v.Coupons) != tc.count {
			t.Fatalf("%+v %v", v, e)
		}
		if tc.count > 0 {
			c := v.Coupons[0]
			if c.Details == nil || !strings.Contains(*c.Details, "2名同時入館") || c.AppOnly == nil || !*c.AppOnly || c.Membership == "unknown" || c.ValidUntil == nil || *c.ValidUntil != "2026-12-31" || !strings.Contains(*c.PriceText, "1人あたり") || len(c.Conditions) == 0 {
				t.Fatalf("%+v", c)
			}
		}
	}
}
func TestParseNearby(t *testing.T) {
	for _, tc := range []struct {
		body  string
		count int
		ok    bool
	}{{`{"result":1,"response":{"onsen_list":[{"onsen_id":"onsen012278","onsen_name":"近い","onsen_url":"https://onsen.nifty.com/x/onsen012278/","onsen_lat":"35.69","onsen_lon":"139.7","day_flg":true,"stay_flg":false,"coupon_flg":false},{"onsen_id":"onsen000001","onsen_name":"遠い","onsen_lat":"36","onsen_lon":"140"}]}}`, 2, true}, {`{"result":1,"response":{"onsen_list":[]}}`, 0, true}, {`{"result":2,"message":"__illegal_request"}`, 0, false}, {`{"result":1,"response":{}}`, 0, false}} {
		v, e := ParseNearby([]byte(tc.body), 35.69, 139.7)
		if (e == nil) != tc.ok || len(v) != tc.count {
			t.Fatalf("%+v %v", v, e)
		}
		if tc.count > 0 {
			if *v[0].DistanceKM != 0 || v[0].URL != Origin+"/x/onsen012278/" || v[0].DayUse == nil || !*v[0].DayUse || v[0].Stay == nil || *v[0].Stay || v[0].CouponAvailable == nil || *v[0].CouponAvailable {
				t.Fatalf("nearby lost source facts: %+v", v[0])
			}
		}
	}
}
func TestDistanceAndNearbyValidation(t *testing.T) {
	for _, tc := range []struct{ a, b, c, d, w float64 }{{35, 139, 35, 139, 0}, {0, 0, 0, 1, 111.195}} {
		if got := DistanceKM(tc.a, tc.b, tc.c, tc.d); got != tc.w {
			t.Fatal(got)
		}
	}
	for _, tc := range []struct {
		o  NearbyOptions
		ok bool
	}{{NearbyOptions{Latitude: 35, Longitude: 139, Zoom: 10}, true}, {NearbyOptions{Latitude: 0, Longitude: 139, Zoom: 10}, false}, {NearbyOptions{Latitude: 35, Longitude: 139, Zoom: 30}, false}} {
		if e := ValidateNearby(tc.o); (e == nil) != tc.ok {
			t.Fatal(e)
		}
	}
}
