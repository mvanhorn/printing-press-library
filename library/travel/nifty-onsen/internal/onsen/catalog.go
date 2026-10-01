package onsen

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type Region struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Group string `json:"group"`
	URL   string `json:"url"`
}

var Prefectures = func() []Region {
	rows := []string{
		"hokkaido|北海道|hokkaido", "aomori|青森県|tohoku", "iwate|岩手県|tohoku", "miyagi|宮城県|tohoku", "akita|秋田県|tohoku", "yamagata|山形県|tohoku", "fukushima|福島県|tohoku",
		"ibaraki|茨城県|kanto", "tochigi|栃木県|kanto", "gunma|群馬県|kanto", "saitama|埼玉県|kanto", "chiba|千葉県|kanto", "tokyo|東京都|kanto", "kanagawa|神奈川県|kanto",
		"niigata|新潟県|hokuriku-koshinetsu", "toyama|富山県|hokuriku-koshinetsu", "ishikawa|石川県|hokuriku-koshinetsu", "fukui|福井県|hokuriku-koshinetsu", "yamanashi|山梨県|hokuriku-koshinetsu", "nagano|長野県|hokuriku-koshinetsu",
		"gifu|岐阜県|tokai", "shizuoka|静岡県|tokai", "aichi|愛知県|tokai", "mie|三重県|tokai",
		"shiga|滋賀県|kinki", "kyoto|京都府|kinki", "osaka|大阪府|kinki", "hyogo|兵庫県|kinki", "nara|奈良県|kinki", "wakayama|和歌山県|kinki",
		"tottori|鳥取県|chugoku", "shimane|島根県|chugoku", "okayama|岡山県|chugoku", "hiroshima|広島県|chugoku", "yamaguchi|山口県|chugoku",
		"tokushima|徳島県|shikoku", "kagawa|香川県|shikoku", "ehime|愛媛県|shikoku", "kochi|高知県|shikoku",
		"fukuoka|福岡県|kyushu-okinawa", "saga|佐賀県|kyushu-okinawa", "nagasaki|長崎県|kyushu-okinawa", "kumamoto|熊本県|kyushu-okinawa", "oita|大分県|kyushu-okinawa", "miyazaki|宮崎県|kyushu-okinawa", "kagoshima|鹿児島県|kyushu-okinawa", "okinawa|沖縄県|kyushu-okinawa"}
	out := make([]Region, 0, len(rows))
	for _, s := range rows {
		p := strings.Split(s, "|")
		out = append(out, Region{p[0], p[1], p[2], Origin + "/" + p[0] + "/search/"})
	}
	return out
}()

func ResolveRegion(s string) (string, error) {
	if s == "" || s == "japan" {
		return "", nil
	}
	for _, r := range Prefectures {
		if s == r.Slug || s == r.Name {
			return r.Slug, nil
		}
	}
	return "", fmt.Errorf("unknown prefecture %q; run regions for Japanese names and source slugs", s)
}

type Filter struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Parameter string `json:"parameter"`
	Value     uint64 `json:"value"`
	Caveat    string `json:"caveat,omitempty"`
}

var Filters = []Filter{
	{"day-use", "日帰り温泉", "c3", 1024, "Source day-use classification; availability is unknown."},
	{"stay", "宿泊", "c3", 2048, "Includes hotel/ryokan listings; add day-use to find source-marked day-use."},
	{"coupon", "クーポンあり", "s4", 1, "Public information only; check eligibility and validity."},
	{"natural", "天然温泉", "c8", 8192, "Source claim, not independent certification."},
	{"sauna", "サウナ", "c8", 1073741824, ""},
	{"outdoor", "露天風呂", "c8", 512, ""},
	{"rock-bath", "岩盤浴", "c8", 67108864, "May incur an additional charge."},
	{"private-bath-or-room", "貸切風呂、個室風呂", "c8", 32768, "Combined source category does not establish a rentable private bath or a private room."},
	{"family-bath", "家族風呂", "c8", 134217728, "Source feature; rental, charge and current inventory unknown."},
	{"day-private-bath", "日帰り貸切風呂", "c8", 2048, "Source category; rental terms and available sessions require facility evidence."},
	{"private-outdoor", "貸切露天風呂", "c8", 16384, "Source feature; no inventory guarantee."},
	{"free-flowing", "源泉かけ流し", "c8", 8388608, "Source claim."},
	{"private-sauna", "個室サウナ", "c8", 64, "Distinct from a private bath."},
	{"parking", "駐車場あり", "c6", 33554432, "Fees and capacity may vary."},
	{"rest-area", "休憩所・休憩室", "c6", 268435456, "Distinct from a private room."},
	{"restaurant", "レストラン", "c6", 1073741824, ""},
	{"barrier-free", "バリアフリー", "c6", 4194304, "Source feature does not establish accessible bathing facilities or individual suitability."},
}

func FilterParams(names []string) (url.Values, error) {
	bits := map[string]uint64{}
	for _, n := range names {
		found := false
		for _, f := range Filters {
			if n == f.Name {
				bits[f.Parameter] |= f.Value
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown filter %q; run filters for supported names", n)
		}
	}
	p := url.Values{}
	for k, v := range bits {
		p.Set(k, strconv.FormatUint(v, 10))
	}
	return p, nil
}
func conditionParams(p url.Values) string {
	var out []string
	for _, key := range []string{"c3", "c6", "c8", "s4"} {
		if p.Get(key) != "" {
			out = append(out, key+"_"+p.Get(key))
		}
	}
	return strings.Join(out, ",")
}
