package walkerplus

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// pp:novel-static-reference
// Routes and Japanese labels are copied from Walkerplus navigation captures.
var prefectures = []CatalogItem{
	pref("ar0101", "北海道", "hokkaido"), pref("ar0202", "青森県", "aomori"), pref("ar0203", "岩手県", "iwate"), pref("ar0204", "宮城県", "miyagi"), pref("ar0205", "秋田県", "akita"), pref("ar0206", "山形県", "yamagata"), pref("ar0207", "福島県", "fukushima"),
	pref("ar0308", "茨城県", "ibaraki"), pref("ar0309", "栃木県", "tochigi"), pref("ar0310", "群馬県", "gunma"), pref("ar0311", "埼玉県", "saitama"), pref("ar0312", "千葉県", "chiba"), pref("ar0313", "東京都", "tokyo"), pref("ar0314", "神奈川県", "kanagawa"),
	pref("ar0415", "新潟県", "niigata"), pref("ar0516", "富山県", "toyama"), pref("ar0517", "石川県", "ishikawa"), pref("ar0518", "福井県", "fukui"), pref("ar0419", "山梨県", "yamanashi"), pref("ar0420", "長野県", "nagano"),
	pref("ar0621", "岐阜県", "gifu"), pref("ar0622", "静岡県", "shizuoka"), pref("ar0623", "愛知県", "aichi"), pref("ar0624", "三重県", "mie"), pref("ar0725", "滋賀県", "shiga"), pref("ar0726", "京都府", "kyoto"), pref("ar0727", "大阪府", "osaka"), pref("ar0728", "兵庫県", "hyogo"), pref("ar0729", "奈良県", "nara"), pref("ar0730", "和歌山県", "wakayama"),
	pref("ar0831", "鳥取県", "tottori"), pref("ar0832", "島根県", "shimane"), pref("ar0833", "岡山県", "okayama"), pref("ar0834", "広島県", "hiroshima"), pref("ar0835", "山口県", "yamaguchi"), pref("ar0936", "徳島県", "tokushima"), pref("ar0937", "香川県", "kagawa"), pref("ar0938", "愛媛県", "ehime"), pref("ar0939", "高知県", "kochi"),
	pref("ar1040", "福岡県", "fukuoka"), pref("ar1041", "佐賀県", "saga"), pref("ar1042", "長崎県", "nagasaki"), pref("ar1043", "熊本県", "kumamoto"), pref("ar1044", "大分県", "oita"), pref("ar1045", "宮崎県", "miyazaki"), pref("ar1046", "鹿児島県", "kagoshima"), pref("ar1047", "沖縄県", "okinawa"),
}

var categoryItems = []CatalogItem{
	category("eg0051", "季節のイベント", "seasonal"), category("eg0055", "祭り", "festivals", "festival"), category("eg0101", "花見", "cherry-blossoms"), category("eg0102", "花火", "fireworks"), category("eg0103", "紅葉", "foliage"), category("eg0104", "イルミネーション", "illuminations"), category("eg0105", "カウントダウン", "countdown"), category("eg0130", "花・自然", "flowers", "nature"), category("eg0131", "ライトアップ", "light-up"), category("eg0133", "年中行事・歳時記", "annual-events"), category("eg0134", "味覚狩り・フルーツ狩り", "fruit-picking"), category("eg0141", "クリスマスイベント", "christmas"), category("eg0144", "福袋・初売り", "new-year-sales"),
	category("eg0135", "祭り", "local-festivals"), category("eg0052", "食べる・買う", "food-shopping"), category("eg0106", "バーゲンセール", "sales"), category("eg0115", "フリーマーケット", "markets", "flea-markets"), category("eg0117", "グルメ・フードフェス", "food-festivals"), category("eg0118", "物産展・観光フェア", "product-fairs"), category("eg0145", "商業施設イベント", "shopping-events"),
	category("eg0053", "文化・芸術・スポーツ", "culture"), category("eg0107", "美術展・博物展", "museums", "art-exhibitions"), category("eg0108", "スポーツイベント", "sports"), category("eg0109", "ライブ・音楽イベント", "music"), category("eg0110", "映画イベント", "film"), category("eg0111", "舞台・演劇", "theater"), category("eg0114", "伝統芸能・お笑いライブ", "performances"), category("eg0140", "フェスティバル・パレード", "parades"),
	category("eg0054", "趣味・生活", "hobbies"), category("eg0120", "体験イベント・アクティビティ", "activities"), category("eg0124", "動物関連イベント", "animals"), category("eg0125", "講演会・トークショー", "talks"), category("eg0126", "展示会", "exhibitions"), category("eg0127", "アニメ・ゲーム", "anime-games"), category("eg0056", "その他", "other"), category("eg0123", "その他のイベント", "other-events"),
}

var knownCities = []CatalogItem{
	city("ar0101100", "札幌市", "sapporo", "ar0101"), city("ar0204100", "仙台市", "sendai", "ar0204"), city("ar0314100", "横浜市", "yokohama", "ar0314"), city("ar0623100", "名古屋市", "nagoya", "ar0623"), city("ar0728100", "神戸市", "kobe", "ar0728"),
	city("ar0313113", "渋谷区", "shibuya", "ar0313"), city("ar0313108", "江東区", "koto", "ar0313"), city("ar0313103", "港区", "minato", "ar0313"), city("ar0313106", "台東区", "taito", "ar0313"), city("ar0313116", "豊島区", "toshima", "ar0313"),
}

func pref(code, name, alias string) CatalogItem {
	return CatalogItem{Code: code, NameJA: name, Aliases: []string{alias}, Path: "/event_list/" + code + "/", Kind: "prefecture"}
}
func category(code, name string, aliases ...string) CatalogItem {
	return CatalogItem{Code: code, NameJA: name, Aliases: aliases, Path: "/event_list/" + code + "/", Kind: "category"}
}
func city(code, name, alias, parent string) CatalogItem {
	return CatalogItem{Code: code, NameJA: name, Aliases: []string{alias}, Path: "/event_list/" + code + "/" + alias + "/", Kind: "city", PrefectureCode: strptr(parent)}
}

func lookup(items []CatalogItem, value string) (CatalogItem, bool) {
	for _, item := range items {
		if value == item.Code || value == item.NameJA {
			return item, true
		}
		for _, alias := range item.Aliases {
			if strings.EqualFold(value, alias) {
				return item, true
			}
		}
	}
	return CatalogItem{}, false
}

var cityCodeRE = regexp.MustCompile(`^ar[0-9]{7}$`)
var eventIDRE = regexp.MustCompile(`^ar[0-9]{4}e[0-9]+$`)

// NormalizeQuery validates without network access and fills bounded defaults.
func NormalizeQuery(q Query) (Query, error) {
	q.cityPath = ""
	q.cityName = ""
	if q.Limit == 0 {
		q.Limit = 10
	}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.MaxPages == 0 {
		q.MaxPages = 3
	}
	if q.MaxDetails == 0 {
		q.MaxDetails = 10
	}
	if q.Limit < 1 || q.Limit > 100 || q.Page < 1 || q.Page > 1000 || q.MaxPages < 1 || q.MaxPages > 20 || q.MaxDetails < 1 || q.MaxDetails > 30 {
		return q, fmt.Errorf("limit must be 1..100, page 1..1000, max-pages 1..20, max-details 1..30")
	}
	if q.Timing == "" {
		q.Timing = "overlap"
	}
	if q.Sort == "" {
		q.Sort = "relevance"
	}
	if q.Timing != "overlap" && q.Timing != "starts" && q.Timing != "ends" {
		return q, fmt.Errorf("timing must be overlap, starts or ends")
	}
	if q.Sort != "relevance" && q.Sort != "start" && q.Sort != "end" && q.Sort != "source" {
		return q, fmt.Errorf("sort must be relevance, start, end or source")
	}
	if q.Prefecture != "" {
		p, ok := lookup(prefectures, q.Prefecture)
		if !ok {
			return q, fmt.Errorf("unknown prefecture %q; use areas", q.Prefecture)
		}
		q.Prefecture = p.Code
	}
	if q.City != "" {
		if c, ok := lookup(knownCities, q.City); ok && (q.Prefecture == "" || value(c.PrefectureCode) == q.Prefecture) {
			q.City = c.Code
			q.cityPath = c.Path
			q.cityName = c.NameJA
			if q.Prefecture == "" {
				q.Prefecture = *c.PrefectureCode
			}
		}
		if cityCodeRE.MatchString(q.City) {
			parent := q.City[:6]
			if _, ok := lookup(prefectures, parent); !ok {
				return q, fmt.Errorf("invalid city prefecture")
			}
			if q.Prefecture != "" && parent != q.Prefecture {
				return q, fmt.Errorf("city does not belong to requested prefecture")
			}
			q.Prefecture = parent
		} else {
			if strings.HasPrefix(q.City, "ar") && len(q.City) > 2 && q.City[2] >= '0' && q.City[2] <= '9' {
				return q, fmt.Errorf("invalid city code; use an areas city code")
			}
			if q.Prefecture == "" || !safeCityName(q.City) {
				return q, fmt.Errorf("unknown city aliases or Japanese names require a prefecture; use areas --prefecture")
			}
		}
	}
	if q.Category != "" {
		c, ok := lookup(categoryItems, q.Category)
		if !ok {
			return q, fmt.Errorf("unknown category %q; use categories", q.Category)
		}
		q.Category = c.Code
	}
	if (q.From == "") != (q.To == "") {
		return q, fmt.Errorf("from and to must be supplied together")
	}
	if q.From != "" {
		from, err := parseDate(q.From)
		if err != nil {
			return q, err
		}
		to, err := parseDate(q.To)
		if err != nil {
			return q, err
		}
		if to.Before(from) || to.Sub(from) > 365*24*time.Hour {
			return q, fmt.Errorf("date interval must be ordered and at most 366 days inclusive; split longer trips")
		}
	}
	return q, nil
}

var safeCityNameRE = regexp.MustCompile("^[\\p{L}\\p{N}-]{1,64}$")

func safeCityName(s string) bool { return safeCityNameRE.MatchString(s) }

var tokyo = time.FixedZone("Asia/Tokyo", 9*60*60)

func parseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, tokyo)
	if err != nil || t.Year() < 1900 || t.Year() > 2200 {
		return t, fmt.Errorf("invalid ISO date %q", s)
	}
	return t, nil
}
func strptr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
func value(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func categoryCode(s string) string {
	if strings.HasPrefix(s, "eg") {
		if n, err := strconv.Atoi(strings.TrimPrefix(s, "eg")); err == nil {
			return fmt.Sprintf("eg%04d", n)
		}
	}
	return s
}

func listingRoutes(q Query) []string {
	area := q.Prefecture
	if q.City != "" {
		area = q.City
	}
	base := "/event_list/"
	if q.cityPath != "" {
		base = q.cityPath
	} else if area != "" {
		base += area + "/"
		if c, ok := lookup(knownCities, area); ok {
			base += c.Aliases[0] + "/"
		}
	}
	if q.Category != "" {
		base += q.Category + "/"
	}
	if q.From == "" {
		return []string{base}
	}
	from, _ := parseDate(q.From)
	to, _ := parseDate(q.To)
	out := []string{}
	seen := map[time.Month]bool{}
	for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, tokyo); !m.After(to); m = m.AddDate(0, 1, 0) {
		if !seen[m.Month()] {
			out = append(out, "/event_list/"+fmt.Sprintf("%02d", m.Month())+"/"+strings.TrimPrefix(base, "/event_list/"))
			seen[m.Month()] = true
		}
	}
	return out
}
